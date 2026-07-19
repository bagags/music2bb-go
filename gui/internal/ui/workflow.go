package ui

import "gioui.org/layout"

const (
	convertStageInput = iota
	convertStageOptions
	convertStageProgress
	convertStageReview
	convertStageWrite
)

var convertStageLabels = [...]string{"导入", "设置", "进度", "审核", "写入"}

func (a *App) maxUnlockedConvertStage() int {
	max := convertStageInput
	if a.inputChosen {
		max = convertStageOptions
	}
	if a.conversionStarted {
		max = convertStageProgress
	}
	if a.conversionMatched {
		max = convertStageReview
		if len(a.outcomes) > 0 {
			max = convertStageWrite
		}
	}
	return max
}

func (a *App) setConvertStage(stage int) bool {
	if stage < convertStageInput || stage > a.maxUnlockedConvertStage() {
		return false
	}
	a.convertStage = stage
	a.stageList.List.Position = layout.Position{}
	return true
}

func (a *App) openConvertStage(stage int) {
	if a.setConvertStage(stage) {
		return
	}
	a.status = "请先完成前一阶段"
}
