//go:build live

package browser

import (
	"context"
	"os"
	"testing"
	"time"
)

// This opt-in smoke test never installs a browser. Run `kg2bb browser install`
// first after the production manifest has real checksums.
func TestLiveKugouExtraction(t *testing.T) {
	rawURL := os.Getenv("KG2BB_TEST_KUGOU_URL")
	if rawURL == "" {
		t.Skip("KG2BB_TEST_KUGOU_URL is not set")
	}
	manager, err := NewManager("")
	if err != nil {
		t.Fatal(err)
	}
	status, err := manager.Status(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if !status.Verified {
		t.Skip("verified browser is not installed")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	songs, err := NewExtractor(manager).Extract(ctx, rawURL)
	if err != nil {
		t.Fatal(err)
	}
	if len(songs) == 0 {
		t.Fatal("live browser extraction returned no songs")
	}
}
