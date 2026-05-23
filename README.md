# manifest

Steam Manifest Downloader

## Requirements

This tool requires [FlareSolverr](https://github.com/FlareSolverr/FlareSolverr) to bypass Cloudflare protection when fetching game data.

## Install FlareSolverr

Download the precompiled binary from the [FlareSolverr releases page](https://github.com/FlareSolverr/FlareSolverr/releases) and follow the installation instructions in the [FlareSolverr README](https://github.com/FlareSolverr/FlareSolverr#installation).

Once installed, start FlareSolverr before running `manifest`.

## Usage

```bash
manifest
```

View all available flags:

```bash
manifest --help
```

**Flags**

| Flag | Short | Default | Description |
|------|-------|---------|-------------|
| `--flaresolverr` | `-f` | `http://localhost:8191/v1` | FlareSolverr server endpoint |
| `--log` | `-l` | `info` | Log level: `debug`, `info`, `warn`, `error` |

If FlareSolverr is running on a different host or port, specify it with `-f`:

```bash
manifest
```