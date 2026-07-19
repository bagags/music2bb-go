package core

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"

	music2bb "github.com/bagags/music2bb-go"
	"github.com/bagags/music2bb-go/m2bb-gui/internal/state"
)

// Session owns one conversion and mirrors the CLI's identity, recovery, and
// idempotent-write policy while keeping presentation out of the backend.
type Session struct {
	backend Backend
	rawURL  string
	options Options
	store   *state.Store

	loginMu      sync.Mutex
	account      music2bb.Account
	loggedIn     bool
	refreshOnce  sync.Once
	refreshErr   error
	writeBlocked bool
	haltReason   music2bb.RiskControlReason
}

func NewSession(backend Backend, rawURL string, options Options) (*Session, error) {
	if backend == nil {
		return nil, errors.New("backend is nil")
	}
	if err := options.Validate(); err != nil {
		return nil, err
	}
	configDir, _ := backend.PersistentStatePaths()
	return &Session{backend: backend, rawURL: strings.TrimSpace(rawURL), options: options, store: state.New(configDir, rawURL)}, nil
}

func (s *Session) Login(ctx context.Context, observer music2bb.Observer) (music2bb.Account, error) {
	s.loginMu.Lock()
	defer s.loginMu.Unlock()
	if s.loggedIn {
		return s.account, nil
	}
	account, err := s.backend.LoginWithOptions(ctx, LoginOptions(s.options.AllowQR), observer)
	if err == nil {
		s.account, s.loggedIn = account, true
	}
	return account, err
}

func (s *Session) Parse(ctx context.Context, observer music2bb.Observer) ([]music2bb.Song, error) {
	if s.rawURL == "" {
		return nil, &music2bb.Error{Category: music2bb.ErrorInvalidInput, Operation: "parse playlist", Message: "请输入歌单链接"}
	}
	incomplete := false
	tracking := music2bb.ObserverFunc(func(event music2bb.ProgressEvent) {
		if event.Kind == music2bb.EventWarning && event.Operation == "parse_playlist" && event.Total > 0 && event.Current < event.Total {
			incomplete = true
		}
		if observer != nil {
			observer.Observe(event)
		}
	})
	songs, err := s.backend.ParsePlaylistWithOptions(ctx, s.rawURL, music2bb.ParseOptions{BrowserPolicy: s.options.BrowserPolicy}, tracking)
	if (err == nil && !incomplete) || s.options.BrowserPolicy == music2bb.BrowserNever || s.backend.Browser() == nil {
		return withStableIDs(songs), err
	}
	status, statusErr := s.backend.Browser().Status(ctx)
	if statusErr != nil || status.Installed {
		return withStableIDs(songs), err
	}
	emit(observer, music2bb.ProgressEvent{Kind: music2bb.EventWarning, Operation: "parse_playlist", Message: "Chromium 尚未就绪，正在安装校验版后重试。"})
	if _, installErr := s.backend.Browser().Install(ctx, true); installErr != nil {
		emit(observer, music2bb.ProgressEvent{Kind: music2bb.EventWarning, Operation: "parse_playlist", Message: fmt.Sprintf("Chromium 安装失败：%v", installErr)})
		return withStableIDs(songs), err
	}
	retrySongs, retryErr := s.backend.ParsePlaylistWithOptions(ctx, s.rawURL, music2bb.ParseOptions{BrowserPolicy: music2bb.BrowserAlways}, observer)
	if err != nil || retryErr == nil {
		return withStableIDs(retrySongs), retryErr
	}
	return withStableIDs(songs), err
}

func (s *Session) Match(ctx context.Context, songs []music2bb.Song, observer music2bb.Observer) ([]music2bb.MatchResult, error) {
	if err := s.prepareSearch(ctx); err != nil {
		return nil, err
	}
	songs = withStableIDs(songs)
	restored, err := s.store.Restore(songs, s.options.Fresh)
	if err != nil {
		return nil, fmt.Errorf("restore conversion state: %w", err)
	}
	outcomes := make([]music2bb.MatchResult, len(songs))
	var pending []music2bb.Song
	var indexes []int
	completed := 0
	for i, song := range songs {
		if out, ok := restored[song.SourceID]; ok {
			outcomes[i] = out
			completed++
			emitOutcome(observer, out, completed, len(songs))
			continue
		}
		outcomes[i] = music2bb.MatchResult{Song: song, NeedsReview: true, ReviewReason: music2bb.ReviewNotSearched, SearchStatus: music2bb.SearchStatusNotSearched}
		pending, indexes = append(pending, song), append(indexes, i)
	}
	if len(pending) == 0 {
		return outcomes, nil
	}
	if s.options.Manual {
		for position, index := range indexes {
			outcomes[index].ReviewReason = music2bb.ReviewNotSearched
			emitOutcome(observer, outcomes[index], completed+position+1, len(songs))
		}
		return outcomes, nil
	}

	tracking := music2bb.ObserverFunc(func(event music2bb.ProgressEvent) {
		if event.Kind == music2bb.EventSong && event.Operation == "match" {
			event.Current += completed
			event.Total = len(songs)
			if event.Outcome != nil {
				_ = s.store.SaveOutcome(*event.Outcome)
			}
		}
		if observer != nil {
			observer.Observe(event)
		}
	})
	matched, matchErr := s.matchPending(ctx, pending, tracking)
	for position, index := range indexes {
		if position < len(matched) {
			outcomes[index] = matched[position]
		}
		if saveErr := s.store.SaveOutcome(outcomes[index]); saveErr != nil && matchErr == nil {
			matchErr = saveErr
		}
	}
	return outcomes, matchErr
}

func (s *Session) matchPending(ctx context.Context, songs []music2bb.Song, observer music2bb.Observer) ([]music2bb.MatchResult, error) {
	identity := music2bb.SearchIdentityAnonymous
	if s.options.Identity == string(music2bb.SearchIdentitySession) {
		if _, err := s.Login(ctx, observer); err != nil {
			return nil, err
		}
		identity = music2bb.SearchIdentitySession
	}
	base := music2bb.MatchOptions{
		SearchPages: s.options.SearchPages, TopK: s.options.TopK, Workers: s.options.Workers,
		Profile: s.options.Profile, Weights: s.options.CustomWeights, SearchIdentity: identity,
		SearchBudget: s.options.SearchBudget, SearchCachePolicy: s.cachePolicy(),
	}
	outcomes, err := s.backend.Match(ctx, songs, base, observer)
	reason := riskReason(err)
	if reason == "" || identity == music2bb.SearchIdentitySession || s.options.Identity != "auto" {
		if reason != "" {
			s.blockWrites(reason)
		}
		return outcomes, err
	}
	if _, loginErr := s.Login(ctx, observer); loginErr != nil {
		s.blockWrites(reason)
		return outcomes, loginErr
	}
	emit(observer, music2bb.ProgressEvent{Kind: music2bb.EventWarning, Operation: "search_identity", Message: "匿名搜索触发风控，已切换登录态继续未完成歌曲。"})
	var fallbackErr error
	for i := range outcomes {
		if outcomes[i].SearchStatus != music2bb.SearchStatusRiskControl && outcomes[i].SearchStatus != music2bb.SearchStatusNotSearched {
			continue
		}
		remaining := s.options.SearchBudget - outcomes[i].RemoteRequests
		if remaining <= 0 {
			outcomes[i].SearchStatus, outcomes[i].ReviewReason, outcomes[i].RiskReason = music2bb.SearchStatusBudgetExhausted, music2bb.ReviewBudgetExhausted, ""
			continue
		}
		one := base
		one.Workers, one.SearchBudget, one.SearchIdentity = 1, remaining, music2bb.SearchIdentitySession
		result, oneErr := s.backend.Match(ctx, []music2bb.Song{songs[i]}, one, observer)
		if len(result) == 1 {
			result[0].RemoteRequests += outcomes[i].RemoteRequests
			result[0].CacheHits += outcomes[i].CacheHits
			outcomes[i] = result[0]
		}
		if risk := riskReason(oneErr); risk != "" {
			s.blockWrites(risk)
			return outcomes, oneErr
		}
		if oneErr != nil && fallbackErr == nil {
			fallbackErr = oneErr
		}
	}
	return outcomes, fallbackErr
}

func (s *Session) Search(ctx context.Context, song music2bb.Song, query string) ([]music2bb.MatchResult, error) {
	identity := music2bb.SearchIdentityAnonymous
	if s.options.Identity == string(music2bb.SearchIdentitySession) {
		if _, err := s.Login(ctx, nil); err != nil {
			return nil, err
		}
		identity = music2bb.SearchIdentitySession
	}
	options := music2bb.CandidateSearchOptions{Limit: 10, Profile: s.options.Profile, Weights: s.options.CustomWeights, SearchIdentity: identity, SearchCachePolicy: s.cachePolicy()}
	results, err := s.backend.SearchCandidatesWithOptions(ctx, song, strings.TrimSpace(query), options)
	if riskReason(err) == "" || s.options.Identity != "auto" || identity == music2bb.SearchIdentitySession {
		return results, err
	}
	if _, loginErr := s.Login(ctx, nil); loginErr != nil {
		return results, loginErr
	}
	options.SearchIdentity = music2bb.SearchIdentitySession
	return s.backend.SearchCandidatesWithOptions(ctx, song, strings.TrimSpace(query), options)
}

func (s *Session) VideoDetail(ctx context.Context, bvid string) (music2bb.Video, error) {
	return s.backend.VideoDetail(ctx, strings.TrimSpace(bvid))
}

func (s *Session) RecordDecision(outcome music2bb.MatchResult, skipped bool) error {
	return s.store.SaveDecision(outcome, skipped)
}
func (s *Session) ClearDecision(song music2bb.Song) error { return s.store.RemoveDecision(song) }

func (s *Session) ListFavorites(ctx context.Context, observer music2bb.Observer) ([]music2bb.Favorite, error) {
	if _, err := s.Login(ctx, observer); err != nil {
		return nil, err
	}
	return s.backend.ListFavorites(ctx)
}

func (s *Session) CreateFavorite(ctx context.Context, request music2bb.CreateFavoriteRequest, observer music2bb.Observer) (music2bb.Favorite, error) {
	if _, err := s.Login(ctx, observer); err != nil {
		return music2bb.Favorite{}, err
	}
	return s.backend.CreateFavorite(ctx, request)
}

func (s *Session) Write(ctx context.Context, favoriteID int64, outcomes []music2bb.MatchResult, observer music2bb.Observer) (music2bb.AddResult, error) {
	if s.writeBlocked {
		return music2bb.AddResult{FavoriteID: favoriteID}, &music2bb.Error{Category: music2bb.ErrorWriteFailed, Operation: "add favorite", Message: "搜索已因平台风控停止，本次转换禁止写入", RiskReason: s.haltReason}
	}
	for _, outcome := range outcomes {
		skipped := !outcome.HasSelection && outcome.Video == nil && !outcome.NeedsReview && outcome.SearchStatus == music2bb.SearchStatusCompleted
		if !outcome.HasSelection && !skipped {
			return music2bb.AddResult{FavoriteID: favoriteID}, &music2bb.Error{Category: music2bb.ErrorInvalidInput, Operation: "add favorite", Message: "仍有歌曲尚未选择或跳过"}
		}
	}
	if _, err := s.Login(ctx, observer); err != nil {
		return music2bb.AddResult{FavoriteID: favoriteID}, err
	}
	already, err := s.store.SuccessfulWrites(favoriteID)
	if err != nil {
		return music2bb.AddResult{FavoriteID: favoriteID}, err
	}
	result := music2bb.AddResult{FavoriteID: favoriteID}
	pending := make([]music2bb.MatchResult, 0, len(outcomes))
	for _, outcome := range outcomes {
		if !outcome.HasSelection || outcome.Video == nil {
			continue
		}
		if _, ok := already[outcome.Video.BVID]; ok {
			result.Skipped = append(result.Skipped, outcome.Video.BVID)
			continue
		}
		pending = append(pending, outcome)
	}
	if len(pending) == 0 {
		return result, nil
	}
	tracking := music2bb.ObserverFunc(func(event music2bb.ProgressEvent) {
		if event.WriteReceipt != nil {
			_ = s.store.SaveWriteReceipt(*event.WriteReceipt)
		}
		if observer != nil {
			observer.Observe(event)
		}
	})
	written, writeErr := s.backend.AddToFavorite(ctx, favoriteID, pending, tracking)
	result.Succeeded = append(result.Succeeded, written.Succeeded...)
	result.Failed = append(result.Failed, written.Failed...)
	result.Skipped = append(result.Skipped, written.Skipped...)
	for _, bvid := range written.Succeeded {
		_ = s.store.SaveWriteReceipt(music2bb.WriteReceipt{FavoriteID: favoriteID, BVID: bvid, Succeeded: true})
	}
	for _, failure := range written.Failed {
		_ = s.store.SaveWriteReceipt(music2bb.WriteReceipt{FavoriteID: favoriteID, BVID: failure.BVID, Reason: failure.Reason})
	}
	return result, writeErr
}

func (s *Session) prepareSearch(ctx context.Context) error {
	s.refreshOnce.Do(func() {
		if s.options.RefreshSearch {
			s.refreshErr = s.backend.ResetAnonymousIdentity(ctx)
		}
	})
	return s.refreshErr
}

func (s *Session) cachePolicy() music2bb.SearchCachePolicy {
	if s.options.RefreshSearch {
		return music2bb.SearchCacheRefresh
	}
	return music2bb.SearchCacheDefault
}

func (s *Session) blockWrites(reason music2bb.RiskControlReason) {
	s.writeBlocked, s.haltReason = true, reason
}

func withStableIDs(songs []music2bb.Song) []music2bb.Song {
	result := append([]music2bb.Song(nil), songs...)
	for i := range result {
		result[i].SourceID = result[i].StableSourceID()
	}
	return result
}

func emit(observer music2bb.Observer, event music2bb.ProgressEvent) {
	if observer != nil {
		observer.Observe(event)
	}
}
func emitOutcome(observer music2bb.Observer, outcome music2bb.MatchResult, current, total int) {
	event := music2bb.ProgressEvent{Kind: music2bb.EventSong, Operation: "match", Current: current, Total: total, Song: &outcome.Song, Outcome: &outcome}
	if outcome.HasSelection {
		event.Match = &outcome
	}
	emit(observer, event)
}

func riskReason(err error) music2bb.RiskControlReason {
	var operation *music2bb.Error
	if errors.As(err, &operation) {
		return operation.RiskReason
	}
	var batch *music2bb.BatchError
	if errors.As(err, &batch) {
		return batch.HaltReason
	}
	return ""
}
