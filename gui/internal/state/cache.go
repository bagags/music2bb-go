package state

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
)

type CacheKind string

const (
	CacheSearch      CacheKind = "search"
	CacheCheckpoints CacheKind = "checkpoints"
	CacheDecisions   CacheKind = "decisions"
	CacheAnonymous   CacheKind = "anonymous-identity"
)

type CacheStatus struct {
	Files int
	Bytes int64
}

func CachePaths(configDir, cacheDir string) map[CacheKind]string {
	return map[CacheKind]string{
		CacheSearch:      filepath.Join(cacheDir, "search", "v1"),
		CacheCheckpoints: filepath.Join(configDir, "conversions", "v1"),
		CacheDecisions:   filepath.Join(configDir, "decisions", "v1"),
		CacheAnonymous:   filepath.Join(configDir, "cookies", "bilibili-anonymous.json"),
	}
}

func Inspect(target string) (CacheStatus, error) {
	var result CacheStatus
	err := filepath.WalkDir(target, func(_ string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			return nil
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		if info.Mode().IsRegular() {
			result.Files++
			result.Bytes += info.Size()
		}
		return nil
	})
	if errors.Is(err, fs.ErrNotExist) {
		return CacheStatus{}, nil
	}
	return result, err
}

func Clear(target string) error { return os.RemoveAll(target) }
