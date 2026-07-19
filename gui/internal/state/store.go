package state

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strings"
	"sync"
	"time"

	music2bb "github.com/bagags/music2bb-go"
)

const (
	version     = 1
	decisionTTL = 7 * 24 * time.Hour
)

type checkpoint struct {
	Version    int                       `json:"version"`
	PlaylistID string                    `json:"playlistID"`
	SourceURL  string                    `json:"sourceURL"`
	UpdatedAt  time.Time                 `json:"updatedAt"`
	Songs      map[string]checkpointSong `json:"songs"`
	Writes     map[string]writeReceipt   `json:"writes,omitempty"`
}

type checkpointSong struct {
	SourceID       string               `json:"sourceID"`
	Outcome        music2bb.MatchResult `json:"outcome"`
	ManualDecision bool                 `json:"manualDecision,omitempty"`
	UpdatedAt      time.Time            `json:"updatedAt"`
}

type writeReceipt struct {
	FavoriteID int64                    `json:"favoriteID"`
	Succeeded  map[string]time.Time     `json:"succeeded"`
	Failed     map[string]failedReceipt `json:"failed,omitempty"`
}

type failedReceipt struct {
	Reason    string    `json:"reason"`
	UpdatedAt time.Time `json:"updatedAt"`
}

type decision struct {
	Version   int             `json:"version"`
	SourceID  string          `json:"sourceID"`
	Kind      string          `json:"kind"`
	Video     *music2bb.Video `json:"video,omitempty"`
	UpdatedAt time.Time       `json:"updatedAt"`
}

// Store reads and writes the same additive v1 checkpoint format as the CLI.
type Store struct {
	mu             sync.Mutex
	now            func() time.Time
	rawURL         string
	playlistID     string
	checkpointPath string
	decisionsDir   string
	doc            checkpoint
	loaded         bool
	loadErr        error
}

func New(configDir, rawURL string) *Store {
	if strings.TrimSpace(configDir) == "" {
		return nil
	}
	id := PlaylistID(rawURL)
	return &Store{
		now: time.Now, rawURL: rawURL, playlistID: id,
		checkpointPath: filepath.Join(configDir, "conversions", "v1", id+".json"),
		decisionsDir:   filepath.Join(configDir, "decisions", "v1"),
	}
}

func (s *Store) Restore(songs []music2bb.Song, fresh bool) (map[string]music2bb.MatchResult, error) {
	result := make(map[string]music2bb.MatchResult)
	if s == nil {
		return result, nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.load(); err != nil {
		return nil, err
	}
	for _, song := range songs {
		song.SourceID = song.StableSourceID()
		manual, ok, err := s.loadDecision(song.SourceID)
		if err != nil {
			return nil, err
		}
		if ok {
			if !fresh && s.now().Sub(manual.UpdatedAt) < decisionTTL {
				result[song.SourceID] = outcomeFromDecision(song, manual)
			}
			continue
		}
		if fresh {
			continue
		}
		entry, ok := s.doc.Songs[song.SourceID]
		if ok && !entry.ManualDecision && entry.Outcome.SearchStatus == music2bb.SearchStatusCompleted {
			out := cloneOutcome(entry.Outcome)
			out.Song = song
			for i := range out.Candidates {
				out.Candidates[i].Song = song
			}
			result[song.SourceID] = out
		}
	}
	if fresh {
		s.doc = newCheckpoint(s.playlistID, s.rawURL)
	}
	return result, nil
}

func (s *Store) SaveOutcome(outcome music2bb.MatchResult) error {
	if s == nil {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.load(); err != nil {
		return err
	}
	outcome = cloneOutcome(outcome)
	outcome.Song.SourceID = outcome.Song.StableSourceID()
	now := s.now().UTC()
	s.doc.Songs[outcome.Song.SourceID] = checkpointSong{SourceID: outcome.Song.SourceID, Outcome: outcome, UpdatedAt: now}
	s.doc.UpdatedAt = now
	return writeJSON(s.checkpointPath, s.doc)
}

func (s *Store) SaveDecision(outcome music2bb.MatchResult, skipped bool) error {
	if s == nil {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.load(); err != nil {
		return err
	}
	sourceID := outcome.Song.StableSourceID()
	now := s.now().UTC()
	manual := decision{Version: version, SourceID: sourceID, Kind: "skip", UpdatedAt: now}
	if !skipped {
		if !outcome.HasSelection || outcome.Video == nil {
			return errors.New("manual selection has no video")
		}
		video := cloneVideo(*outcome.Video)
		manual.Kind, manual.Video = "select", &video
	}
	if err := writeJSON(s.decisionPath(sourceID), manual); err != nil {
		return err
	}
	outcome = cloneOutcome(outcome)
	outcome.Song.SourceID = sourceID
	outcome.ManualOverride = !skipped
	outcome.NeedsReview = false
	outcome.ReviewReason = music2bb.ReviewNone
	outcome.SearchStatus = music2bb.SearchStatusCompleted
	if skipped {
		outcome.Video, outcome.HasSelection, outcome.Matched = nil, false, false
	}
	s.doc.Songs[sourceID] = checkpointSong{SourceID: sourceID, Outcome: outcome, ManualDecision: true, UpdatedAt: now}
	s.doc.UpdatedAt = now
	return writeJSON(s.checkpointPath, s.doc)
}

func (s *Store) RemoveDecision(song music2bb.Song) error {
	if s == nil {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.load(); err != nil {
		return err
	}
	sourceID := song.StableSourceID()
	if err := os.Remove(s.decisionPath(sourceID)); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	delete(s.doc.Songs, sourceID)
	s.doc.UpdatedAt = s.now().UTC()
	return writeJSON(s.checkpointPath, s.doc)
}

func (s *Store) SuccessfulWrites(favoriteID int64) (map[string]struct{}, error) {
	result := make(map[string]struct{})
	if s == nil {
		return result, nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.load(); err != nil {
		return nil, err
	}
	receipt, ok := s.doc.Writes[fmt.Sprint(favoriteID)]
	if !ok || receipt.FavoriteID != favoriteID {
		return result, nil
	}
	for bvid := range receipt.Succeeded {
		result[bvid] = struct{}{}
	}
	return result, nil
}

func (s *Store) SaveWriteReceipt(item music2bb.WriteReceipt) error {
	if s == nil || item.FavoriteID <= 0 || strings.TrimSpace(item.BVID) == "" {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.load(); err != nil {
		return err
	}
	key := fmt.Sprint(item.FavoriteID)
	receipt := s.doc.Writes[key]
	if receipt.FavoriteID == 0 {
		receipt = writeReceipt{FavoriteID: item.FavoriteID, Succeeded: make(map[string]time.Time), Failed: make(map[string]failedReceipt)}
	}
	if receipt.Succeeded == nil {
		receipt.Succeeded = make(map[string]time.Time)
	}
	if receipt.Failed == nil {
		receipt.Failed = make(map[string]failedReceipt)
	}
	now := s.now().UTC()
	if item.Succeeded {
		receipt.Succeeded[item.BVID] = now
		delete(receipt.Failed, item.BVID)
	} else {
		receipt.Failed[item.BVID] = failedReceipt{Reason: item.Reason, UpdatedAt: now}
	}
	s.doc.Writes[key] = receipt
	s.doc.UpdatedAt = now
	return writeJSON(s.checkpointPath, s.doc)
}

func (s *Store) load() error {
	if s.loaded {
		return s.loadErr
	}
	s.loaded = true
	payload, err := os.ReadFile(s.checkpointPath)
	if errors.Is(err, fs.ErrNotExist) {
		s.doc = newCheckpoint(s.playlistID, s.rawURL)
		return nil
	}
	if err != nil {
		s.loadErr = err
		return err
	}
	var doc checkpoint
	if err := decodeJSON(payload, &doc); err != nil || doc.Version != version || doc.PlaylistID != s.playlistID || doc.Songs == nil {
		if err == nil {
			err = errors.New("schema or playlist identity mismatch")
		}
		s.loadErr = fmt.Errorf("conversion checkpoint %s is corrupt; original file preserved: %w", s.checkpointPath, err)
		return s.loadErr
	}
	if doc.Writes == nil {
		doc.Writes = make(map[string]writeReceipt)
	}
	s.doc = doc
	return nil
}

func (s *Store) loadDecision(sourceID string) (decision, bool, error) {
	payload, err := os.ReadFile(s.decisionPath(sourceID))
	if errors.Is(err, fs.ErrNotExist) {
		return decision{}, false, nil
	}
	if err != nil {
		return decision{}, false, err
	}
	var value decision
	if err := decodeJSON(payload, &value); err != nil || value.Version != version || value.SourceID != sourceID || (value.Kind != "select" && value.Kind != "skip") || (value.Kind == "select" && value.Video == nil) || value.UpdatedAt.IsZero() {
		if err == nil {
			err = errors.New("schema or source identity mismatch")
		}
		return decision{}, false, fmt.Errorf("manual decision %s is corrupt; original file preserved: %w", s.decisionPath(sourceID), err)
	}
	return value, true, nil
}

func (s *Store) decisionPath(sourceID string) string {
	digest := sha256.Sum256([]byte(sourceID))
	return filepath.Join(s.decisionsDir, hex.EncodeToString(digest[:])+".json")
}

func newCheckpoint(id, rawURL string) checkpoint {
	return checkpoint{Version: version, PlaylistID: id, SourceURL: rawURL, Songs: make(map[string]checkpointSong), Writes: make(map[string]writeReceipt)}
}

func outcomeFromDecision(song music2bb.Song, value decision) music2bb.MatchResult {
	out := music2bb.MatchResult{Song: song, SearchStatus: music2bb.SearchStatusCompleted}
	if value.Kind == "skip" {
		return out
	}
	video := cloneVideo(*value.Video)
	out.Video, out.Score, out.Matched, out.HasSelection, out.ManualOverride = &video, 999, true, true, true
	candidate := out
	out.Candidates = []music2bb.MatchResult{candidate}
	return out
}

func cloneVideo(video music2bb.Video) music2bb.Video {
	video.Tags = append([]string(nil), video.Tags...)
	return video
}

func cloneOutcome(out music2bb.MatchResult) music2bb.MatchResult {
	if out.Video != nil {
		video := cloneVideo(*out.Video)
		out.Video = &video
	}
	if out.Failure != nil {
		failure := *out.Failure
		out.Failure = &failure
	}
	out.Candidates = append([]music2bb.MatchResult(nil), out.Candidates...)
	for i := range out.Candidates {
		out.Candidates[i].Candidates = nil
		if out.Candidates[i].Video != nil {
			video := cloneVideo(*out.Candidates[i].Video)
			out.Candidates[i].Video = &video
		}
	}
	return out
}

// PlaylistID canonicalizes provider share links before hashing.
func PlaylistID(rawURL string) string {
	canonical := strings.TrimSpace(rawURL)
	if parsed, err := url.Parse(canonical); err == nil && parsed.Host != "" {
		if id := providerID(parsed); id != "" {
			canonical = id
		} else {
			parsed.Scheme, parsed.Host, parsed.Fragment, parsed.Path = strings.ToLower(parsed.Scheme), strings.ToLower(parsed.Host), "", path.Clean(parsed.Path)
			query := parsed.Query()
			for key := range query {
				lower := strings.ToLower(key)
				if strings.HasPrefix(lower, "utm_") || lower == "spm_id_from" || lower == "share_source" {
					query.Del(key)
				}
			}
			parsed.RawQuery = query.Encode()
			canonical = parsed.String()
		}
	}
	digest := sha256.Sum256([]byte(canonical))
	return hex.EncodeToString(digest[:])
}

func providerID(parsed *url.URL) string {
	host := strings.ToLower(parsed.Hostname())
	if host == "music.apple.com" || strings.HasSuffix(host, ".music.apple.com") {
		for _, segment := range strings.Split(parsed.Path, "/") {
			if strings.HasPrefix(segment, "pl.") {
				return "applemusic:" + segment
			}
		}
	}
	if host == "kugou.com" || strings.HasSuffix(host, ".kugou.com") {
		id := parsed.Query().Get("specialid")
		if id == "" || id == "-2147483648" {
			id = parsed.Query().Get("global_specialid")
		}
		if id != "" {
			return "kugou:" + id
		}
	}
	return ""
}

func decodeJSON(payload []byte, target any) error {
	decoder := json.NewDecoder(bytes.NewReader(payload))
	if err := decoder.Decode(target); err != nil {
		return err
	}
	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		if err == nil {
			return errors.New("multiple JSON values")
		}
		return err
	}
	return nil
}

func writeJSON(destination string, value any) error {
	payload, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	payload = append(payload, '\n')
	dir := filepath.Dir(destination)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(dir, ".state-*.tmp")
	if err != nil {
		return err
	}
	name := tmp.Name()
	defer os.Remove(name)
	if err := tmp.Chmod(0o600); err != nil {
		tmp.Close()
		return err
	}
	if _, err := tmp.Write(payload); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(name, destination)
}
