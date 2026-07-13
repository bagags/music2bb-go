package browser

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"runtime"
	"strings"
)

// PinnedRevision is the only Chromium snapshot revision accepted by the
// default browser manager. Changing it also requires updating every checksum
// in manifest.json.
const PinnedRevision = 1321438

// Artifact describes one pinned Chromium archive and its executable inside
// the extracted archive.
type Artifact struct {
	Revision    int    `json:"revision"`
	URL         string `json:"url"`
	SHA256      string `json:"sha256"`
	Executable  string `json:"executable"`
	ApproxBytes int64  `json:"approx_bytes,omitempty"`
}

// Manifest is embedded in the executable so download verification does not
// depend on mutable remote metadata.
type Manifest struct {
	Schema    int                 `json:"schema"`
	Artifacts map[string]Artifact `json:"artifacts"`
}

//go:embed manifest.json
var embeddedManifest []byte

func loadEmbeddedManifest() (Manifest, error) {
	var manifest Manifest
	if err := json.Unmarshal(embeddedManifest, &manifest); err != nil {
		return Manifest{}, fmt.Errorf("decode embedded manifest: %w", err)
	}
	if manifest.Schema != 1 {
		return Manifest{}, fmt.Errorf("unsupported manifest schema %d", manifest.Schema)
	}
	return manifest, nil
}

func currentPlatform() string { return runtime.GOOS + "/" + runtime.GOARCH }

func validChecksum(value string) bool {
	if len(value) != 64 {
		return false
	}
	for _, char := range strings.ToLower(value) {
		if !strings.ContainsRune("0123456789abcdef", char) {
			return false
		}
	}
	return true
}
