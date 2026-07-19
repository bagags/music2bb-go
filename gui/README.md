# music2bb Desktop

`music2bb Desktop` is a native, cross-platform Gio frontend for the reusable
`music2bb` Go engine. It does not start an HTTP server and does not render an
HTML/JavaScript web UI.

## Features

- Parse Kugou, Apple Music playlists and albums, and generic online playlists, including the same
  verified Chromium fallback used by the CLI.
- Automatic or fully manual Bilibili matching, standard/classical profiles,
  custom weights, anonymous/session/automatic identity selection, persistent
  search cache, refresh, request budgets, and concurrent matching.
- A persistent operation banner with real progress, elapsed/idle time, the
  current song, prominent cancellation, per-song logs, request/cache counters,
  and explicit warnings while Bilibili is rate-limiting or slow to respond.
- Complete review workspace with ranked candidates, component scores, manual
  searches, direct BVID lookup, skip/undo, and source/video links.
- QR/stored-cookie login, logout, isolated anonymous-identity reset.
- List and create private/public favorites, write reviewed results, and retain
  per-video receipts so a retry does not duplicate successful writes.
- CLI-compatible conversion checkpoints and seven-day cross-playlist manual
  decisions, plus selective cache/state inspection and clearing.
- Browser status, system Chrome/Chromium detection, verified managed install,
  and managed-cache clear actions.
- Check the latest verified core release from the About page and open its
  download page. Automatic replacement stays disabled until desktop release
  artifacts are published.

## Run

From this directory:

```powershell
go mod download
go run -ldflags="-H windowsgui" .
```

On macOS or Linux, use `go run .`. Linux needs the Gio platform development
packages listed at <https://gioui.org/doc/install/linux>. Windows needs no C
compiler; macOS uses the system SDK.

An optional portable state directory can be selected at startup:

```text
music2bb-desktop --config-dir D:\portable\music2bb

# Prefer a specific system Chrome or Chromium executable.
music2bb-desktop --browser-executable "C:\Program Files\Google\Chrome\Application\chrome.exe"
```

## Build and package

```powershell
# Current platform
go build -trimpath -ldflags="-s -w -H windowsgui" -o dist/music2bb-desktop.exe .

# Test UI-independent workflow/state code
go test ./...

# Static checks
go vet ./...
```

For application bundles install the official Gio packaging helper and run it
on each target OS:

```text
go install gioui.org/cmd/gogio@latest
gogio -target windows -o dist/music2bb-desktop.exe .
gogio -target macos -o dist/music2bb-desktop.app .
gogio -target linux -o dist/music2bb-desktop .
```

Native packaging should be performed on the target OS (or in a suitably
provisioned cross-build environment). The application stores account cookies,
checkpoints and decisions in the engine's resolved config directory; replaceable
search/browser data is stored in its resolved cache directory.
