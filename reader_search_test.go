package main

import (
	"github.com/egoist/mygo/ui"
	"ngareader/internal/nga"
	"testing"
)

func TestLoadedBodySearchMatchesTextAndNeverLoadsOtherPages(t *testing.T) {
	a, tr := continuousReader(t)
	a.thread.Posts[0].Content = "[quote]引用中的苹果[/quote] Ｇｏ 语言 [s:ac:哭笑]"
	a.resetMarkdown()
	a.search.Query = "go"
	if got := a.searchMatches(); len(got) != 1 || got[0] != 0 {
		t.Fatalf("case/width search: %v", got)
	}
	a.search.Query = "苹果"
	if got := a.searchMatches(); len(got) != 1 {
		t.Fatal("quote body missing")
	}
	a.search.Query = "不存在的下一页内容"
	if len(a.searchMatches()) != 0 || len(tr.pages) != 1 {
		t.Fatal("search fetched or invented unloaded pages")
	}
	if a.md.LoadImage != nil {
		t.Fatal("search enabled external image loading")
	}
}
func TestNativeBodySearchShortcutJumpAndEscape(t *testing.T) {
	a, _ := continuousReader(t)
	a.thread.Posts[13].Content = "定位关键词"
	a.resetMarkdown()
	tt := ui.NewTester(a.view, 1280, 820)
	tt.Key(ui.Cmd, ui.KeyF)
	if !a.search.Open || !tt.HasText("查找已载入正文") {
		t.Fatal("body search shortcut did not open")
	}
	a.search.Query = "定位关键词"
	a.moveSearch(0)
	for i := 0; i < 6; i++ {
		tt.Frame()
	}
	if a.scroll.Y < 1000 || !tt.HasText("1 / 1 楼") {
		t.Fatalf("search did not locate matching floor: y=%v state=%+v", a.scroll.Y, a.search)
	}
	if a.thread.Posts[13].Content != "定位关键词" {
		t.Fatal("search rewrote post content")
	}
	writePreview(t, tt, "ux-body-search")
	tt.Key(0, ui.KeyEscape)
	if a.search.Open {
		t.Fatal("Escape did not close search")
	}
}
func TestSearchRespectsAuthorFilterAndAppendedPages(t *testing.T) {
	a, _ := continuousReader(t)
	a.appendNextPage()
	a.thread.Posts[25].Content = "新页关键词"
	a.rendered[25].source = nga.MarkdownOnDemand(a.thread.Posts[25].Content)
	a.rendered[25].post = a.thread.Posts[25]
	a.search.Query = "新页关键词"
	if got := a.searchMatches(); len(got) != 1 || got[0] != 25 {
		t.Fatal("appended page not searchable")
	}
	a.onlyAuthor = true
	a.demo = true
	a.thread.MainAuthorID = 777
	if len(a.searchMatches()) != 0 {
		t.Fatal("hidden author floor matched")
	}
}
