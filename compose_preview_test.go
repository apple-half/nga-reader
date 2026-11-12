package main

import (
	"github.com/egoist/mygo/ui"
	"ngareader/internal/nga"
	"os"
	"path/filepath"
	"testing"
)

func TestComposerPreviewPreservesDraftAndDoesNotSendOrLoadImages(t *testing.T) {
	a, f := postingReader(t)
	a.openComposer(nga.PostTarget{Action: "new", FID: -7}, "大漩涡")
	source := "[quote]预览引用[/quote]\n[b]粗体示例[/b] [s:ac:哭笑]\n[img]./mon_202611/10/a.jpg[/img]"
	a.compose.Draft.Subject = "预览标题"
	a.compose.Draft.Content = source
	a.saveComposer()
	reads := f.reads
	tt := ui.NewTester(a.view, 980, 620)
	if err := tt.Click("预览正文"); err != nil {
		t.Fatal(err)
	}
	if !a.compose.Preview || !tt.HasText("预览引用") || !tt.HasText("粗体示例") || !tt.HasText("展示图片") {
		t.Fatal("preview not rendered")
	}
	if a.compose.Draft.Content != source || f.writes != 0 || f.reads != reads || a.pictureWeb != nil || a.pictureAddress != "" || a.md.LoadImage != nil {
		t.Fatal("preview changed draft or made external request")
	}
	writePreview(t, tt, "ux-compose-preview")
	if err := tt.Click("编辑正文"); err != nil {
		t.Fatal(err)
	}
	if a.compose.Preview || a.compose.Draft.Content != source {
		t.Fatal("edit mode lost source")
	}
}
func TestComposerSavedTimeOnlyAdvancesAfterSuccessfulWrite(t *testing.T) {
	a, _ := postingReader(t)
	a.openComposer(nga.PostTarget{Action: "reply", FID: -7, TID: 1001}, "测试")
	a.compose.Draft.Content = "第一次草稿"
	if !a.saveComposer() || a.compose.SavedAt.IsZero() {
		t.Fatal("no successful save timestamp")
	}
	before := a.compose.SavedAt
	bad := filepath.Join(t.TempDir(), "blocked")
	os.WriteFile(bad, []byte("file"), 0600)
	a.dir = bad
	a.compose.Draft.Content = "失败后仍保留的正文"
	if a.saveComposer() || !a.compose.SavedAt.Equal(before) || a.compose.SaveError == "" || a.compose.Draft.Content != "失败后仍保留的正文" {
		t.Fatal("failed save misreported success or lost source")
	}
}
