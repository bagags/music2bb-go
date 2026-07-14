//go:build bundled_chromium

package browser

import _ "embed"

// compiledChromiumArchive is populated by release builds after the pinned
// archive for the target platform has been downloaded and verified.
//
//go:embed chromium.zip
var compiledChromiumArchive []byte
