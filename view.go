package main

import (
	"fmt"
	"strings"
	"time"

	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/markdown"
	"ngareader/internal/local"
	"ngareader/internal/nga"
)

type palette struct{ side, list, selected, hover, accent ui.Color }

func (a *reader) theme(c *ui.Context) palette {
	theme := *c.Theme()
	if a.state.Theme == "深色" {
		theme = *ui.DarkTheme()
	} else if a.state.Theme == "浅色" || a.state.Theme == "NGA经典屎黄色" {
		theme = *ui.LightTheme()
	}
	p := palette{}
	if theme.Dark {
		theme.Background = ui.Hex("#202020")
		theme.Text = ui.Hex("#e5e5e2")
		theme.TextMuted = ui.Hex("#a3a39a")
		theme.Border = ui.Hex("#343431")
		theme.Surface = ui.Hex("#282826")
		theme.SurfaceHover = ui.Hex("#343530")
		theme.SurfacePressed = ui.Hex("#3d3e39")
		theme.Accent = ui.Hex("#d6dfce")
		theme.AccentText = ui.Hex("#22251f")
		p = palette{ui.Hex("#181818"), ui.Hex("#1c1c1c"), ui.Hex("#333530"), ui.Hex("#292b27"), ui.Hex("#bbd2b0")}
	} else {
		theme.Background = ui.Hex("#ffffff")
		theme.Text = ui.Hex("#252522")
		theme.TextMuted = ui.Hex("#72726b")
		theme.Border = ui.Hex("#e6e7e2")
		theme.Surface = ui.Hex("#f5f5f2")
		theme.SurfaceHover = ui.Hex("#e9ebe4")
		theme.SurfacePressed = ui.Hex("#e0e2d9")
		theme.Accent = ui.Hex("#3c4e39")
		theme.AccentText = ui.Hex("#ffffff")
		p = palette{ui.Hex("#f6f6f5"), ui.Hex("#fafaf9"), ui.Hex("#e7e8e3"), ui.Hex("#eeefec"), ui.Hex("#426744")}
	}
	if a.state.Theme == "NGA经典屎黄色" {
		theme.Background = ui.Hex("#fff8e7")
		theme.Text = ui.Hex("#591902")
		theme.TextMuted = ui.Hex("#82634a")
		theme.Border = ui.Hex("#e4d2a4")
		theme.Surface = ui.Hex("#fff0cd")
		theme.SurfaceHover = ui.Hex("#f5deb0")
		theme.SurfacePressed = ui.Hex("#efcf94")
		theme.Accent = ui.Hex("#671f05")
		theme.AccentText = ui.Hex("#fff8e7")
		p = palette{ui.Hex("#fff0cd"), ui.Hex("#fff8e7"), ui.Hex("#efcf94"), ui.Hex("#f5deb0"), ui.Hex("#003c7d")}
	}
	theme.FontSize = 13
	theme.Spacing = 3
	theme.Radius = 7
	theme.InactiveAccent = theme.Accent
	c.SetTheme(&theme)
	if a.md.Theme == nil || a.md.Theme.FontSize != float32(a.state.FontSize) || a.md.Theme.Text != theme.Text {
		a.md.Theme = &markdown.Theme{FontSize: float32(a.state.FontSize), Text: theme.Text, Muted: theme.TextMuted, Link: p.accent, Strong: theme.Text, StrongWeight: 500, LineHeight: 1.7, BlockSpacing: .85, QuoteBar: theme.Border, QuoteText: theme.TextMuted, QuoteBarWidth: 2}
	}
	return p
}

func iconButton(c *ui.Context, key, icon, label string, disabled bool) ui.Element {
	b := ui.ButtonBase(c.Key(key)).Label(label).Tooltip(label).Size(30, 30).Padding(6).Radius(6).Disabled(disabled)
	if b.Hovered() {
		b.Background(c.Theme().SurfaceHover)
	}
	return b.Children(func() {
		if s := icons[icon]; s != nil {
			ui.Icon(c, s).Size(17, 17)
		}
	})
}
func navButton(c *ui.Context, key, icon, label, count string, selected bool, p palette, fn func()) {
	b := ui.ButtonBase(c.Key(key)).Label(label).FillWidth().Padding(10, 11).Gap(9).Radius(7)
	if selected {
		b.Background(p.selected)
	} else if b.Hovered() {
		b.Background(p.hover)
	}
	b.Children(func() {
		ui.Icon(c, icons[icon]).Size(16, 16)
		ui.Text(c, label).Grow(1).SingleLine()
		if count != "" {
			ui.Text(c, count).FontSize(11).TextColor(c.Theme().TextMuted)
		}
	}).OnClick(fn)
}
func (a *reader) view(c *ui.Context) {
	p := a.theme(c)
	if c.Shortcut(0, ui.KeyF5) {
		if a.selected.ID > 0 {
			a.loadThread(max(1, a.thread.Page))
		} else {
			a.loadTopics(max(1, a.topics.Page))
		}
	}
	ui.Column(c).Fill().Background(c.Theme().Background).Children(func() {
		a.titlebar(c, p)
		if a.focused {
			ui.Column(c).Grow(1).MinHeight(0).Children(func() { a.readerPane(c, p) })
		} else {
			split := ui.Split(c, &a.state.SidebarWidth, func() { a.sidebar(c, p) }, func() {
				ui.Split(c, &a.state.TopicWidth, func() { a.topicPane(c, p) }, func() { a.readerPane(c, p) }).SplitReset(310).Fill()
			}).SplitReset(180).Grow(1).MinHeight(0)
			if split.Changed() {
				a.save()
			}
		}
		a.statusbar(c, p)
	})
	a.dialogs(c, p)
	a.helpDialog(c)
	a.composerView(c)
	a.updatesView(c)
}

func (a *reader) titlebar(c *ui.Context, p palette) {
	bar := c.TitleBar()
	ui.Row(c).Height(max(44, bar.Height)).Padding(0, bar.Right+13, 0, bar.Left+13).Gap(9).Background(p.side).BorderWidth(0, 0, 1, 0).BorderColor(c.Theme().Border).DragWindow().Children(func() {
		ui.Icon(c, icons["app"]).Size(17, 17)
		ui.Text(c, "工作台").FontWeight(500)
		ui.Text(c, "/ NGA Reader").FontSize(12).TextColor(c.Theme().TextMuted)
		ui.Spacer(c).Grow(1)
		if a.demo {
			ui.Text(c, "演示数据").FontSize(11).TextColor(p.accent)
			ui.Button(c, "返回论坛").FontSize(11).Padding(4, 8).OnClick(a.returnToLive)
		} else if a.client.HasSession() {
			ui.Text(c, "本地会话").FontSize(11).TextColor(c.Theme().TextMuted)
		} else {
			ui.Text(c, "未登录").FontSize(11).TextColor(c.Theme().TextMuted)
		}
	})
}
func (a *reader) sidebar(c *ui.Context, p palette) {
	ui.Column(c).Fill().Padding(16, 10, 10, 10).Gap(4).Background(p.side).Children(func() {
		navButton(c, "nav-board", "topics", "全部话题", "", a.mode == "board", p, func() { a.mode = "board"; a.filter = "" })
		navButton(c, "nav-fav", "bookmark", "我的收藏", fmt.Sprint(len(a.state.Favorites)), a.mode == "favorites", p, func() { a.mode = "favorites"; a.filter = "" })
		navButton(c, "nav-history", "clock", "最近阅读", "", a.mode == "history", p, func() { a.mode = "history"; a.filter = "" })
		updateCount := ""
		if a.updates.Offer != nil {
			updateCount = "●"
		}
		navButton(c, "nav-updates", "refresh", "软件更新", updateCount, false, p, func() { a.checkUpdates(true) })
		ui.Row(c).Padding(19, 10, 3, 10).Children(func() {
			ui.Text(c, "我的板块").FontSize(11).TextColor(c.Theme().TextMuted).Grow(1)
			iconButton(c, "add-board", "plus", "添加板块", false).OnClick(func() { a.formError = ""; a.addOpen = true })
		})
		ui.Scroll(c).Grow(1).MinHeight(0).Gap(3).Children(func() {
			label := "导入收藏板块"
			if a.loadingFavorites {
				label = "正在导入…"
			}
			ui.Button(c, label).Label("导入收藏板块").FontSize(11).Padding(7, 9).Margin(0, 6, 9, 6).Disabled(a.loadingFavorites).OnClick(a.importFavoriteBoards)
			for _, board := range a.state.Boards {
				b := board
				row := ui.ButtonBase(c.Key(b.ID)).Label(b.Name).FillWidth().Padding(9, 10).Gap(9).Radius(6)
				if b.ID == a.board.ID && a.mode == "board" {
					row.TextColor(p.accent)
				}
				if row.Hovered() {
					row.Background(p.hover)
				}
				row.Children(func() {
					ui.Text(c, "#").TextColor(c.Theme().TextMuted)
					ui.Text(c, b.Name).SingleLine().Grow(1)
					ui.Textf(c, "%d", b.ID).FontSize(11).TextColor(c.Theme().TextMuted)
				}).OnClick(func() { a.selectBoard(b) })
			}
		})
		navButton(c, "settings", "settings", "阅读设置", "", false, p, func() { a.settingsOpen = true; a.formError = "" })
		navButton(c, "help", "help", "使用说明", "", false, p, func() { a.helpOpen = true })
		ui.Divider(c).Margin(7, 0)
		ui.Row(c).Padding(5, 8).Gap(9).Children(func() {
			ui.Icon(c, icons["user"]).Size(21, 21).TextColor(c.Theme().TextMuted)
			ui.Column(c).Gap(3).Grow(1).Children(func() {
				if a.demo {
					ui.Text(c, "本地演示").FontSize(12)
					ui.Text(c, "不含真实账号").FontSize(11).TextColor(c.Theme().TextMuted)
				} else if a.client.HasSession() {
					ui.Text(c, "会话已载入").FontSize(12)
					ui.Text(c, "仅在本机使用").FontSize(11).TextColor(c.Theme().TextMuted)
				} else {
					ui.Button(c, "登录 NGA").FontSize(12).Padding(5, 7).OnClick(a.openLogin)
				}
			})
		})
	})
}
func (a *reader) topicPane(c *ui.Context, p palette) {
	ui.Column(c).Fill().Background(p.list).Children(func() {
		ui.Row(c).Height(57).Padding(0, 17).Gap(5).BorderWidth(0, 0, 1, 0).BorderColor(c.Theme().Border).Children(func() {
			ui.Column(c).Grow(1).Gap(3).Children(func() {
				name := a.board.Name
				if a.mode == "favorites" {
					name = "我的收藏"
				} else if a.mode == "history" {
					name = "最近阅读"
				}
				ui.Text(c, name).FontSize(15).FontWeight(500).SingleLine()
				ui.Text(c, "NGA / 综合讨论").FontSize(11).TextColor(c.Theme().TextMuted)
			})
			ui.Button(c, "发帖").FontSize(12).Padding(5, 9).Disabled(a.mode != "board" || a.compose.Sending).OnClick(func() { a.openComposer(nga.PostTarget{Action: "new", FID: a.board.ID}, a.board.Name) })
			iconButton(c, "refresh", "refresh", "刷新帖子列表", a.loadingTopics || a.demo || a.mode != "board").OnClick(func() { a.loadTopics(max(1, a.topics.Page)) })
		})
		ui.TextInput(c.Key("filter"), &a.filter).Placeholder("筛选当前页话题…").Label("筛选话题").Margin(13, 13, 8, 13).FontSize(12)
		ui.Row(c).Padding(0, 15, 8, 15).Gap(14).Children(func() {
			b := ui.ButtonBase(c.Key("new")).Padding(4, 0).Label("最新讨论")
			if !a.hot {
				b.TextColor(c.Theme().Text)
			} else {
				b.TextColor(c.Theme().TextMuted)
			}
			b.Children(func() { ui.Text(c, "最新讨论").FontSize(12) }).OnClick(func() { a.hot = false })
			b = ui.ButtonBase(c.Key("hot")).Padding(4, 0).Label("按回复数排序")
			if a.hot {
				b.TextColor(c.Theme().Text)
			} else {
				b.TextColor(c.Theme().TextMuted)
			}
			b.Children(func() { ui.Text(c, "热门").FontSize(12) }).OnClick(func() { a.hot = true })
			ui.Spacer(c).Grow(1)
			iconButton(c, "open-link", "link", "打开帖子链接", false).OnClick(func() { a.linkOpen = true; a.formError = "" })
		})
		items := a.displayTopics()
		a.listState.Key = func(i int) any { return items[i].ID }
		a.listState.Label = func(i int) string { return items[i].Title }
		ui.List(c, &a.listState, len(items), func(i int) {
			t := items[i]
			seen, changed := local.ReadStatus(a.state.History, t, a.readingUID())
			row := ui.ButtonBase(c.Key(t.ID)).Label(t.Title).FillWidth().Padding(13, 11).Margin(2, 7).Radius(7)
			if a.state.Theme == "NGA经典屎黄色" && i%2 == 0 {
				row.Background(p.side)
			}
			if t.ID == a.selected.ID {
				row.Background(p.selected)
			} else if row.Hovered() {
				row.Background(p.hover)
			}
			row.Children(func() {
				ui.Column(c).FillWidth().Gap(8).Children(func() {
					title := ui.Text(c, t.Title).FontSize(13).LineHeight(1.65).MaxLines(2)
					if seen && !changed && t.ID != a.selected.ID {
						title.TextColor(c.Theme().TextMuted)
					} else if a.state.Theme == "NGA经典屎黄色" {
						title.TextColor(p.accent)
					}
					ui.Row(c).Gap(5).Children(func() {
						if changed {
							ui.Text(c, "有新回复").FontSize(10).TextColor(p.accent).Tooltip("回复数比上次查看时增加，不代表精确未读楼数")
						} else if seen {
							ui.Text(c, "已读").FontSize(10).TextColor(c.Theme().TextMuted)
						}
						author := t.Author
						if author == "" {
							author = "匿名用户"
						}
						ui.Text(c, author).FontSize(11).TextColor(c.Theme().TextMuted).SingleLine().Grow(1)
						ui.Icon(c, icons["topics"]).Size(12, 12).TextColor(c.Theme().TextMuted)
						ui.Textf(c, "%d", t.Replies).FontSize(11).TextColor(c.Theme().TextMuted)
					})
				})
			}).OnClick(func() { a.selectTopic(t) })
		}).Grow(1).MinHeight(0).Children(func() {
			if len(items) == 0 {
				message := "暂无话题"
				if a.loadingTopics && a.mode == "board" {
					message = "正在加载话题…"
				} else if a.topicErr != nil && a.mode == "board" {
					message = errorMessage(a.topicErr)
				} else if a.filter != "" {
					message = "没有匹配的话题"
				} else if a.mode == "favorites" {
					message = "收藏的帖子会显示在这里"
				} else if a.mode == "history" {
					message = "阅读记录会显示在这里"
				}
				ui.Text(c, message).FontSize(12).TextColor(c.Theme().TextMuted).Padding(20, 17)
			}
		})
		ui.Row(c).Height(44).Padding(0, 10).Gap(5).BorderWidth(1, 0, 0, 0).BorderColor(c.Theme().Border).Children(func() {
			ui.Textf(c, "%d 条话题", len(items)).FontSize(11).TextColor(c.Theme().TextMuted).Grow(1)
			iconButton(c, "list-prev", "left", "上一页话题", a.mode != "board" || a.loadingTopics || a.topics.Page <= 1 || a.demo).OnClick(func() { a.loadTopics(a.topics.Page - 1) })
			ui.Textf(c, "%d", max(1, a.topics.Page)).FontSize(11).TextColor(c.Theme().TextMuted)
			iconButton(c, "list-next", "right", "下一页话题", a.mode != "board" || a.loadingTopics || !a.topics.HasNext || a.demo).OnClick(func() { a.loadTopics(a.topics.Page + 1) })
		})
	})
}
func (a *reader) statusbar(c *ui.Context, p palette) {
	ui.Row(c).Height(27).Padding(0, 12).Gap(8).Background(p.side).BorderWidth(1, 0, 0, 0).BorderColor(c.Theme().Border).Children(func() {
		text := "就绪"
		if a.demo {
			text = "演示模式 · 全部内容均为样例"
		} else if a.loadingFavorites {
			text = "正在导入 NGA 收藏板块…"
		} else if a.loadingThread || a.loadingTopics {
			text = "正在连接 NGA…"
		} else if a.topicErr != nil {
			text = errorMessage(a.topicErr)
		} else if a.client.HasSession() {
			text = "本地会话 · NGA"
		} else {
			text = "未登录 · NGA"
		}
		if a.message != "" && time.Now().Before(a.messageUntil) {
			text = a.message
			c.After(time.Until(a.messageUntil))
		}
		ui.Text(c, text).FontSize(11).TextColor(c.Theme().TextMuted).SingleLine().Grow(1)
		if ui.ButtonBase(c.Key("hide")).Label("快速隐藏").Padding(3, 0).Children(func() { ui.Text(c, "快速隐藏  Ctrl Shift Space").FontSize(11).TextColor(c.Theme().TextMuted) }).Clicked() {
			a.toggleHidden()
		}
	})
}

func errorMessage(err error) string {
	if err == nil {
		return ""
	}
	message := err.Error()
	if strings.Contains(message, "未登录") || strings.Contains(message, "访客") {
		return "此板块需要登录 NGA 后访问"
	}
	return message
}
