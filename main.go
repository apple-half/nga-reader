package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
	"ngareader/internal/local"
	"ngareader/internal/nga"
)

const appVersion = "0.1.10"

func main() {
	licenses := flag.Bool("licenses", false, "输出程序内嵌的第三方许可后退出")
	demo := flag.Bool("demo", false, "打开明确标注的离线演示，不连接账号")
	check := flag.Bool("check-api", false, "检查未登录 API，输出不含凭据的结果后退出")
	directory := flag.String("data-dir", "", "指定本地数据目录（默认 EXE 同级 data）")
	flag.Parse()
	if *licenses {
		fmt.Print(thirdPartyNotices)
		return
	}
	exe, err := os.Executable()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return
	}
	dir := *directory
	if dir == "" {
		configRoot := ""
		if runtime.GOOS == "darwin" {
			configRoot, err = os.UserConfigDir()
			if err != nil {
				fmt.Fprintln(os.Stderr, "无法确定 macOS 用户数据目录：", err)
				return
			}
		}
		dir, err = defaultDataDirectory(exe, runtime.GOOS, configRoot)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			return
		}
	}
	dir, err = filepath.Abs(dir)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return
	}
	mygo.App.SetName("NGAReader")
	var a *reader
	if *check {
		a = newReader(dir)
	} else {
		if !mygo.App.RequestSingleInstanceLock() {
			return
		}
		var releaseOwner func()
		a, releaseOwner, err = openOwnedReader(dir)
		if err != nil {
			mygo.App.WhenReady(func() { mygo.Dialog.Error("无法打开 NGA Reader", err.Error()); mygo.App.Quit() })
			if runErr := mygo.App.Run(); runErr != nil {
				fmt.Fprintln(os.Stderr, runErr)
			}
			return
		}
		defer releaseOwner()
		mygo.App.OnSecondInstance(func([]string, string) {
			if a.win != nil && !a.win.IsDestroyed() {
				a.win.Show()
				a.win.Focus()
			}
		})
	}
	if *check {
		ctx, cancel := context.WithTimeout(context.Background(), 25*time.Second)
		defer cancel()
		_, err := a.client.Topics(ctx, -7, 1)
		kind := "ok"
		message := "接口返回成功"
		var api *nga.APIError
		if err != nil {
			message = err.Error()
			kind = "network"
			if errors.As(err, &api) {
				kind = api.Kind
			}
		}
		raw, _ := json.Marshal(map[string]string{"status": kind, "message": message})
		fmt.Println(string(raw))
		a.cancel()
		return
	}
	mygo.App.SetVersion(appVersion)
	mygo.App.SetMenu(nil)
	mygo.App.SetPath(mygo.PathUserData, filepath.Join(dir, "runtime"))
	mygo.App.SetPath(mygo.PathCache, filepath.Join(dir, "cache"))
	mygo.App.SetPath(mygo.PathLogs, filepath.Join(dir, "logs"))
	a.keys = protectedKeys{}
	if *demo {
		a.useDemo()
	} else if a.state.RememberLogin {
		cookies, err := local.LoadSession(dir, a.keys)
		if err != nil {
			a.notify(err.Error())
		} else {
			a.client.SetCookies(cookies)
		}
	}
	source := mygo.ThemeDark
	if a.state.Theme == "浅色" || a.state.Theme == "NGA经典屎黄色" {
		source = mygo.ThemeLight
	} else if a.state.Theme == "跟随系统" {
		source = mygo.ThemeSystem
	}
	mygo.Theme.SetSource(source)
	mygo.App.WhenReady(func() {
		a.win = mygo.NewWindow(mygo.WindowOptions{Title: "工作台", Width: 1280, Height: 820, MinWidth: 980, MinHeight: 620, TitleBarStyle: mygo.TitleBarHidden, TitleBarHeight: 44, BackgroundColor: "#202020", Content: ui.View(a.view)})
		_ = a.win.SetIcon(windowIcon())
		a.win.OnClose(func(event *mygo.CloseEvent) {
			if !a.canCloseForUpdate() {
				event.PreventDefault()
				return
			}
			a.shutdown()
		})
		a.updaterEngine, err = newUpdateEngine()
		if err != nil {
			a.notify("自动更新配置无效，仍可手动下载新版")
		}
		a.startUpdateChecks()
		if err := mygo.GlobalShortcut.Register("Ctrl+Shift+Space", a.toggleHidden); err != nil {
			a.notify("隐藏快捷键已被占用，可点击底栏隐藏，通过托盘恢复")
		}
		tray, err := mygo.NewTray(mygo.TrayOptions{Icon: windowIcon(), ToolTip: "工作台", Menu: mygo.NewMenu([]*mygo.MenuItem{{Label: "显示工作台", Click: func(*mygo.MenuItem, *mygo.Window) { a.win.Show(); a.win.Focus() }}, {Label: "退出", Click: func(*mygo.MenuItem, *mygo.Window) { a.shutdown(); mygo.App.Quit() }}})})
		if err == nil {
			a.tray = tray
		}
		if !a.demo {
			a.loadTopics(1)
		}
	})
	mygo.App.OnBeforeQuit(func(event *mygo.QuitEvent) {
		if !a.canCloseForUpdate() {
			event.PreventDefault()
		}
	})
	mygo.App.OnQuit(a.shutdown)
	if err := mygo.App.Run(); err != nil {
		fmt.Fprintln(os.Stderr, "无法启动工作台：", err)
	}
}
