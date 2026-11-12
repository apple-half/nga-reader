package main

import (
	"github.com/egoist/mygo/ui"
	"testing"
)

func TestExperienceSettingsRemainReachableAtMinimumWindow(t *testing.T) {
	a := newReader(t.TempDir())
	defer a.cancel()
	a.useDemo()
	a.settingsOpen = true
	tt := ui.NewTester(a.view, 980, 620)
	r, ok := tt.Find("阅读设置")
	if !ok || r.Y < 0 {
		t.Fatalf("settings header offscreen %+v", r)
	}
	tt.Scroll(650, 400, 0, 1200)
	writePreview(t, tt, "experience-settings-scrolled")
	footer, _ := tt.Find("查看界面演示")
	// Text metrics differ across system fonts. Compare against a complete
	// instance of the same control, rather than a Windows-specific height.
	reference := ui.NewTester(func(c *ui.Context) { a.theme(c); ui.Button(c, "查看界面演示") }, 300, 100)
	full, ok := reference.Find("查看界面演示")
	if !ok || footer.H+.5 < full.H {
		t.Fatalf("settings footer is clipped after scrolling: %+v", footer)
	}
	if err := tt.Click("查看界面演示"); err != nil {
		t.Fatal("settings footer unreachable:", err)
	}
}

func TestExperienceAuditThreeThemesAndNarrowColumns(t *testing.T) {
	for _, theme := range []string{"深色", "浅色", "NGA经典屎黄色"} {
		a := newReader(t.TempDir())
		defer a.cancel()
		a.useDemo()
		a.state.Theme = theme
		a.state.SidebarWidth = 280
		a.state.TopicWidth = 450
		tt := ui.NewTester(a.view, 980, 620)
		for _, label := range []string{"查找正文（Ctrl/Cmd+F）", "字号与主题设置", "阅读后退", "阅读前进", "加载下一页"} {
			if label == "加载下一页" {
				continue
			}
			r, ok := tt.Find(label)
			if !ok || r.W < 29 || r.X+r.W > 981 || r.Y < 0 {
				t.Fatalf("%s clipped %s %+v", theme, label, r)
			}
		}
		writePreview(t, tt, "experience-"+theme+"-min")
		a.settingsOpen = true
		tt.Frame()
		writePreview(t, tt, "experience-"+theme+"-settings")
		a.settingsOpen = false
		a.pictureItems = []string{"https://img.nga.cn/attachments/one.jpg", "https://img.nga.cn/attachments/two.jpg"}
		a.pictureZoom = 1
		imageUI := ui.NewTester(a.pictureView, 600, 480)
		for _, label := range []string{"下一张", "放大图片", "重置图片缩放", "关闭预览"} {
			r, ok := imageUI.Find(label)
			if !ok || r.X < 0 || r.Y < 0 || r.X+r.W > 601 || r.Y+r.H > 481 {
				t.Fatalf("gallery clipped %s %+v", label, r)
			}
		}
		writePreview(t, imageUI, "experience-"+theme+"-gallery")
	}
}
