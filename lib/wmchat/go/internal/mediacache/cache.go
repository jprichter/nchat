// Package mediacache stores incoming attachments beneath a profile's tmp directory.
package mediacache

import (
	"crypto/rand"
	"crypto/sha256"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/sys/unix"
)

// DisplayName removes path components and terminal controls independently of IDs.
func DisplayName(name string) string {
	name = filepath.Base(strings.ReplaceAll(name, "\\", "/"))
	name = strings.Map(func(r rune) rune {
		if r < 32 || r > 126 || r == '/' || r == '\\' {
			return '_'
		}
		return r
	}, name)
	name = strings.Trim(name, ". ")
	if len(name) > 100 {
		name = name[:100]
	}
	if name == "" {
		return "attachment"
	}
	return name
}

func Path(profile, id, displayName string) string {
	return filepath.Join(profile, "tmp", fmt.Sprintf("%x-%s", sha256.Sum256([]byte(id)), DisplayName(displayName)))
}

// Fetch accepts canonical, immediate children only, including legacy filenames.
// Directory descriptors and O_NOFOLLOW prevent path substitution; linkat publishes
// a complete file atomically without replacing any destination (including links).
func Fetch(profile, target string, download func() ([]byte, error)) error {
	cache, err := filepath.Abs(filepath.Join(profile, "tmp"))
	if err != nil {
		return err
	}
	absolute, err := filepath.Abs(target)
	if err != nil || target != filepath.Clean(target) || filepath.Dir(absolute) != cache || strings.ContainsAny(target, "\\\x00") {
		return errors.New("invalid attachment cache path")
	}
	name := filepath.Base(absolute)
	profileFD, err := unix.Open(profile, unix.O_RDONLY|unix.O_DIRECTORY|unix.O_NOFOLLOW|unix.O_CLOEXEC, 0)
	if err != nil {
		return err
	}
	defer unix.Close(profileFD)
	fd, err := unix.Openat(profileFD, "tmp", unix.O_RDONLY|unix.O_DIRECTORY|unix.O_NOFOLLOW|unix.O_CLOEXEC, 0)
	if err != nil {
		return err
	}
	defer unix.Close(fd)
	var stat unix.Stat_t
	err = unix.Fstatat(fd, name, &stat, unix.AT_SYMLINK_NOFOLLOW)
	if err == nil {
		if stat.Mode&unix.S_IFMT != unix.S_IFREG || stat.Nlink != 1 {
			return errors.New("unsafe existing attachment")
		}
		return verifyCacheDirectory(profileFD, fd)
	}
	if !errors.Is(err, unix.ENOENT) {
		return err
	}
	data, err := download()
	if err != nil {
		return err
	}
	temp := ".download-" + rand.Text()
	tempFD, err := unix.Openat(fd, temp, unix.O_WRONLY|unix.O_CREAT|unix.O_EXCL|unix.O_NOFOLLOW|unix.O_CLOEXEC, 0600)
	if err != nil {
		return err
	}
	defer unix.Unlinkat(fd, temp, 0)
	file := os.NewFile(uintptr(tempFD), temp)
	_, writeErr := file.Write(data)
	closeErr := file.Close()
	if writeErr != nil {
		return writeErr
	}
	if closeErr != nil {
		return closeErr
	}
	if err := unix.Linkat(fd, temp, fd, name, 0); err != nil {
		return err
	}
	return verifyCacheDirectory(profileFD, fd)
}

func verifyCacheDirectory(profileFD, cacheFD int) error {
	var opened, current unix.Stat_t
	if err := unix.Fstat(cacheFD, &opened); err != nil {
		return err
	}
	if err := unix.Fstatat(profileFD, "tmp", &current, unix.AT_SYMLINK_NOFOLLOW); err != nil {
		return err
	}
	if current.Mode&unix.S_IFMT != unix.S_IFDIR || opened.Dev != current.Dev || opened.Ino != current.Ino {
		return errors.New("attachment cache directory changed")
	}
	return nil
}
