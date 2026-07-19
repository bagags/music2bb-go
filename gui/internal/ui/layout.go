package ui

import (
	"fmt"
	"image"
	"image/color"
	"strings"
	"time"

	"gioui.org/layout"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/text"
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"
)

var (
	colorPanel  = color.NRGBA{R: 22, G: 27, B: 34, A: 255}
	colorPanel2 = color.NRGBA{R: 33, G: 38, B: 45, A: 255}
	colorBorder = color.NRGBA{R: 48, G: 54, B: 61, A: 255}
	colorMuted  = color.NRGBA{R: 139, G: 148, B: 158, A: 255}
	colorAccent = color.NRGBA{R: 63, G: 185, B: 80, A: 255}
	colorWarn   = color.NRGBA{R: 210, G: 153, B: 34, A: 255}
	colorDanger = color.NRGBA{R: 248, G: 81, B: 73, A: 255}
)

func (a *App) layout(gtx layout.Context) layout.Dimensions {
	paint.Fill(gtx.Ops, a.theme.Palette.Bg)
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(a.layoutHeader),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			if !a.busy && a.lastError == "" {
				return layout.Dimensions{}
			}
			return a.layoutOperationBanner(gtx)
		}),
		layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
			if gtx.Constraints.Max.X >= gtx.Dp(unit.Dp(960)) {
				return layout.Flex{Axis: layout.Horizontal}.Layout(gtx,
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						gtx.Constraints.Min.X = gtx.Dp(unit.Dp(190))
						gtx.Constraints.Max.X = gtx.Constraints.Min.X
						return a.layoutSideNav(gtx)
					}),
					layout.Flexed(1, a.layoutPage),
				)
			}
			return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
				layout.Rigid(a.layoutTopNav),
				layout.Flexed(1, a.layoutPage),
			)
		}),
		layout.Rigid(a.layoutStatusBar),
	)
}

func (a *App) layoutOperationBanner(gtx layout.Context) layout.Dimensions {
	now := time.Now()
	stage := a.telemetry.Stage
	if stage == "" {
		stage = a.operation
	}
	progress := a.telemetry.fraction()
	if a.telemetry.Total == 0 {
		progress = a.progress
	}
	return background(gtx, colorPanel2, func(gtx layout.Context) layout.Dimensions {
		return layout.Inset{Top: unit.Dp(10), Right: unit.Dp(16), Bottom: unit.Dp(10), Left: unit.Dp(16)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
						layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
							title := stage
							if a.telemetry.Total > 0 {
								title = fmt.Sprintf("%s  %d/%d", stage, a.telemetry.Current, a.telemetry.Total)
							}
							label := material.Body1(a.theme, title)
							label.Color = colorAccent
							return label.Layout(gtx)
						}),
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							label := material.Caption(a.theme, fmt.Sprintf("已用时 %s · 距上次进展 %s", a.telemetry.elapsed(now), a.telemetry.quietFor(now)))
							label.Color = colorMuted
							return label.Layout(gtx)
						}),
						layout.Rigid(spacerH(10)),
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							if !a.busy {
								return layout.Dimensions{}
							}
							return material.Button(a.theme, &a.cancelOp, "取消").Layout(gtx)
						}),
					)
				}),
				layout.Rigid(spacer(6)),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					bar := material.ProgressBar(a.theme, progress)
					bar.Color = colorAccent
					bar.TrackColor = colorBorder
					return bar.Layout(gtx)
				}),
				layout.Rigid(spacer(5)),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					current := a.status
					if a.telemetry.CurrentSong != "" {
						current = "最近完成：" + a.telemetry.CurrentSong
					}
					if a.busy && a.telemetry.quietFor(now) >= 15*time.Second {
						current += " · 正在等待 Bilibili 响应或限速调度，界面仍可取消"
					}
					label := material.Caption(a.theme, current)
					if a.busy && a.telemetry.quietFor(now) >= 15*time.Second {
						label.Color = colorWarn
					} else {
						label.Color = colorMuted
					}
					label.MaxLines = 2
					return label.Layout(gtx)
				}),
				layout.Rigid(spacer(4)),
				layout.Rigid(a.layoutTelemetryMetrics),
			)
		})
	})
}

func (a *App) layoutTelemetryMetrics(gtx layout.Context) layout.Dimensions {
	values := []string{
		fmt.Sprintf("完成 %d", a.telemetry.Completed),
		fmt.Sprintf("已选 %d", a.telemetry.Selected),
		fmt.Sprintf("待审 %d", a.telemetry.NeedsReview),
		fmt.Sprintf("失败 %d", a.telemetry.Failed),
		fmt.Sprintf("远程请求 %d", a.telemetry.RemoteRequests),
		fmt.Sprintf("缓存命中 %d", a.telemetry.CacheHits),
	}
	children := make([]layout.FlexChild, 0, len(values)*2)
	for index, value := range values {
		value := value
		if index > 0 {
			children = append(children, layout.Rigid(spacerH(16)))
		}
		children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			label := material.Caption(a.theme, value)
			label.Color = colorMuted
			return label.Layout(gtx)
		}))
	}
	return layout.Flex{}.Layout(gtx, children...)
}

func (a *App) layoutHeader(gtx layout.Context) layout.Dimensions {
	return background(gtx, colorPanel, func(gtx layout.Context) layout.Dimensions {
		return layout.UniformInset(unit.Dp(16)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
				layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
					return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							title := material.H5(a.theme, "music2bb Desktop")
							title.Color = colorAccent
							return title.Layout(gtx)
						}),
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							label := material.Caption(a.theme, "在线歌单 → Bilibili 收藏夹 · 原生跨平台桌面工作区")
							label.Color = colorMuted
							return label.Layout(gtx)
						}),
					)
				}),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					if a.busy {
						return material.Loader(a.theme).Layout(gtx)
					}
					label := material.Caption(a.theme, "v"+a.version)
					label.Color = colorMuted
					return label.Layout(gtx)
				}),
			)
		})
	})
}

func (a *App) layoutSideNav(gtx layout.Context) layout.Dimensions {
	return background(gtx, colorPanel, func(gtx layout.Context) layout.Dimensions {
		return layout.Inset{Top: unit.Dp(12), Right: unit.Dp(10), Bottom: unit.Dp(12), Left: unit.Dp(10)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			children := make([]layout.FlexChild, 0, 6)
			for i, title := range []string{"转换工作区", "账号", "收藏夹", "Chromium", "缓存与恢复", "关于"} {
				i, title := i, title
				children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions { return a.navButton(gtx, i, title) }))
			}
			return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
		})
	})
}

func (a *App) layoutTopNav(gtx layout.Context) layout.Dimensions {
	return background(gtx, colorPanel, func(gtx layout.Context) layout.Dimensions {
		children := make([]layout.FlexChild, 0, 6)
		for i, title := range []string{"转换", "账号", "收藏夹", "浏览器", "缓存", "关于"} {
			i, title := i, title
			children = append(children, layout.Flexed(1, func(gtx layout.Context) layout.Dimensions { return a.navButton(gtx, i, title) }))
		}
		return layout.Flex{}.Layout(gtx, children...)
	})
}

func (a *App) navButton(gtx layout.Context, index int, title string) layout.Dimensions {
	button := material.Button(a.theme, &a.nav[index], title)
	button.Background = colorPanel
	if a.activeTab == index {
		button.Background = colorPanel2
		button.Color = colorAccent
	}
	button.Inset = layout.UniformInset(unit.Dp(10))
	return button.Layout(gtx)
}

func (a *App) layoutPage(gtx layout.Context) layout.Dimensions {
	gtx.Constraints.Min = image.Point{}
	return layout.UniformInset(unit.Dp(14)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		switch a.activeTab {
		case tabAccount:
			return a.layoutAccount(gtx)
		case tabFavorites:
			return a.layoutFavorites(gtx)
		case tabBrowser:
			return a.layoutBrowser(gtx)
		case tabStorage:
			return a.layoutStorage(gtx)
		case tabAbout:
			return a.layoutAbout(gtx)
		default:
			return a.layoutConvert(gtx)
		}
	})
}

func (a *App) layoutStatusBar(gtx layout.Context) layout.Dimensions {
	return background(gtx, colorPanel, func(gtx layout.Context) layout.Dimensions {
		return layout.Inset{Top: unit.Dp(7), Right: unit.Dp(14), Bottom: unit.Dp(7), Left: unit.Dp(14)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
				layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
					text := a.status
					if a.lastError != "" {
						text += " · " + a.lastError
					}
					label := material.Caption(a.theme, text)
					if a.lastError != "" {
						label.Color = colorDanger
					} else {
						label.Color = colorMuted
					}
					label.MaxLines = 1
					return label.Layout(gtx)
				}),
			)
		})
	})
}

func background(gtx layout.Context, bg color.NRGBA, content layout.Widget) layout.Dimensions {
	return layout.Stack{}.Layout(gtx,
		layout.Expanded(func(gtx layout.Context) layout.Dimensions {
			paint.FillShape(gtx.Ops, bg, clip.Rect{Max: gtx.Constraints.Min}.Op())
			return layout.Dimensions{Size: gtx.Constraints.Min}
		}),
		layout.Stacked(content),
	)
}

func (a *App) card(gtx layout.Context, content layout.Widget) layout.Dimensions {
	return background(gtx, colorPanel, func(gtx layout.Context) layout.Dimensions {
		return layout.UniformInset(unit.Dp(14)).Layout(gtx, content)
	})
}

func (a *App) sectionTitle(gtx layout.Context, title, subtitle string) layout.Dimensions {
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			label := material.H6(a.theme, title)
			label.Color = colorAccent
			return label.Layout(gtx)
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			if subtitle == "" {
				return layout.Dimensions{}
			}
			label := material.Caption(a.theme, subtitle)
			label.Color = colorMuted
			return label.Layout(gtx)
		}),
	)
}

func (a *App) editor(gtx layout.Context, editor *widget.Editor, hint string) layout.Dimensions {
	return a.card(gtx, func(gtx layout.Context) layout.Dimensions {
		style := material.Editor(a.theme, editor, hint)
		return style.Layout(gtx)
	})
}

func (a *App) label(gtx layout.Context, value string, muted bool) layout.Dimensions {
	style := material.Body2(a.theme, value)
	if muted {
		style.Color = colorMuted
	}
	return style.Layout(gtx)
}

func (a *App) metric(gtx layout.Context, name, value string) layout.Dimensions {
	return layout.Flex{}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			l := material.Caption(a.theme, name+"  ")
			l.Color = colorMuted
			return l.Layout(gtx)
		}),
		layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
			l := material.Caption(a.theme, value)
			l.Alignment = text.End
			return l.Layout(gtx)
		}),
	)
}

func (a *App) logsText() string {
	if len(a.logs) == 0 {
		return "尚无运行日志"
	}
	return strings.Join(a.logs, "\n")
}

func (a *App) recentLogs(limit int) []string {
	if limit <= 0 || len(a.logs) == 0 {
		return nil
	}
	start := len(a.logs) - limit
	if start < 0 {
		start = 0
	}
	return a.logs[start:]
}

func formatBytes(bytes int64) string {
	if bytes < 1024 {
		return fmt.Sprintf("%d B", bytes)
	}
	if bytes < 1024*1024 {
		return fmt.Sprintf("%.1f KiB", float64(bytes)/1024)
	}
	return fmt.Sprintf("%.1f MiB", float64(bytes)/(1024*1024))
}
