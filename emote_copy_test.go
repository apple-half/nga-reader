package main

import (
	"github.com/egoist/mygo/ui"
	"ngareader/internal/nga"
	"strings"
	"testing"
)

func TestNativeEmoteCopyIncludesCodesAcrossParagraphsAndQuotes(t *testing.T) {
	a := newReader(t.TempDir())
	defer a.cancel()
	source := nga.MarkdownOnDemand("前[s:ac:哭笑]中[s:1]后\n\n[quote]引用[s:5]结束[/quote]")
	tt := ui.NewTester(func(c *ui.Context) { a.theme(c); a.bodyMarkdown(c, source) }, 600, 360)
	tt.ClickAt(15, 12)
	tt.Command("selectAll")
	tt.Command("copy")
	got := tt.Clipboard()
	for _, want := range []string{"前[s:ac:哭笑]中[s:0:1]后", "引用[s:0:5]结束"} {
		if !strings.Contains(got, want) {
			t.Fatalf("copy lost %q: %q", want, got)
		}
	}
	if strings.ContainsAny(got, "\ufffc\u2060") || strings.Contains(got, "ngareader-emote:") {
		t.Fatalf("clipboard contains internal objects: %q", got)
	}
}
func TestEmoteCopyPartialRangeMapsExactObjectIdentity(t *testing.T) {
	s := emoteCopyParagraph{Objects: map[int]string{1: "[s:ac:哭笑]", 3: "[s:0:5]"}}
	for _, c := range []struct {
		start, end int
		want       string
	}{{0, 5, "前[s:ac:哭笑]中[s:0:5]后"}, {1, 2, "[s:ac:哭笑]"}, {3, 4, "[s:0:5]"}, {2, 3, "中"}} {
		got := s.selected(ui.SelectedText{Text: "前\ufffc中\ufffc后", Start: c.start, End: c.end})
		if got != c.want {
			t.Fatalf("range %d:%d got %q want %q", c.start, c.end, got, c.want)
		}
	}
}
func TestAnimatedEmoteDetectionFollowsRenderableLinks(t *testing.T) {
	for _, source := range []string{
		"```\n[](<ngareader-emote://asset/redface.gif>)\n```",
		"`[](<ngareader-emote://asset/redface.gif>)`",
		"literal ngareader-emote://asset/redface.gif text",
		"![[](<ngareader-emote://asset/redface.gif>)](https://example.com/a.png)",
	} {
		if containsAnimatedEmote(source) {
			t.Fatalf("non-rendered text scheduled animation: %q", source)
		}
	}
	if !containsAnimatedEmote(nga.MarkdownOnDemand("前[s:5]后")) || !containsAnimatedEmote(nga.MarkdownOnDemand("[quote]引用[s:5][/quote]")) {
		t.Fatal("actual GIF no longer classified")
	}
	if containsAnimatedEmote(nga.MarkdownOnDemand("[s:ac:哭笑]")) {
		t.Fatal("static emote classified animated")
	}
}

func TestNativeEmoteTableCopyKeepsCodesAndCellSeparators(t *testing.T) {
	a := newReader(t.TempDir())
	defer a.cancel()
	source := "| 名称 | 表情 |\n|---|---|\n| 甲 | " + nga.MarkdownOnDemand("[s:ac:哭笑]") + " |\n| 乙 | " + nga.MarkdownOnDemand("[s:5]") + " |\n"
	tt := ui.NewTester(func(c *ui.Context) { a.theme(c); a.bodyMarkdown(c, source) }, 600, 360)
	r, ok := tt.Find("名称")
	if !ok {
		t.Fatal("missing table header")
	}
	tt.ClickAt(r.X+2, r.Y+r.H/2)
	tt.Command("selectAll")
	tt.Command("copy")
	if got := tt.Clipboard(); got != "名称\t表情\n甲\t[s:ac:哭笑]\n乙\t[s:0:5]" {
		t.Fatalf("table clipboard lost contents: %q", got)
	}
}
