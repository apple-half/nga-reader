package main

import (
	"fmt"

	"github.com/egoist/mygo/ui"
)

func (a *reader) updatesView(c *ui.Context) {
	panel := ui.DialogBase(c.Key("github-updates"), &a.updates.Open, func(backdrop, panel ui.Element) {
		backdrop.Background(ui.RGBA(0, 0, 0, .5))
		panel.Width(650).MaxWidthPercent(94).Height(500).MaxHeightPercent(92).Padding(22).Radius(14).Gap(14).Background(c.Theme().Background).Border(1, c.Theme().Border)
		s := &a.updates
		busy := s.Installing || s.Restarting
		ui.Row(c).Gap(10).Children(func() {
			ui.Text(c, "软件更新").FontSize(19).FontWeight(500).Grow(1)
			ui.Button(c, "关闭更新").Disabled(busy).OnClick(func() { s.Open = false })
		})
		ui.Textf(c, "当前版本 %s · GitHub / apple-half/nga-reader", appVersion).FontSize(11).TextColor(c.Theme().TextMuted)
		ui.Scroll(c.Key("update-notes")).Grow(1).MinHeight(0).FillWidth().Children(func() {
			ui.Column(c).FillWidth().Gap(12).Children(func() {
				if s.Offer != nil {
					info := s.Offer.Info()
					label := "新版本 " + info.Version
					if info.Prerelease {
						label += " · 预发布"
					}
					ui.Text(c, label).FontSize(17).FontWeight(500)
					notes := []rune(info.Notes)
					if len(notes) > 15000 {
						notes = append(notes[:15000], []rune("\n更多说明请查看发布页。")...)
					}
					ui.Text(c, string(notes)).FontSize(13).LineHeight(1.65).Selectable()
					if !info.Installable {
						ui.Text(c, info.Reason).FontSize(12).LineHeight(1.6).TextColor(c.Theme().TextMuted)
					}
				}
				if s.Message != "" {
					ui.Text(c, s.Message).FontSize(12).LineHeight(1.6).TextColor(c.Theme().TextMuted)
				}
				if s.Total > 0 {
					fraction := min(1, max(0, float64(s.Downloaded)/float64(s.Total)))
					ui.Box(c).FillWidth().Height(5).Radius(3).Background(c.Theme().Border).Children(func() {
						ui.Box(c).WidthPercent(float32(fraction * 100)).Height(5).Radius(3).Background(c.Theme().Accent)
					})
					ui.Text(c, fmt.Sprintf("%.1f / %.1f MB", float64(s.Downloaded)/(1<<20), float64(s.Total)/(1<<20))).FontSize(12).TextColor(c.Theme().TextMuted)
				}
				if s.Error != "" {
					ui.Text(c, s.Error).FontSize(12).LineHeight(1.6).TextColor(c.Theme().Danger).Selectable()
				}
				if a.compose.Sending {
					ui.Text(c, "正在发表帖子，请等发送结束后再更新。").FontSize(12).TextColor(c.Theme().TextMuted)
				}
				if a.client.HasSession() && !a.state.RememberLogin {
					ui.Text(c, "当前没有记住本机登录，重启后需要重新登录；已有草稿和设置会保留。").FontSize(12).LineHeight(1.6).TextColor(c.Theme().TextMuted)
				}
			})
		})
		ui.Checkbox(c, &a.state.AutoCheckUpdates, "启动时自动检查更新").Disabled(busy || a.demo).OnChange(a.save)
		ui.Divider(c)
		ui.Row(c).Gap(8).Wrap().Children(func() {
			ui.Button(c, "查看发布页").Disabled(busy || a.demo).OnClick(a.openUpdateRelease)
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
	if panel.Dismissed() && (a.updates.Installing || a.updates.Restarting) {
		a.updates.Open = true
	}
}
