package main

import (
	"fmt"
	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/markdown/parser"
	"golang.org/x/text/cases"
	"golang.org/x/text/unicode/norm"
	"ngareader/internal/nga"
	"strings"
)

type readerSearchState struct {
	Open, Pending bool
	Query         string
	Index, Warm   int
}

func normalizedSearch(text string) string { return cases.Fold().String(norm.NFKC.String(text)) }
func plainPostText(content string) string {
	d := parser.Parse(nga.Markdown(content, true))
	var b strings.Builder
	for i := 1; i < d.Len(); i++ {
		if d.Block(i).Kind == parser.CodeBlock {
			b.WriteString(d.Lines(i))
		} else {
			b.WriteString(parser.PlainText(d.Inlines(i)))
		}
		b.WriteByte('\n')
	}
	return b.String()
}
func (a *reader) searchMatches() []int {
	query := normalizedSearch(strings.TrimSpace(a.search.Query))
	if query == "" {
		return nil
	}
	var out []int
	for i := range a.rendered {
		r := &a.rendered[i]
		if a.onlyAuthor && a.demo && r.post.AuthorID != a.thread.MainAuthorID {
			continue
		}
		if r.searchText == "" {
			r.searchText = normalizedSearch(plainPostText(r.post.Content))
		}
		if strings.Contains(r.searchText, query) {
			out = append(out, i)
		}
	}
	return out
}
func (a *reader) openReaderSearch() {
	if a.selected.ID > 0 && !a.navigationBusy() {
		a.search.Open = true
	}
}
func (a *reader) moveSearch(delta int) {
	matches := a.searchMatches()
	if len(matches) == 0 {
		a.search.Index = 0
		a.search.Pending = false
		return
	}
	a.search.Index = (a.search.Index + delta + len(matches)) % len(matches)
	a.search.Pending = true
	a.search.Warm = 0
}
func (a *reader) searchActiveIndex() int {
	matches := a.searchMatches()
	if len(matches) == 0 {
		return -1
	}
	a.search.Index = min(max(0, a.search.Index), len(matches)-1)
	return matches[a.search.Index]
}
func (a *reader) readerSearchView(c *ui.Context) {
	if !a.search.Open {
		return
	}
	s := &a.search
	ui.Column(c).Padding(8, 12).Gap(5).BorderWidth(0, 0, 1, 0).BorderColor(c.Theme().Border).Children(func() {
		ui.Row(c).Gap(5).Children(func() {
			input := ui.TextInput(c.Key("body-search-input"), &s.Query).Label("查找已载入正文").Placeholder("查找已载入正文…").MaxLength(240).AutoFocus().Grow(1).OnChange(func() { s.Index = 0; a.moveSearch(0) })
			ui.Button(c, "↑").Label("上一匹配楼层").OnClick(func() { a.moveSearch(-1) })
			ui.Button(c, "↓").Label("下一匹配楼层").OnClick(func() { a.moveSearch(1) })
			ui.Button(c, "关闭").Label("关闭正文查找").OnClick(func() { s.Open = false; s.Pending = false })
			if input.Submitted() {
				a.moveSearch(1)
			}
		})
		n := len(a.searchMatches())
		label := "仅查找当前已载入内容，按楼层定位"
		if strings.TrimSpace(s.Query) != "" {
			if n == 0 {
				label = "没有匹配的楼层 · 仅查找已载入内容"
			} else {
				label = fmt.Sprintf("%d / %d 楼 · 仅查找已载入内容", min(s.Index, n-1)+1, n)
			}
		}
		ui.Text(c, label).FontSize(11).TextColor(c.Theme().TextMuted)
	})
}
func (a *reader) observeSearch(c *ui.Context, viewport ui.Rect, rows []readingRow) {
	if !a.search.Open || !a.search.Pending {
		return
	}
	index := a.searchActiveIndex()
	if index < 0 {
		a.search.Pending = false
		return
	}
	target := a.rendered[index]
	for _, row := range rows {
		if row.Post.Floor != target.post.Floor || max(1, row.Page) != max(1, target.page) {
			continue
		}
		if row.Bounds.H > 0 && viewport.H > 0 {
			a.reading.Pending = nil
			a.reading.Settling = 2
			a.scroll.Y = max(0, a.scroll.Y+row.Bounds.Y-viewport.Y-8)
			a.search.Pending = false
			c.Invalidate()
			return
		}
	}
	if a.search.Warm < 4 {
		a.search.Warm++
		c.Invalidate()
	} else {
		a.search.Pending = false
	}
}
