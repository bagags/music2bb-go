package core

import (
	"context"
	"strings"
	"time"

	music2bb "github.com/bagags/music2bb-go"
)

// Backend is the stable public engine surface consumed by the desktop app.
type Backend interface {
	LoginWithOptions(context.Context, music2bb.LoginOptions, music2bb.Observer) (music2bb.Account, error)
	Logout(context.Context) error
	ResetAnonymousIdentity(context.Context) error
	ParsePlaylistWithOptions(context.Context, string, music2bb.ParseOptions, music2bb.Observer) ([]music2bb.Song, error)
	Match(context.Context, []music2bb.Song, music2bb.MatchOptions, music2bb.Observer) ([]music2bb.MatchResult, error)
	SearchCandidatesWithOptions(context.Context, music2bb.Song, string, music2bb.CandidateSearchOptions) ([]music2bb.MatchResult, error)
	VideoDetail(context.Context, string) (music2bb.Video, error)
	ListFavorites(context.Context) ([]music2bb.Favorite, error)
	CreateFavorite(context.Context, music2bb.CreateFavoriteRequest) (music2bb.Favorite, error)
	AddToFavorite(context.Context, int64, []music2bb.MatchResult, music2bb.Observer) (music2bb.AddResult, error)
	Browser() *music2bb.BrowserManager
	PersistentStatePaths() (string, string)
}

type Options struct {
	SearchPages   int
	TopK          int
	Workers       int
	SearchBudget  int
	Profile       music2bb.MatchProfile
	Identity      string
	BrowserPolicy music2bb.BrowserPolicy
	Manual        bool
	ReviewAll     bool
	AllowQR       bool
	Fresh         bool
	RefreshSearch bool
	CustomWeights *music2bb.MatchWeights
}

func DefaultOptions() Options {
	return Options{
		SearchPages: 3, TopK: 5, Workers: 2, SearchBudget: 4,
		Profile: music2bb.MatchProfileStandard, Identity: "auto",
		BrowserPolicy: music2bb.BrowserAuto, AllowQR: true,
	}
}

func (o Options) Validate() error {
	if o.SearchPages < 1 || o.TopK < 1 || o.Workers < 1 || o.SearchBudget < 1 {
		return &music2bb.Error{Category: music2bb.ErrorInvalidInput, Operation: "options", Message: "页数、候选数、并发数和请求预算都必须大于 0"}
	}
	if o.Profile != music2bb.MatchProfileStandard && o.Profile != music2bb.MatchProfileClassical {
		return &music2bb.Error{Category: music2bb.ErrorInvalidInput, Operation: "options", Message: "未知匹配策略"}
	}
	if o.Identity != "auto" && o.Identity != string(music2bb.SearchIdentityAnonymous) && o.Identity != string(music2bb.SearchIdentitySession) {
		return &music2bb.Error{Category: music2bb.ErrorInvalidInput, Operation: "options", Message: "未知搜索身份"}
	}
	if o.BrowserPolicy != music2bb.BrowserAuto && o.BrowserPolicy != music2bb.BrowserNever && o.BrowserPolicy != music2bb.BrowserAlways {
		return &music2bb.Error{Category: music2bb.ErrorInvalidInput, Operation: "options", Message: "未知浏览器策略"}
	}
	if o.CustomWeights != nil {
		w := o.CustomWeights
		if w.Title < 0 || w.Artist < 0 || w.Quality < 0 || w.Official < 0 || w.Popularity < 0 || w.Uploader < 0 || w.Title+w.Artist+w.Quality+w.Official+w.Popularity+w.Uploader <= 0 {
			return &music2bb.Error{Category: music2bb.ErrorInvalidInput, Operation: "options", Message: "自定义权重必须非负且至少一项为正"}
		}
	}
	return nil
}

// ParseManualSongs accepts one "name - artist" item per non-empty line.
func ParseManualSongs(text string) []music2bb.Song {
	var songs []music2bb.Song
	for _, line := range strings.Split(strings.ReplaceAll(text, "\r\n", "\n"), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, " - ", 2)
		song := music2bb.Song{Name: strings.TrimSpace(parts[0])}
		if len(parts) == 2 {
			song.Artist = strings.TrimSpace(parts[1])
		}
		if song.Name != "" {
			song.SourceID = song.StableSourceID()
			songs = append(songs, song)
		}
	}
	return songs
}

func LoginOptions(allowQR bool) music2bb.LoginOptions {
	return music2bb.LoginOptions{UseStoredCookies: true, AllowQR: allowQR, Timeout: 3 * time.Minute}
}

func ReviewReasonText(reason music2bb.ReviewReason) string {
	switch reason {
	case music2bb.ReviewNoCandidates:
		return "没有候选"
	case music2bb.ReviewSearchFailed:
		return "搜索失败"
	case music2bb.ReviewWeakTitle:
		return "标题证据不足"
	case music2bb.ReviewArtistUnverified:
		return "歌手证据不足"
	case music2bb.ReviewAmbiguous:
		return "候选过于接近"
	case music2bb.ReviewRiskControl:
		return "平台风控"
	case music2bb.ReviewNotSearched:
		return "尚未搜索"
	case music2bb.ReviewBudgetExhausted:
		return "请求预算已用尽"
	default:
		return "已完成"
	}
}
