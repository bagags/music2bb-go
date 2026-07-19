package ui

import (
	"fmt"
	"image"
	"strings"

	"gioui.org/layout"
	"gioui.org/text"
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"
	music2bb "github.com/bagags/music2bb-go"
	"github.com/bagags/music2bb-go/m2bb-gui/internal/core"
)

func (a *App) layoutConvert(gtx layout.Context) layout.Dimensions {
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return a.sectionTitle(gtx, "转换工作区", "按阶段导入、匹配、审核并写入 Bilibili 收藏夹")
		}),
		layout.Rigid(spacer(9)),
		layout.Rigid(a.layoutConvertStages),
		layout.Rigid(spacer(10)),
		layout.Flexed(1, a.layoutConvertStage),
	)
}

func (a *App) layoutConvertStages(gtx layout.Context) layout.Dimensions {
	maxUnlocked := a.maxUnlockedConvertStage()
	children := make([]layout.FlexChild, 0, len(convertStageLabels)*2-1)
	for index, title := range convertStageLabels {
		index, title := index, title
		if index > 0 {
			children = append(children, layout.Rigid(spacerH(7)))
		}
		children = append(children, layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
			style := material.Button(a.theme, &a.stageTabs[index], fmt.Sprintf("%d  %s", index+1, title))
			style.Inset = layout.Inset{Top: unit.Dp(9), Right: unit.Dp(8), Bottom: unit.Dp(9), Left: unit.Dp(8)}
			switch {
			case index == a.convertStage:
				style.Background, style.Color = colorAccent, a.theme.Palette.ContrastFg
			case index <= maxUnlocked:
				style.Background, style.Color = colorPanel2, a.theme.Palette.Fg
			default:
				style.Background, style.Color = colorPanel, colorMuted
			}
			return style.Layout(gtx)
		}))
	}
	return layout.Flex{}.Layout(gtx, children...)
}

func (a *App) layoutConvertStage(gtx layout.Context) layout.Dimensions {
	switch a.convertStage {
	case convertStageOptions:
		return a.layoutOptionsStage(gtx)
	case convertStageProgress:
		return a.layoutProgressStage(gtx)
	case convertStageReview:
		return a.layoutReviewStage(gtx)
	case convertStageWrite:
		return a.layoutWriteStage(gtx)
	default:
		return a.layoutSimpleConvertStage(gtx, a.layoutConversionInput)
	}
}

func (a *App) layoutSimpleConvertStage(gtx layout.Context, content layout.Widget) layout.Dimensions {
	return a.stageList.Layout(gtx, 1, func(gtx layout.Context, _ int) layout.Dimensions {
		return content(gtx)
	})
}

func (a *App) layoutOptionsStage(gtx layout.Context) layout.Dimensions {
	return a.stageList.Layout(gtx, 1, func(gtx layout.Context, _ int) layout.Dimensions {
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			layout.Rigid(a.layoutConversionOptions),
			layout.Rigid(spacer(10)),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return a.layoutStageActions(gtx, "返回导入", "开始解析并匹配", true)
			}),
		)
	})
}

func (a *App) layoutProgressStage(gtx layout.Context) layout.Dimensions {
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(a.layoutRealtimeActivity),
		layout.Rigid(spacer(10)),
		layout.Flexed(1, a.layoutLogPanel),
		layout.Rigid(spacer(9)),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return a.layoutStageActions(gtx, "返回设置", "进入审核", a.conversionMatched)
		}),
	)
}

func (a *App) layoutReviewStage(gtx layout.Context) layout.Dimensions {
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
			gtx.Constraints.Min = gtx.Constraints.Max
			return a.layoutReviewWorkspace(gtx)
		}),
		layout.Rigid(spacer(9)),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return a.layoutStageActions(gtx, "查看进度与日志", fmt.Sprintf("继续写入（待处理 %d）", unresolvedCount(a.outcomes)), unresolvedCount(a.outcomes) == 0)
		}),
	)
}

func (a *App) layoutWriteStage(gtx layout.Context) layout.Dimensions {
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
			gtx.Constraints.Min = gtx.Constraints.Max
			return a.layoutWritePanel(gtx)
		}),
		layout.Rigid(spacer(9)),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return a.layoutStageActions(gtx, "返回审核", "", false)
		}),
	)
}

func (a *App) layoutStageActions(gtx layout.Context, backLabel, nextLabel string, nextEnabled bool) layout.Dimensions {
	return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			style := material.Button(a.theme, &a.stageBack, backLabel)
			style.Background = colorPanel2
			return style.Layout(gtx)
		}),
		layout.Flexed(1, func(gtx layout.Context) layout.Dimensions { return layout.Dimensions{} }),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			if nextLabel == "" {
				return layout.Dimensions{}
			}
			style := material.Button(a.theme, &a.stageNext, nextLabel)
			if !nextEnabled {
				style.Background, style.Color = colorBorder, colorMuted
			}
			return style.Layout(gtx)
		}),
	)
}

func (a *App) layoutRealtimeActivity(gtx layout.Context) layout.Dimensions {
	return a.card(gtx, func(gtx layout.Context) layout.Dimensions {
		stage := a.telemetry.Stage
		if stage == "" {
			stage = "等待开始"
		}
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return a.sectionTitle(gtx, "实时活动", "阶段、单曲完成事件和远程/缓存统计会持续保留在这里")
			}),
			layout.Rigid(spacer(7)),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return a.label(gtx, fmt.Sprintf("%s · %d/%d · 最近完成 %s", stage, a.telemetry.Current, a.telemetry.Total, firstNonEmpty(a.telemetry.CurrentSong, "—")), false)
			}),
			layout.Rigid(spacer(5)),
			layout.Rigid(a.layoutTelemetryMetrics),
			layout.Rigid(spacer(6)),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return a.label(gtx, firstNonEmpty(a.status, "等待开始"), true)
			}),
		)
	})
}

func (a *App) layoutConversionInput(gtx layout.Context) layout.Dimensions {
	return a.card(gtx, func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			layout.Rigid(func(gtx layout.Context) layout.Dimensions { return a.label(gtx, "在线歌单链接", true) }),
			layout.Rigid(spacer(5)),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
					layout.Flexed(1, func(gtx layout.Context) layout.Dimensions { return a.editor(gtx, &a.playlistURL, "https://…") }),
					layout.Rigid(spacerH(8)),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return material.Button(a.theme, &a.startURL, "使用链接，继续设置").Layout(gtx)
					}),
				)
			}),
			layout.Rigid(spacer(10)),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return a.label(gtx, "解析失败时，也可直接输入歌曲（每行：歌名 - 歌手）", true)
			}),
			layout.Rigid(spacer(5)),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				gtx.Constraints.Min.Y = gtx.Dp(unit.Dp(90))
				return a.editor(gtx, &a.manualSongs, "Song - Artist")
			}),
			layout.Rigid(spacer(6)),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return material.Button(a.theme, &a.startManual, "使用手工歌曲，继续设置").Layout(gtx)
			}),
		)
	})
}

func (a *App) layoutConversionOptions(gtx layout.Context) layout.Dimensions {
	return a.card(gtx, func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return a.sectionTitle(gtx, "匹配设置", "与 CLI 默认值一致；高级权重可按本次转换覆盖")
			}),
			layout.Rigid(spacer(8)),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{Spacing: layout.SpaceBetween}.Layout(gtx,
					layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
						return a.compactEditor(gtx, &a.searchPages, "每查询页数")
					}),
					layout.Rigid(spacerH(8)), layout.Flexed(1, func(gtx layout.Context) layout.Dimensions { return a.compactEditor(gtx, &a.topK, "保留候选") }),
					layout.Rigid(spacerH(8)), layout.Flexed(1, func(gtx layout.Context) layout.Dimensions { return a.compactEditor(gtx, &a.workers, "并发歌曲") }),
					layout.Rigid(spacerH(8)), layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
						return a.compactEditor(gtx, &a.searchBudget, "每首预算")
					}),
				)
			}),
			layout.Rigid(spacer(8)),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{Axis: layout.Horizontal}.Layout(gtx,
					layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
						return a.enumGroup(gtx, "匹配策略", &a.profile, []enumOption{{"standard", "标准（歌手导向）"}, {"classical", "古典（作品导向）"}})
					}),
					layout.Rigid(spacerH(14)),
					layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
						return a.enumGroup(gtx, "搜索身份", &a.identity, []enumOption{{"auto", "自动回退"}, {"anonymous", "匿名"}, {"session", "登录态"}})
					}),
					layout.Rigid(spacerH(14)),
					layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
						return a.enumGroup(gtx, "浏览器回退", &a.browserPolicy, []enumOption{{"auto", "自动"}, {"never", "禁用"}, {"always", "要求可用"}})
					}),
				)
			}),
			layout.Rigid(spacer(6)),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{}.Layout(gtx,
					layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
						return material.CheckBox(a.theme, &a.manualMode, "完全手动匹配").Layout(gtx)
					}),
					layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
						return material.CheckBox(a.theme, &a.reviewAll, "审核全部推荐").Layout(gtx)
					}),
					layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
						return material.CheckBox(a.theme, &a.allowQR, "允许二维码登录").Layout(gtx)
					}),
				)
			}),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{}.Layout(gtx,
					layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
						return material.CheckBox(a.theme, &a.fresh, "忽略 checkpoint/历史决定").Layout(gtx)
					}),
					layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
						style := material.CheckBox(a.theme, &a.refreshSearch, "刷新搜索缓存并重置匿名身份")
						return style.Layout(gtx)
					}),
					layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
						return material.CheckBox(a.theme, &a.customWeights, "使用自定义权重").Layout(gtx)
					}),
				)
			}),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				if !a.customWeights.Value {
					return layout.Dimensions{}
				}
				return a.layoutWeights(gtx)
			}),
		)
	})
}

func (a *App) layoutWeights(gtx layout.Context) layout.Dimensions {
	return layout.Flex{}.Layout(gtx,
		layout.Flexed(1, func(gtx layout.Context) layout.Dimensions { return a.compactEditor(gtx, &a.weightTitle, "标题") }), layout.Rigid(spacerH(5)),
		layout.Flexed(1, func(gtx layout.Context) layout.Dimensions { return a.compactEditor(gtx, &a.weightArtist, "歌手") }), layout.Rigid(spacerH(5)),
		layout.Flexed(1, func(gtx layout.Context) layout.Dimensions { return a.compactEditor(gtx, &a.weightQuality, "质量") }), layout.Rigid(spacerH(5)),
		layout.Flexed(1, func(gtx layout.Context) layout.Dimensions { return a.compactEditor(gtx, &a.weightOfficial, "官方") }), layout.Rigid(spacerH(5)),
		layout.Flexed(1, func(gtx layout.Context) layout.Dimensions { return a.compactEditor(gtx, &a.weightPopularity, "热度") }), layout.Rigid(spacerH(5)),
		layout.Flexed(1, func(gtx layout.Context) layout.Dimensions { return a.compactEditor(gtx, &a.weightUploader, "UP 主") }),
	)
}

func (a *App) layoutReviewWorkspace(gtx layout.Context) layout.Dimensions {
	return layout.Flex{}.Layout(gtx,
		layout.Flexed(.36, func(gtx layout.Context) layout.Dimensions { return a.card(gtx, a.layoutSongList) }),
		layout.Rigid(spacerH(10)),
		layout.Flexed(.64, func(gtx layout.Context) layout.Dimensions { return a.card(gtx, a.layoutOutcomeDetail) }),
	)
}

func (a *App) layoutSongList(gtx layout.Context) layout.Dimensions {
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return a.sectionTitle(gtx, fmt.Sprintf("歌曲 %d", len(a.songs)), fmt.Sprintf("已选 %d · 待处理 %d", selectedCount(a.outcomes), unresolvedCount(a.outcomes)))
		}),
		layout.Rigid(spacer(6)),
		layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
			return a.songList.Layout(gtx, len(a.songs), func(gtx layout.Context, index int) layout.Dimensions {
				if index >= len(a.songClicks) {
					return layout.Dimensions{}
				}
				for a.songClicks[index].Clicked(gtx) {
					a.selectedSong = index
					a.syncCandidateClicks()
					if index < len(a.outcomes) {
						a.searchQuery.SetText(a.outcomes[index].Song.SearchKeywordFull())
					}
				}
				return a.songClicks[index].Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					bg := colorPanel
					if index == a.selectedSong {
						bg = colorPanel2
					}
					return background(gtx, bg, func(gtx layout.Context) layout.Dimensions {
						return layout.Inset{Top: unit.Dp(8), Bottom: unit.Dp(8), Left: unit.Dp(8), Right: unit.Dp(8)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
							name, status := a.songs[index].Name, "等待"
							statusColor := colorMuted
							if index < len(a.outcomes) {
								out := a.outcomes[index]
								switch {
								case out.NeedsReview:
									status, statusColor = "待审", colorWarn
								case out.HasSelection:
									status, statusColor = "已选", colorAccent
								case out.SearchStatus == music2bb.SearchStatusCompleted:
									status = "跳过"
								}
							}
							return layout.Flex{}.Layout(gtx,
								layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
									label := material.Body2(a.theme, name)
									label.MaxLines = 1
									return label.Layout(gtx)
								}),
								layout.Rigid(func(gtx layout.Context) layout.Dimensions {
									label := material.Caption(a.theme, status)
									label.Color = statusColor
									return label.Layout(gtx)
								}),
							)
						})
					})
				})
			})
		}),
	)
}

func (a *App) layoutOutcomeDetail(gtx layout.Context) layout.Dimensions {
	out := a.currentOutcome()
	if out == nil {
		return a.label(gtx, "请选择歌曲查看匹配详情", true)
	}
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			subtitle := out.Song.Artist
			if subtitle == "" {
				subtitle = "未知歌手"
			}
			return a.sectionTitle(gtx, out.Song.Name, subtitle+" · "+core.ReviewReasonText(out.ReviewReason))
		}),
		layout.Rigid(spacer(6)),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions { return a.layoutOutcomeMetadata(gtx, out) }),
		layout.Rigid(spacer(6)),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions { return a.layoutSelectedVideo(gtx, out) }),
		layout.Rigid(spacer(8)),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
				layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
					return a.editor(gtx, &a.searchQuery, out.Song.SearchKeywordFull())
				}),
				layout.Rigid(spacerH(6)), layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return material.Button(a.theme, &a.manualSearch, "搜索").Layout(gtx)
				}),
				layout.Rigid(spacerH(6)), layout.Flexed(.45, func(gtx layout.Context) layout.Dimensions { return a.editor(gtx, &a.bvidInput, "BVID") }),
				layout.Rigid(spacerH(6)), layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return material.Button(a.theme, &a.directBVID, "使用 BVID").Layout(gtx)
				}),
			)
		}),
		layout.Rigid(spacer(8)),
		layout.Flexed(1, func(gtx layout.Context) layout.Dimensions { return a.layoutCandidates(gtx, out) }),
		layout.Rigid(spacer(8)),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Spacing: layout.SpaceBetween}.Layout(gtx,
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return material.Button(a.theme, &a.keepSelection, "确认当前推荐").Layout(gtx)
				}),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					style := material.Button(a.theme, &a.skipSong, "跳过")
					style.Background = colorWarn
					return style.Layout(gtx)
				}),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return material.Button(a.theme, &a.undoDecision, "撤销人工决定").Layout(gtx)
				}),
			)
		}),
	)
}

func (a *App) layoutOutcomeMetadata(gtx layout.Context, out *music2bb.MatchResult) layout.Dimensions {
	failure := "—"
	if out.Failure != nil {
		failure = out.Failure.Reason
	}
	risk := "—"
	if out.RiskReason != "" {
		risk = string(out.RiskReason)
	}
	return background(gtx, colorPanel2, func(gtx layout.Context) layout.Dimensions {
		return layout.UniformInset(unit.Dp(9)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return a.label(gtx, fmt.Sprintf("来源：%s · 专辑：%s · 时长：%s", firstNonEmpty(out.Song.Artist, "未知歌手"), firstNonEmpty(out.Song.Album, "—"), firstNonEmpty(out.Song.Duration, "—")), true)
				}),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return a.label(gtx, fmt.Sprintf("搜索：%s · 身份：%s · 远程请求：%d · 缓存命中：%d · 风控：%s", searchStatusLabel(out.SearchStatus), identityLabel(out.SearchIdentity), out.RemoteRequests, out.CacheHits, risk), true)
				}),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return a.label(gtx, fmt.Sprintf("来源 ID：%s · 失败：%s", firstNonEmpty(out.Song.SourceID, "—"), failure), true)
				}),
			)
		})
	})
}

func (a *App) layoutSelectedVideo(gtx layout.Context, out *music2bb.MatchResult) layout.Dimensions {
	if out.Video == nil {
		return a.label(gtx, "尚未选择视频", true)
	}
	return background(gtx, colorPanel2, func(gtx layout.Context) layout.Dimensions {
		return layout.UniformInset(unit.Dp(10)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			tags := "—"
			if len(out.Video.Tags) > 0 {
				tags = strings.Join(out.Video.Tags, "、")
			}
			return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					label := material.Body1(a.theme, out.Video.Title)
					label.MaxLines = 2
					return label.Layout(gtx)
				}),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return a.label(gtx, fmt.Sprintf("UP：%s · %s · %s · 总分 %.1f · 官方 %t · 认证 %t", out.Video.Uploader, out.Video.Duration, out.Video.BVID, out.Score, out.Video.IsOfficial, out.Video.IsVerified), true)
				}),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return a.label(gtx, fmt.Sprintf("播放 %d · 收藏 %d · 弹幕 %d · 标签 %s", out.Video.PlayCount, out.Video.FavoriteCount, out.Video.DanmakuCount, tags), true)
				}),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return a.label(gtx, fmt.Sprintf("评分：标题 %.1f · 歌手 %.1f · 质量 %.1f · 官方 %.1f · 热度 %.1f · UP %.1f", out.TitleScore, out.ArtistScore, out.QualityScore, out.OfficialScore, out.PopularityScore, out.UploaderScore), true)
				}),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return layout.Flex{}.Layout(gtx, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return material.Button(a.theme, &a.openVideo, "打开视频").Layout(gtx)
					}), layout.Rigid(spacerH(6)), layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return material.Button(a.theme, &a.openSource, "打开歌单").Layout(gtx)
					}))
				}),
			)
		})
	})
}

func (a *App) layoutCandidates(gtx layout.Context, out *music2bb.MatchResult) layout.Dimensions {
	if len(out.Candidates) == 0 {
		return a.label(gtx, "没有候选。可修改关键词搜索，或直接输入 BVID。", true)
	}
	return a.candidateList.Layout(gtx, len(out.Candidates), func(gtx layout.Context, index int) layout.Dimensions {
		if index >= len(a.candidateClicks) {
			return layout.Dimensions{}
		}
		candidate := out.Candidates[index]
		for a.candidateClicks[index].Clicked(gtx) {
			if candidate.Video != nil {
				a.selectVideo(*out, candidate)
			}
		}
		return layout.Inset{Bottom: unit.Dp(6)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			return a.candidateClicks[index].Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return background(gtx, colorPanel2, func(gtx layout.Context) layout.Dimensions {
					return layout.UniformInset(unit.Dp(10)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
						if candidate.Video == nil {
							return a.label(gtx, "无效候选", true)
						}
						return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
							layout.Rigid(func(gtx layout.Context) layout.Dimensions {
								label := material.Body2(a.theme, fmt.Sprintf("%d. %s", index+1, candidate.Video.Title))
								label.MaxLines = 2
								return label.Layout(gtx)
							}),
							layout.Rigid(func(gtx layout.Context) layout.Dimensions {
								return a.label(gtx, fmt.Sprintf("%s · %s · 总分 %.1f | 标题 %.1f 歌手 %.1f 质量 %.1f 官方 %.1f 热度 %.1f UP %.1f", candidate.Video.Uploader, candidate.Video.BVID, candidate.Score, candidate.TitleScore, candidate.ArtistScore, candidate.QualityScore, candidate.OfficialScore, candidate.PopularityScore, candidate.UploaderScore), true)
							}),
						)
					})
				})
			})
		})
	})
}

func (a *App) layoutWritePanel(gtx layout.Context) layout.Dimensions {
	return a.card(gtx, func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return a.sectionTitle(gtx, "写入收藏夹", "所有歌曲必须已选择或显式跳过；成功回执会避免重试重复写入")
			}),
			layout.Rigid(spacer(8)),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return material.Button(a.theme, &a.loadFavorites, "登录并刷新收藏夹").Layout(gtx)
			}),
			layout.Rigid(spacer(6)),
			layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
				if len(a.favorites) == 0 {
					return a.label(gtx, "尚未加载收藏夹", true)
				}
				return a.favoriteList.Layout(gtx, len(a.favorites), func(gtx layout.Context, index int) layout.Dimensions {
					if index >= len(a.favoriteClicks) {
						return layout.Dimensions{}
					}
					favorite := a.favorites[index]
					for a.favoriteClicks[index].Clicked(gtx) {
						a.selectedFavorite = favorite.ID
						a.writeConfirm.Value = false
					}
					return a.favoriteClicks[index].Layout(gtx, func(gtx layout.Context) layout.Dimensions {
						label := fmt.Sprintf("%s  (%d)", favorite.Title, favorite.MediaCount)
						if a.selectedFavorite == favorite.ID {
							label = "● " + label
						} else {
							label = "○ " + label
						}
						return layout.UniformInset(unit.Dp(7)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
							return a.label(gtx, label, a.selectedFavorite != favorite.ID)
						})
					})
				})
			}),
			layout.Rigid(spacer(6)),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return material.CheckBox(a.theme, &a.writeConfirm, fmt.Sprintf("确认把 %d 个已选视频写入目标收藏夹", selectedCount(a.outcomes))).Layout(gtx)
			}),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				style := material.Button(a.theme, &a.writeResults, "开始写入")
				if unresolvedCount(a.outcomes) > 0 || a.selectedFavorite == 0 {
					style.Background = colorBorder
				}
				return style.Layout(gtx)
			}),
		)
	})
}

func (a *App) layoutLogPanel(gtx layout.Context) layout.Dimensions {
	return a.card(gtx, func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return a.sectionTitle(gtx, "运行日志", "缓存、身份、预算、回退与写入事件")
			}),
			layout.Rigid(spacer(6)),
			layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
				if len(a.logs) == 0 {
					return a.label(gtx, "尚无运行日志", true)
				}
				return a.logList.Layout(gtx, len(a.logs), func(gtx layout.Context, index int) layout.Dimensions {
					return layout.Inset{Bottom: unit.Dp(4)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
						label := material.Caption(a.theme, a.logs[index])
						label.Color = colorMuted
						return label.Layout(gtx)
					})
				})
			}),
		)
	})
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

type enumOption struct{ key, label string }

func (a *App) enumGroup(gtx layout.Context, title string, group *widget.Enum, options []enumOption) layout.Dimensions {
	children := []layout.FlexChild{layout.Rigid(func(gtx layout.Context) layout.Dimensions { return a.label(gtx, title, true) })}
	for _, option := range options {
		option := option
		children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return material.RadioButton(a.theme, group, option.key, option.label).Layout(gtx)
		}))
	}
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
}

func (a *App) compactEditor(gtx layout.Context, editor *widget.Editor, hint string) layout.Dimensions {
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			label := material.Caption(a.theme, hint)
			label.Color = colorMuted
			label.Alignment = text.Middle
			return label.Layout(gtx)
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions { return a.editor(gtx, editor, hint) }),
	)
}

func spacer(height unit.Dp) layout.Widget {
	return func(gtx layout.Context) layout.Dimensions {
		return layout.Dimensions{Size: image.Pt(0, gtx.Dp(height))}
	}
}
func spacerH(width unit.Dp) layout.Widget {
	return func(gtx layout.Context) layout.Dimensions { return layout.Dimensions{Size: image.Pt(gtx.Dp(width), 0)} }
}
