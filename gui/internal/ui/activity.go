package ui

import (
	"fmt"
	"strings"
	"time"

	music2bb "github.com/bagags/music2bb-go"
	"github.com/bagags/music2bb-go/m2bb-gui/internal/core"
)

type runtimeTelemetry struct {
	Stage          string
	Current        int
	Total          int
	CurrentSong    string
	StartedAt      time.Time
	LastActivity   time.Time
	Outcomes       map[string]music2bb.MatchResult
	Completed      int
	Selected       int
	NeedsReview    int
	Skipped        int
	Failed         int
	RemoteRequests int
	CacheHits      int
	AnonymousReqs  int
	SessionReqs    int
	WriteSucceeded int
	WriteFailed    int
}

func (t *runtimeTelemetry) reset(operation string, now time.Time) {
	*t = runtimeTelemetry{
		Stage: operation, StartedAt: now, LastActivity: now,
		Outcomes: make(map[string]music2bb.MatchResult),
	}
}

func (t *runtimeTelemetry) beginStage(stage string, total int, now time.Time) {
	t.Stage, t.Current, t.Total, t.CurrentSong, t.LastActivity = stage, 0, total, "", now
}

func (t *runtimeTelemetry) apply(event music2bb.ProgressEvent, now time.Time) string {
	t.LastActivity = now
	if stage := operationLabel(event.Operation); stage != "" {
		t.Stage = stage
	}
	if event.Total > 0 {
		t.Current, t.Total = event.Current, event.Total
	}
	if event.Song != nil {
		t.CurrentSong = strings.TrimSpace(strings.Join(nonEmpty(event.Song.Name, event.Song.Artist), " - "))
	}
	if event.Kind == music2bb.EventSong && event.Outcome != nil {
		outcome := *event.Outcome
		// Current is part of the key because the same source song may legitimately
		// appear more than once in a playlist.
		key := fmt.Sprintf("event-%d:%s", event.Current, outcome.Song.StableSourceID())
		t.Outcomes[key] = outcome
		t.recalculate()
		return describeOutcomeEvent(event.Current, event.Total, outcome)
	}
	if event.Kind == music2bb.EventVideo && event.WriteReceipt != nil {
		if event.WriteReceipt.Succeeded {
			t.WriteSucceeded++
			return fmt.Sprintf("[%d/%d] 写入成功：%s", event.Current, event.Total, event.WriteReceipt.BVID)
		}
		t.WriteFailed++
		return fmt.Sprintf("[%d/%d] 写入失败：%s · %s", event.Current, event.Total, event.WriteReceipt.BVID, event.WriteReceipt.Reason)
	}
	if event.Kind == music2bb.EventQR {
		return "Bilibili 登录二维码已生成，等待扫码确认"
	}
	if strings.TrimSpace(event.Message) != "" {
		prefix := ""
		if event.Kind == music2bb.EventWarning {
			prefix = "警告："
		}
		return prefix + strings.TrimSpace(event.Message)
	}
	return ""
}

func (t *runtimeTelemetry) recalculate() {
	t.Completed, t.Selected, t.NeedsReview, t.Skipped, t.Failed = 0, 0, 0, 0, 0
	t.RemoteRequests, t.CacheHits, t.AnonymousReqs, t.SessionReqs = 0, 0, 0, 0
	for _, outcome := range t.Outcomes {
		if outcome.SearchStatus != music2bb.SearchStatusNotSearched {
			t.Completed++
		}
		if outcome.HasSelection {
			t.Selected++
		} else if outcome.NeedsReview {
			t.NeedsReview++
		} else if outcome.SearchStatus == music2bb.SearchStatusCompleted {
			t.Skipped++
		}
		if outcome.Failure != nil || outcome.SearchStatus == music2bb.SearchStatusFailed {
			t.Failed++
		}
		t.RemoteRequests += outcome.RemoteRequests
		t.CacheHits += outcome.CacheHits
		switch outcome.SearchIdentity {
		case music2bb.SearchIdentityAnonymous:
			t.AnonymousReqs += outcome.RemoteRequests
		case music2bb.SearchIdentitySession:
			t.SessionReqs += outcome.RemoteRequests
		}
	}
}

func (t runtimeTelemetry) fraction() float32 {
	if t.Total <= 0 {
		return 0
	}
	value := float32(t.Current) / float32(t.Total)
	if value < 0 {
		return 0
	}
	if value > 1 {
		return 1
	}
	return value
}

func (t runtimeTelemetry) elapsed(now time.Time) time.Duration {
	if t.StartedAt.IsZero() {
		return 0
	}
	return now.Sub(t.StartedAt).Round(time.Second)
}

func (t runtimeTelemetry) quietFor(now time.Time) time.Duration {
	if t.LastActivity.IsZero() {
		return 0
	}
	return now.Sub(t.LastActivity).Round(time.Second)
}

func describeOutcomeEvent(current, total int, outcome music2bb.MatchResult) string {
	result := core.ReviewReasonText(outcome.ReviewReason)
	if outcome.HasSelection && outcome.Video != nil {
		result = fmt.Sprintf("已选 %s（%.1f 分）", outcome.Video.Title, outcome.Score)
	} else if !outcome.NeedsReview && outcome.SearchStatus == music2bb.SearchStatusCompleted {
		result = "已跳过"
	}
	if outcome.Failure != nil {
		result += " · " + outcome.Failure.Reason
	}
	return fmt.Sprintf("[%d/%d] %s - %s → %s · 身份 %s · 远程 %d · 缓存 %d",
		current, total, outcome.Song.Name, outcome.Song.Artist, result,
		identityLabel(outcome.SearchIdentity), outcome.RemoteRequests, outcome.CacheHits)
}

func operationLabel(operation string) string {
	switch operation {
	case "login":
		return "Bilibili 登录"
	case "parse_playlist":
		return "解析歌单"
	case "match":
		return "Bilibili 匹配"
	case "search_identity":
		return "切换搜索身份"
	case "add_favorite":
		return "写入收藏夹"
	default:
		return ""
	}
}

func identityLabel(identity music2bb.SearchIdentity) string {
	switch identity {
	case music2bb.SearchIdentityAnonymous:
		return "匿名"
	case music2bb.SearchIdentitySession:
		return "登录态"
	default:
		return "—"
	}
}

func searchStatusLabel(status music2bb.SearchStatus) string {
	switch status {
	case music2bb.SearchStatusCompleted:
		return "已完成"
	case music2bb.SearchStatusRiskControl:
		return "风控停止"
	case music2bb.SearchStatusNotSearched:
		return "尚未搜索"
	case music2bb.SearchStatusBudgetExhausted:
		return "预算用尽"
	case music2bb.SearchStatusFailed:
		return "搜索失败"
	default:
		return "—"
	}
}

func nonEmpty(values ...string) []string {
	result := make([]string, 0, len(values))
	for _, value := range values {
		if value = strings.TrimSpace(value); value != "" {
			result = append(result, value)
		}
	}
	return result
}
