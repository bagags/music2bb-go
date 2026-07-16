package cli

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	music2bb "github.com/bagags/music2bb-go"
)

func TestConversionStateRestoresBySourceIDAcrossPlaylistChanges(t *testing.T) {
	root := t.TempDir()
	now := time.Date(2026, 7, 16, 12, 0, 0, 0, time.UTC)
	state := newConversionState(root, "https://music.example/list?id=1&utm_source=test", func() time.Time { return now })
	songA := music2bb.Song{Name: "A", SourceID: "source:a"}
	songB := music2bb.Song{Name: "B", SourceID: "source:b"}
	videoA := music2bb.Video{BVID: "BVA", Title: "A"}
	videoB := music2bb.Video{BVID: "BVB", Title: "B"}
	for _, outcome := range []music2bb.MatchResult{
		{Song: songA, Video: &videoA, HasSelection: true, SearchStatus: music2bb.SearchStatusCompleted},
		{Song: songB, Video: &videoB, HasSelection: true, SearchStatus: music2bb.SearchStatusCompleted},
	} {
		if err := state.saveOutcome(outcome); err != nil {
			t.Fatal(err)
		}
	}

	reloaded := newConversionState(root, "https://music.example/list?utm_source=other&id=1", func() time.Time { return now })
	songC := music2bb.Song{Name: "C", SourceID: "source:c"}
	restored, err := reloaded.restore([]music2bb.Song{songB, songC, songA}, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(restored) != 2 || restored["source:a"].Video.BVID != "BVA" || restored["source:b"].Video.BVID != "BVB" {
		t.Fatalf("restored = %#v", restored)
	}
	if _, ok := restored["source:c"]; ok {
		t.Fatal("new playlist song unexpectedly restored")
	}
}

func TestManualDecisionsReuseAcrossPlaylistsAndHardExpire(t *testing.T) {
	root := t.TempDir()
	now := time.Date(2026, 7, 16, 12, 0, 0, 0, time.UTC)
	clock := func() time.Time { return now }
	songA := music2bb.Song{Name: "A", SourceID: "source:a"}
	songB := music2bb.Song{Name: "B", SourceID: "source:b"}
	video := music2bb.Video{BVID: "BV-selected", Title: "Manual"}
	first := newConversionState(root, "https://music.example/one", clock)
	if err := first.saveDecision(music2bb.MatchResult{Song: songA, Video: &video, HasSelection: true}, false); err != nil {
		t.Fatal(err)
	}
	if err := first.saveDecision(music2bb.MatchResult{Song: songB}, true); err != nil {
		t.Fatal(err)
	}

	second := newConversionState(root, "https://music.example/two", clock)
	restored, err := second.restore([]music2bb.Song{songB, songA}, false)
	if err != nil {
		t.Fatal(err)
	}
	if got := restored["source:a"]; !got.ManualOverride || got.Video == nil || got.Video.BVID != "BV-selected" {
		t.Fatalf("selected decision = %#v", got)
	}
	if got := restored["source:b"]; got.HasSelection || got.NeedsReview {
		t.Fatalf("skip decision = %#v", got)
	}

	now = now.Add(manualDecisionTTL)
	expired := newConversionState(root, "https://music.example/one", clock)
	restored, err = expired.restore([]music2bb.Song{songA, songB}, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(restored) != 0 {
		t.Fatalf("expired decisions restored from decision or checkpoint: %#v", restored)
	}
}

func TestAutomaticOutcomeRetiresExpiredManualDecision(t *testing.T) {
	root := t.TempDir()
	now := time.Date(2026, 7, 16, 12, 0, 0, 0, time.UTC)
	clock := func() time.Time { return now }
	song := music2bb.Song{Name: "Song", SourceID: "source:song"}
	manualVideo := music2bb.Video{BVID: "BV-manual"}
	state := newConversionState(root, "https://music.example/list", clock)
	if err := state.saveDecision(music2bb.MatchResult{Song: song, Video: &manualVideo, HasSelection: true}, false); err != nil {
		t.Fatal(err)
	}
	now = now.Add(manualDecisionTTL)
	if restored, err := state.restore([]music2bb.Song{song}, false); err != nil || len(restored) != 0 {
		t.Fatalf("expired restore = %#v, %v", restored, err)
	}
	automaticVideo := music2bb.Video{BVID: "BV-automatic"}
	if err := state.saveOutcome(music2bb.MatchResult{Song: song, Video: &automaticVideo, HasSelection: true, SearchStatus: music2bb.SearchStatusCompleted}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(state.decisionPath(stableSongID(song))); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("expired decision still exists: %v", err)
	}
	reloaded := newConversionState(root, "https://music.example/list", clock)
	restored, err := reloaded.restore([]music2bb.Song{song}, false)
	if err != nil || restored[stableSongID(song)].Video == nil || restored[stableSongID(song)].Video.BVID != "BV-automatic" {
		t.Fatalf("automatic checkpoint restore = %#v, %v", restored, err)
	}
}

func TestAutomaticOutcomeClearsManualMarkerAfterDecisionCacheClear(t *testing.T) {
	root := t.TempDir()
	song := music2bb.Song{Name: "Song", SourceID: "source:song"}
	manualVideo := music2bb.Video{BVID: "BV-manual"}
	state := newConversionState(root, "https://music.example/list", time.Now)
	if err := state.saveDecision(music2bb.MatchResult{Song: song, Video: &manualVideo, HasSelection: true}, false); err != nil {
		t.Fatal(err)
	}
	if err := os.RemoveAll(state.decisionsDir); err != nil {
		t.Fatal(err)
	}
	reloaded := newConversionState(root, "https://music.example/list", time.Now)
	if restored, err := reloaded.restore([]music2bb.Song{song}, false); err != nil || len(restored) != 0 {
		t.Fatalf("cleared decision restore = %#v, %v", restored, err)
	}
	automaticVideo := music2bb.Video{BVID: "BV-automatic"}
	if err := reloaded.saveOutcome(music2bb.MatchResult{Song: song, Video: &automaticVideo, HasSelection: true, SearchStatus: music2bb.SearchStatusCompleted}); err != nil {
		t.Fatal(err)
	}
	final := newConversionState(root, "https://music.example/list", time.Now)
	restored, err := final.restore([]music2bb.Song{song}, false)
	if err != nil || restored[stableSongID(song)].Video == nil || restored[stableSongID(song)].Video.BVID != "BV-automatic" {
		t.Fatalf("automatic checkpoint restore = %#v, %v", restored, err)
	}
}

func TestFreshIgnoresCheckpointAndDecisionWithoutAffectingSearchSemantics(t *testing.T) {
	root := t.TempDir()
	now := time.Unix(1000, 0)
	song := music2bb.Song{Name: "Song", SourceID: "source:song"}
	video := music2bb.Video{BVID: "BV1"}
	state := newConversionState(root, "https://music.example/list", func() time.Time { return now })
	if err := state.saveOutcome(music2bb.MatchResult{Song: song, Video: &video, HasSelection: true, SearchStatus: music2bb.SearchStatusCompleted}); err != nil {
		t.Fatal(err)
	}
	if err := state.saveDecision(music2bb.MatchResult{Song: song, Video: &video, HasSelection: true}, false); err != nil {
		t.Fatal(err)
	}

	fresh := newConversionState(root, "https://music.example/list", func() time.Time { return now })
	restored, err := fresh.restore([]music2bb.Song{song}, true)
	if err != nil {
		t.Fatal(err)
	}
	if len(restored) != 0 {
		t.Fatalf("fresh restored = %#v", restored)
	}
	if info, err := os.Stat(state.checkpointPath); err != nil {
		t.Fatal(err)
	} else if info.Mode().Perm() != 0o600 {
		t.Fatalf("checkpoint mode = %v", info.Mode().Perm())
	}
	if info, err := os.Stat(state.decisionPath(stableSongID(song))); err != nil {
		t.Fatal(err)
	} else if info.Mode().Perm() != 0o600 {
		t.Fatalf("decision mode = %v", info.Mode().Perm())
	}
}

func TestCorruptCheckpointAndDecisionAreReportedAndPreserved(t *testing.T) {
	root := t.TempDir()
	song := music2bb.Song{Name: "Song", SourceID: "source:song"}

	checkpoint := newConversionState(root, "https://music.example/checkpoint", time.Now)
	if err := os.MkdirAll(filepath.Dir(checkpoint.checkpointPath), 0o700); err != nil {
		t.Fatal(err)
	}
	corrupt := []byte("{broken checkpoint")
	if err := os.WriteFile(checkpoint.checkpointPath, corrupt, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := checkpoint.restore([]music2bb.Song{song}, false); err == nil || !strings.Contains(err.Error(), "original file preserved") {
		t.Fatalf("checkpoint error = %v", err)
	}
	if got, _ := os.ReadFile(checkpoint.checkpointPath); !reflect.DeepEqual(got, corrupt) {
		t.Fatalf("checkpoint was changed: %q", got)
	}

	decision := newConversionState(root, "https://music.example/decision", time.Now)
	decisionPath := decision.decisionPath(stableSongID(song))
	if err := os.MkdirAll(filepath.Dir(decisionPath), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(decisionPath, corrupt, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := decision.restore([]music2bb.Song{song}, false); err == nil || !strings.Contains(err.Error(), "original file preserved") {
		t.Fatalf("decision error = %v", err)
	}
	if got, _ := os.ReadFile(decisionPath); !reflect.DeepEqual(got, corrupt) {
		t.Fatalf("decision was changed: %q", got)
	}
}

func TestWriteReceiptsAreAdditiveAndPartitionedByFavorite(t *testing.T) {
	root := t.TempDir()
	state := newConversionState(root, "https://music.example/list", func() time.Time { return time.Unix(1234, 0) })
	if err := state.saveWriteReceipt(music2bb.WriteReceipt{FavoriteID: 9, BVID: "BV-retry", Reason: "temporary"}); err != nil {
		t.Fatal(err)
	}
	if err := state.saveWriteSuccess(9, "BV-one"); err != nil {
		t.Fatal(err)
	}
	if err := state.saveWriteSuccess(10, "BV-two"); err != nil {
		t.Fatal(err)
	}

	reloaded := newConversionState(root, "https://music.example/list", time.Now)
	for favoriteID, want := range map[int64]string{9: "BV-one", 10: "BV-two"} {
		got, err := reloaded.successfulWrites(favoriteID)
		if err != nil {
			t.Fatal(err)
		}
		if _, ok := got[want]; !ok || len(got) != 1 {
			t.Fatalf("favorite %d receipts = %#v", favoriteID, got)
		}
	}
	if err := reloaded.loadCheckpointLocked(); err != nil {
		t.Fatal(err)
	}
	failed := reloaded.document.Writes["9"].Failed["BV-retry"]
	if failed.Reason != "temporary" || failed.UpdatedAt.IsZero() {
		t.Fatalf("failed receipt = %#v", failed)
	}
	if err := reloaded.saveWriteSuccess(9, "BV-retry"); err != nil {
		t.Fatal(err)
	}
	if _, ok := reloaded.document.Writes["9"].Failed["BV-retry"]; ok {
		t.Fatal("successful retry did not replace the failed receipt")
	}
}

func TestLegacyV1CheckpointWithoutWritesRemainsReadable(t *testing.T) {
	root := t.TempDir()
	state := newConversionState(root, "https://music.example/list", time.Now)
	legacy := `{"version":1,"playlistID":"` + state.playlistID + `","sourceURL":"https://music.example/list","updatedAt":"2026-07-16T00:00:00Z","songs":{}}`
	if err := os.MkdirAll(filepath.Dir(state.checkpointPath), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(state.checkpointPath, []byte(legacy), 0o600); err != nil {
		t.Fatal(err)
	}
	if receipts, err := state.successfulWrites(9); err != nil || len(receipts) != 0 {
		t.Fatalf("legacy receipts = %#v, %v", receipts, err)
	}
}
