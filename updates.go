package main

import (
	"context"
	_ "embed"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/netproxy"
	"ngareader/internal/appupdate"
	"ngareader/internal/local"
)

//go:embed resources/update-public.txt
var updatePublicKey string

type updateInfo struct {
	Version, Notes, ReleaseURL, Reason string
	Prerelease, Installable            bool
}
type updateOffer interface {
	Info() updateInfo
	Install(context.Context, func(int64, int64)) error
}
type updateEngine interface {
	Check(context.Context) (updateOffer, error)
}
type updateState struct {
	Open, Checking, Installing, ReadyToRestart bool
	Offer                                      updateOffer
	Error, Message                             string
	Downloaded, Total                          int64
}
type nativeUpdateEngine struct{ client *appupdate.Client }
type nativeUpdateOffer struct {
	info   updateInfo
	update *mygo.Update
}

func (o *nativeUpdateOffer) Info() updateInfo { return o.info }
func (o *nativeUpdateOffer) Install(ctx context.Context, progress func(int64, int64)) error {
	if o.update == nil {
		return fmt.Errorf("当前程序位置不支持自动安装，请下载便携版更新")
	}
	if runtime.GOOS == "windows" {
		executable, err := os.Executable()
		if err != nil {
			return err
		}
		if err := appupdate.ValidateWindowsManagedFiles(filepath.Dir(executable)); err != nil {
			return err
		}
	}
	return o.update.Install(ctx, progress)
}
func newUpdateEngine() (updateEngine, error) {
	pub, err := appupdate.PublicKey(updatePublicKey)
	if err != nil {
		return nil, err
	}
	return &nativeUpdateEngine{client: appupdate.NewClient(&http.Client{Transport: netproxy.Transport(), Timeout: 2 * time.Minute}, pub)}, nil
}
func installationReason(platform, executable string, enabled bool) string {
	if platform == "windows" && !strings.EqualFold(filepath.Base(executable), "NGAReader.exe") {
		return "程序名称已更改，请使用新版便携包内的NGAReader.exe更新。"
	}
	if !enabled {
		return "当前构建或程序位置不支持自动安装，请将正式便携版放在可写目录后使用；也可手动下载新版。"
	}
	return ""
}
func (e *nativeUpdateEngine) Check(ctx context.Context) (updateOffer, error) {
	offer, err := e.client.Latest(ctx, appVersion, runtime.GOOS+"-"+runtime.GOARCH)
	if err != nil || offer == nil {
		return nil, err
	}
	executable, _ := os.Executable()
	reason := installationReason(runtime.GOOS, executable, mygo.Updater.Enabled())
	native := &nativeUpdateOffer{info: updateInfo{Version: offer.Manifest.Version, Notes: offer.Manifest.Notes, Prerelease: offer.Prerelease, ReleaseURL: "https://github.com/" + appupdate.Repository + "/releases/tag/v" + offer.Manifest.Version, Reason: reason, Installable: reason == ""}}
	if reason == "" {
		mygo.Updater.SetSource(appupdate.Source{Client: e.client, Offer: *offer})
		mygo.Updater.SetPrereleases(true)
		native.update, err = mygo.Updater.Check(ctx)
		if err != nil {
			return nil, err
		}
		if native.update == nil || native.update.Version != offer.Manifest.Version {
			return nil, fmt.Errorf("更新版本在检查时发生变化，请重新检查")
		}
	}
	return native, nil
}
func (a *reader) updateTask(fn func()) {
	if a.runUpdateTask != nil {
		a.runUpdateTask(fn)
	} else {
		go fn()
	}
}
func (a *reader) checkUpdates(manual bool) {
	if manual {
		a.updates.Open = true
		a.settingsOpen = false
	}
	if a.demo {
		a.updates.Message = "离线演示模式不检查或安装更新。"
		return
	}
	if a.updates.Checking || a.updates.Installing || a.updates.ReadyToRestart {
		return
	}
	if a.updaterEngine == nil {
		a.updates.Error = "此构建未配置自动更新，请使用正式便携版。"
		return
	}
	ctx, token := a.updateGate.Begin(a.ctx)
	ctx, cancel := context.WithTimeout(ctx, 45*time.Second)
	a.updates.Checking = true
	a.updates.Error = ""
	a.updates.Message = "正在检查GitHub上的新版本…"
	a.updateTask(func() {
		defer cancel()
		offer, err := a.updaterEngine.Check(ctx)
		a.update(func() {
			if !a.updateGate.Accept(token) {
				return
			}
			a.updates.Checking = false
			a.updates.Offer = nil
			if err != nil {
				a.updates.Error = "检查更新失败：" + err.Error()
				a.updates.Message = "当前版本可以继续使用。"
				return
			}
			a.state.LastUpdateCheck = time.Now()
			a.save()
			a.updates.Offer = offer
			if offer == nil {
				a.updates.Message = "当前没有更高版本的可用签名更新。"
				return
			}
			a.updates.Message = "发现新版本，点击“下载并安装”后才会更新。"
			if !manual {
				a.notify("发现新版" + offer.Info().Version + "，可从侧栏“软件更新”查看")
			}
		})
	})
}
func (a *reader) shouldCheckUpdates(now time.Time) bool {
	return a.state.AutoCheckUpdates && !a.demo && !a.closing && !a.updates.Checking && !a.updates.Installing && !a.updates.ReadyToRestart && (a.state.LastUpdateCheck.IsZero() || a.state.LastUpdateCheck.After(now) || now.Sub(a.state.LastUpdateCheck) >= 24*time.Hour)
}
func (a *reader) startUpdateChecks() {
	go func() {
		timer := time.NewTimer(10 * time.Second)
		defer timer.Stop()
		for {
			select {
			case <-a.ctx.Done():
				return
			case <-timer.C:
				a.update(func() {
					if a.shouldCheckUpdates(time.Now()) {
						a.checkUpdates(false)
					}
				})
				timer.Reset(24 * time.Hour)
			}
		}
	}()
}
func (a *reader) saveBeforeUpdate() bool {
	if a.compose.Sending {
		a.updates.Error = "正在发表帖子，请等发送结束后再更新。"
		return false
	}
	if a.compose.Open && !a.closeComposer() {
		a.updates.Error = "草稿未能保存，已停止更新。"
		return false
	}
	if err := local.Save(a.dir, a.state); err != nil {
		a.updates.Error = "设置未能保存，已停止更新：" + err.Error()
		return false
	}
	if a.state.RememberLogin && a.client.HasSession() {
		if a.keys == nil {
			a.updates.Error = "无法保存记住的登录，已停止更新。"
			return false
		}
		if err := local.SaveSession(a.dir, a.client.Cookies(), a.keys); err != nil {
			a.updates.Error = "登录状态未能保存，已停止更新：" + err.Error()
			return false
		}
	}
	return true
}
func (a *reader) installUpdate() {
	if a.demo || a.updates.Offer == nil || a.updates.Installing || a.updates.ReadyToRestart {
		return
	}
	if info := a.updates.Offer.Info(); !info.Installable {
		a.updates.Error = info.Reason
		return
	}
	// Secure-storage operations can pump native events. Freeze this offer,
	// cancel an older check and lock installation before saving any session.
	offer := a.updates.Offer
	ctx, token := a.updateGate.Begin(a.ctx)
	ctx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	a.updates.Checking = false
	a.updates.Installing = true
	if !a.saveBeforeUpdate() {
		cancel()
		a.updates.Installing = false
		return
	}
	a.updates.Open = true
	a.updates.Error = ""
	a.updates.Message = "正在下载并校验更新，请等待完成。"
	a.updates.Downloaded = 0
	a.updates.Total = 0
	a.updateTask(func() {
		defer cancel()
		err := offer.Install(ctx, func(done, total int64) {
			a.update(func() {
				if a.updateGate.Accept(token) {
					a.updates.Downloaded = max(0, done)
					a.updates.Total = max(0, total)
				}
			})
		})
		a.update(func() {
			if !a.updateGate.Accept(token) {
				return
			}
			a.updates.Installing = false
			if err != nil {
				a.updates.Error = "更新未完成：" + err.Error()
				a.updates.Message = "请保留当前窗口，可稍后重试或手动下载新版。"
				return
			}
			a.updates.ReadyToRestart = true
			a.updates.Error = ""
			a.updates.Message = "更新已安装，重启后使用新版。"
			a.notify("更新已准备好，可从“软件更新”重启")
		})
	})
}
func (a *reader) canCloseForUpdate() bool {
	if a.updates.Installing {
		a.updates.Open = true
		a.updates.Error = "正在安装更新，请等待完成后再退出。"
		return false
	}
	return true
}
func (a *reader) restartUpdate() {
	if !a.updates.ReadyToRestart || !a.canCloseForUpdate() || !a.saveBeforeUpdate() {
		return
	}
	a.shutdown() // Releases directory ownership before MyGo starts the new process.
	if a.relaunchUpdate != nil {
		a.relaunchUpdate()
	} else {
		mygo.App.Relaunch()
	}
}
func (a *reader) openUpdateRelease() {
	if a.win == nil || a.updates.Installing {
		return
	}
	address := "https://github.com/" + appupdate.Repository + "/releases"
	if a.updates.Offer != nil {
		address = a.updates.Offer.Info().ReleaseURL
	}
	if err := mygo.Shell.OpenExternal(address); err != nil {
		a.updates.Error = "无法打开GitHub下载页"
	}
}
