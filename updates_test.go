package main

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
	"ngareader/internal/local"
	"ngareader/internal/nga"
)

type fakeUpdateOffer struct {
	info     updateInfo
	installs int
	err      error
}

func (f *fakeUpdateOffer) Info() updateInfo { return f.info }
func (f *fakeUpdateOffer) Install(_ context.Context, progress func(int64, int64)) error {
	f.installs++
	progress(50, 100)
	progress(100, 100)
	return f.err
}

type fakeUpdateEngine struct {
	offer  updateOffer
	err    error
	checks int
}

func (f *fakeUpdateEngine) Check(context.Context) (updateOffer, error) {
	f.checks++
	return f.offer, f.err
}
func updateTestReader(t *testing.T) (*reader, *fakeUpdateEngine, *fakeUpdateOffer) {
	t.Helper()
	a := newReader(t.TempDir())
	t.Cleanup(a.cancel)
	offer := &fakeUpdateOffer{info: updateInfo{Version: "0.1.8", Notes: "更新说明：改进阅读体验。", Installable: true, Prerelease: true}}
	engine := &fakeUpdateEngine{offer: offer}
	a.updaterEngine = engine
	a.runUpdateTask = func(fn func()) { fn() }
	return a, engine, offer
}

func TestUpdateDefaultsAndOldSettingsKeepAutomaticChecks(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "settings.json"), []byte(`{"theme":"浅色"}`), 0600); err != nil {
		t.Fatal(err)
	}
	s, err := local.Load(dir)
	if err != nil || !s.AutoCheckUpdates {
		t.Fatal("old settings disabled selected default")
	}
	s.AutoCheckUpdates = false
	if err := local.Save(dir, s); err != nil {
		t.Fatal(err)
	}
	s, err = local.Load(dir)
	if err != nil || s.AutoCheckUpdates {
		t.Fatal("manual preference not preserved")
	}
}
func TestUpdateBackgroundCheckDoesNotStealComposerOrInstall(t *testing.T) {
	a, engine, offer := updateTestReader(t)
	a.compose.Open = true
	a.checkUpdates(false)
	if engine.checks != 1 || a.updates.Offer == nil || a.updates.Open || !a.compose.Open || offer.installs != 0 {
		t.Fatal("background check installed or stole focus")
	}
}
func TestUpdateManualCheckAndNativePanel(t *testing.T) {
	a, _, _ := updateTestReader(t)
	tt := ui.NewTester(a.view, 980, 620)
	if err := tt.Click("软件更新"); err != nil {
		t.Fatal(err)
	}
	if !a.updates.Open || !tt.HasText("下载并安装") || !tt.HasText("启动时自动检查更新") {
		t.Fatal("update controls missing")
	}
	writePreview(t, tt, "github-update-available")
}
func TestUpdateInstallRequiresSavedDraftAndNoActivePost(t *testing.T) {
	a, _, offer := updateTestReader(t)
	a.checkUpdates(true)
	a.compose.Sending = true
	a.installUpdate()
	if offer.installs != 0 || a.updates.Installing {
		t.Fatal("install interrupted forum submission")
	}
	a.compose.Sending = false
	a.compose.Open = true
	a.compose.Draft = local.Draft{UID: 123, Target: nga.PostTarget{Action: "reply", TID: 9}, Content: "未发表的内容"}
	a.installUpdate()
	saved, err := local.LoadDrafts(a.dir)
	if err != nil || len(saved) != 1 || saved[0].Content != "未发表的内容" || offer.installs != 1 || !a.updates.ReadyToRestart {
		t.Fatal("draft not protected before update")
	}
}
func TestUpdateInstallSaveFailureAndDownloadFailureKeepRunning(t *testing.T) {
	a, _, offer := updateTestReader(t)
	a.checkUpdates(true)
	block := filepath.Join(t.TempDir(), "file")
	_ = os.WriteFile(block, []byte("x"), 0600)
	a.dir = block
	a.installUpdate()
	if offer.installs != 0 || a.updates.Error == "" {
		t.Fatal("install went ahead after save failure")
	}
	a.dir = t.TempDir()
	offer.err = errors.New("下载中断")
	a.installUpdate()
	if a.updates.ReadyToRestart || a.updates.Installing || a.closing || a.updates.Error == "" {
		t.Fatal("failed download stopped app")
	}
}
func TestUpdateCannotCloseOrSendDuringInstall(t *testing.T) {
	a, _, offer := updateTestReader(t)
	a.checkUpdates(true)
	var queued func()
	a.runUpdateTask = func(fn func()) { queued = fn }
	a.installUpdate()
	if !a.updates.Installing || a.canCloseForUpdate() {
		t.Fatal("installation allowed normal exit")
	}
	a.openComposer(nga.PostTarget{Action: "new", FID: -7}, "板块")
	a.useDemo()
	if a.compose.Open || a.demo {
		t.Fatal("write/session transition while installing")
	}
	if queued == nil {
		t.Fatal("no installer worker")
	}
	queued()
	if offer.installs != 1 || !a.updates.ReadyToRestart {
		t.Fatal("install result lost")
	}
}
func TestUpdateRestartReleasesDirectoryBeforeLaunching(t *testing.T) {
	dir := t.TempDir()
	a, release, err := openOwnedReader(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer release()
	a.updates.ReadyToRestart = true
	called := false
	a.relaunchUpdate = func() {
		lock, err := local.LockDataDirectory(dir)
		if err != nil {
			t.Fatal("restart saw old lock:", err)
		}
		lock.Close()
		called = true
	}
	a.restartUpdate()
	if !called || !a.closing {
		t.Fatal("restart did not complete shutdown first")
	}
}
func TestUpdateCheckErrorsAndDemoDoNotTriggerInstall(t *testing.T) {
	a, engine, offer := updateTestReader(t)
	engine.err = errors.New("GitHub不可用")
	a.checkUpdates(true)
	if a.updates.Error == "" || a.updates.Offer != nil {
		t.Fatal("check error hidden")
	}
	a.useDemo()
	a.checkUpdates(true)
	if engine.checks != 1 || offer.installs != 0 {
		t.Fatal("demo contacted updates")
	}
}
func TestUpdateCheckScheduleAndRenamedExeFallback(t *testing.T) {
	a, _, _ := updateTestReader(t)
	now := time.Now()
	if !a.shouldCheckUpdates() {
		t.Fatal("first check skipped")
	}
	a.state.LastUpdateCheck = now
	if !a.shouldCheckUpdates() {
		t.Fatal("persisted timestamp suppressed startup check")
	}
	if !a.shouldCheckUpdates() {
		t.Fatal("daily check missing")
	}
	a.state.AutoCheckUpdates = false
	if a.shouldCheckUpdates() {
		t.Fatal("disabled checks still run")
	}
	if installationReason("windows", "C:/app/Work.exe", true) == "" {
		t.Fatal("renamed Windows executable allowed")
	}
	if installationReason("windows", "C:/app/NGAReader.exe", true) != "" {
		t.Fatal("official filename rejected")
	}
	if installationReason("darwin", "/readonly/NGAReader.app/Contents/MacOS/NGAReader", false) == "" {
		t.Fatal("read-only bundle allowed")
	}
}

type reentrantUpdateKeys struct {
	hook   func()
	called bool
}

func (k *reentrantUpdateKeys) Get(string) ([]byte, error) {
	if !k.called {
		k.called = true
		k.hook()
	}
	return make([]byte, 32), nil
}
func (*reentrantUpdateKeys) Set(string, []byte) error { return nil }
func (*reentrantUpdateKeys) Delete(string) error      { return nil }
func TestUpdateSessionSaveCannotReenterInstallation(t *testing.T) {
	a, _, offer := updateTestReader(t)
	a.checkUpdates(true)
	a.state.RememberLogin = true
	if err := a.client.ImportCookieHeader("ngaPassportUid=123; ngaPassportCid=synthetic"); err != nil {
		t.Fatal(err)
	}
	lockedDuringSave := false
	a.keys = &reentrantUpdateKeys{hook: func() { lockedDuringSave = a.updates.Installing; a.installUpdate() }}
	a.installUpdate()
	if !lockedDuringSave || offer.installs != 1 {
		t.Fatalf("native save reentered updater: locked=%t installs=%d", lockedDuringSave, offer.installs)
	}
}

func TestUpdateRestartFreezesActionsBeforeNativeSave(t *testing.T) {
	a, _, _ := updateTestReader(t)
	a.updates.ReadyToRestart = true
	a.state.RememberLogin = true
	if err := a.client.ImportCookieHeader("ngaPassportUid=123; ngaPassportCid=synthetic"); err != nil {
		t.Fatal(err)
	}
	relaunches, postingWorkers := 0, 0
	a.relaunchUpdate = func() { relaunches++ }
	a.runPosting = func(func()) { postingWorkers++ }
	closeAllowed, sessionAllowed := false, false
	a.keys = &reentrantUpdateKeys{hook: func() {
		closeAllowed = a.canCloseForUpdate()
		sessionAllowed = a.allowSessionChange()
		a.restartUpdate()
		a.openComposer(nga.PostTarget{Action: "new", FID: -7}, "板块")
	}}
	a.restartUpdate()
	if relaunches != 1 || postingWorkers != 0 || a.compose.Open || closeAllowed || sessionAllowed {
		t.Fatalf("restart reentered: launches=%d posts=%d open=%t close=%t session=%t", relaunches, postingWorkers, a.compose.Open, closeAllowed, sessionAllowed)
	}
	if !a.closing || !a.canCloseForUpdate() {
		t.Fatal("intended relaunch cannot quit")
	}
}

func TestUpdateNonzeroCheckDurationDoesNotSkipAutomaticTick(t *testing.T) {
	a, _, _ := updateTestReader(t)
	started := time.Now()
	a.state.LastUpdateCheck = started.Add(2 * time.Second)
	if !a.shouldCheckUpdates() {
		t.Fatal("next daily timer tick skipped after nonzero check duration")
	}
	a.updates.Checking = true
	if a.shouldCheckUpdates() {
		t.Fatal("overlapping check allowed")
	}
}

func TestUpdateRestartSaveFailureClearsGuard(t *testing.T) {
	a, _, _ := updateTestReader(t)
	a.updates.ReadyToRestart = true
	if err := os.WriteFile(filepath.Join(a.dir, "settings.json"), []byte("old"), 0600); err != nil {
		t.Fatal(err)
	}
	bad := filepath.Join(a.dir, "not-a-directory")
	if err := os.WriteFile(bad, []byte("x"), 0600); err != nil {
		t.Fatal(err)
	}
	a.dir = bad
	a.relaunchUpdate = func() { t.Fatal("restart despite failed save") }
	a.restartUpdate()
	if a.updates.Restarting || a.closing || !a.canCloseForUpdate() || a.updates.Error == "" {
		t.Fatal("failed save left restart locked")
	}
}
