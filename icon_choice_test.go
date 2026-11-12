package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/egoist/mygo/ui"
	"ngareader/internal/local"
)

func storedIcon(t *testing.T, s local.State) string {
	t.Helper()
	raw, err := json.Marshal(s)
	if err != nil {
		t.Fatal(err)
	}
	var v struct {
		AppIcon string `json:"appIcon"`
	}
	if err := json.Unmarshal(raw, &v); err != nil {
		t.Fatal(err)
	}
	return v.AppIcon
}

func TestSettingsIconSelectionPersistsAndSwitchesBack(t *testing.T) {
	a := newReader(t.TempDir())
	defer a.cancel()
	a.settingsOpen = true
	tt := ui.NewTester(a.view, 980, 620)
	if err := tt.Click("使用工位摸鱼图标"); err != nil {
		t.Fatal(err)
	}
	reloaded := newReader(a.dir)
	defer reloaded.cancel()
	if storedIcon(t, reloaded.state) != "office" {
		t.Fatal("cartoon selection did not survive restart")
	}
	if err := tt.Click("使用简约工作图标"); err != nil {
		t.Fatal(err)
	}
	s, err := local.Load(a.dir)
	if err != nil || storedIcon(t, s) != "work" {
		t.Fatal("work selection was not restored", err)
	}
	writePreview(t, tt, "icon-choice-settings")
}

func TestIconSelectionSaveFailureRetainsPreviousChoice(t *testing.T) {
	a := newReader(t.TempDir())
	defer a.cancel()
	a.settingsOpen = true
	blocked := filepath.Join(t.TempDir(), "not-a-directory")
	if err := os.WriteFile(blocked, []byte("unchanged"), 0600); err != nil {
		t.Fatal(err)
	}
	a.dir = blocked
	tt := ui.NewTester(a.view, 980, 620)
	if err := tt.Click("使用工位摸鱼图标"); err != nil {
		t.Fatal(err)
	}
	if storedIcon(t, a.state) != "work" || a.message == "" {
		t.Fatal("failed write silently changed the selection")
	}
}

func TestDemoIconSelectionStaysTemporary(t *testing.T) {
	a := newReader(t.TempDir())
	defer a.cancel()
	if err := local.Save(a.dir, a.state); err != nil {
		t.Fatal(err)
	}
	a.useDemo()
	a.settingsOpen = true
	tt := ui.NewTester(a.view, 980, 620)
	if err := tt.Click("使用工位摸鱼图标"); err != nil {
		t.Fatal(err)
	}
	if storedIcon(t, a.state) != "office" || storedIcon(t, *a.liveState) != "work" {
		t.Fatal("demo did not keep separate choices")
	}
	s, err := local.Load(a.dir)
	if err != nil || storedIcon(t, s) != "work" {
		t.Fatal("demo wrote to live settings", err)
	}
	a.cancel()
	a.returnToLive()
	if storedIcon(t, a.state) != "work" {
		t.Fatal("return from demo retained temporary icon")
	}
}
