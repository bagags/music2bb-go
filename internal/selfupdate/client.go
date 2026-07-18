package selfupdate

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

const (
	defaultRepository  = "bagags/music2bb-go"
	defaultAPIBaseURL  = "https://api.github.com"
	maxMetadataBytes   = 2 << 20
	maxChecksumBytes   = 16 << 10
	maxArchiveBytes    = 128 << 20
	maxExecutableBytes = 64 << 20
)

// Client checks GitHub Releases and replaces the running music2bb executable.
type Client struct {
	CurrentVersion string
	Repository     string
	APIBaseURL     string
	HTTPClient     *http.Client

	goos       string
	goarch     string
	executable func() (string, error)
	replace    func(string, string) (bool, error)
}

type release struct {
	TagName    string  `json:"tag_name"`
	Draft      bool    `json:"draft"`
	Prerelease bool    `json:"prerelease"`
	Assets     []asset `json:"assets"`
}

type asset struct {
	Name               string `json:"name"`
	BrowserDownloadURL string `json:"browser_download_url"`
}

// New creates a GitHub Releases updater for the running version.
func New(currentVersion string) *Client {
	return &Client{CurrentVersion: currentVersion}
}

// Check returns the current version, latest stable version, and whether the
// latest release is newer. Development builds are eligible for replacement by
// the latest stable release.
func (c *Client) Check(ctx context.Context) (string, string, bool, error) {
	latest, err := c.latestRelease(ctx)
	if err != nil {
		return c.currentVersion(), "", false, err
	}
	if _, _, err := c.releaseAssets(latest); err != nil {
		return c.currentVersion(), latest.TagName, false, err
	}
	current := c.currentVersion()
	return current, latest.TagName, isNewerVersion(latest.TagName, current), nil
}

// Update downloads, verifies, and installs the latest stable release. The
// deferred result is true on Windows, where a helper finishes replacement
// after the current process exits.
func (c *Client) Update(ctx context.Context) (from, to string, deferred bool, err error) {
	latest, err := c.latestRelease(ctx)
	if err != nil {
		return c.currentVersion(), "", false, err
	}
	from = c.currentVersion()
	to = latest.TagName
	if !isNewerVersion(to, from) {
		return from, to, false, nil
	}

	archiveAsset, checksumAsset, err := c.releaseAssets(latest)
	if err != nil {
		return from, to, false, err
	}
	temporaryDir, err := os.MkdirTemp("", "music2bb-update-")
	if err != nil {
		return from, to, false, fmt.Errorf("create update directory: %w", err)
	}
	removeTemporaryDir := true
	defer func() {
		if removeTemporaryDir {
			_ = os.RemoveAll(temporaryDir)
		}
	}()

	checksumPath := filepath.Join(temporaryDir, checksumAsset.Name)
	if err := c.download(ctx, checksumAsset.BrowserDownloadURL, checksumPath, maxChecksumBytes); err != nil {
		return from, to, false, fmt.Errorf("download checksum: %w", err)
	}
	expectedChecksum, err := readChecksum(checksumPath)
	if err != nil {
		return from, to, false, err
	}

	archivePath := filepath.Join(temporaryDir, archiveAsset.Name)
	if err := c.download(ctx, archiveAsset.BrowserDownloadURL, archivePath, maxArchiveBytes); err != nil {
		return from, to, false, fmt.Errorf("download release archive: %w", err)
	}
	if err := verifyChecksum(archivePath, expectedChecksum); err != nil {
		return from, to, false, err
	}

	packageName, archiveName, executableName, err := artifactNames(c.platform())
	if err != nil {
		return from, to, false, err
	}
	if archiveAsset.Name != archiveName {
		return from, to, false, fmt.Errorf("release asset changed during update: got %q, want %q", archiveAsset.Name, archiveName)
	}
	stagedPath := filepath.Join(temporaryDir, executableName)
	if err := extractExecutable(archivePath, packageName, executableName, stagedPath); err != nil {
		return from, to, false, err
	}

	executable := c.executable
	if executable == nil {
		executable = os.Executable
	}
	targetPath, err := executable()
	if err != nil {
		return from, to, false, fmt.Errorf("locate current executable: %w", err)
	}
	if resolved, resolveErr := filepath.EvalSymlinks(targetPath); resolveErr == nil {
		targetPath = resolved
	}
	targetPath, err = filepath.Abs(targetPath)
	if err != nil {
		return from, to, false, fmt.Errorf("resolve current executable: %w", err)
	}

	replace := c.replace
	if replace == nil {
		replace = replaceExecutable
	}
	deferred, err = replace(stagedPath, targetPath)
	if err != nil {
		return from, to, false, fmt.Errorf("replace current executable: %w", err)
	}
	if deferred {
		removeTemporaryDir = false
	}
	return from, to, deferred, nil
}

func (c *Client) latestRelease(ctx context.Context) (release, error) {
	repository := c.Repository
	if repository == "" {
		repository = defaultRepository
	}
	baseURL := strings.TrimRight(c.APIBaseURL, "/")
	if baseURL == "" {
		baseURL = defaultAPIBaseURL
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, baseURL+"/repos/"+repository+"/releases/latest", nil)
	if err != nil {
		return release{}, fmt.Errorf("create release request: %w", err)
	}
	request.Header.Set("Accept", "application/vnd.github+json")
	request.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	request.Header.Set("User-Agent", "music2bb/"+c.currentVersion())

	client := c.HTTPClient
	if client == nil {
		client = &http.Client{Timeout: 5 * time.Minute}
	}
	response, err := client.Do(request)
	if err != nil {
		return release{}, fmt.Errorf("request latest GitHub release: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		message, _ := io.ReadAll(io.LimitReader(response.Body, 4<<10))
		return release{}, fmt.Errorf("GitHub Releases returned %s: %s", response.Status, strings.TrimSpace(string(message)))
	}

	var latest release
	decoder := json.NewDecoder(io.LimitReader(response.Body, maxMetadataBytes))
	if err := decoder.Decode(&latest); err != nil {
		return release{}, fmt.Errorf("decode latest GitHub release: %w", err)
	}
	if latest.Draft || latest.Prerelease || !validSemanticVersion(latest.TagName) {
		return release{}, fmt.Errorf("latest release has invalid stable tag %q", latest.TagName)
	}
	return latest, nil
}

func (c *Client) releaseAssets(latest release) (asset, asset, error) {
	_, archiveName, _, err := artifactNames(c.platform())
	if err != nil {
		return asset{}, asset{}, err
	}
	checksumName := archiveName + ".sha256"
	var archiveAsset, checksumAsset asset
	for _, candidate := range latest.Assets {
		switch candidate.Name {
		case archiveName:
			archiveAsset = candidate
		case checksumName:
			checksumAsset = candidate
		}
	}
	if archiveAsset.BrowserDownloadURL == "" || checksumAsset.BrowserDownloadURL == "" {
		return asset{}, asset{}, fmt.Errorf("release %s does not contain %s and its checksum", latest.TagName, archiveName)
	}
	return archiveAsset, checksumAsset, nil
}

func (c *Client) download(ctx context.Context, rawURL, destination string, limit int64) error {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return err
	}
	request.Header.Set("User-Agent", "music2bb/"+c.currentVersion())
	client := c.HTTPClient
	if client == nil {
		client = &http.Client{Timeout: 5 * time.Minute}
	}
	response, err := client.Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return fmt.Errorf("download returned %s", response.Status)
	}
	if response.ContentLength > limit {
		return fmt.Errorf("download is too large: %d bytes", response.ContentLength)
	}
	file, err := os.OpenFile(destination, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	written, copyErr := io.Copy(file, io.LimitReader(response.Body, limit+1))
	closeErr := file.Close()
	if copyErr != nil {
		return copyErr
	}
	if closeErr != nil {
		return closeErr
	}
	if written > limit {
		return fmt.Errorf("download exceeds %d bytes", limit)
	}
	return nil
}

func (c *Client) currentVersion() string {
	if strings.TrimSpace(c.CurrentVersion) == "" {
		return "dev"
	}
	return strings.TrimSpace(c.CurrentVersion)
}

func (c *Client) platform() (string, string) {
	goos, goarch := c.goos, c.goarch
	if goos == "" {
		goos = runtime.GOOS
	}
	if goarch == "" {
		goarch = runtime.GOARCH
	}
	return goos, goarch
}

func artifactNames(goos, goarch string) (packageName, archiveName, executableName string, err error) {
	if goos != "darwin" && goos != "linux" && goos != "windows" {
		return "", "", "", fmt.Errorf("self-update is unsupported on %s/%s", goos, goarch)
	}
	if goarch != "amd64" && goarch != "arm64" {
		return "", "", "", fmt.Errorf("self-update is unsupported on %s/%s", goos, goarch)
	}
	packageName = "music2bb-" + goos + "-" + goarch
	executableName = "music2bb"
	archiveName = packageName + ".tar.gz"
	if goos == "windows" {
		executableName += ".exe"
		archiveName = packageName + ".zip"
	}
	return packageName, archiveName, executableName, nil
}

func readChecksum(checksumPath string) (string, error) {
	payload, err := os.ReadFile(checksumPath)
	if err != nil {
		return "", fmt.Errorf("read release checksum: %w", err)
	}
	fields := strings.Fields(string(payload))
	if len(fields) == 0 || len(fields[0]) != sha256.Size*2 {
		return "", errors.New("release checksum is invalid")
	}
	expected := strings.ToLower(fields[0])
	if _, err := hex.DecodeString(expected); err != nil {
		return "", errors.New("release checksum is invalid")
	}
	return expected, nil
}

func verifyChecksum(filename, expected string) error {
	file, err := os.Open(filename)
	if err != nil {
		return err
	}
	defer file.Close()
	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return err
	}
	actual := hex.EncodeToString(hash.Sum(nil))
	if actual != strings.ToLower(expected) {
		return fmt.Errorf("release checksum mismatch: got %s, want %s", actual, expected)
	}
	return nil
}

func extractExecutable(archivePath, packageName, executableName, destination string) error {
	expectedPath := path.Join(packageName, executableName)
	if strings.HasSuffix(archivePath, ".zip") {
		return extractExecutableFromZip(archivePath, expectedPath, destination)
	}
	return extractExecutableFromTarGzip(archivePath, expectedPath, destination)
}

func extractExecutableFromZip(archivePath, expectedPath, destination string) error {
	reader, err := zip.OpenReader(archivePath)
	if err != nil {
		return fmt.Errorf("open release zip: %w", err)
	}
	defer reader.Close()
	for _, entry := range reader.File {
		if cleanArchivePath(entry.Name) != expectedPath {
			continue
		}
		if !entry.Mode().IsRegular() || entry.UncompressedSize64 > maxExecutableBytes {
			return errors.New("release executable entry is invalid")
		}
		source, err := entry.Open()
		if err != nil {
			return err
		}
		err = writeExecutable(source, destination)
		closeErr := source.Close()
		if err != nil {
			return err
		}
		return closeErr
	}
	return fmt.Errorf("release archive does not contain %s", expectedPath)
}

func extractExecutableFromTarGzip(archivePath, expectedPath, destination string) error {
	file, err := os.Open(archivePath)
	if err != nil {
		return err
	}
	defer file.Close()
	gzipReader, err := gzip.NewReader(file)
	if err != nil {
		return fmt.Errorf("open release tar.gz: %w", err)
	}
	defer gzipReader.Close()
	tarReader := tar.NewReader(gzipReader)
	for {
		header, err := tarReader.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return fmt.Errorf("read release tar.gz: %w", err)
		}
		if cleanArchivePath(header.Name) != expectedPath {
			continue
		}
		if header.Typeflag != tar.TypeReg || header.Size < 0 || header.Size > maxExecutableBytes {
			return errors.New("release executable entry is invalid")
		}
		return writeExecutable(io.LimitReader(tarReader, header.Size), destination)
	}
	return fmt.Errorf("release archive does not contain %s", expectedPath)
}

func cleanArchivePath(name string) string {
	return path.Clean(strings.TrimPrefix(strings.ReplaceAll(name, "\\", "/"), "./"))
}

func writeExecutable(source io.Reader, destination string) error {
	file, err := os.OpenFile(destination, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o700)
	if err != nil {
		return err
	}
	written, copyErr := io.Copy(file, io.LimitReader(source, maxExecutableBytes+1))
	syncErr := file.Sync()
	closeErr := file.Close()
	if copyErr != nil {
		return copyErr
	}
	if syncErr != nil {
		return syncErr
	}
	if closeErr != nil {
		return closeErr
	}
	if written > maxExecutableBytes {
		return errors.New("release executable is too large")
	}
	return nil
}
