package tui

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCleanSource(t *testing.T) {
	home, _ := os.UserHomeDir()
	for in, want := range map[string]string{
		"  '/tmp/my\\ file.torrent'\n":  "/tmp/my file.torrent",
		`"magnet:?xt=urn:btih:ab&tr=x"`: "magnet:?xt=urn:btih:ab&tr=x",
		"~/a.torrent":                   filepath.Join(home, "a.torrent"),
	} {
		if got := cleanSource(in); got != want {
			t.Errorf("cleanSource(%q) = %q, want %q", in, got, want)
		}
	}
}
