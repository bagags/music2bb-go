//go:build !windows

package selfupdate

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
)

func replaceExecutable(stagedPath, targetPath string) (bool, error) {
	current, err := os.Stat(targetPath)
	if err != nil {
		return false, err
	}
	staged, err := os.Open(stagedPath)
	if err != nil {
		return false, err
	}
	defer staged.Close()

	temporary, err := os.CreateTemp(filepath.Dir(targetPath), ".music2bb-update-*")
	if err != nil {
		return false, fmt.Errorf("create replacement beside executable: %w", err)
	}
	temporaryPath := temporary.Name()
	removeTemporary := true
	defer func() {
		_ = temporary.Close()
		if removeTemporary {
			_ = os.Remove(temporaryPath)
		}
	}()

	if _, err := io.Copy(temporary, staged); err != nil {
		return false, err
	}
	mode := current.Mode().Perm()
	if mode&0o111 == 0 {
		mode |= 0o755
	}
	if err := temporary.Chmod(mode); err != nil {
		return false, err
	}
	if err := temporary.Sync(); err != nil {
		return false, err
	}
	if err := temporary.Close(); err != nil {
		return false, err
	}
	if err := os.Rename(temporaryPath, targetPath); err != nil {
		return false, err
	}
	removeTemporary = false
	return false, nil
}
