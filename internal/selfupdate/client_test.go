package selfupdate

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestArtifactNamesCoverReleaseMatrix(t *testing.T) {
	tests := []struct {
		goos, goarch, archive, executable string
	}{
		{"darwin", "amd64", "music2bb-darwin-amd64.tar.gz", "music2bb"},
		{"darwin", "arm64", "music2bb-darwin-arm64.tar.gz", "music2bb"},
		{"linux", "amd64", "music2bb-linux-amd64.tar.gz", "music2bb"},
		{"linux", "arm64", "music2bb-linux-arm64.tar.gz", "music2bb"},
		{"windows", "amd64", "music2bb-windows-amd64.zip", "music2bb.exe"},
		{"windows", "arm64", "music2bb-windows-arm64.zip", "music2bb.exe"},
	}
	for _, test := range tests {
		t.Run(test.goos+"_"+test.goarch, func(t *testing.T) {
			packageName, archiveName, executableName, err := artifactNames(test.goos, test.goarch)
			if err != nil {
				t.Fatal(err)
			}
			if packageName != "music2bb-"+test.goos+"-"+test.goarch || archiveName != test.archive || executableName != test.executable {
				t.Fatalf("names = %q, %q, %q", packageName, archiveName, executableName)
			}
		})
	}
	if _, _, _, err := artifactNames("linux", "386"); err == nil {
		t.Fatal("unsupported architecture was accepted")
	}
}

func TestVersionComparison(t *testing.T) {
	tests := []struct {
		latest, current string
		want            bool
	}{
		{"v1.2.4", "v1.2.3", true},
		{"v1.10.0", "v1.9.9", true},
		{"v2.0.0", "v1.99.99", true},
		{"v1.2.3", "v1.2.3", false},
		{"v1.2.3", "v1.3.0", false},
		{"v1.2.3", "v1.2.3-rc.1", true},
		{"v1.2.3-rc.2", "v1.2.3-rc.1", true},
		{"v1.2.3", "dev", true},
		{"not-a-version", "v1.2.3", false},
	}
	for _, test := range tests {
		if got := isNewerVersion(test.latest, test.current); got != test.want {
			t.Errorf("isNewerVersion(%q, %q) = %t, want %t", test.latest, test.current, got, test.want)
		}
	}
	for _, invalid := range []string{"v1", "v1.2", "v01.2.3", "v1.2.3-01", "release-1.2.3"} {
		if validSemanticVersion(invalid) {
			t.Errorf("%q was accepted", invalid)
		}
	}
}

func TestCheckAndUpdateVerifiedRelease(t *testing.T) {
	const (
		packageName   = "music2bb-linux-amd64"
		archiveName   = packageName + ".tar.gz"
		executableNew = "new executable"
	)
	archive := tarGzipArchive(t, packageName+"/music2bb", []byte(executableNew))
	digest := sha256.Sum256(archive)
	checksum := []byte(fmt.Sprintf("%x  %s\n", digest, archiveName))

	var server *httptest.Server
	server = httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		switch request.URL.Path {
		case "/repos/test/repository/releases/latest":
			if request.Header.Get("User-Agent") != "music2bb/v1.2.3" {
				t.Errorf("user agent = %q", request.Header.Get("User-Agent"))
			}
			_ = json.NewEncoder(response).Encode(release{
				TagName: "v1.3.0",
				Assets: []asset{
					{Name: archiveName, BrowserDownloadURL: server.URL + "/assets/" + archiveName},
					{Name: archiveName + ".sha256", BrowserDownloadURL: server.URL + "/assets/" + archiveName + ".sha256"},
				},
			})
		case "/assets/" + archiveName:
			_, _ = response.Write(archive)
		case "/assets/" + archiveName + ".sha256":
			_, _ = response.Write(checksum)
		default:
			http.NotFound(response, request)
		}
	}))
	defer server.Close()

	target := filepath.Join(t.TempDir(), "music2bb")
	if err := os.WriteFile(target, []byte("old executable"), 0o755); err != nil {
		t.Fatal(err)
	}
	resolvedTarget, err := filepath.EvalSymlinks(target)
	if err != nil {
		t.Fatal(err)
	}
	var replacement []byte
	client := &Client{
		CurrentVersion: "v1.2.3",
		Repository:     "test/repository",
		APIBaseURL:     server.URL,
		HTTPClient:     server.Client(),
		goos:           "linux",
		goarch:         "amd64",
		executable:     func() (string, error) { return target, nil },
		replace: func(stagedPath, targetPath string) (bool, error) {
			if targetPath != resolvedTarget {
				t.Fatalf("target = %q, want %q", targetPath, resolvedTarget)
			}
			var err error
			replacement, err = os.ReadFile(stagedPath)
			return false, err
		},
	}
	current, latest, available, err := client.Check(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if current != "v1.2.3" || latest != "v1.3.0" || !available {
		t.Fatalf("check = %q, %q, %t", current, latest, available)
	}
	from, to, deferred, err := client.Update(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if from != current || to != latest || deferred || string(replacement) != executableNew {
		t.Fatalf("update = %q, %q, %t, replacement %q", from, to, deferred, replacement)
	}
}

func TestUpdateRejectsChecksumMismatch(t *testing.T) {
	const archiveName = "music2bb-linux-amd64.tar.gz"
	archive := tarGzipArchive(t, "music2bb-linux-amd64/music2bb", []byte("new"))
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		if request.URL.Path == "/repos/test/repository/releases/latest" {
			_ = json.NewEncoder(response).Encode(release{TagName: "v2.0.0", Assets: []asset{
				{Name: archiveName, BrowserDownloadURL: "http://" + request.Host + "/archive"},
				{Name: archiveName + ".sha256", BrowserDownloadURL: "http://" + request.Host + "/checksum"},
			}})
			return
		}
		if request.URL.Path == "/checksum" {
			_, _ = io.WriteString(response, "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa  archive\n")
			return
		}
		_, _ = response.Write(archive)
	}))
	defer server.Close()
	client := &Client{CurrentVersion: "v1.0.0", Repository: "test/repository", APIBaseURL: server.URL, HTTPClient: server.Client(), goos: "linux", goarch: "amd64"}
	if _, _, _, err := client.Update(context.Background()); err == nil || !bytes.Contains([]byte(err.Error()), []byte("checksum mismatch")) {
		t.Fatalf("error = %v", err)
	}
}

func TestExtractExecutableFromZip(t *testing.T) {
	archivePath := filepath.Join(t.TempDir(), "music2bb-windows-amd64.zip")
	file, err := os.Create(archivePath)
	if err != nil {
		t.Fatal(err)
	}
	writer := zip.NewWriter(file)
	entry, err := writer.Create("music2bb-windows-amd64/music2bb.exe")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := entry.Write([]byte("windows executable")); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
	destination := filepath.Join(t.TempDir(), "music2bb.exe")
	if err := extractExecutable(archivePath, "music2bb-windows-amd64", "music2bb.exe", destination); err != nil {
		t.Fatal(err)
	}
	payload, err := os.ReadFile(destination)
	if err != nil {
		t.Fatal(err)
	}
	if string(payload) != "windows executable" {
		t.Fatalf("payload = %q", payload)
	}
}

func tarGzipArchive(t *testing.T, name string, payload []byte) []byte {
	t.Helper()
	var output bytes.Buffer
	gzipWriter := gzip.NewWriter(&output)
	tarWriter := tar.NewWriter(gzipWriter)
	if err := tarWriter.WriteHeader(&tar.Header{Name: name, Mode: 0o755, Size: int64(len(payload)), Typeflag: tar.TypeReg}); err != nil {
		t.Fatal(err)
	}
	if _, err := tarWriter.Write(payload); err != nil {
		t.Fatal(err)
	}
	if err := tarWriter.Close(); err != nil {
		t.Fatal(err)
	}
	if err := gzipWriter.Close(); err != nil {
		t.Fatal(err)
	}
	return output.Bytes()
}
