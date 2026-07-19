package ui

import (
	"bytes"
	"context"
	"fmt"
	"image"
	"image/color"
	_ "image/png"
	"strconv"
	"strings"
	"time"

	"gioui.org/app"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"
	music2bb "github.com/bagags/music2bb-go"
	"github.com/bagags/music2bb-go/m2bb-gui/internal/core"
	"github.com/bagags/music2bb-go/m2bb-gui/internal/state"
	qrcode "github.com/skip2/go-qrcode"
)

const (
	tabConvert = iota
	tabAccount
	tabFavorites
	tabBrowser
	tabStorage
	tabAbout
)

type uiMessage func(*App)

type App struct {
	window  *app.Window
	backend core.Backend
	version string
	theme   *material.Theme
	ops     op.Ops
	ctx     context.Context
	cancel  context.CancelFunc

	messages  chan uiMessage
	activeTab int
	nav       [6]widget.Clickable

	busy      bool
	operation string
	opCancel  context.CancelFunc
	status    string
	lastError string
	progress  float32
	logs      []string
	telemetry runtimeTelemetry
	account   *music2bb.Account
	qrImage   image.Image

	playlistURL       widget.Editor
	manualSongs       widget.Editor
	searchQuery       widget.Editor
	bvidInput         widget.Editor
	searchPages       widget.Editor
	topK              widget.Editor
	workers           widget.Editor
	searchBudget      widget.Editor
	weightTitle       widget.Editor
	weightArtist      widget.Editor
	weightQuality     widget.Editor
	weightOfficial    widget.Editor
	weightPopularity  widget.Editor
	weightUploader    widget.Editor
	profile           widget.Enum
	identity          widget.Enum
	browserPolicy     widget.Enum
	manualMode        widget.Bool
	reviewAll         widget.Bool
	allowQR           widget.Bool
	fresh             widget.Bool
	refreshSearch     widget.Bool
	customWeights     widget.Bool
	startURL          widget.Clickable
	startManual       widget.Clickable
	stageTabs         [5]widget.Clickable
	stageBack         widget.Clickable
	stageNext         widget.Clickable
	cancelOp          widget.Clickable
	convertStage      int
	inputChosen       bool
	inputManual       bool
	conversionStarted bool
	conversionMatched bool

	session         *core.Session
	songs           []music2bb.Song
	outcomes        []music2bb.MatchResult
	selectedSong    int
	songList        widget.List
	songClicks      []widget.Clickable
	candidateList   widget.List
	candidateClicks []widget.Clickable
	manualSearch    widget.Clickable
	directBVID      widget.Clickable
	keepSelection   widget.Clickable
	skipSong        widget.Clickable
	undoDecision    widget.Clickable
	openVideo       widget.Clickable
	openSource      widget.Clickable

	favorites        []music2bb.Favorite
	selectedFavorite int64
	favoriteList     widget.List
	favoriteClicks   []widget.Clickable
	loadFavorites    widget.Clickable
	writeConfirm     widget.Bool
	writeResults     widget.Clickable
	favoriteTitle    widget.Editor
	favoriteIntro    widget.Editor
	favoritePublic   widget.Bool
	createFavorite   widget.Clickable

	loginButton  widget.Clickable
	logoutButton widget.Clickable
	resetAnon    widget.Clickable

	browserStatus  music2bb.BrowserStatus
	browserLoaded  bool
	refreshBrowser widget.Clickable
	installBrowser widget.Clickable
	clearBrowser   widget.Clickable

	cacheStatuses map[state.CacheKind]state.CacheStatus
	cacheChecks   map[state.CacheKind]*widget.Bool
	refreshCache  widget.Clickable
	clearCache    widget.Clickable

	openProject    widget.Clickable
	openLicense    widget.Clickable
	releaseChecker *music2bb.ReleaseChecker
	updateInfo     *music2bb.UpdateInfo
	updateCheck    widget.Clickable
	openReleases   widget.Clickable
	pageList       widget.List
	stageList      widget.List
	logList        widget.List
}

func New(window *app.Window, backend core.Backend, version string) *App {
	ctx, cancel := context.WithCancel(context.Background())
	a := &App{
		window: window, backend: backend, version: version,
		theme: material.NewTheme(), ctx: ctx, cancel: cancel,
		messages: make(chan uiMessage, 256), selectedSong: -1,
		cacheStatuses: make(map[state.CacheKind]state.CacheStatus),
		cacheChecks:   make(map[state.CacheKind]*widget.Bool),
	}
	a.releaseChecker = music2bb.NewReleaseChecker(version)
	a.theme.Palette.Bg = rgba(0x0d1117ff)
	a.theme.Palette.Fg = rgba(0xe6edf3ff)
	a.theme.Palette.ContrastBg = rgba(0x238636ff)
	a.theme.Palette.ContrastFg = rgba(0xffffffff)
	a.theme.TextSize = unit.Sp(15)

	a.playlistURL.SingleLine = true
	a.searchQuery.SingleLine = true
	a.bvidInput.SingleLine = true
	a.manualSongs.SingleLine = false
	a.manualSongs.SetText("")
	for editor, value := range map[*widget.Editor]string{
		&a.searchPages: "3", &a.topK: "5", &a.workers: "2", &a.searchBudget: "4",
		&a.weightTitle: "40", &a.weightArtist: "25", &a.weightQuality: "10",
		&a.weightOfficial: "10", &a.weightPopularity: "10", &a.weightUploader: "5",
	} {
		editor.SingleLine = true
		editor.SetText(value)
	}
	a.profile.Value = string(music2bb.MatchProfileStandard)
	a.identity.Value = "auto"
	a.browserPolicy.Value = string(music2bb.BrowserAuto)
	a.allowQR.Value = true
	a.songList.List.Axis = layout.Vertical
	a.candidateList.List.Axis = layout.Vertical
	a.favoriteList.List.Axis = layout.Vertical
	a.pageList.List.Axis = layout.Vertical
	a.stageList.List.Axis = layout.Vertical
	a.logList.List.Axis = layout.Vertical
	for _, kind := range []state.CacheKind{state.CacheSearch, state.CacheCheckpoints, state.CacheDecisions, state.CacheAnonymous} {
		a.cacheChecks[kind] = new(widget.Bool)
	}
	a.status = "就绪"
	return a
}

func (a *App) Run() error {
	defer a.cancel()
	for {
		switch event := a.window.Event().(type) {
		case app.DestroyEvent:
			if a.opCancel != nil {
				a.opCancel()
			}
			return event.Err
		case app.FrameEvent:
			a.drainMessages()
			gtx := app.NewContext(&a.ops, event)
			a.handleActions(gtx)
			a.layout(gtx)
			event.Frame(gtx.Ops)
		}
	}
}

func (a *App) drainMessages() {
	for {
		select {
		case message := <-a.messages:
			message(a)
		default:
			return
		}
	}
}

func (a *App) post(message uiMessage) {
	select {
	case a.messages <- message:
	case <-a.ctx.Done():
		return
	}
	a.window.Invalidate()
}

func (a *App) begin(operation string, run func(context.Context) uiMessage) {
	if a.busy {
		return
	}
	ctx, cancel := context.WithCancel(a.ctx)
	a.busy, a.operation, a.opCancel = true, operation, cancel
	a.status, a.lastError, a.progress = operation, "", 0
	a.telemetry.reset(operation, time.Now())
	a.appendLog("开始：" + operation)
	a.pageList.List.Position = layout.Position{}
	go func() {
		ticker := time.NewTicker(time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				a.post(func(*App) {})
			}
		}
	}()
	go func() {
		message := run(ctx)
		cancel()
		if message == nil {
			message = func(*App) {}
		}
		a.post(func(app *App) {
			message(app)
			app.busy, app.operation, app.opCancel = false, "", nil
			if app.status == operation {
				app.status = "就绪"
			}
		})
	}()
}

func (a *App) observer() music2bb.Observer {
	return music2bb.ObserverFunc(func(event music2bb.ProgressEvent) {
		var qr image.Image
		if event.Kind == music2bb.EventQR && event.QRPayload != "" {
			if payload, err := qrcode.Encode(event.QRPayload, qrcode.Medium, 320); err == nil {
				qr, _, _ = image.Decode(bytes.NewReader(payload))
			}
		}
		a.post(func(app *App) {
			now := time.Now()
			if line := app.telemetry.apply(event, now); line != "" {
				app.appendLog(line)
			}
			if event.Total > 0 {
				app.progress = app.telemetry.fraction()
			}
			if event.Message != "" {
				app.status = event.Message
			}
			if qr != nil {
				app.qrImage = qr
				app.activeTab = tabAccount
				app.pageList.List.Position = layout.Position{}
				app.status = "二维码已生成，请在 3 分钟内扫码并在手机端确认"
			}
		})
	})
}

func (a *App) appendLog(message string) {
	message = strings.TrimSpace(message)
	if message == "" {
		return
	}
	a.logs = append(a.logs, time.Now().Format("15:04:05")+"  "+message)
	if len(a.logs) > 200 {
		a.logs = append([]string(nil), a.logs[len(a.logs)-200:]...)
	}
}

func (a *App) fail(err error) {
	if err == nil {
		return
	}
	a.lastError, a.status = err.Error(), "操作失败"
	a.appendLog("错误：" + err.Error())
}

func (a *App) handleActions(gtx layout.Context) {
	for i := range a.nav {
		for a.nav[i].Clicked(gtx) {
			a.activeTab = i
		}
	}
	for a.cancelOp.Clicked(gtx) {
		if a.opCancel != nil {
			a.opCancel()
			a.status = "正在取消…"
		}
	}
	for a.startURL.Clicked(gtx) {
		a.selectConversionInput(false)
	}
	for a.startManual.Clicked(gtx) {
		a.selectConversionInput(true)
	}
	for index := range a.stageTabs {
		for a.stageTabs[index].Clicked(gtx) {
			a.openConvertStage(index)
		}
	}
	for a.stageBack.Clicked(gtx) {
		if a.convertStage > convertStageInput {
			a.setConvertStage(a.convertStage - 1)
		}
	}
	for a.stageNext.Clicked(gtx) {
		switch a.convertStage {
		case convertStageOptions:
			a.startConversion(a.inputManual)
		case convertStageProgress:
			if a.conversionMatched {
				a.setConvertStage(convertStageReview)
			}
		case convertStageReview:
			if unresolvedCount(a.outcomes) == 0 {
				a.setConvertStage(convertStageWrite)
			} else {
				a.status = fmt.Sprintf("仍有 %d 首歌曲需要审核或跳过", unresolvedCount(a.outcomes))
			}
		}
	}
	for a.manualSearch.Clicked(gtx) {
		a.searchCandidates()
	}
	for a.directBVID.Clicked(gtx) {
		a.lookupBVID()
	}
	for a.keepSelection.Clicked(gtx) {
		a.keepCurrent()
	}
	for a.skipSong.Clicked(gtx) {
		a.skipCurrent()
	}
	for a.undoDecision.Clicked(gtx) {
		a.undoCurrent()
	}
	for a.openVideo.Clicked(gtx) {
		if out := a.currentOutcome(); out != nil && out.Video != nil {
			_ = openURL(out.Video.URL())
		}
	}
	for a.openSource.Clicked(gtx) {
		if strings.HasPrefix(a.playlistURL.Text(), "http") {
			_ = openURL(a.playlistURL.Text())
		}
	}
	for a.loadFavorites.Clicked(gtx) {
		a.reloadFavorites()
	}
	for a.createFavorite.Clicked(gtx) {
		a.createFavoriteAction()
	}
	for a.writeResults.Clicked(gtx) {
		a.writeAction()
	}
	for a.loginButton.Clicked(gtx) {
		a.loginAction()
	}
	for a.logoutButton.Clicked(gtx) {
		a.logoutAction()
	}
	for a.resetAnon.Clicked(gtx) {
		a.resetAnonymousAction()
	}
	for a.refreshBrowser.Clicked(gtx) {
		a.browserStatusAction()
	}
	for a.installBrowser.Clicked(gtx) {
		a.browserInstallAction()
	}
	for a.clearBrowser.Clicked(gtx) {
		a.browserClearAction()
	}
	for a.refreshCache.Clicked(gtx) {
		a.cacheStatusAction()
	}
	for a.clearCache.Clicked(gtx) {
		a.cacheClearAction()
	}
	for a.openProject.Clicked(gtx) {
		_ = openURL("https://github.com/bagags/music2bb")
	}
	for a.openLicense.Clicked(gtx) {
		_ = openURL("https://github.com/bagags/music2bb/blob/dev/LICENSE.md")
	}
	for a.updateCheck.Clicked(gtx) {
		a.checkUpdateAction()
	}
	for a.openReleases.Clicked(gtx) {
		_ = openURL("https://github.com/bagags/music2bb-go/releases")
	}
}

func (a *App) selectConversionInput(manual bool) {
	if a.busy {
		return
	}
	if manual {
		if len(core.ParseManualSongs(a.manualSongs.Text())) == 0 {
			a.fail(errorsText("请至少输入一首歌曲"))
			return
		}
	} else if strings.TrimSpace(a.playlistURL.Text()) == "" {
		a.fail(errorsText("请输入在线歌单链接"))
		return
	}
	a.inputChosen, a.inputManual = true, manual
	a.conversionStarted, a.conversionMatched = false, false
	a.session, a.songs, a.outcomes, a.selectedSong = nil, nil, nil, -1
	a.telemetry, a.logs = runtimeTelemetry{}, nil
	a.lastError = ""
	if manual {
		a.status = fmt.Sprintf("已选择手工歌曲：%d 首", len(core.ParseManualSongs(a.manualSongs.Text())))
	} else {
		a.status = "已选择在线歌单，下一步配置匹配参数"
	}
	a.setConvertStage(convertStageOptions)
}

func (a *App) startConversion(manualInput bool) {
	options, err := a.readOptions()
	if err != nil {
		a.fail(err)
		return
	}
	rawURL := strings.TrimSpace(a.playlistURL.Text())
	manualText := a.manualSongs.Text()
	if manualInput && len(core.ParseManualSongs(manualText)) == 0 {
		a.fail(errorsText("请至少输入一首歌曲"))
		return
	}
	a.logs = nil
	a.logList.List.Position = layout.Position{}
	a.writeConfirm.Value = false
	a.conversionStarted, a.conversionMatched = true, false
	a.session, a.songs, a.outcomes, a.selectedSong = nil, nil, nil, -1
	a.setConvertStage(convertStageProgress)
	a.begin("正在准备转换…", func(ctx context.Context) uiMessage {
		session, err := core.NewSession(a.backend, rawURL, options)
		if err != nil {
			return func(app *App) { app.fail(err) }
		}
		var songs []music2bb.Song
		if manualInput {
			songs = core.ParseManualSongs(manualText)
		} else {
			songs, err = session.Parse(ctx, a.observer())
		}
		if err != nil && len(songs) == 0 {
			return func(app *App) { app.fail(err) }
		}
		if len(songs) == 0 {
			return func(app *App) { app.fail(errorsText("没有获取到歌曲")) }
		}
		a.post(func(app *App) {
			placeholders := make([]music2bb.MatchResult, len(songs))
			for index, song := range songs {
				placeholders[index] = music2bb.MatchResult{
					Song: song, NeedsReview: true, ReviewReason: music2bb.ReviewNotSearched,
					SearchStatus: music2bb.SearchStatusNotSearched,
				}
			}
			app.session, app.songs, app.outcomes, app.selectedSong = session, songs, placeholders, 0
			app.ensureSongClicks(len(songs))
			app.syncCandidateClicks()
			app.telemetry.beginStage("Bilibili 匹配", len(songs), time.Now())
			app.progress = 0
			app.status = fmt.Sprintf("正在匹配 0/%d：后台会按限速逐页搜索，可随时取消", len(songs))
			app.appendLog(fmt.Sprintf("开始 Bilibili 匹配：%d 首 · %d 并发 · 每首最多 %d 个远程请求 · 身份 %s · 策略 %s",
				len(songs), options.Workers, options.SearchBudget, options.Identity, options.Profile))
		})
		outcomes, matchErr := session.Match(ctx, songs, a.observer())
		return func(app *App) {
			app.session, app.songs, app.outcomes = session, songs, outcomes
			app.conversionMatched = true
			if options.ReviewAll {
				for i := range app.outcomes {
					if app.outcomes[i].HasSelection {
						app.outcomes[i].NeedsReview = true
					}
				}
			}
			app.ensureSongClicks(len(songs))
			app.syncCandidateClicks()
			app.progress = 1
			if matchErr != nil {
				app.lastError = matchErr.Error()
				app.appendLog("部分匹配未完成：" + matchErr.Error())
			}
			app.status = fmt.Sprintf("匹配完成：%d/%d 已选择，%d 待处理", selectedCount(outcomes), len(outcomes), unresolvedCount(app.outcomes))
			app.setConvertStage(convertStageReview)
		}
	})
}

func (a *App) readOptions() (core.Options, error) {
	defaults := core.DefaultOptions()
	var err error
	if defaults.SearchPages, err = positiveInt(a.searchPages.Text()); err != nil {
		return defaults, fmt.Errorf("搜索页数: %w", err)
	}
	if defaults.TopK, err = positiveInt(a.topK.Text()); err != nil {
		return defaults, fmt.Errorf("候选数: %w", err)
	}
	if defaults.Workers, err = positiveInt(a.workers.Text()); err != nil {
		return defaults, fmt.Errorf("并发数: %w", err)
	}
	if defaults.SearchBudget, err = positiveInt(a.searchBudget.Text()); err != nil {
		return defaults, fmt.Errorf("请求预算: %w", err)
	}
	defaults.Profile = music2bb.MatchProfile(a.profile.Value)
	defaults.Identity = a.identity.Value
	defaults.BrowserPolicy = music2bb.BrowserPolicy(a.browserPolicy.Value)
	defaults.Manual, defaults.ReviewAll, defaults.AllowQR = a.manualMode.Value, a.reviewAll.Value, a.allowQR.Value
	defaults.Fresh, defaults.RefreshSearch = a.fresh.Value, a.refreshSearch.Value
	if a.customWeights.Value {
		values := []*widget.Editor{&a.weightTitle, &a.weightArtist, &a.weightQuality, &a.weightOfficial, &a.weightPopularity, &a.weightUploader}
		numbers := make([]float64, len(values))
		for i, editor := range values {
			if numbers[i], err = strconv.ParseFloat(strings.TrimSpace(editor.Text()), 64); err != nil {
				return defaults, errorsText("权重必须是数字")
			}
		}
		defaults.CustomWeights = &music2bb.MatchWeights{Title: numbers[0], Artist: numbers[1], Quality: numbers[2], Official: numbers[3], Popularity: numbers[4], Uploader: numbers[5]}
	}
	return defaults, defaults.Validate()
}

func positiveInt(text string) (int, error) {
	value, err := strconv.Atoi(strings.TrimSpace(text))
	if err != nil || value < 1 {
		return 0, errorsText("必须是大于 0 的整数")
	}
	return value, nil
}

type textError string

func (e textError) Error() string   { return string(e) }
func errorsText(value string) error { return textError(value) }

func selectedCount(outcomes []music2bb.MatchResult) int {
	count := 0
	for _, out := range outcomes {
		if out.HasSelection {
			count++
		}
	}
	return count
}
func unresolvedCount(outcomes []music2bb.MatchResult) int {
	count := 0
	for _, out := range outcomes {
		if out.NeedsReview || (!out.HasSelection && out.SearchStatus != music2bb.SearchStatusCompleted) {
			count++
		}
	}
	return count
}

func (a *App) currentOutcome() *music2bb.MatchResult {
	if a.selectedSong < 0 || a.selectedSong >= len(a.outcomes) {
		return nil
	}
	return &a.outcomes[a.selectedSong]
}

func (a *App) ensureSongClicks(length int) {
	if len(a.songClicks) != length {
		a.songClicks = make([]widget.Clickable, length)
	}
}
func (a *App) syncCandidateClicks() {
	out := a.currentOutcome()
	length := 0
	if out != nil {
		length = len(out.Candidates)
	}
	if len(a.candidateClicks) != length {
		a.candidateClicks = make([]widget.Clickable, length)
	}
}

func (a *App) searchCandidates() {
	out := a.currentOutcome()
	if a.session == nil || out == nil {
		return
	}
	song := out.Song
	query := strings.TrimSpace(a.searchQuery.Text())
	if query == "" {
		query = song.SearchKeywordFull()
		a.searchQuery.SetText(query)
	}
	a.begin("正在手动搜索…", func(ctx context.Context) uiMessage {
		candidates, err := a.session.Search(ctx, song, query)
		return func(app *App) {
			if err != nil {
				app.fail(err)
				return
			}
			if current := app.currentOutcome(); current != nil && current.Song.StableSourceID() == song.StableSourceID() {
				current.Candidates = candidates
				current.NeedsReview = true
				app.syncCandidateClicks()
				app.status = fmt.Sprintf("找到 %d 个候选", len(candidates))
			}
		}
	})
}

func (a *App) lookupBVID() {
	out := a.currentOutcome()
	if a.session == nil || out == nil {
		return
	}
	bvid, song := strings.TrimSpace(a.bvidInput.Text()), out.Song
	if bvid == "" {
		a.fail(errorsText("请输入 BVID"))
		return
	}
	a.begin("正在读取视频…", func(ctx context.Context) uiMessage {
		video, err := a.session.VideoDetail(ctx, bvid)
		return func(app *App) {
			if err != nil {
				app.fail(err)
				return
			}
			if current := app.currentOutcome(); current != nil && current.Song.StableSourceID() == song.StableSourceID() {
				app.selectVideo(*current, music2bb.MatchResult{Song: song, Video: &video, Score: 999, HasSelection: true, Matched: true})
			}
		}
	})
}

func (a *App) selectVideo(original music2bb.MatchResult, candidate music2bb.MatchResult) {
	if a.session == nil || a.currentOutcome() == nil {
		return
	}
	candidate.Song, candidate.Candidates = original.Song, original.Candidates
	candidate.HasSelection, candidate.Matched, candidate.ManualOverride = candidate.Video != nil, candidate.Video != nil, candidate.Video != nil
	candidate.NeedsReview, candidate.ReviewReason, candidate.SearchStatus = false, music2bb.ReviewNone, music2bb.SearchStatusCompleted
	*a.currentOutcome() = candidate
	if err := a.session.RecordDecision(candidate, false); err != nil {
		a.fail(err)
		return
	}
	a.status = "已保存人工选择"
}

func (a *App) keepCurrent() {
	out := a.currentOutcome()
	if out == nil || out.Video == nil || a.session == nil {
		return
	}
	out.NeedsReview, out.ReviewReason, out.SearchStatus = false, music2bb.ReviewNone, music2bb.SearchStatusCompleted
	out.HasSelection, out.Matched = true, true
	if err := a.session.RecordDecision(*out, false); err != nil {
		a.fail(err)
		return
	}
	a.status = "已确认推荐结果"
}

func (a *App) skipCurrent() {
	out := a.currentOutcome()
	if out == nil || a.session == nil {
		return
	}
	out.Video, out.HasSelection, out.Matched, out.NeedsReview = nil, false, false, false
	out.ReviewReason, out.SearchStatus = music2bb.ReviewNone, music2bb.SearchStatusCompleted
	if err := a.session.RecordDecision(*out, true); err != nil {
		a.fail(err)
		return
	}
	a.status = "已跳过歌曲"
}

func (a *App) undoCurrent() {
	out := a.currentOutcome()
	if out == nil || a.session == nil {
		return
	}
	if err := a.session.ClearDecision(out.Song); err != nil {
		a.fail(err)
		return
	}
	out.Video, out.HasSelection, out.Matched, out.ManualOverride = nil, false, false, false
	out.NeedsReview, out.ReviewReason, out.SearchStatus = true, music2bb.ReviewNotSearched, music2bb.SearchStatusNotSearched
	a.status = "已撤销人工决定，可重新搜索"
}

func (a *App) reloadFavorites() {
	if a.session == nil {
		options := core.DefaultOptions()
		options.AllowQR = a.allowQR.Value
		a.session, _ = core.NewSession(a.backend, "desktop:favorites", options)
	}
	a.begin("正在读取收藏夹…", func(ctx context.Context) uiMessage {
		favorites, err := a.session.ListFavorites(ctx, a.observer())
		return func(app *App) {
			if err != nil {
				app.fail(err)
				return
			}
			app.favorites = favorites
			app.favoriteClicks = make([]widget.Clickable, len(favorites))
			app.status = fmt.Sprintf("已读取 %d 个收藏夹", len(favorites))
		}
	})
}

func (a *App) createFavoriteAction() {
	title, intro, public := strings.TrimSpace(a.favoriteTitle.Text()), a.favoriteIntro.Text(), a.favoritePublic.Value
	if title == "" {
		a.fail(errorsText("收藏夹名称不能为空"))
		return
	}
	if a.session == nil {
		options := core.DefaultOptions()
		a.session, _ = core.NewSession(a.backend, "desktop:favorites", options)
	}
	a.begin("正在创建收藏夹…", func(ctx context.Context) uiMessage {
		favorite, err := a.session.CreateFavorite(ctx, music2bb.CreateFavoriteRequest{Title: title, Intro: intro, Private: !public}, a.observer())
		return func(app *App) {
			if err != nil {
				app.fail(err)
				return
			}
			app.favorites = append(app.favorites, favorite)
			app.favoriteClicks = make([]widget.Clickable, len(app.favorites))
			app.selectedFavorite = favorite.ID
			app.favoriteTitle.SetText("")
			app.favoriteIntro.SetText("")
			app.status = "收藏夹已创建"
		}
	})
}

func (a *App) writeAction() {
	if a.session == nil || a.selectedFavorite <= 0 || len(a.outcomes) == 0 {
		return
	}
	if !a.writeConfirm.Value {
		a.fail(errorsText("请先勾选写入确认"))
		return
	}
	favoriteID := a.selectedFavorite
	outcomes := append([]music2bb.MatchResult(nil), a.outcomes...)
	a.begin("正在写入收藏夹…", func(ctx context.Context) uiMessage {
		result, err := a.session.Write(ctx, favoriteID, outcomes, a.observer())
		return func(app *App) {
			if err != nil {
				app.fail(err)
			}
			app.status = fmt.Sprintf("写入完成：成功 %d，失败 %d，已存在 %d", len(result.Succeeded), len(result.Failed), len(result.Skipped))
			app.appendLog(app.status)
			app.writeConfirm.Value = false
		}
	})
}

func (a *App) loginAction() {
	options := core.DefaultOptions()
	options.AllowQR = a.allowQR.Value
	session, _ := core.NewSession(a.backend, "desktop:account", options)
	a.begin("正在登录…", func(ctx context.Context) uiMessage {
		account, err := session.Login(ctx, a.observer())
		return func(app *App) {
			if err != nil {
				app.fail(err)
				return
			}
			app.account = &account
			app.qrImage = nil
			app.status = "已登录：" + account.Name
		}
	})
}

func (a *App) logoutAction() {
	a.begin("正在退出登录…", func(ctx context.Context) uiMessage {
		err := a.backend.Logout(ctx)
		return func(app *App) {
			if err != nil {
				app.fail(err)
				return
			}
			app.account, app.session, app.qrImage = nil, nil, nil
			app.status = "已清除本地登录状态"
		}
	})
}

func (a *App) resetAnonymousAction() {
	a.begin("正在重置匿名身份…", func(ctx context.Context) uiMessage {
		err := a.backend.ResetAnonymousIdentity(ctx)
		return func(app *App) {
			if err != nil {
				app.fail(err)
				return
			}
			app.status = "匿名设备状态已重置"
		}
	})
}

func (a *App) browserStatusAction() {
	a.begin("正在读取 Chromium 状态…", func(ctx context.Context) uiMessage {
		status, err := a.backend.Browser().Status(ctx)
		return func(app *App) {
			if err != nil {
				app.fail(err)
				return
			}
			app.browserStatus, app.browserLoaded, app.status = status, true, "Chromium 状态已更新"
		}
	})
}

func (a *App) browserInstallAction() {
	a.begin("正在安装并校验 Chromium…", func(ctx context.Context) uiMessage {
		status, err := a.backend.Browser().Install(ctx, true)
		return func(app *App) {
			if err != nil {
				app.fail(err)
				return
			}
			app.browserStatus, app.browserLoaded, app.status = status, true, "Chromium 已安装并校验"
		}
	})
}

func (a *App) browserClearAction() {
	a.begin("正在清除 Chromium…", func(ctx context.Context) uiMessage {
		err := a.backend.Browser().Clear(ctx)
		return func(app *App) {
			if err != nil {
				app.fail(err)
				return
			}
			app.browserLoaded = false
			app.status = "Chromium 缓存已清除"
		}
	})
}

func (a *App) checkUpdateAction() {
	if a.releaseChecker == nil {
		a.fail(errorsText("当前构建不支持更新检查"))
		return
	}
	a.begin("正在检查更新…", func(ctx context.Context) uiMessage {
		info, err := a.releaseChecker.Check(ctx)
		return func(app *App) {
			if err != nil {
				app.fail(err)
				return
			}
			app.updateInfo = &info
			if info.Available {
				app.status = fmt.Sprintf("发现新版本：%s", info.LatestVersion)
			} else {
				app.status = "已是最新版本"
			}
		}
	})
}

func (a *App) cacheStatusAction() {
	configDir, cacheDir := a.backend.PersistentStatePaths()
	paths := state.CachePaths(configDir, cacheDir)
	a.begin("正在统计状态文件…", func(context.Context) uiMessage {
		statuses := make(map[state.CacheKind]state.CacheStatus)
		for kind, target := range paths {
			status, err := state.Inspect(target)
			if err != nil {
				return func(app *App) { app.fail(err) }
			}
			statuses[kind] = status
		}
		return func(app *App) { app.cacheStatuses, app.status = statuses, "状态统计已更新" }
	})
}

func (a *App) cacheClearAction() {
	configDir, cacheDir := a.backend.PersistentStatePaths()
	paths := state.CachePaths(configDir, cacheDir)
	selected := make([]state.CacheKind, 0)
	for kind, checked := range a.cacheChecks {
		if checked.Value {
			selected = append(selected, kind)
		}
	}
	if len(selected) == 0 {
		a.fail(errorsText("请先选择要清理的状态"))
		return
	}
	a.begin("正在清理所选状态…", func(ctx context.Context) uiMessage {
		for _, kind := range selected {
			if kind == state.CacheAnonymous {
				if err := a.backend.ResetAnonymousIdentity(ctx); err != nil {
					return func(app *App) { app.fail(err) }
				}
			}
			if err := state.Clear(paths[kind]); err != nil {
				return func(app *App) { app.fail(err) }
			}
		}
		return func(app *App) {
			for _, kind := range selected {
				app.cacheChecks[kind].Value = false
				app.cacheStatuses[kind] = state.CacheStatus{}
			}
			app.status = "所选状态已清理（账号 Cookie 未受影响）"
		}
	})
}

func rgba(value uint32) color.NRGBA {
	return color.NRGBA{R: uint8(value >> 24), G: uint8(value >> 16), B: uint8(value >> 8), A: uint8(value)}
}
