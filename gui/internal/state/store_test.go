package state

import (
	"testing"

	music2bb "github.com/bagags/music2bb-go"
)

func TestPlaylistIDIgnoresTrackingAndUsesProviderIdentity(t *testing.T) {
	a := PlaylistID("https://m.kugou.com/share/zlist.html?specialid=42&utm_source=x")
	b := PlaylistID("https://www.kugou.com/anything?specialid=42")
	if a != b {
		t.Fatalf("provider IDs differ: %s %s", a, b)
	}
}

func TestStoreRestoresManualDecisionAndWriteReceipt(t *testing.T) {
	dir := t.TempDir()
	song := music2bb.Song{Name: "Song", Artist: "Artist"}
	video := music2bb.Video{BVID: "BV1", Tags: []string{"tag"}}
	outcome := music2bb.MatchResult{Song: song, Video: &video, HasSelection: true, Matched: true, SearchStatus: music2bb.SearchStatusCompleted}
	store := New(dir, "https://example.com/list")
	if err := store.SaveDecision(outcome, false); err != nil {
		t.Fatal(err)
	}
	if err := store.SaveWriteReceipt(music2bb.WriteReceipt{FavoriteID: 7, BVID: "BV1", Succeeded: true}); err != nil {
		t.Fatal(err)
	}

	reloaded := New(dir, "https://example.com/list")
	restored, err := reloaded.Restore([]music2bb.Song{song}, false)
	if err != nil {
		t.Fatal(err)
	}
	got := restored[song.StableSourceID()]
	if !got.HasSelection || got.Video == nil || got.Video.BVID != "BV1" || !got.ManualOverride {
		t.Fatalf("restored = %#v", got)
	}
	writes, err := reloaded.SuccessfulWrites(7)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := writes["BV1"]; !ok {
		t.Fatal("successful write was not restored")
	}
}
