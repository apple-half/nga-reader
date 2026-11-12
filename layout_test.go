package main

import (
	"github.com/egoist/mygo/ui"
	"ngareader/internal/local"
	"testing"
)

func TestLayoutPreservesReaderAndPreferences(t *testing.T) {
	for _, w := range []float32{980, 1280, 1920} {
		l := layoutFor(w, 280, 450, false)
		if w-l.Sidebar-l.Topics-8 < 420 {
			t.Fatalf("reader squeezed at %v: %+v", w, l)
		}
	}
	a := newReader(t.TempDir())
	defer a.cancel()
	a.useDemo()
	a.state.SidebarWidth = 280
	a.state.TopicWidth = 450
	tt := ui.NewTester(a.view, 980, 620)
	for _, label := range []string{"字号与主题设置", "切换专注阅读", "收藏帖子"} {
		r, ok := tt.Find(label)
		if !ok || r.W < 29 || r.X+r.W > 981 {
			t.Fatalf("clipped toolbar %s: %+v", label, r)
		}
	}
	if a.state.SidebarWidth != 280 || a.state.TopicWidth != 450 {
		t.Fatal("auto fitting destroyed preferences")
	}
	if err := tt.Click("收起板块栏"); err != nil {
		t.Fatal(err)
	}
	if tt.HasText("我的板块") {
		t.Fatal("sidebar still visible")
	}
	if err := tt.Click("展开板块栏"); err != nil {
		t.Fatal(err)
	}
	if !tt.HasText("我的板块") {
		t.Fatal("sidebar not restored")
	}
	a.resetLayout()
	if a.state.SidebarWidth != 180 || a.state.TopicWidth != 310 {
		t.Fatal("reset did not restore widths")
	}
	writePreview(t, tt, "ux-layout-min")
}

func TestWindowMemoryNormalizesBeforeUse(t *testing.T) {
	dir := t.TempDir()
	s := local.Defaults()
	s.WindowWidth = 99999
	s.WindowHeight = -1
	if err := local.Save(dir, s); err != nil {
		t.Fatal(err)
	}
	s, err := local.Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	if s.WindowWidth != 3840 || s.WindowHeight != 620 {
		t.Fatalf("unsafe remembered size: %dx%d", s.WindowWidth, s.WindowHeight)
	}
}
