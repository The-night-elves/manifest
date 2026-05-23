package cmd

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/andygrunwald/vdf"
	"github.com/pterm/pterm"
	"github.com/spf13/cobra"
	"resty.dev/v3"
)

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "manifest",
	Short: "steam manifest downloader",
	Long:  `steam manifest downloader implement by golang`,
	// Uncomment the following line if your bare application
	// has an action associated with it:
	Run: func(cmd *cobra.Command, args []string) {
		logLevel, err := cmd.Flags().GetString("log")
		if err != nil {
			slog.Error(err.Error())
			return
		}
		flaresolverrEndpoin, err := cmd.Flags().GetString("flaresolverr")
		if err != nil {
			slog.Error(err.Error())
			return
		}
		client := NewCFClient("steam_ui", flaresolverrEndpoin)
		err = client.CreateSession()
		if err != nil {
			slog.Error(err.Error())
			return
		}
		defer client.DestroySession()

		logLevel = "debug"
		switch logLevel {
		case "debug":
			slog.SetLogLoggerLevel(slog.LevelDebug)
		case "warn":
			slog.SetLogLoggerLevel(slog.LevelWarn)
		case "error":
			slog.SetLogLoggerLevel(slog.LevelError)
		default:
			slog.SetLogLoggerLevel(slog.LevelInfo)
		}

		name, err := pterm.DefaultInteractiveTextInput.Show("please input a game name or AppID")
		if err != nil {
			slog.Error(err.Error())
			return
		} else if len(name) == 0 {
			slog.Error("input game name or AppID is empty")
			return
		}

		body, err := client.GetByName(name)
		if err != nil {
			slog.Error(err.Error())
			return
		}
		slog.Debug("GetByName", slog.String("body", string(body)))
		var gamesInfo SearchGameAppResp
		if err = json.Unmarshal(body, &gamesInfo); err != nil {
			slog.Error(err.Error())
			return
		}
		gameInfo, err := gamesInfo.SelectApp()
		if err != nil {
			slog.Error(err.Error())
			return
		}
		if err = gameInfo.FindManifest(); err != nil {
			slog.Error(err.Error())
			return
		}
		// dir 目录下的所有文件拖拽到 steam tools 中
		pterm.DefaultBasicText.Sprintf("dir %s ", gameInfo.GetDirPath())
	},
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	err := rootCmd.Execute()
	if err != nil {
		os.Exit(1)
	}
}

func init() {
	// Here you will define your flags and configuration settings.
	// Cobra supports persistent flags, which, if defined here,
	// will be global for your application.

	// rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is $HOME/.manifest.yaml)")

	// Cobra also supports local flags, which will only run
	// when this action is called directly.
	rootCmd.Flags().StringP("log", "l", "info", "日志等级, debug, info, warn, error")
	// flareSolverr url
	rootCmd.Flags().StringP("flaresolverr", "f", "http://localhost:8191/v1", "flaresolverr server endpoint")
}

type SearchGameAppResp struct {
	Games []GameInfo `json:"games"`
}

func (s *SearchGameAppResp) SelectApp() (*GameInfo, error) {
	if len(s.Games) == 0 {
		return nil, errors.New("game not found")
	}
	data := make(pterm.TableData, 1+len(s.Games))
	data[0] = []string{"Index", "AppID", "GameName", "GameType", "NameSimpleChinese"}
	for i, game := range s.Games {
		data[i+1] = []string{
			strconv.Itoa(i),
			strconv.Itoa(game.AppID),
			game.Name,
			game.Type,
			game.NameSimpleChinese,
		}
	}
	err := pterm.DefaultTable.WithHasHeader().WithData(data).Render()
	if err != nil {
		return nil, err
	}
	index, err := pterm.DefaultInteractiveTextInput.Show("please input a game index[0-" + strconv.Itoa(len(s.Games)-1) + "]")
	if err != nil {
		return nil, err
	}
	i, err := strconv.Atoi(index)
	if err != nil {
		return nil, err
	}
	if i < 0 || i >= len(s.Games) {
		return nil, errors.New("index out of range")
	}
	return &s.Games[i], nil
}

type GameInfo struct {
	AppID             int    `json:"appid"`
	Name              string `json:"name"`
	Type              string `json:"type"`
	NameSimpleChinese string `json:"name_schinese"`
	IsFreeApp         bool   `json:"isfreeapp"`
	UpdateTime        string `json:"update_time"`
	ChangeNumber      int    `json:"change_number"`
}

// FindManifest find manifest
func (g *GameInfo) FindManifest() error {
	repos := [...]string{
		"SteamAutoCracks/ManifestHub",
		"Auiowu/ManifestAutoUpdate",
		"tymolu233/ManifestAutoUpdate-fix",
	}
	dirPath, err := g.CreateDirIfNotExist()
	if err != nil {
		return err
	}
	defer func() {
		if err != nil {
			_ = g.RemoveDir()
		}
	}()
	var errs []error
	for _, repo := range repos {
		if err = g.SaveManifestByRepo(repo, dirPath); err == nil {
			return nil
		}
		errs = append(errs, err)
	}
	err = errors.Join(append(errs, errors.New("manifest not found"))...)
	return err
}

func (g *GameInfo) SaveManifestByRepo(repo, dirPath string) error {
	spinner, err := pterm.DefaultSpinner.Start("find manifest by " + repo)
	if err != nil {
		return err
	}
	defer spinner.Stop()
	remote, err := g.FindManifestByRepo(repo)
	if err != nil {
		return err
	}
	spinner.UpdateText("get manifest by " + repo)
	tree, err := g.GetTree(remote)
	if err != nil {
		return err
	}
	spinner.UpdateText("save manifest by " + repo)
	if err = tree.GetAndSave(dirPath, g.AppID); err != nil {
		return err
	}
	spinner.Success("done")
	return nil
}

func (g *GameInfo) FindManifestByRepo(repo string) (string, error) {
	remote := "https://api.github.com/repos/" + repo + "/branches/" + strconv.Itoa(g.AppID)
	resp, err := http.Get(remote)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	slog.Debug("FindManifestByRepo", slog.String("body", string(raw)), slog.String("url", remote))
	// GitHub repo info
	var data struct {
		Commit struct {
			Commit struct {
				Tree struct {
					URL string `json:"url"`
				} `json:"tree"`
			} `json:"commit"`
		} `json:"commit"`
	}

	err = json.Unmarshal(raw, &data)
	if err != nil {
		return "", err
	}

	return data.Commit.Commit.Tree.URL, nil
}

func (g *GameInfo) GetTree(remote string) (*Tree, error) {
	if len(remote) == 0 {
		return nil, errors.New("remote is empty")
	}
	resp, err := http.Get(remote)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	slog.Debug("GetTree", slog.String("body", string(raw)), slog.String("url", remote))
	repoFileInfo := Tree{}
	err = json.Unmarshal(raw, &repoFileInfo)
	if err != nil {
		return nil, err
	}

	return &repoFileInfo, nil
}

// CreateDirIfNotExist 不存在是创建保存目录
func (g *GameInfo) CreateDirIfNotExist() (string, error) {
	dir := g.GetDirPath()
	slog.Debug("CreateDirIfNotExist", slog.String("dir", dir))
	// 判断文件夹是否存在
	_, err := os.Stat(dir)
	if err == nil {
		return dir, nil
	}
	err = os.MkdirAll(dir, os.ModePerm)
	if err != nil {
		return "", err
	}
	return dir, nil
}

// RemoveDir 删除目录
func (g *GameInfo) RemoveDir() error {
	dir := g.GetDirPath()
	_, err := os.Stat(dir)
	if err == nil {
		return os.RemoveAll(dir)
	}
	return nil
}

func (g *GameInfo) GetDirPath() string {
	return "[" + strconv.Itoa(g.AppID) + "]"
}

type Tree struct {
	Entries []*TreeEntry `json:"tree,omitempty"`

	// hanle Entries vdf set value
	Depots Depots
}

type TreeEntry struct {
	Path     string `json:"path,omitempty"`
	Mode     string `json:"mode,omitempty"`
	Type     string `json:"type,omitempty"`
	Size     int    `json:"size,omitempty"`
	Content  string `json:"content,omitempty"`
	Encoding string `json:"encoding,omitempty"`
	URL      string `json:"url,omitempty"`
}

func (t *Tree) GetAndSave(dirPath string, appID int) error {
	if t == nil {
		return nil
	}
	t.FilterByPath()
	for i := range t.Entries {
		err := t.Entries[i].GetAndSave(t, dirPath)
		if err != nil {
			return err
		}
	}

	return t.SaveVDFLua(dirPath, appID)
}

func (t *Tree) SaveVDFLua(dirPath string, appID int) error {
	if t == nil {
		return nil
	}
	appIDStr := strconv.Itoa(appID)
	fileName := filepath.Join(dirPath, appIDStr+".lua")
	var sb bytes.Buffer
	sb.WriteString(fmt.Sprintf("addappid(%d)\n", appID))
	for depotID, descryptionKey := range t.Depots.Entries {
		sb.WriteString(fmt.Sprintf("addappid(%s,0,%q)\n", depotID, descryptionKey))
		// find depot id manifest
		for i := range t.Entries {
			// t.Entries[i].Path format {depot_id}_{manifest_id}.manifest
			if strings.HasPrefix(t.Entries[i].Path, depotID) && strings.HasSuffix(t.Entries[i].Path, ".manifest") {
				sb.WriteString(fmt.Sprintf("setManifestid(%s,%q)", depotID,
					t.Entries[i].Path[len(depotID)+1:len(t.Entries[i].Path)-len(".manifest")]))
			}
		}
	}

	return os.WriteFile(fileName, sb.Bytes(), 0644)
}

func (t *Tree) FilterByPath() {
	if t == nil || len(t.Entries) == 0 {
		return
	}
	for i := 0; i < len(t.Entries); {
		if strings.EqualFold(t.Entries[i].Path, "key.vdf") || strings.HasSuffix(t.Entries[i].Path, ".manifest") {
			i++
			continue
		}
		t.Entries = append(t.Entries[:i], t.Entries[i+1:]...)
	}
}

func (r *TreeEntry) GetAndSave(tree *Tree, dirPath string) error {
	if r == nil {
		return nil
	}
	// 判断文件是否存在
	fileName := filepath.Join(dirPath, r.Path)
	_, err := os.Stat(fileName)
	if err == nil {
		return nil
	}
	resp, err := http.Get(r.URL)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	slog.Debug("GetAndSave", slog.String("body", string(raw)),
		slog.String("url", r.URL), slog.String("path", r.Path),
		slog.String("fileName", fileName),
	)
	entry := TreeEntry{}
	err = json.Unmarshal(raw, &entry)
	if err != nil {
		return err
	}
	content, err := entry.GetContent()
	if err != nil {
		return err
	}
	if strings.HasSuffix(r.Path, ".manifest") {
		err = os.WriteFile(fileName, content, 0666)
		if err != nil {
			return err
		}
	} else if strings.HasSuffix(r.Path, ".vdf") {
		depots := Depots{}
		err = depots.InitFromTreeEntry(&entry)
		if err != nil {
			return err
		}
		tree.Depots = depots
	}

	return nil
}

// GetContent returns the content of r, decoding it if necessary.
func (r *TreeEntry) GetContent() ([]byte, error) {
	if r == nil {
		return nil, nil
	}
	switch r.Encoding {
	case "base64":
		if len(r.Content) == 0 {
			return nil, errors.New("malformed response: base64 encoding of null content")
		}
		return base64.StdEncoding.DecodeString(r.Content)
	case "":
		return []byte(r.Content), nil
	case "none":
		return nil, errors.New("unsupported content encoding: none, this may occur when file size > 1 MB, if that is the case consider using DownloadContents")
	default:
		return nil, errors.New("unsupported content encoding: " + r.Encoding)
	}
}

type Depots struct {
	// key is depots id, value is DecryptionKey
	Entries map[string]string
}

func (d *Depots) InitFromTreeEntry(entry *TreeEntry) error {
	raw, err := entry.GetContent()
	if err != nil {
		return err
	}
	parser := vdf.NewParser(bytes.NewReader(raw))
	parse, err := parser.Parse()
	if err != nil {
		return err
	}
	return d.InitFromMapAny(parse)
}

func (d *Depots) InitFromMapAny(src map[string]any) error {
	if d == nil {
		return nil
	}
	depots, ok := src["depots"].(map[string]any)
	if !ok {
		return errors.New("depots not found")
	}
	d.Entries = make(map[string]string)
	for depotID, val := range depots {
		depotInfo, ok := val.(map[string]any)
		if !ok {
			return fmt.Errorf("depot id %s value not equal map[string]any %T", depotID, val)
		}
		descryptionKey, ok := depotInfo["DecryptionKey"].(string)
		if !ok {
			return fmt.Errorf("depot id %s DecryptionKey not equal string %T", depotID, val)
		}
		d.Entries[depotID] = descryptionKey
	}

	return nil
}

type FSRequest struct {
	Cmd        string `json:"cmd"`
	URL        string `json:"url"`
	MaxTimeout int    `json:"maxTimeout"`
	Session    string `json:"session,omitempty"`
}

type FSCookie struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

type FSSolution struct {
	Status    int        `json:"status"`
	Body      string     `json:"response"`
	Cookies   []FSCookie `json:"cookies"`
	UserAgent string     `json:"userAgent"`
}

type FSResponse struct {
	Status   string     `json:"status"`
	Message  string     `json:"message"`
	Solution FSSolution `json:"solution"`
}

func (m *FSResponse) GetJSONBody() ([]byte, error) {
	if m == nil {
		return nil, nil
	}

	left := strings.IndexByte(m.Solution.Body, '{')
	if left < 0 {
		return nil, errors.New("malformed response: missing JSON body")
	}
	right := strings.LastIndexByte(m.Solution.Body, '}')
	if right < left {
		return nil, errors.New("malformed response: missing JSON body")
	}
	return []byte(m.Solution.Body[left : right+1]), nil
}

type CFClient struct {
	client  *resty.Client
	session string
}

func NewCFClient(session, endpoint string) *CFClient {
	return &CFClient{
		client:  resty.New().SetTimeout(90 * time.Second).SetBaseURL(endpoint),
		session: session, // pin session name，Flare Solverr It will reuse the same browser
	}
}

func (c *CFClient) call(payload FSRequest) (*FSResponse, error) {
	resp, err := c.client.R().
		SetBody(payload).
		SetContentType("application/json").
		Post(c.client.BaseURL())
	if err != nil {
		return nil, err
	}
	slog.Debug("call",
		slog.Any("payload", payload),
		slog.String("raw", string(resp.Bytes())),
	)
	var result FSResponse
	if resp.StatusCode() != http.StatusOK {
		return nil, fmt.Errorf("FlareSolverr err: %s", result.Message)
	}
	err = json.Unmarshal(resp.Bytes(), &result)
	if err != nil {
		return nil, err
	} else if result.Status != "ok" {
		return nil, fmt.Errorf("FlareSolverr err: %s", result.Message)
	}

	return &result, nil
}

// GetByName All requests go through FlareSolver, no need for HTTP at all. Direct client connection
func (c *CFClient) GetByName(name string) ([]byte, error) {
	const Remote = "https://steamui.com/api/loadGames.php"
	values := url.Values{}
	values.Add("search", name)
	encode := values.Encode()
	remote := Remote + "?" + encode

	slog.Debug("Get", slog.String("remote", remote))

	spinner, err := pterm.DefaultSpinner.Start("Searching for " + name + "...")
	if err != nil {
		return nil, err
	}

	result, err := c.call(FSRequest{
		Cmd:        "request.get",
		URL:        remote,
		MaxTimeout: 60000,
		Session:    c.session, // Reuse sessions, use those that have already passed the challenge directly, no need to revalidate
	})
	if err != nil {
		spinner.Fail("Search failed: " + err.Error())
		return nil, err
	}

	spinner.Success("Search completed")
	return result.GetJSONBody()
}

// CreateSession reuse the same browser
func (c *CFClient) CreateSession() error {
	_, err := c.call(FSRequest{Cmd: "sessions.create", Session: c.session})
	return err
}

// DestroySession close the browser
func (c *CFClient) DestroySession() error {
	_, err := c.call(FSRequest{Cmd: "sessions.destroy", Session: c.session})
	return err
}
