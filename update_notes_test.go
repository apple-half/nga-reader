package main

import (
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/egoist/mygo/ui"
)

func TestCompactUpdateNotesSelectsLegacyChangesAndBoundsUnicode(t *testing.T) {
	body := compactUpdateNotes(legacyUpdateNotes(t))
	if !strings.Contains(body, "继续上次阅读") || strings.Contains(body, "下载免安装版") || strings.Contains(body, "### 自动更新") {
		t.Fatal("legacy changes section not selected")
	}
	if !strings.Contains(compactUpdateNotes(" \r\n "), "未提供更新摘要") {
		t.Fatal("empty notes have no fallback")
	}
	body = compactUpdateNotes(strings.Repeat("中文😀", 1800))
	if !utf8.ValidString(body) || len([]rune(body)) > 2450 || !strings.Contains(body, "更多内容") {
		t.Fatal("large Unicode notes are not bounded safely")
	}
	body = compactUpdateNotes("## 更新内容\r\n\r\n- 修复显示\r\n\r\n### 细节\r\n保留细节\r\n\r\n## 下载\r\n不要显示")
	if !strings.Contains(body, "保留细节") || strings.Contains(body, "不要显示") {
		t.Fatal("section boundary lost nested changes or included downloads")
	}
}

func TestUpdateNotesCacheRefreshesAndUnsafeContentDoesNotOpen(t *testing.T) {
	a, _, offer := updateTestReader(t)
	offer.info.Notes = "<script>alert('unsafe')</script>\n\n[不安全链接](javascript:alert(1))"
	a.checkUpdates(true)
	tt := ui.NewTester(a.view, 980, 620)
	if tt.HasText("<script>") {
		t.Fatal("raw HTML shown in app update notes")
	}
	if err := tt.Click("不安全链接"); err != nil {
		t.Fatal(err)
	}
	if len(tt.OpenedURLs()) != 0 {
		t.Fatal("unsafe link opened")
	}
	offer.info.Notes = "- 重新检查后说明已更新"
	tt.Frame()
	if !tt.HasText("重新检查后说明已更新") || tt.HasText("不安全链接") {
		t.Fatal("old changelog remained cached after recheck")
	}
}
