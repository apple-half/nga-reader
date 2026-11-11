package main

import (
	"fmt"
	"unicode/utf8"

	"github.com/egoist/mygo/ui"
	"ngareader/internal/nga"
)

func (a *reader) composerView(c *ui.Context) {
	a.checkComposerDestination()
	panel := ui.DialogBase(c.Key("text-composer"), &a.compose.Open, func(backdrop, panel ui.Element) {
		backdrop.Background(ui.RGBA(0, 0, 0, .5))
		panel.Width(720).MaxWidthPercent(94).Height(566).MaxHeightPercent(92).Padding(22).Radius(14).Gap(13).Background(c.Theme().Background).Border(1, c.Theme().Border)
		s := &a.compose
		locked := s.Sending || s.Preparing || s.Draft.Uncertain
		title, sendLabel := "回复帖子", "发送回复"
		if s.Draft.Target.Action == "new" {
			title = "写一篇新帖"
			sendLabel = "发表帖子"
		} else if s.Draft.Target.Action == "quote" {
			title = "引用回复"
		}
		ui.Row(c).Gap(10).Children(func() {
			ui.Text(c, title).FontSize(19).FontWeight(500).Grow(1)
			ui.Button(c, "保存并关闭").FontSize(11).Disabled(s.Sending).OnClick(func() { a.closeComposer() })
		})
		ui.Scroll(c.Key("compose-content")).Grow(1).MinHeight(0).FillWidth().Children(func() {
			ui.Column(c).FillWidth().Gap(12).Children(func() {
				identity := fmt.Sprintf("NGA 账号 · UID %d", s.Draft.UID)
				if a.demo {
					identity = "离线演示 · 不会发表内容"
				}
				ui.Text(c, identity).FontSize(11).TextColor(c.Theme().TextMuted)
				if s.Draft.Target.Action == "new" {
					choices := []string{s.BoardChoice}
					for _, b := range a.state.Boards {
						label := a.composeBoardLabel(b.ID)
						if label != s.BoardChoice {
							choices = append(choices, label)
						}
					}
					ui.Select(c.Key("compose-board"), &s.BoardChoice, choices).Label("发布板块").FillWidth().Disabled(locked).OnChange(func() {
						for _, b := range a.state.Boards {
							if a.composeBoardLabel(b.ID) == s.BoardChoice {
								a.openComposer(nga.PostTarget{Action: "new", FID: b.ID}, b.Name)
								break
							}
						}
					})
					ui.TextInput(c.Key("compose-title"), &s.Draft.Subject).Label("帖子标题").Placeholder("给这篇帖子起个标题…").MaxLength(nga.MaxPostSubjectRunes).Disabled(locked).FillWidth().OnChange(func() { a.saveComposer() })
					categories := []string{"不加分类"}
					for _, item := range s.Categories {
						categories = append(categories, item)
					}
					if s.Draft.Category != "" {
						found := false
						for _, item := range categories {
							if item == s.Draft.Category {
								found = true
							}
						}
						if !found {
							categories = append(categories, s.Draft.Category)
						}
					}
					ui.Select(c.Key("compose-category"), &s.CategoryChoice, categories).Label("主题分类（可选）").Disabled(locked).FillWidth().OnChange(func() {
						s.Draft.Category = s.CategoryChoice
						if s.CategoryChoice == "不加分类" {
							s.Draft.Category = ""
						}
						a.saveComposer()
					})
				} else {
					ui.Text(c, s.Draft.Label).FontSize(13).FontWeight(500).LineHeight(1.5)
					ui.Text(c, s.BoardChoice).FontSize(11).TextColor(c.Theme().TextMuted)
				}
				ui.TextArea(c.Key("compose-body"), &s.Draft.Content).Label("正文").Placeholder("写下你想说的话…\n支持文字和 NGA BBCode，不上传图片。").Height(210).FillWidth().MaxLength(nga.MaxPostContentRunes).Disabled(locked).OnChange(func() { a.saveComposer() })
				ui.Row(c).Gap(8).Children(func() {
					hint := "Enter 换行 · 草稿自动保存到本机"
					if a.demo {
						hint = "Enter 换行 · 离线演示"
					}
					ui.Text(c, hint).FontSize(11).TextColor(c.Theme().TextMuted).Grow(1)
					ui.Textf(c, "%d / %d", utf8.RuneCountInString(s.Draft.Content), nga.MaxPostContentRunes).FontSize(11).TextColor(c.Theme().TextMuted)
				})
				if s.Preparing {
					ui.Text(c, "正在检查发言权限和读取发布信息…").FontSize(12).TextColor(c.Theme().TextMuted)
				}
				if s.Info != "" {
					ui.Text(c, s.Info).FontSize(12).LineHeight(1.6).TextColor(c.Theme().TextMuted)
				}
				if s.Error != "" {
					ui.Text(c, s.Error).FontSize(12).LineHeight(1.6).TextColor(c.Theme().Danger).Selectable()
				}
				if s.SaveError != "" {
					ui.Text(c, s.SaveError).FontSize(12).TextColor(c.Theme().Danger).Selectable()
				}
				if s.Draft.Uncertain && !s.Sending {
					ui.Checkbox(c, &s.RetryAcknowledged, "我已检查原帖或板块，确认没有发表").FontSize(12)
					ui.Button(c, "解除重复发送保护").Disabled(!s.RetryAcknowledged).OnClick(a.acknowledgeComposer)
					ui.Button(c, "确认已发表，清理草稿").OnClick(a.confirmComposerPublished)
				}
			})
		})
		ui.Divider(c)
		ui.Row(c).Gap(8).Wrap().Children(func() {
			ui.Button(c, "丢弃草稿").Disabled(s.Sending || s.Draft.Uncertain).OnClick(a.discardComposer)
			if !a.demo {
				ui.Button(c, "查看原帖或板块").Disabled(s.Sending).OnClick(a.openComposerOriginal)
				if !s.Ready && !s.Preparing {
					ui.Button(c, "重新准备").Disabled(s.Sending).OnClick(a.prepareComposer)
				}
				if s.Error != "" {
					ui.Button(c, "登录 NGA").Disabled(s.Sending).OnClick(a.loginForComposer)
				}
			}
			ui.Spacer(c).Grow(1)
			if s.Sending {
				sendLabel = "正在发送…"
			}
			ui.PrimaryButton(c, sendLabel).Disabled(a.demo || locked || !s.Ready).OnClick(a.sendComposer)
		})
	})
	if panel.Dismissed() {
		a.compose.Open = true
		a.closeComposer()
	}
}
