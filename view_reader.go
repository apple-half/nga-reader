package main

import (
	"errors"
	"fmt"
	"time"

	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/markdown"
	"ngareader/internal/nga"
)

func (a *reader) readerPane(c *ui.Context, p palette) {
	ui.Column(c).Fill().Background(c.Theme().Background).Children(func() {
		ui.Row(c).Height(57).Padding(0, 19).Gap(6).BorderWidth(0, 0, 1, 0).BorderColor(c.Theme().Border).Children(func() {
			ui.Text(c, "话题").FontSize(12).TextColor(c.Theme().TextMuted)
			ui.Icon(c, icons["right"]).Size(13, 13).TextColor(c.Theme().TextMuted)
			ui.Text(c, "帖子阅读").FontSize(12).Grow(1)
			ui.Button(c, "回复").FontSize(12).Padding(5, 9).Disabled(a.selected.ID <= 0 || a.loadingThread || a.threadErr != nil || a.compose.Sending).OnClick(func() {
				a.openComposer(nga.PostTarget{Action: "reply", TID: a.selected.ID, FID: a.thread.Topic.FID}, a.selected.Title)
			})
			ui.Text(c, "图片按需").FontSize(11).TextColor(c.Theme().TextMuted).Tooltip("默认不加载图片，点击正文中的“展示图片”后预览").Padding(5, 8)
			label := "收藏帖子"
			if a.isFavorite(a.selected.ID) {
				label = "取消收藏"
			}
			iconButton(c, "bookmark", "bookmark", label, a.selected.ID <= 0).OnClick(a.toggleFavorite)
			iconButton(c, "focus", "focus", "切换专注阅读", false).OnClick(func() { a.focused = !a.focused })
			iconButton(c, "font", "text", "字号与主题设置", false).OnClick(func() { a.settingsOpen = true; a.formError = "" })
		})
		if a.selected.ID <= 0 {
			a.emptyReader(c, p)
		} else {
			rows := []readingRow{}
			viewport := ui.Scroll(c.Key(fmt.Sprintf("%d/%d/%t", a.selected.ID, a.thread.Page, a.onlyAuthor))).TrackScroll(&a.scroll).Grow(1).MinHeight(0).Padding(29, 32, 20, 32).Children(func() {
				ui.Column(c).FillWidth().MaxWidth(760).Margin(0, ui.Auto).Gap(0).Children(func() {
					category := a.board.Name
					if a.demo {
						category += " / 演示帖子"
					}
					ui.Text(c, "#  "+category).FontSize(11).TextColor(c.Theme().TextMuted).Margin(0, 0, 12, 0)
					ui.Text(c, a.selected.Title).FontSize(22).FontWeight(500).LineHeight(1.5).Margin(0, 0, 21, 0).Selectable()
					if a.loadingThread {
						ui.Text(c, "正在加载正文…").FontSize(13).TextColor(c.Theme().TextMuted)
						return
					}
					if a.threadErr != nil {
						ui.Text(c, errorMessage(a.threadErr)).TextColor(c.Theme().TextMuted).Margin(0, 0, 17, 0)
						ui.Row(c).Gap(8).Children(func() {
							ui.Button(c, "重试").OnClick(func() { a.loadThread(max(1, a.thread.Page)) })
							var api *nga.APIError
							if !a.client.HasSession() || (errors.As(a.threadErr, &api) && api.Kind == "login") {
								ui.Button(c, "登录 NGA").OnClick(a.openLogin)
							} else {
								ui.Button(c, "打开原帖").OnClick(a.openOriginal)
							}
						})
						return
					}
					for i, r := range a.rendered {
						post := r
						if a.onlyAuthor && a.demo && post.post.AuthorID != a.thread.MainAuthorID {
							continue
						}
						if i == 1 {
							ui.Row(c).Padding(21, 0, 17, 0).Gap(8).BorderWidth(1, 0, 0, 0).BorderColor(c.Theme().Border).Children(func() {
								label := fmt.Sprintf("回复  %d", a.selected.Replies)
								if a.thread.TotalRows == 0 {
									count := len(a.rendered)
									if count > 0 && a.rendered[0].post.Floor == 0 {
										count--
									}
									label = fmt.Sprintf("本页回复  %d", count)
								}
								ui.Text(c, label).FontSize(12).FontWeight(500).Grow(1)
							})
						}
						row := ui.Column(c.Key(post.post.Floor)).FillWidth().Gap(0).Margin(0, 0, 23, 0).Children(func() {
							ui.Row(c).Gap(9).Margin(0, 0, 12, 0).Children(func() {
								initial := "?"
								if chars := []rune(post.post.Author); len(chars) > 0 {
									initial = string(chars[0])
								}
								ui.Box(c).Size(29, 29).Radius(15).Background(p.selected).Center().Children(func() { ui.Text(c, initial).FontSize(12).TextColor(p.accent) })
								ui.Text(c, post.post.Author).FontSize(12).FontWeight(500).SingleLine()
								if !post.post.PostDate.IsZero() {
									ui.Text(c, dateLabel(post.post.PostDate)).FontSize(11).TextColor(c.Theme().TextMuted)
								}
								ui.Spacer(c).Grow(1)
								floor := fmt.Sprintf("#%d", post.post.Floor)
								if post.post.AuthorID == a.thread.MainAuthorID {
									floor = "楼主 · " + floor
								}
								ui.Text(c, floor).FontSize(11).TextColor(c.Theme().TextMuted)
								ui.Button(c, "引用回复").Label(fmt.Sprintf("引用回复第%d楼", post.post.Floor)).FontSize(11).Padding(4, 6).Disabled(a.compose.Sending).OnClick(func() {
									a.openComposer(nga.PostTarget{Action: "quote", TID: a.selected.ID, FID: a.thread.Topic.FID, PID: post.post.ID}, fmt.Sprintf("%s · #%d %s", a.selected.Title, post.post.Floor, post.post.Author))
								})
							})
							markdown.View(c, post.source, a.md).FillWidth()
						})
						rows = append(rows, readingRow{Post: post.post, Bounds: row.Bounds(), Animated: post.animated})
					}
					if len(a.rendered) == 0 {
						ui.Text(c, "这一页没有可显示的回复").TextColor(c.Theme().TextMuted)
					}
				})
			})
			a.observeReadingView(c, viewport.Bounds(), rows)
			if animatedEmoteVisible(viewport.Bounds(), rows) {
				c.After(a.emoteView.nextFrameDelay())
			}
		}
		a.readerNavigation(c, p)
	})
}
func (a *reader) emptyReader(c *ui.Context, p palette) {
	ui.Column(c).Grow(1).MinHeight(0).Padding(36).Center().Gap(14).Children(func() {
		ui.Icon(c, icons["app"]).Size(31, 31).TextColor(c.Theme().TextMuted)
		heading := "从左侧选择一个话题"
		description := "帖子正文和回复会显示在这里。"
		if !a.client.HasSession() {
			heading = "登录后，开始阅读"
			description = "大漩涡需要 NGA 账号登录。"
		}
		if a.loadingTopics {
			description = "正在加载板块话题…"
		} else if a.topicErr != nil {
			description = errorMessage(a.topicErr)
		}
		ui.Text(c, heading).FontSize(20).FontWeight(500)
		ui.Text(c, description).FontSize(13).TextColor(c.Theme().TextMuted).MaxWidth(440).TextAlign(ui.Center)
		ui.Row(c).Gap(9).Margin(9, 0, 0, 0).Children(func() {
			if !a.client.HasSession() {
				ui.PrimaryButton(c, "登录 NGA").OnClick(a.openLogin)
			} else {
				ui.Button(c, "刷新话题").Disabled(a.loadingTopics).OnClick(func() { a.loadTopics(1) })
			}
			ui.Button(c, "先看演示").OnClick(a.useDemo)
		})
	})
}
func dateLabel(t time.Time) string {
	if t.Year() == time.Now().Year() && t.YearDay() == time.Now().YearDay() {
		return "今天 " + t.Format("15:04")
	}
	return t.Format("01-02 15:04")
}
