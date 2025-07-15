package cmd

import "testing"

func TestDepots_InitFromTreeEntry(t *testing.T) {

	type args struct {
		entry *TreeEntry
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		{
			name: "decode",
			args: args{
				entry: &TreeEntry{
					Content:  "ImRlcG90cyIKewogICAgIjM0ODk3MDEiCiAgICB7CiAgICAgICAgIkRlY3J5cHRpb25LZXkiICIyMjY2Y2EzMWY2ZjI2NjZlYmU3NTkwNjIwMWU3MmY5Mzk4Njk0MDMwZmYxYzgyMTRhYWNlYmNmNzRkYzk0ZWUzIgogICAgfQogICAgIjM1OTYxODAiCiAgICB7CiAgICAgICAgIkRlY3J5cHRpb25LZXkiICJhNTVmNzk2MDdjMzJhN2UyNGEzODg0NjRkZGMzY2ZiYzkxYjJkY2VlNjY5MTA2MWJjYTNmYjMyZDIzMWNjYjMyIgogICAgfQogICAgIjM1OTYxOTAiCiAgICB7CiAgICAgICAgIkRlY3J5cHRpb25LZXkiICIxZTY1ZjhiYWEyNTlmNDg3ZGNkYzFkM2RjZTFlMTk4ZGQyM2M5ZjdiNzcyMzZlNGU1NmZjYzlmNmI5MTVhNDQ5IgogICAgfQp9",
					Encoding: "base64",
				},
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := &Depots{}
			if err := d.InitFromTreeEntry(tt.args.entry); (err != nil) != tt.wantErr {
				t.Errorf("InitFromTreeEntry() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
