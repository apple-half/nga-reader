package main

import (
	"fmt"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
	"ngareader/internal/local"
	"ngareader/internal/nga"
)

func dialog(c *ui.Context, open *bool, title string, body func()) {
	ui.DialogBase(c.Key(title), open, func(backdrop, panel ui.Element) {
		backdrop.Background(ui.RGBA(0, 0, 0, .45))
		panel.Width(470).MaxWidthPercent(92).Padding(23).Radius(12).Gap(17).Background(c.Theme().Background).Border(1, c.Theme().Border)
		ui.Row(c).Gap(10).Children(func() {
			ui.Text(c, title).FontSize(18).FontWeight(500).Grow(1)
			ui.Button(c, "关闭").FontSize(11).Padding(4, 7).OnClick(func() { *open = false })
		})
		body()
	})
}
func (a *reader) dialogs(c *ui.Context, p palette) {
	dialog(c, &a.settingsOpen, "阅读设置", func() {
		ui.Row(c).Gap(10).Children(func() {
			ui.Textf(c, "NGA Reader %s", appVersion).FontSize(12).Grow(1)
			ui.Button(c, "检查更新").OnClick(func() { a.checkUpdates(true) })
		})
		ui.Row(c).Gap(18).Children(func() {
			ui.Text(c, "界面主题").Grow(1)
			ui.Select(c, &a.state.Theme, []string{"深色", "浅色", "NGA经典屎黄色", "跟随系统"}).Width(185).Label("界面主题").OnChange(func() { a.applyAppearance(); a.save() })
		})
		ui.Row(c).Gap(13).Children(func() {
			ui.Text(c, "正文字号").Grow(1)
			ui.Button(c, "A−").Label("减小字号").Disabled(a.state.FontSize <= 12).OnClick(func() { a.state.FontSize--; a.save() })
			ui.Textf(c, "%d px", a.state.FontSize).Width(44).TextAlign(ui.Center)
			ui.Button(c, "A+").Label("增大字号").Disabled(a.state.FontSize >= 22).OnClick(func() { a.state.FontSize++; a.save() })
		})
		ui.Text(c, "图片默认折叠，点击正文中的“展示图片”后，在 App 内打开预览。关闭预览会释放图片引擎。").FontSize(12).TextColor(c.Theme().TextMuted).LineHeight(1.7)
		ui.Button(c, "恢复默认分栏").OnClick(a.resetLayout)
		ui.Checkbox(c, &a.state.AutoNextPage, "滚动到底部自动加载下一页").OnChange(a.save)
		ui.Divider(c)
		ui.Text(c, "NGA 账号").FontWeight(500)
		ui.Text(c, "在 NGA 官方页面登录，成功后自动关闭登录页并返回工作台。").FontSize(12).TextColor(c.Theme().TextMuted).LineHeight(1.7)
		ui.Checkbox(c, &a.state.RememberLogin, "记住本机登录（使用系统加密凭据存储）").OnChange(a.persistSessionPreference)
		ui.Row(c).Gap(9).Wrap().Children(func() {
			ui.Button(c, "登录 NGA").OnClick(func() { a.settingsOpen = false; a.openLogin() })
			ui.Button(c, "导入会话 Cookie").OnClick(func() { a.settingsOpen = false; a.cookieOpen = true; a.cookieInput = ""; a.formError = "" })
			if a.client.HasSession() {
				ui.Button(c, "退出登录").OnClick(func() { a.signOut(); a.settingsOpen = false })
			}
		})
		ui.Divider(c)
		ui.Text(c, "快捷键  Ctrl + Shift + Space  隐藏 / 恢复\n窗口关闭即退出，托盘菜单也可退出。").FontSize(12).LineHeight(1.7).TextColor(c.Theme().TextMuted)
		ui.Row(c).Gap(8).Children(func() {
			ui.Button(c, "打开数据目录").OnClick(func() {
				if a.win != nil {
					_ = mygo.Shell.OpenPath(a.dir)
				}
			})
			ui.Button(c, "查看界面演示").OnClick(func() { a.settingsOpen = false; a.useDemo() })
		})
	})
	a.boardPickerView(c)
	dialog(c, &a.linkOpen, "打开帖子链接", func() {
		input := ui.TextInput(c, &a.linkInput).Label("NGA 帖子链接").Placeholder("https://bbs.nga.cn/read.php?tid=…").MaxLength(1200).AutoFocus()
		if a.formError != "" {
			ui.Text(c, a.formError).FontSize(12).TextColor(c.Theme().Danger)
		}
		ui.Row(c).Justify(ui.End).Children(func() {
			ui.PrimaryButton(c, "打开").OnClick(a.submitLink)
		})
		if input.Submitted() {
			a.submitLink()
		}
	})
	dialog(c, &a.cookieOpen, "导入本机会话", func() {
		ui.Text(c, "从自己已登录的 NGA 浏览器会话中复制 Cookie 值，在这里本地导入。它可以访问你的账号，请勿发到聊天或交给他人。").FontSize(12).TextColor(c.Theme().TextMuted).LineHeight(1.7)
		ui.TextInput(c, &a.cookieInput).Label("会话 Cookie").Password().Placeholder("ngaPassportUid=…; ngaPassportCid=…").MaxLength(16384).AutoFocus()
		ui.Checkbox(c, &a.state.RememberLogin, "记住本机登录")
		if a.formError != "" {
			ui.Text(c, a.formError).FontSize(12).TextColor(c.Theme().Danger)
		}
		ui.Row(c).Justify(ui.End).Children(func() { ui.PrimaryButton(c, "导入并验证").OnClick(a.importSession) })
	})
}
func (a *reader) applyAppearance() {
	if a.win == nil {
		return
	}
	source := mygo.ThemeSystem
	if a.state.Theme == "深色" {
		source = mygo.ThemeDark
	} else if a.state.Theme == "浅色" || a.state.Theme == "NGA经典屎黄色" {
		source = mygo.ThemeLight
	}
	mygo.Theme.SetSource(source)
}
func (a *reader) persistSessionPreference() {
	if a.keys == nil {
		return
	}
	if a.state.RememberLogin && a.client.HasSession() {
		if err := local.SaveSession(a.dir, a.client.Cookies(), a.keys); err != nil {
			a.state.RememberLogin = false
			a.notify(err.Error())
		}
	}
	if !a.state.RememberLogin {
		if err := local.ClearSession(a.dir, a.keys); err != nil {
			a.notify(err.Error())
		}
	}
	a.save()
}
func (a *reader) signOut() {
	if !a.allowSessionChange() {
		return
	}
	a.favoriteGate.Begin(a.ctx)
	a.loadingFavorites = false
	a.topicGate.Begin(a.ctx)
	a.threadGate.Begin(a.ctx)
	a.resetReadingSession()
	a.client.SetCookies(nil)
	a.state.RememberLogin = false
	if a.keys != nil {
		if err := local.ClearSession(a.dir, a.keys); err != nil {
			a.notify(err.Error())
		}
	}
	if a.authWindow != nil && !a.authWindow.IsDestroyed() {
		a.authWindow.Destroy()
	}
	a.selected = nga.Topic{}
	a.thread = nga.ThreadPage{}
	a.threadErr = nil
	a.resetMarkdown()
	a.save()
	a.loadTopics(1)
}
func (a *reader) openOriginal() {
	if a.win != nil && a.selected.ID > 0 && !a.demo {
		if err := mygo.Shell.OpenExternal(fmt.Sprintf("https://bbs.nga.cn/read.php?tid=%d&page=%d", a.selected.ID, max(1, a.thread.Page))); err != nil {
			a.notify("无法打开原帖")
		}
	}
}

func (a *reader) openComposerOriginal() {
	if a.win == nil || a.demo || a.compose.Sending {
		return
	}
	target := a.compose.Draft.Target
	address := fmt.Sprintf("https://bbs.nga.cn/thread.php?fid=%d", target.FID)
	if target.TID > 0 {
		address = fmt.Sprintf("https://bbs.nga.cn/read.php?tid=%d&page=e", target.TID)
	}
	if err := mygo.Shell.OpenExternal(address); err != nil {
		a.compose.Error = "无法打开官方页面"
	}
}
