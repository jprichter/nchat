package mediacache

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestIncomingNames(t *testing.T) {
	profile := t.TempDir()
	for _, id := range []string{"../../outside-cache", "/absolute", "a\\b", "", "normal-id"} {
		for _, display := range []string{"../../document.txt", "C:\\foo\\report.pdf", "\x00\n", "..", strings.Repeat("a", 500)} {
			path := Path(profile, id, display)
			if filepath.Dir(path) != filepath.Join(profile, "tmp") || len(filepath.Base(path)) > 165 || strings.ContainsAny(path, "\x00\n\\") {
				t.Fatalf("unsafe path %q", path)
			}
		}
	}
	if DisplayName("../../report.pdf") != "report.pdf" || Path(profile, "a", "x") == Path(profile, "b", "x") {
		t.Fatal("display name or ID hashing failed")
	}
}

func newCache(t *testing.T) (string, string) {
	t.Helper()
	profile := t.TempDir()
	cache := filepath.Join(profile, "tmp")
	if err := os.Mkdir(cache, 0700); err != nil {
		t.Fatal(err)
	}
	return profile, cache
}

func TestFetchAndCache(t *testing.T) {
	profile, cache := newCache(t)
	target := filepath.Join(cache, "legacy-message-document.txt")
	calls := 0
	download := func() ([]byte, error) { calls++; return []byte("media"), nil }
	for range 2 {
		if err := Fetch(profile, target, download); err != nil {
			t.Fatal(err)
		}
	}
	data, _ := os.ReadFile(target)
	stat, _ := os.Stat(target)
	if calls != 1 || string(data) != "media" || stat.Mode().Perm() != 0600 {
		t.Fatal("invalid cached file or permissions")
	}
}

func TestRejectUnsafeTargetsBeforeDownload(t *testing.T) {
	profile, cache := newCache(t)
	outside := filepath.Join(profile, "outside")
	if err := os.WriteFile(outside, []byte("untouched"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(cache, "link")); err != nil {
		t.Fatal(err)
	}
	if err := os.Link(outside, filepath.Join(cache, "hardlink")); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(cache, "directory"), 0700); err != nil {
		t.Fatal(err)
	}
	for _, target := range []string{outside, "/etc/passwd", cache + "/../outside", cache + "/sub/../file", cache + "/link", cache + "/hardlink", cache + "/directory", cache + "/sub/file", "", cache + "/a\\b", cache + "/a\x00"} {
		t.Run(target, func(t *testing.T) {
			if err := Fetch(profile, target, func() ([]byte, error) { t.Fatal("download called for unsafe target"); return nil, nil }); err == nil {
				t.Fatal("unsafe target accepted")
			}
		})
	}
	data, _ := os.ReadFile(outside)
	if string(data) != "untouched" {
		t.Fatal("outside file changed")
	}
}

func TestSymlinkCache(t *testing.T) {
	profile := t.TempDir()
	outside := t.TempDir()
	if err := os.Symlink(outside, filepath.Join(profile, "tmp")); err != nil {
		t.Fatal(err)
	}
	if err := Fetch(profile, filepath.Join(profile, "tmp", "file"), func() ([]byte, error) { t.Fatal("download called"); return nil, nil }); err == nil {
		t.Fatal("symlink cache accepted")
	}
}

func TestDestinationAppearingDuringDownload(t *testing.T) {
	for _, symlink := range []bool{false, true} {
		profile, cache := newCache(t)
		outside := filepath.Join(profile, "outside")
		os.WriteFile(outside, []byte("untouched"), 0600)
		target := filepath.Join(cache, "target")
		err := Fetch(profile, target, func() ([]byte, error) {
			if symlink {
				return []byte("new"), os.Symlink(outside, target)
			}
			return []byte("new"), os.WriteFile(target, []byte("winner"), 0600)
		})
		if !errors.Is(err, os.ErrExist) {
			t.Fatalf("want exclusive publication failure, got %v", err)
		}
		data, _ := os.ReadFile(outside)
		if string(data) != "untouched" {
			t.Fatal("outside file changed")
		}
		entries, _ := os.ReadDir(cache)
		if len(entries) != 1 {
			t.Fatal("temporary file leaked")
		}
	}
}

func TestCacheReplacementDuringDownload(t *testing.T) {
	profile, cache := newCache(t)
	outside := t.TempDir()
	target := filepath.Join(cache, "target")
	err := Fetch(profile, target, func() ([]byte, error) {
		if err := os.Rename(cache, cache+"-old"); err != nil {
			return nil, err
		}
		return []byte("media"), os.Symlink(outside, cache)
	})
	if err == nil {
		t.Fatal("replaced cache was reported as downloaded")
	}
	if _, err := os.Stat(filepath.Join(outside, "target")); !os.IsNotExist(err) {
		t.Fatal("write escaped open directory")
	}
}

func TestFailedDownloadLeavesNoFile(t *testing.T) {
	profile, cache := newCache(t)
	err := Fetch(profile, filepath.Join(cache, "file"), func() ([]byte, error) { return nil, errors.New("failed") })
	entries, _ := os.ReadDir(cache)
	if err == nil || len(entries) != 0 {
		t.Fatal("failed download published data")
	}
}
