//go:build !windows

package selfupdate

import (
	"os"
	"path/filepath"
	"testing"
)

func TestReplaceExecutableAtomically(t *testing.T) {
	directory := t.TempDir()
	target := filepath.Join(directory, "music2bb")
	staged := filepath.Join(directory, "staged")
	if err := os.WriteFile(target, []byte("old"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(staged, []byte("new"), 0o700); err != nil {
		t.Fatal(err)
	}
	deferred, err := replaceExecutable(staged, target)
	if err != nil {
		t.Fatal(err)
	}
	if deferred {
		t.Fatal("Unix replacement was deferred")
	}
	payload, err := os.ReadFile(target)
	if err != nil {
		t.Fatal(err)
	}
	if string(payload) != "new" {
		t.Fatalf("payload = %q", payload)
	}
	info, err := os.Stat(target)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o755 {
		t.Fatalf("mode = %o", info.Mode().Perm())
	}
}
