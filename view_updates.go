package main

import (
	"fmt"

	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/markdown"
)

type updateNotesCache struct {
	raw, body string
	markdown  *markdown.Options
}

func (a *reader) updateChangelog(c *ui.Context, raw string) {
	notes := &a.updates.notes
	if notes.markdown == nil {
		// No image loader or HTML renderer: update notes cannot request images
		// or execute a web page. Native links retain MyGo's safe-scheme checks.
		notes.markdown = &markdown.Options{WrapCode: true, HTML: func(*ui.Context, string) {}}
	}
	if raw != notes.raw || notes.body == "" {
		notes.raw, notes.body = raw, compactUpdateNotes(raw)
	}
	notes.markdown.Theme = &markdown.Theme{FontSize: 13, Text: c.Theme().Text, Muted: c.Theme().TextMuted, Link: c.Theme().Accent, Strong: c.Theme().Text, StrongWeight: 500, LineHeight: 1.65, BlockSpacing: .7}
	markdown.View(c, notes.body, notes.markdown).FillWidth()
}

func (a *reader) updatesView(c *ui.Context) {
	s := &a.updates
	busy := s.Installing || s.Restarting
	showNotes := s.Offer != nil && !busy && !s.ReadyToRestart
	panel := ui.DialogBase(c.Key("github-updates"), &s.Open, func(backdrop, panel ui.Element) {
		backdrop.Background(ui.RGBA(0, 0, 0, .5))
		height := float32(300)
		if s.Offer != nil {
			height = 370
		}
		if showNotes {
			height = 480
		}
		panel.Width(620).MaxWidthPercent(94).Height(height).MaxHeightPercent(92).Padding(22).Radius(14).Gap(12).Background(c.Theme().Background).Border(1, c.Theme().Border)
		ui.Row(c).AlignItems(ui.Center).Gap(10).Children(func() {
			ui.Text(c, "软件更新").FontSize(19).FontWeight(500).Grow(1)
			ui.Button(c, "关闭更新").Disabled(busy).OnClick(func() { s.Open = false })
		})
		if s.Offer != nil {
			info := s.Offer.Info()
			ui.Row(c).AlignItems(ui.Center).Gap(10).Children(func() {
				ui.Text(c, appVersion+"  →").FontSize(14).TextColor(c.Theme().TextMuted)
				ui.Text(c, info.Version).FontSize(22).FontWeight(500)
				if info.Prerelease {
					ui.Text(c, "预发布").FontSize(11).Padding(3, 7).Radius(5).Background(c.Theme().Surface).TextColor(c.Theme().TextMuted)
				}
			})
		} else {
			ui.Text(c, "当前版本 "+appVersion).FontSize(12).TextColor(c.Theme().TextMuted)
		}
		if showNotes {
			ui.Text(c, "本次更新").FontSize(13).FontWeight(500)
			ui.Scroll(c.Key("update-notes-" + s.Offer.Info().Version)).Grow(1).MinHeight(0).FillWidth().Children(func() {
				a.updateChangelog(c, s.Offer.Info().Notes)
			})
		} else {
			ui.Spacer(c).Grow(1)
		}
		// Keep status and actions outside the changelog's scroll container.
		ui.Column(c).FillWidth().Gap(6).Children(func() {
			if s.Offer != nil && !s.Offer.Info().Installable {
				ui.Text(c, s.Offer.Info().Reason).FontSize(12).LineHeight(1.5).MaxLines(3).TextColor(c.Theme().TextMuted)
			}
			if s.Message != "" {
				ui.Text(c, s.Message).FontSize(12).LineHeight(1.5).MaxLines(2).TextColor(c.Theme().TextMuted)
			}
			if s.Total > 0 {
				fraction := min(1, max(0, float64(s.Downloaded)/float64(s.Total)))
				ui.Box(c).FillWidth().Height(5).Radius(3).Background(c.Theme().Border).Children(func() {
					ui.Box(c).WidthPercent(float32(fraction * 100)).Height(5).Radius(3).Background(c.Theme().Accent)
				})
				ui.Text(c, fmt.Sprintf("%.1f / %.1f MB", float64(s.Downloaded)/(1<<20), float64(s.Total)/(1<<20))).FontSize(12).TextColor(c.Theme().TextMuted)
			}
			if s.Error != "" {
				ui.Text(c, s.Error).FontSize(12).LineHeight(1.5).MaxLines(3).TextColor(c.Theme().Danger).Selectable().Tooltip(s.Error)
			}
			if a.compose.Sending {
				ui.Text(c, "正在发表帖子，请等发送结束后再更新。").FontSize(12).TextColor(c.Theme().TextMuted)
			}
			if s.Offer != nil {
				hint := "更新会保留设置、收藏、阅读记录和草稿。"
				if a.client.HasSession() && !a.state.RememberLogin {
					hint += "当前登录仅本次有效，重启后需要重新登录。"
				}
				ui.Text(c, hint).FontSize(11).LineHeight(1.5).TextColor(c.Theme().TextMuted)
			}
		})
		ui.Divider(c)
		ui.Checkbox(c, &a.state.AutoCheckUpdates, "启动时自动检查更新").Disabled(busy || a.demo).OnChange(a.save)
		ui.Row(c).AlignItems(ui.Center).Gap(8).Wrap().Children(func() {
			label := "打开下载页"
			if s.Offer != nil {
				label = "完整更新说明"
				if !s.Offer.Info().Installable {
					label = "手动下载"
				}
			}
			ui.Button(c, label).Disabled(busy || a.demo).OnClick(a.openUpdateRelease)
			ui.Button(c, "重新检查").Disabled(s.Checking || busy || s.ReadyToRestart || a.demo).OnClick(func() { a.checkUpdates(true) })
			ui.Spacer(c).Grow(1)
			if s.ReadyToRestart {
				ui.PrimaryButton(c, "重启使用新版").Disabled(busy || a.compose.Sending).OnClick(a.restartUpdate)
			} else if s.Offer != nil {
				label := "下载并安装"
				if s.Installing {
					label = "正在安装…"
				}
				ui.PrimaryButton(c, label).Disabled(s.Installing || a.compose.Sending || !s.Offer.Info().Installable || a.demo).OnClick(a.installUpdate)
			}
		})
	})
	if panel.Dismissed() && busy {
		s.Open = true
	}
}
