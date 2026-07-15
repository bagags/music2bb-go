package browser

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestChromiumCreditsMatchProvenance(t *testing.T) {
	manifest, err := loadEmbeddedManifest()
	if err != nil {
		t.Fatal(err)
	}
	_, testFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("cannot locate credits test")
	}
	creditsPath := filepath.Join(filepath.Dir(testFile), "..", "..", manifest.Credits.File)
	credits, err := os.ReadFile(creditsPath)
	if err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(credits)
	if got := hex.EncodeToString(sum[:]); got != manifest.Credits.SHA256 {
		t.Fatalf("credits SHA-256 = %s, want %s", got, manifest.Credits.SHA256)
	}
	contents := string(credits)
	if len(credits) < 10_000_000 || strings.Count(contents, `<div class="product">`) < 700 {
		t.Fatalf("credits appear truncated: %d bytes", len(credits))
	}
	for _, marker := range []string{
		"The Chromium Project",
		"Redistribution and use in source and binary forms",
		`<span class="title">incremental-font-transfer</span>`,
		`<span class="title">skera</span>`,
		`<span class="title">write-fonts</span>`,
	} {
		if !strings.Contains(contents, marker) {
			t.Errorf("credits are missing %q", marker)
		}
	}
	if strings.Contains(contents, "This is sample credits page") ||
		strings.Contains(contents, "generate_about_credits=true") {
		t.Fatal("credits contain Chromium's placeholder page")
	}
}
