package core

import (
	"context"
	"testing"

	music2bb "github.com/bagags/music2bb-go"
)

type fakeBackend struct {
	dir       string
	matchRuns int
	writes    int
}

func (f *fakeBackend) LoginWithOptions(context.Context, music2bb.LoginOptions, music2bb.Observer) (music2bb.Account, error) {
	return music2bb.Account{ID: 1, Name: "tester"}, nil
}
func (f *fakeBackend) Logout(context.Context) error                 { return nil }
func (f *fakeBackend) ResetAnonymousIdentity(context.Context) error { return nil }
func (f *fakeBackend) ParsePlaylistWithOptions(context.Context, string, music2bb.ParseOptions, music2bb.Observer) ([]music2bb.Song, error) {
	return nil, nil
}
func (f *fakeBackend) Match(_ context.Context, songs []music2bb.Song, _ music2bb.MatchOptions, observer music2bb.Observer) ([]music2bb.MatchResult, error) {
	f.matchRuns++
	result := make([]music2bb.MatchResult, len(songs))
	for i, song := range songs {
		video := music2bb.Video{BVID: "BV" + song.Name, Title: song.Name}
		result[i] = music2bb.MatchResult{Song: song, Video: &video, Matched: true, HasSelection: true, SearchStatus: music2bb.SearchStatusCompleted}
		if observer != nil {
			copy := result[i]
			observer.Observe(music2bb.ProgressEvent{Kind: music2bb.EventSong, Operation: "match", Current: i + 1, Total: len(songs), Song: &copy.Song, Match: &copy, Outcome: &copy})
		}
	}
	return result, nil
}
func (f *fakeBackend) SearchCandidatesWithOptions(context.Context, music2bb.Song, string, music2bb.CandidateSearchOptions) ([]music2bb.MatchResult, error) {
	return nil, nil
}
func (f *fakeBackend) VideoDetail(context.Context, string) (music2bb.Video, error) {
	return music2bb.Video{}, nil
}
func (f *fakeBackend) ListFavorites(context.Context) ([]music2bb.Favorite, error) {
	return []music2bb.Favorite{{ID: 9, Title: "target"}}, nil
}
func (f *fakeBackend) CreateFavorite(context.Context, music2bb.CreateFavoriteRequest) (music2bb.Favorite, error) {
	return music2bb.Favorite{ID: 9, Title: "target"}, nil
}
func (f *fakeBackend) AddToFavorite(_ context.Context, favoriteID int64, matches []music2bb.MatchResult, observer music2bb.Observer) (music2bb.AddResult, error) {
	f.writes++
	result := music2bb.AddResult{FavoriteID: favoriteID}
	for _, match := range matches {
		result.Succeeded = append(result.Succeeded, match.Video.BVID)
		if observer != nil {
			receipt := music2bb.WriteReceipt{FavoriteID: favoriteID, BVID: match.Video.BVID, Succeeded: true}
			observer.Observe(music2bb.ProgressEvent{Kind: music2bb.EventVideo, Operation: "add_favorite", WriteReceipt: &receipt})
		}
	}
	return result, nil
}
func (f *fakeBackend) Browser() *music2bb.BrowserManager { return nil }
func (f *fakeBackend) PersistentStatePaths() (string, string) {
	return f.dir, f.dir
}

func TestSessionRestoresMatchesAndSkipsSuccessfulWriteRetry(t *testing.T) {
	backend := &fakeBackend{dir: t.TempDir()}
	session, err := NewSession(backend, "https://example.test/list", DefaultOptions())
	if err != nil {
		t.Fatal(err)
	}
	songs := []music2bb.Song{{Name: "one"}, {Name: "two"}}
	first, err := session.Match(context.Background(), songs, nil)
	if err != nil {
		t.Fatal(err)
	}
	second, err := session.Match(context.Background(), songs, nil)
	if err != nil {
		t.Fatal(err)
	}
	if backend.matchRuns != 1 || len(second) != 2 {
		t.Fatalf("match runs = %d, restored = %#v", backend.matchRuns, second)
	}

	written, err := session.Write(context.Background(), 9, first, nil)
	if err != nil || len(written.Succeeded) != 2 {
		t.Fatalf("first write = %#v, %v", written, err)
	}
	retry, err := session.Write(context.Background(), 9, first, nil)
	if err != nil || len(retry.Skipped) != 2 || backend.writes != 1 {
		t.Fatalf("retry = %#v, writes = %d, err = %v", retry, backend.writes, err)
	}
}

func TestSessionWriteRejectsUnresolvedSong(t *testing.T) {
	backend := &fakeBackend{dir: t.TempDir()}
	session, err := NewSession(backend, "manual", DefaultOptions())
	if err != nil {
		t.Fatal(err)
	}
	_, err = session.Write(context.Background(), 9, []music2bb.MatchResult{{
		Song: music2bb.Song{Name: "pending"}, NeedsReview: true, SearchStatus: music2bb.SearchStatusNotSearched,
	}}, nil)
	if err == nil || backend.writes != 0 {
		t.Fatalf("write error = %v, writes = %d", err, backend.writes)
	}
}
