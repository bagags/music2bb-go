package ui

import (
	"strings"
	"testing"
	"time"

	music2bb "github.com/bagags/music2bb-go"
)

func TestRuntimeTelemetryTracksOutcomeAndFormatsVisibleLog(t *testing.T) {
	now := time.Date(2026, 7, 18, 12, 0, 0, 0, time.UTC)
	var telemetry runtimeTelemetry
	telemetry.reset("转换", now)
	video := music2bb.Video{BVID: "BV1", Title: "Candidate"}
	outcome := music2bb.MatchResult{
		Song:  music2bb.Song{Name: "Song", Artist: "Artist", SourceID: "source-1"},
		Video: &video, Score: 88.5, HasSelection: true,
		SearchIdentity: music2bb.SearchIdentityAnonymous,
		SearchStatus:   music2bb.SearchStatusCompleted,
		RemoteRequests: 3, CacheHits: 1,
	}
	line := telemetry.apply(music2bb.ProgressEvent{
		Kind: music2bb.EventSong, Operation: "match", Current: 1, Total: 4,
		Song: &outcome.Song, Outcome: &outcome,
	}, now.Add(time.Second))
	if telemetry.Stage != "Bilibili 匹配" || telemetry.Current != 1 || telemetry.Total != 4 {
		t.Fatalf("progress = %#v", telemetry)
	}
	if telemetry.Completed != 1 || telemetry.Selected != 1 || telemetry.RemoteRequests != 3 || telemetry.CacheHits != 1 || telemetry.AnonymousReqs != 3 {
		t.Fatalf("metrics = %#v", telemetry)
	}
	for _, want := range []string{"[1/4]", "Song - Artist", "Candidate", "远程 3", "缓存 1"} {
		if !strings.Contains(line, want) {
			t.Fatalf("log %q missing %q", line, want)
		}
	}
}

func TestRuntimeTelemetryWriteReceiptAndQuietTime(t *testing.T) {
	now := time.Date(2026, 7, 18, 12, 0, 0, 0, time.UTC)
	var telemetry runtimeTelemetry
	telemetry.reset("写入", now)
	receipt := music2bb.WriteReceipt{FavoriteID: 9, BVID: "BV9", Succeeded: true}
	line := telemetry.apply(music2bb.ProgressEvent{
		Kind: music2bb.EventVideo, Operation: "add_favorite", Current: 1, Total: 2,
		WriteReceipt: &receipt,
	}, now.Add(2*time.Second))
	if telemetry.WriteSucceeded != 1 || !strings.Contains(line, "写入成功") {
		t.Fatalf("telemetry = %#v, line = %q", telemetry, line)
	}
	if got := telemetry.quietFor(now.Add(17 * time.Second)); got != 15*time.Second {
		t.Fatalf("quiet time = %s", got)
	}
}

func TestRuntimeTelemetryCountsDuplicateSourceSongsSeparately(t *testing.T) {
	now := time.Date(2026, 7, 18, 10, 0, 0, 0, time.UTC)
	telemetry := runtimeTelemetry{}
	telemetry.reset("匹配", now)
	telemetry.beginStage("Bilibili 匹配", 2, now)
	outcome := music2bb.MatchResult{
		Song:         music2bb.Song{Name: "重复歌曲", SourceID: "same-source-id"},
		SearchStatus: music2bb.SearchStatusCompleted,
		HasSelection: true,
	}

	for current := 1; current <= 2; current++ {
		telemetry.apply(music2bb.ProgressEvent{
			Kind: music2bb.EventSong, Operation: "match", Current: current, Total: 2,
			Song: &outcome.Song, Outcome: &outcome,
		}, now.Add(time.Duration(current)*time.Second))
	}

	if telemetry.Completed != 2 || telemetry.Selected != 2 {
		t.Fatalf("duplicate source songs collapsed: completed=%d selected=%d", telemetry.Completed, telemetry.Selected)
	}
}
