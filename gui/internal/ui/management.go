package ui

import (
	"fmt"
	"image"
	"strings"

	"gioui.org/layout"
	"gioui.org/op/paint"
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"
	music2bb "github.com/bagags/music2bb-go"
	"github.com/bagags/music2bb-go/m2bb-gui/internal/state"
)

func (a *App) layoutAccount(gtx layout.Context) layout.Dimensions {
	return a.pageList.Layout(gtx, 1, func(gtx layout.Context, _ int) layout.Dimensions {
		children := []layout.FlexChild{
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return a.sectionTitle(gtx, "Bilibili 账号", "优先复用加密边界内的本地 Cookie；需要时显示扫码二维码")
			}),
			layout.Rigid(spacer(10)),
		}
		if a.qrImage != nil {
			children = append(children,
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return a.card(gtx, func(gtx layout.Context) layout.Dimensions {
						return layout.Flex{Axis: layout.Vertical, Alignment: layout.Middle}.Layout(gtx,
							layout.Rigid(func(gtx layout.Context) layout.Dimensions {
								return a.sectionTitle(gtx, "扫码登录", "请在 3 分钟内使用 Bilibili 客户端扫码，并在手机端确认登录")
							}),
							layout.Rigid(spacer(8)),
							layout.Rigid(func(gtx layout.Context) layout.Dimensions {
								gtx.Constraints.Min = image.Pt(gtx.Dp(unit.Dp(320)), gtx.Dp(unit.Dp(320)))
								gtx.Constraints.Max = gtx.Constraints.Min
								return widget.Image{Src: paint.NewImageOp(a.qrImage), Fit: widget.Contain}.Layout(gtx)
							}),
							layout.Rigid(spacer(6)),
							layout.Rigid(func(gtx layout.Context) layout.Dimensions {
								return a.label(gtx, "扫码后还需要在手机上点击确认；窗口会继续轮询登录结果。", true)
							}),
						)
					})
				}),
				layout.Rigid(spacer(10)),
			)
		}
		if a.lastError != "" {
			children = append(children,
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return background(gtx, colorPanel2, func(gtx layout.Context) layout.Dimensions {
						return layout.UniformInset(unit.Dp(12)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
							label := material.Body2(a.theme, "登录失败："+a.lastError)
							label.Color = colorDanger
							return label.Layout(gtx)
						})
					})
				}),
				layout.Rigid(spacer(10)),
			)
		}
		children = append(children,
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return a.card(gtx, func(gtx layout.Context) layout.Dimensions {
					account := "未确认登录状态"
					if a.account != nil {
						account = fmt.Sprintf("已登录：%s  (MID %d)", a.account.Name, a.account.ID)
					}
					return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
						layout.Rigid(func(gtx layout.Context) layout.Dimensions { return a.label(gtx, account, a.account == nil) }),
						layout.Rigid(spacer(8)),
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							return material.CheckBox(a.theme, &a.allowQR, "本地 Cookie 无效时允许二维码登录").Layout(gtx)
						}),
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							return layout.Flex{}.Layout(gtx,
								layout.Rigid(func(gtx layout.Context) layout.Dimensions {
									return material.Button(a.theme, &a.loginButton, "登录 / 验证").Layout(gtx)
								}), layout.Rigid(spacerH(8)),
								layout.Rigid(func(gtx layout.Context) layout.Dimensions {
									style := material.Button(a.theme, &a.logoutButton, "退出并清除本地登录")
									style.Background = colorWarn
									return style.Layout(gtx)
								}), layout.Rigid(spacerH(8)),
								layout.Rigid(func(gtx layout.Context) layout.Dimensions {
									return material.Button(a.theme, &a.resetAnon, "重置匿名搜索身份").Layout(gtx)
								}),
							)
						}),
					)
				})
			}),
		)
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
	})
}

func (a *App) layoutFavorites(gtx layout.Context) layout.Dimensions {
	return a.pageList.Layout(gtx, 1, func(gtx layout.Context, _ int) layout.Dimensions {
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return a.sectionTitle(gtx, "收藏夹管理", "列出账号收藏夹，或创建默认私有的新收藏夹")
			}),
			layout.Rigid(spacer(10)),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return material.Button(a.theme, &a.loadFavorites, "登录并刷新收藏夹").Layout(gtx)
			}),
			layout.Rigid(spacer(10)),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return a.card(gtx, func(gtx layout.Context) layout.Dimensions {
					return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							return a.sectionTitle(gtx, fmt.Sprintf("已有收藏夹 %d", len(a.favorites)), "点击可设为转换写入目标")
						}),
						layout.Rigid(spacer(6)),
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							if len(a.favorites) == 0 {
								return a.label(gtx, "尚未加载", true)
							}
							return a.favoriteList.Layout(gtx, len(a.favorites), func(gtx layout.Context, index int) layout.Dimensions {
								if index >= len(a.favoriteClicks) {
									return layout.Dimensions{}
								}
								favorite := a.favorites[index]
								for a.favoriteClicks[index].Clicked(gtx) {
									a.selectedFavorite = favorite.ID
								}
								return a.favoriteClicks[index].Layout(gtx, func(gtx layout.Context) layout.Dimensions {
									prefix := "○"
									if a.selectedFavorite == favorite.ID {
										prefix = "●"
									}
									return background(gtx, colorPanel2, func(gtx layout.Context) layout.Dimensions {
										return layout.UniformInset(unit.Dp(9)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
											return layout.Flex{}.Layout(gtx,
												layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
													return a.label(gtx, fmt.Sprintf("%s  %s", prefix, favorite.Title), false)
												}),
												layout.Rigid(func(gtx layout.Context) layout.Dimensions {
													return a.label(gtx, fmt.Sprintf("ID %d · %d 项", favorite.ID, favorite.MediaCount), true)
												}),
											)
										})
									})
								})
							})
						}),
					)
				})
			}),
			layout.Rigid(spacer(10)),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return a.card(gtx, func(gtx layout.Context) layout.Dimensions {
					return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							return a.sectionTitle(gtx, "新建收藏夹", "默认仅自己可见；公开选项必须显式勾选")
						}),
						layout.Rigid(spacer(6)),
						layout.Rigid(func(gtx layout.Context) layout.Dimensions { return a.editor(gtx, &a.favoriteTitle, "收藏夹名称") }),
						layout.Rigid(spacer(6)),
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							return a.editor(gtx, &a.favoriteIntro, "简介（可选）")
						}),
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							return material.CheckBox(a.theme, &a.favoritePublic, "公开可见").Layout(gtx)
						}),
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							return material.Button(a.theme, &a.createFavorite, "创建收藏夹").Layout(gtx)
						}),
					)
				})
			}),
		)
	})
}

func (a *App) layoutBrowser(gtx layout.Context) layout.Dimensions {
	return a.pageList.Layout(gtx, 1, func(gtx layout.Context, _ int) layout.Dimensions {
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return a.sectionTitle(gtx, "浏览器管理", "优先使用已安装的 Chrome/Chromium；没有可用系统浏览器时才下载并校验托管版")
			}),
			layout.Rigid(spacer(10)),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return a.card(gtx, func(gtx layout.Context) layout.Dimensions {
					if !a.browserLoaded {
						return a.label(gtx, "点击“刷新状态”读取当前平台的固定版本与安装状态。", true)
					}
					s := a.browserStatus
					return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							status := "未找到可用浏览器"
							subtitle := fmt.Sprintf("%s · Chromium %s · revision %d", s.Platform, s.Version, s.Revision)
							switch {
							case s.Source == music2bb.BrowserSourceSystem && s.Installed:
								status, subtitle = "正在使用系统浏览器", "无需下载；系统浏览器不适用托管包校验"
							case s.Installed && s.Verified:
								status = "托管 Chromium 已安装并校验"
							case s.Installed:
								status = "托管 Chromium 已安装但未通过校验"
							}
							return a.sectionTitle(gtx, status, subtitle)
						}),
						layout.Rigid(spacer(8)),
						layout.Rigid(func(gtx layout.Context) layout.Dimensions { return a.metric(gtx, "源码提交", s.ChromiumCommit) }),
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							return a.metric(gtx, "归档大小", formatBytes(s.ApproxBytes))
						}),
						layout.Rigid(func(gtx layout.Context) layout.Dimensions { return a.metric(gtx, "归档 SHA-256", s.ExpectedSHA256) }),
						layout.Rigid(func(gtx layout.Context) layout.Dimensions { return a.metric(gtx, "可执行文件", s.ExecutablePath) }),
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							source := "托管下载版"
							if s.Source == music2bb.BrowserSourceSystem {
								source = "系统 Chrome/Chromium"
							}
							return a.metric(gtx, "浏览器来源", source)
						}),
					)
				})
			}),
			layout.Rigid(spacer(10)),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{}.Layout(gtx,
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return material.Button(a.theme, &a.refreshBrowser, "刷新状态").Layout(gtx)
					}), layout.Rigid(spacerH(8)),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						label := "下载并校验托管 Chromium"
						if a.browserLoaded && a.browserStatus.Source == music2bb.BrowserSourceSystem && a.browserStatus.Installed {
							label = "系统浏览器已就绪"
						}
						return material.Button(a.theme, &a.installBrowser, label).Layout(gtx)
					}), layout.Rigid(spacerH(8)),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						style := material.Button(a.theme, &a.clearBrowser, "清理托管浏览器缓存")
						style.Background = colorWarn
						return style.Layout(gtx)
					}),
				)
			}),
		)
	})
}

func (a *App) layoutStorage(gtx layout.Context) layout.Dimensions {
	kinds := []struct {
		kind          state.CacheKind
		title, detail string
	}{
		{state.CacheSearch, "搜索缓存", "可重建的未评分 Bilibili 搜索页（正结果 7 天、空结果 1 小时）"},
		{state.CacheCheckpoints, "转换 checkpoint", "按歌单保存已完成匹配与逐视频写入回执"},
		{state.CacheDecisions, "人工决定", "跨歌单复用 7 天的选择或跳过决定"},
		{state.CacheAnonymous, "匿名设备身份", "与账号 Cookie 隔离的 Bilibili 匿名搜索设备状态"},
	}
	return a.pageList.Layout(gtx, 1, func(gtx layout.Context, _ int) layout.Dimensions {
		children := []layout.FlexChild{
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return a.sectionTitle(gtx, "缓存与恢复状态", "清理动作只影响明确选择的附加状态，永远不会删除账号 Cookie")
			}),
			layout.Rigid(spacer(10)),
		}
		for _, item := range kinds {
			item := item
			children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				status := a.cacheStatuses[item.kind]
				return layout.Inset{Bottom: unit.Dp(8)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					return a.card(gtx, func(gtx layout.Context) layout.Dimensions {
						return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
							layout.Rigid(func(gtx layout.Context) layout.Dimensions {
								return material.CheckBox(a.theme, a.cacheChecks[item.kind], "").Layout(gtx)
							}),
							layout.Flexed(1, func(gtx layout.Context) layout.Dimensions { return a.sectionTitle(gtx, item.title, item.detail) }),
							layout.Rigid(func(gtx layout.Context) layout.Dimensions {
								return a.label(gtx, fmt.Sprintf("%d 文件 · %s", status.Files, formatBytes(status.Bytes)), true)
							}),
						)
					})
				})
			}))
		}
		children = append(children,
			layout.Rigid(spacer(4)),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{}.Layout(gtx,
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return material.Button(a.theme, &a.refreshCache, "刷新统计").Layout(gtx)
					}), layout.Rigid(spacerH(8)),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						style := material.Button(a.theme, &a.clearCache, "清理所选状态")
						style.Background = colorWarn
						return style.Layout(gtx)
					}),
				)
			}),
		)
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
	})
}

func (a *App) layoutAbout(gtx layout.Context) layout.Dimensions {
	configDir, cacheDir := a.backend.PersistentStatePaths()
	return a.pageList.Layout(gtx, 1, func(gtx layout.Context, _ int) layout.Dimensions {
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return a.sectionTitle(gtx, "关于 music2bb Desktop", "真正的 Gio 原生桌面前端；不启动 Web 服务，不使用 HTML/JavaScript UI")
			}),
			layout.Rigid(spacer(10)),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return a.card(gtx, func(gtx layout.Context) layout.Dimensions {
					return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
						layout.Rigid(func(gtx layout.Context) layout.Dimensions { return a.label(gtx, "版本  "+a.version, false) }),
						layout.Rigid(spacer(6)),
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							return a.label(gtx, "桌面层只依赖 music2bb 的稳定公共 Engine API；Kugou、Apple Music、Bilibili、浏览器、匹配器和网络协议继续由主项目内部实现。", true)
						}),
						layout.Rigid(spacer(10)),
						layout.Rigid(func(gtx layout.Context) layout.Dimensions { return a.metric(gtx, "配置目录", configDir) }),
						layout.Rigid(func(gtx layout.Context) layout.Dimensions { return a.metric(gtx, "缓存目录", cacheDir) }),
						layout.Rigid(spacer(10)),
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							return a.sectionTitle(gtx, "应用更新", "检查最新稳定版；当前 Releases 尚未发布桌面构件，请从下载页获取匹配平台的 GUI 包")
						}),
						layout.Rigid(spacer(6)),
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							return material.Button(a.theme, &a.updateCheck, "检查更新").Layout(gtx)
						}),
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							if a.updateInfo == nil {
								return layout.Dimensions{}
							}
							info := a.updateInfo
							if !info.Available {
								return a.label(gtx, fmt.Sprintf("当前 %s，已是最新版本", info.CurrentVersion), true)
							}
							return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
								layout.Rigid(func(gtx layout.Context) layout.Dimensions {
									return a.label(gtx, fmt.Sprintf("当前 %s，可用核心发行版 %s", info.CurrentVersion, info.LatestVersion), true)
								}),
								layout.Rigid(func(gtx layout.Context) layout.Dimensions {
									return material.Button(a.theme, &a.openReleases, "打开下载页面").Layout(gtx)
								}),
							)
						}),
						layout.Rigid(spacer(10)),
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							return a.label(gtx, "许可：GNU Affero General Public License v3.0。Chromium 及其他依赖的第三方声明以主项目发布包和仓库文件为准。", true)
						}),
						layout.Rigid(spacer(8)),
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							return layout.Flex{}.Layout(gtx,
								layout.Rigid(func(gtx layout.Context) layout.Dimensions {
									return material.Button(a.theme, &a.openProject, "打开项目主页").Layout(gtx)
								}), layout.Rigid(spacerH(8)),
								layout.Rigid(func(gtx layout.Context) layout.Dimensions {
									return material.Button(a.theme, &a.openLicense, "查看完整许可证").Layout(gtx)
								}),
							)
						}),
					)
				})
			}),
		)
	})
}

var _ = strings.Builder{}
