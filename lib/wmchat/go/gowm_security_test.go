package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	_ "github.com/d99kris/nchat/lib/wmchat/go/internal/testcallbacks"
	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/store"
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

func TestAttachmentDownloadWarningRedactsErrors(t *testing.T) {
	const sensitiveURL = "https://mmg.whatsapp.net/private-media?token=secret"
	for _, tc := range []struct {
		name string
		err  error
		want string
	}{
		{"proxy policy", &url.Error{Op: "Get", URL: sensitiveURL, Err: fmt.Errorf("proxy credentials: %w", whatsmeow.ErrMediaProxy)}, "attachment download failed: " + whatsmeow.ErrMediaProxy.Error()},
		{"other failure", errors.New(sensitiveURL), "attachment download failed"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := attachmentDownloadWarning(tc.err); got != tc.want {
				t.Fatalf("unexpected warning: %q", got)
			}
		})
	}
}

func TestProxyDownloadFailureDiagnostic(t *testing.T) {
	client := whatsmeow.NewClient(&store.Device{}, nil)
	proxyURL, err := url.Parse("http://user:secret@127.0.0.1:1")
	if err != nil {
		t.Fatal(err)
	}
	client.SetProxy(http.ProxyURL(proxyURL))
	_, err = DownloadFromFileInfo(client, DownloadInfo{Url: "https://mmg.whatsapp.net/private-media?token=secret"})
	if !errors.Is(err, whatsmeow.ErrMediaProxy) {
		t.Fatalf("expected proxy policy error, got %v", err)
	}
	warning := attachmentDownloadWarning(err)
	if !strings.Contains(warning, whatsmeow.ErrMediaProxy.Error()) || strings.Contains(warning, "secret") || strings.Contains(warning, "private-media") {
		t.Fatalf("missing policy reason or unredacted warning: %q", warning)
	}
}
