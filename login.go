package main

import (
	"context"
	_ "embed"
	"errors"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
	"ngareader/internal/local"
	"ngareader/internal/nga"
)

//go:embed resources/login-ui.js
var loginUIScript string

func (a *reader) adoptLoginCookies(values []*http.Cookie) error {
	temporary := nga.NewClient(nil)
	temporary.SetCookies(values)
	if !temporary.HasSession() {
		return errors.New("尚未检测到 NGA 登录会话。请在官方页面完成登录，成功后会自动返回。")
	}
	if !a.allowSessionChange() {
		return errors.New("正在发送或草稿保存失败，暂时不能切换登录会话")
	}
	a.client.SetCookies(temporary.Cookies())
	return nil
}

func (a *reader) loginView(c *ui.Context) {
	a.theme(c)
	ui.Column(c).Fill().AlignItems(ui.Stretch).Background(c.Theme().Background).Children(func() {
		ui.Column(c).Padding(12, 18).Gap(5).Children(func() {
			ui.Text(c, "NGA 官方登录").FontSize(16).FontWeight(500)
			ui.Text(c, "在下方官方页面登录，成功后会自动关闭此窗口并返回工作台。").FontSize(12).TextColor(c.Theme().TextMuted)
		})
		ui.Divider(c)
		if a.authView != nil && !a.authView.IsDestroyed() && !a.authLoading {
			ui.WebView(c, a.authView).Grow(1).MinHeight(0)
		} else {
			ui.Column(c).Grow(1).AlignItems(ui.Center).Justify(ui.Center).Children(func() {
				if a.authLoading {
					ui.Text(c, "正在打开 NGA 官方登录页…")
				} else {
					ui.Text(c, "NGA 官方登录页面")
				}
			})
		}
		ui.Divider(c)
		ui.Column(c).Padding(12, 18).Gap(8).Children(func() {
			if a.authError != "" {
				ui.Text(c, a.authError).FontSize(12).TextColor(c.Theme().Danger)
			}
			ui.Row(c).Gap(12).AlignItems(ui.Center).Children(func() {
				ui.Checkbox(c, &a.state.RememberLogin, "记住本机登录").Grow(1).OnChange(a.persistSessionPreference)
				ui.Button(c, "重新加载").Disabled(a.authLoading || a.authBusy).OnClick(a.reloadLogin)
				ui.PrimaryButton(c, "完成登录并返回").Disabled(a.authLoading || a.authBusy).OnClick(a.finishLogin)
			})
			ui.Text(c, "https://bbs.nga.cn  ·  等待登录，检测成功后自动返回").FontSize(11).TextColor(c.Theme().TextMuted)
		})
	})
}

// The monitor belongs to one live login window and stops on close or success.
// A cancelled attempt must never read Cookies: MyGo could otherwise create a
// hidden WebView to service a cookie request without an existing page.
func pollLogin(ctx context.Context, interval time.Duration, check func() bool) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if ctx.Err() != nil || check() {
				return
			}
		}
	}
}

type protectedKeys struct{}

func (protectedKeys) Get(key string) ([]byte, error) {
	value, err := mygo.Secrets.Service("NGAReader.portable").Get(key)
	if errors.Is(err, mygo.ErrSecretNotFound) {
		return nil, local.ErrNoSecret
	}
	return value, err
}
func (protectedKeys) Set(key string, value []byte) error {
	return mygo.Secrets.Service("NGAReader.portable").Set(key, value)
}
func (protectedKeys) Delete(key string) error {
	return mygo.Secrets.Service("NGAReader.portable").Delete(key)
}

func webviewAvailable() bool {
	if runtime.GOOS != "windows" {
		return true
	}
	for _, base := range []string{os.Getenv("ProgramFiles(x86)"), os.Getenv("ProgramFiles"), os.Getenv("LOCALAPPDATA")} {
		if base == "" {
			continue
		}
		paths, _ := filepath.Glob(filepath.Join(base, "Microsoft", "EdgeWebView", "Application", "*", "msedgewebview2.exe"))
		if len(paths) > 0 {
			return true
		}
	}
	return false
}
func (a *reader) openLogin() {
	if !a.allowSessionChange() {
		return
	}
	if a.win == nil {
		return
	}
	if a.demo {
		a.returnToLive()
	}
	if a.authWindow != nil && !a.authWindow.IsDestroyed() {
		a.authWindow.Show()
		a.authWindow.Focus()
		return
	}
	if !webviewAvailable() {
		a.settingsOpen = true
		a.notify("未检测到 WebView2。可使用阅读设置中的“导入会话 Cookie”登录；原生阅读不需要浏览器组件。")
		return
	}
	a.authError = ""
	session := mygo.NewEphemeralSession()
	a.authSession = session
	window := mygo.NewWindow(mygo.WindowOptions{Title: "NGA 官方登录", Width: 940, Height: 820, MinWidth: 680, MinHeight: 680, Parent: a.win, Content: ui.View(a.loginView)})
	a.authWindow = window
	window.OnClosed(func() {
		if a.authWindow == window {
			if a.authCancel != nil {
				a.authCancel()
				a.authCancel = nil
			}
			a.authWindow = nil
			a.authView = nil
			a.authSession = nil
			a.authLoading = false
			if a.win != nil && !a.win.IsDestroyed() {
				a.win.Invalidate()
			}
		}
	})
	view, err := window.NewWebView(mygo.WebViewOptions{Page: mygo.PageOptions{Session: session, DevTools: mygo.DevToolsDisabled, Untrusted: true, PreloadScript: loginUIScript}})
	if err != nil {
		a.authError = "无法创建官方登录页面，请关闭后重试"
		window.Invalidate()
		return
	}
	a.authView = view
	view.Page().OnWillNavigate(func(e *mygo.NavigateEvent) {
		u, err := url.Parse(e.URL)
		if err != nil || u.Scheme != "https" || u.User != nil || !loginHost(u.Hostname()) {
			e.PreventDefault()
			a.authError = "登录窗口仅允许 NGA 官方 HTTPS 页面"
			window.Invalidate()
		}
	})
	view.Page().OnDidFailLoad(func(*mygo.LoadError) {
		if a.authWindow == window {
			a.authError = "官方页面加载失败，可点击“重新加载”重试"
			window.Invalidate()
		}
	})
	a.reloadLogin()
}

func (a *reader) reloadLogin() {
	if a.authWindow == nil || a.authWindow.IsDestroyed() || a.authView == nil || a.authView.IsDestroyed() {
		return
	}
	if a.authCancel != nil {
		a.authCancel()
	}
	ctx, cancel := context.WithCancel(a.ctx)
	a.authCancel = cancel
	a.authLoading, a.authError = true, ""
	window, view := a.authWindow, a.authView
	window.Invalidate()
	go func() {
		document, err := a.client.LoginDocument(ctx)
		mygo.RunOnMain(func() {
			if ctx.Err() != nil || a.authWindow != window || window.IsDestroyed() {
				return
			}
			a.authLoading = false
			if err != nil {
				a.authError = "无法打开 NGA 官方登录页，请检查网络后重新加载"
				window.Invalidate()
				return
			}
			view.Page().LoadHTML(document, nga.LoginURL)
			window.Invalidate()
		})
		if err != nil || ctx.Err() != nil {
			return
		}
		pollLogin(ctx, time.Second, func() bool {
			stop := true
			mygo.RunOnMain(func() {
				if ctx.Err() != nil || a.authWindow != window || window.IsDestroyed() || view.IsDestroyed() {
					return
				}
				stop = a.checkLogin(false)
			})
			return stop
		})
	}()
}
func loginHost(host string) bool {
	for _, domain := range []string{"nga.cn", "ngabbs.com", "178.com", "ngacn.cc"} {
		if host == domain || strings.HasSuffix(host, "."+domain) {
			return true
		}
	}
	return false
}

func (a *reader) finishLogin() {
	a.checkLogin(true)
	if a.authWindow != nil && !a.authWindow.IsDestroyed() {
		a.authWindow.Invalidate()
	}
}

func (a *reader) checkLogin(manual bool) bool {
	if a.authBusy {
		return false
	}
	if a.authWindow == nil || a.authWindow.IsDestroyed() || a.authSession == nil {
		if manual {
			a.authError = "请先打开官方登录窗口"
		}
		return true
	}
	window, session := a.authWindow, a.authSession
	a.authBusy = true
	defer func() { a.authBusy = false }()
	cookies, err := session.Cookies.List(mygo.CookieFilter{Domain: "nga.cn"})
	// Cookies.List pumps native events while waiting. Closing/reopening the
	// login window during that wait must not adopt the previous attempt.
	if a.authWindow != window || window.IsDestroyed() || a.authSession != session {
		return true
	}
	if err != nil {
		if manual {
			a.authError = "无法读取登录状态，请在官方窗口完成登录后重试"
		}
		return false
	}
	values := make([]*http.Cookie, 0, len(cookies))
	for _, c := range cookies {
		values = append(values, &http.Cookie{Name: c.Name, Value: c.Value, Domain: c.Domain, Path: c.Path, Expires: c.Expires, Secure: c.Secure, HttpOnly: c.HTTPOnly})
	}
	if err := a.adoptLoginCookies(values); err != nil {
		if manual {
			a.authError = err.Error()
		}
		return false
	}
	a.favoriteGate.Begin(a.ctx)
	a.loadingFavorites = false
	a.persistSessionPreference()
	a.authError = ""
	if a.authCancel != nil {
		a.authCancel()
		a.authCancel = nil
	}
	if a.authWindow != nil && !a.authWindow.IsDestroyed() {
		a.authWindow.Destroy()
	}
	a.authWindow = nil
	a.authView = nil
	a.authSession = nil
	a.notify("会话已载入，正在连接论坛…")
	if a.win != nil && !a.win.IsDestroyed() {
		a.win.Show()
		a.win.Focus()
		a.win.Invalidate()
	}
	a.loadTopics(1)
	if a.importAfterLogin {
		a.importAfterLogin = false
		a.importFavoriteBoards()
	}
	a.resumeComposerAfterLogin()
	return true
}
func (a *reader) importSession() {
	if a.compose.Sending {
		a.formError = "正在发送，暂时不能切换账号"
		return
	}
	remember := a.state.RememberLogin
	// Validate before changing the active session or leaving demo mode.
	temporary := nga.NewClient(nil)
	if err := temporary.ImportCookieHeader(a.cookieInput); err != nil {
		a.formError = err.Error()
		return
	}
	if !a.allowSessionChange() {
		return
	}
	if a.demo {
		a.returnToLive()
	}
	a.state.RememberLogin = remember
	a.client.SetCookies(temporary.Cookies())
	a.favoriteGate.Begin(a.ctx)
	a.loadingFavorites = false
	a.cookieInput = ""
	a.cookieOpen = false
	a.formError = ""
	a.persistSessionPreference()
	a.notify("会话已载入，正在连接论坛…")
	a.loadTopics(1)
	if a.importAfterLogin {
		a.importAfterLogin = false
		a.importFavoriteBoards()
	}
	a.resumeComposerAfterLogin()
}
