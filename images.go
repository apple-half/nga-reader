package main

import (
	"net/url"
	"strings"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/markdown/parser"
	"ngareader/internal/nga"
)

func (a *reader) imageLinkBlock(c *ui.Context, d *parser.Document, block int) bool {
	if d.Block(block).Kind != parser.Paragraph {
		return false
	}
	in := d.Inlines(block)
	var addresses []string
	for k := 0; k < len(in); k = in[k].Next {
		n := in[k]
		if n.Kind == parser.Link && parser.PlainText(in[k+1:n.Next]) == "展示图片" {
			address, err := nga.ImageURL(n.Dest)
			if err != nil {
				return false
			}
			addresses = append(addresses, address)
		} else if n.Kind != parser.Text || strings.TrimSpace(n.Text) != "" {
			return false
		}
	}
	if len(addresses) == 0 {
		return false
	}
	for _, address := range addresses {
		ui.Row(c).FillWidth().AlignItems(ui.Center).Gap(12).Padding(12).Radius(7).Background(c.Theme().Surface).Border(1, c.Theme().Border).Children(func() {
			ui.Text(c, "图片已折叠").FontSize(12).TextColor(c.Theme().TextMuted).Grow(1)
			ui.Button(c, "展示图片").OnClick(func() { a.openPicture(address) })
		})
	}
	return true
}

// openPicture is reached only by an explicit image-link click. The ephemeral
// WebView handles the CDN's official verification itself; no credentials,
// verification cookies or browser-profile data pass through the Go client.
func (a *reader) openPicture(address string) {
	safe, err := nga.ImageURL(address)
	if err != nil {
		a.notify("图片地址暂不支持")
		return
	}
	a.pictureAddress, a.pictureError = safe, ""
	if a.win == nil || a.win.IsDestroyed() {
		return
	}
	if a.pictureWindow != nil && !a.pictureWindow.IsDestroyed() {
		if a.pictureWeb != nil && !a.pictureWeb.IsDestroyed() {
			_ = a.pictureWeb.Page().LoadURL(safe)
		}
		a.pictureWindow.Show()
		a.pictureWindow.Focus()
		a.pictureWindow.Invalidate()
		return
	}
	window := mygo.NewWindow(mygo.WindowOptions{Title: "NGA 图片预览", Width: 900, Height: 820, MinWidth: 600, MinHeight: 480, Parent: a.win, Content: ui.View(a.pictureView)})
	a.pictureWindow = window
	window.OnClosed(func() {
		if a.pictureWindow == window {
			a.pictureWindow, a.pictureWeb = nil, nil
			a.pictureAddress, a.pictureError = "", ""
		}
	})
	if !webviewAvailable() {
		a.pictureError = "未检测到系统 WebView2，可点击“浏览器打开”查看这张图片。"
		window.Invalidate()
		return
	}
	web, err := window.NewWebView(mygo.WebViewOptions{Page: mygo.PageOptions{Session: mygo.NewEphemeralSession(), Untrusted: true, DevTools: mygo.DevToolsDisabled, PinchZoom: true}})
	if err != nil {
		a.pictureError = "无法打开图片预览，请关闭后重新点击“展示图片”，或使用“浏览器打开”。"
		window.Invalidate()
		return
	}
	a.pictureWeb = web
	web.Page().OnWillNavigate(func(e *mygo.NavigateEvent) {
		if !pictureNavigationAllowed(e.URL) {
			e.PreventDefault()
			a.pictureError = "已阻止跳转到 NGA 图床以外的地址。"
			window.Invalidate()
		}
	})
	web.Page().OnDidFailLoad(func(*mygo.LoadError) {
		if a.pictureWindow == window {
			a.pictureError = "图片加载失败，请重试；也可以点击“浏览器打开”。"
			window.Invalidate()
		}
	})
	_ = web.Page().LoadURL(safe)
	window.Invalidate()
}

func pictureNavigationAllowed(address string) bool {
	u, err := url.Parse(address)
	if err != nil || u.Scheme != "https" {
		return false
	}
	_, err = nga.ImageURL(address)
	return err == nil
}

func (a *reader) pictureView(c *ui.Context) {
	a.theme(c)
	ui.Column(c).Fill().AlignItems(ui.Stretch).Background(c.Theme().Background).Children(func() {
		ui.Column(c).Padding(12, 16).Gap(5).Children(func() {
			ui.Text(c, "图片预览").FontSize(16).FontWeight(500)
			ui.Text(c, "如出现 NGA 图床验证，请在此窗口完成。关闭窗口后返回阅读。").FontSize(12).TextColor(c.Theme().TextMuted)
		})
		ui.Divider(c)
		if a.pictureWeb != nil && !a.pictureWeb.IsDestroyed() {
			ui.WebView(c, a.pictureWeb).Grow(1).MinHeight(0)
		} else {
			ui.Column(c).Grow(1).Center().Children(func() { ui.Text(c, "图片将在这里显示").TextColor(c.Theme().TextMuted) })
		}
		ui.Divider(c)
		ui.Column(c).Padding(12, 16).Gap(9).Children(func() {
			if a.pictureError != "" {
				ui.Text(c, a.pictureError).FontSize(12).TextColor(c.Theme().Danger)
			}
			ui.Row(c).Gap(8).Wrap().Children(func() {
				ui.Button(c, "重新加载").Disabled(a.pictureWeb == nil).OnClick(func() {
					if a.pictureWeb != nil && !a.pictureWeb.IsDestroyed() {
						a.pictureError = ""
						_ = a.pictureWeb.Page().LoadURL(a.pictureAddress)
					}
				})
				ui.Button(c, "浏览器打开").Disabled(a.pictureAddress == "").OnClick(func() {
					if safe, err := nga.ImageURL(a.pictureAddress); err == nil {
						_ = mygo.Shell.OpenExternal(safe)
					}
				})
				ui.Spacer(c).Grow(1)
				ui.PrimaryButton(c, "关闭预览").OnClick(func() {
					if a.pictureWindow != nil && !a.pictureWindow.IsDestroyed() {
						a.pictureWindow.Destroy()
					}
				})
			})
		})
	})
}
