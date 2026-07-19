package ui

import (
	"testing"

	music2bb "github.com/bagags/music2bb-go"
)

func TestConvertStagesUnlockInOrder(t *testing.T) {
	app := &App{}
	if got := app.maxUnlockedConvertStage(); got != convertStageInput {
		t.Fatalf("initial max stage = %d", got)
	}
	if app.setConvertStage(convertStageOptions) {
		t.Fatal("options opened before input was selected")
	}

	app.inputChosen = true
	if got := app.maxUnlockedConvertStage(); got != convertStageOptions {
		t.Fatalf("selected-input max stage = %d", got)
	}
	app.conversionStarted = true
	if got := app.maxUnlockedConvertStage(); got != convertStageProgress {
		t.Fatalf("started max stage = %d", got)
	}
	app.conversionMatched = true
	if got := app.maxUnlockedConvertStage(); got != convertStageReview {
		t.Fatalf("matched-empty max stage = %d", got)
	}
	app.outcomes = []music2bb.MatchResult{{}}
	if got := app.maxUnlockedConvertStage(); got != convertStageWrite {
		t.Fatalf("matched max stage = %d", got)
	}
}

func TestSetConvertStageResetsOnlyStageScroll(t *testing.T) {
	app := &App{inputChosen: true, convertStage: convertStageInput}
	app.stageList.List.Position.First = 5
	app.songList.List.Position.First = 3
	if !app.setConvertStage(convertStageOptions) {
		t.Fatal("setConvertStage returned false")
	}
	if app.stageList.List.Position.First != 0 {
		t.Fatalf("stage scroll was not reset: %#v", app.stageList.List.Position)
	}
	if app.songList.List.Position.First != 3 {
		t.Fatal("review song position should be preserved")
	}
}
