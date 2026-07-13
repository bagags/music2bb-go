package browser

import (
	"context"
	"reflect"
	"testing"

	"github.com/gguage/music-to-bb/internal/model"
)

func TestDecodeBrowserSongsFiltersAndDeduplicates(t *testing.T) {
	payload := `[
      {"name":" Song ","artist":" Singer "},
      {"name":"Song","artist":"Singer"},
      {"name":"正在加载","artist":""},
      {"name":"Other","artist":""}
    ]`
	want := []model.Song{{Name: "Song", Artist: "Singer"}, {Name: "Other"}}
	got, err := decodeBrowserSongs(payload)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("decodeBrowserSongs = %#v, want %#v", got, want)
	}
}

func TestExtractorNeverInstallsBrowser(t *testing.T) {
	manifest := Manifest{Schema: 1, Artifacts: map[string]Artifact{
		"test/amd64": {Revision: 7, Executable: "test/chrome"},
	}}
	manager, err := NewManagerWithOptions(ManagerOptions{
		CacheDir: t.TempDir(), Platform: "test/amd64", Manifest: manifest,
	})
	if err != nil {
		t.Fatal(err)
	}
	extractor := NewExtractor(manager)
	_, err = extractor.Extract(context.Background(), "https://example.test")
	if !IsKind(err, ErrorNotInstalled) {
		t.Fatalf("Extract error = %v, want %s", err, ErrorNotInstalled)
	}
}
