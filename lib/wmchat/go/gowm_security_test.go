package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	_ "github.com/d99kris/nchat/lib/wmchat/go/internal/testcallbacks"
	"go.mau.fi/whatsmeow"
)

func TestDownloadFromFileIdRejectsLegacyEscape(t *testing.T) {
	profile := t.TempDir()
	if err := os.Mkdir(filepath.Join(profile, "tmp"), 0700); err != nil {
		t.Fatal(err)
	}
	// No live WhatsApp session: any attempted download would fail on this client.
	const connID = -123
	clients[connID] = &whatsmeow.Client{}
	paths[connID] = profile
	t.Cleanup(func() { delete(clients, connID); delete(paths, connID) })
	outside := filepath.Join(profile, "outside")
	if err := os.WriteFile(outside, []byte("untouched"), 0600); err != nil {
		t.Fatal(err)
	}
	for _, target := range []string{outside, profile + "/tmp/../outside", "/etc/passwd"} {
		serialized, err := json.Marshal(DownloadInfo{Version: downloadInfoVersion, TargetPath: target})
		if err != nil {
			t.Fatal(err)
		}
		path, status := DownloadFromFileId(connID, string(serialized))
		if path != "" || status != FileStatusDownloadFailed {
			t.Fatalf("unsafe cached path accepted: %s %d", path, status)
		}
	}
	data, _ := os.ReadFile(outside)
	if string(data) != "untouched" {
		t.Fatal("outside file overwritten")
	}
	// Previously downloaded, valid legacy entries remain usable offline.
	target := filepath.Join(profile, "tmp", "legacy-document.txt")
	if err := os.WriteFile(target, []byte("cached"), 0600); err != nil {
		t.Fatal(err)
	}
	serialized, _ := json.Marshal(DownloadInfo{Version: downloadInfoVersion, TargetPath: target})
	path, status := DownloadFromFileId(connID, string(serialized))
	if path != target || status != FileStatusDownloaded {
		t.Fatal("valid legacy attachment rejected")
	}
}
