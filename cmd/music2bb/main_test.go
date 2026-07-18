package main

import (
	"go/parser"
	"go/token"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/bagags/music2bb-go/internal/cli"
)

func TestMainConsumesPublicBackend(t *testing.T) {
	file, err := parser.ParseFile(token.NewFileSet(), "main.go", nil, parser.ImportsOnly)
	if err != nil {
		t.Fatal(err)
	}
	allowedInternal := map[string]bool{
		"github.com/bagags/music2bb-go/internal/cli":        true,
		"github.com/bagags/music2bb-go/internal/selfupdate": true,
	}
	for _, imported := range file.Imports {
		path, err := strconv.Unquote(imported.Path.Value)
		if err != nil {
			t.Fatal(err)
		}
		if strings.HasPrefix(path, "github.com/bagags/music2bb-go/internal/") && !allowedInternal[path] {
			t.Errorf("main directly imports backend implementation %s", path)
		}
	}
}

func TestRunRejectsInvalidBrowserExecutable(t *testing.T) {
	exit := run([]string{"version", "--browser-executable", filepath.Join(t.TempDir(), "missing")})
	if exit != cli.ExitInvalidInput {
		t.Fatalf("exit = %d, want %d", exit, cli.ExitInvalidInput)
	}
}
