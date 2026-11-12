package main

import (
	"fmt"
	"github.com/egoist/mygo/ui"
	"io"
	"net/http"
	"ngareader/internal/local"
	"ngareader/internal/nga"
	"strconv"
	"strings"
	"testing"
	"time"
)

type continuousTransport struct {
	pages               []int
	fail, repeat, empty bool
	total               int
}

func (s *continuousTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	s.pages = append(s.pages, page)
	if s.fail {
		return nil, fmt.Errorf("synthetic offline")
	}
	dataPage := page
	if s.repeat {
		dataPage = 1
	}
	var posts []string
	if !s.empty {
		for i := 0; i < 20; i++ {
			f := (dataPage-1)*20 + i
			posts = append(posts, fmt.Sprintf(`{"pid":%d,"lou":%d,"authorid":2,"content":"第%d楼 %s"}`, 100+f, f, f, strings.Repeat("正文示例。", 35)))
		}
	}
	body := fmt.Sprintf(`{"data":{"__T":{"tid":1001,"fid":-7,"subject":"连续阅读","replies":%d},"__R":[%s],"__R__ROWS":%d,"__R__ROWS_PAGE":20}}`, max(0, s.total-1), strings.Join(posts, ","), s.total)
	return &http.Response{StatusCode: 200, Header: http.Header{"Content-Type": []string{"application/json"}}, Body: io.NopCloser(strings.NewReader(body)), Request: r}, nil
}
func continuousReader(t *testing.T) (*reader, *continuousTransport) {
	t.Helper()
	a := newReader(t.TempDir())
	t.Cleanup(a.cancel)
	tr := &continuousTransport{total: 260}
	a.client = nga.NewClient(&http.Client{Transport: tr})
	a.runReadTask = func(f func()) { f() }
	a.selectTopic(nga.Topic{ID: 1001, FID: -7, Title: "连续阅读"})
	return a, tr
}
func TestContinuousAppendPreservesScrollAndPageMapping(t *testing.T) {
	a, tr := continuousReader(t)
	a.scroll.Y = 600
	a.appendNextPage()
	if len(a.rendered) != 40 || a.scroll.Y != 600 || a.rendered[20].page != 2 || fmt.Sprint(tr.pages) != "[1 2]" {
		t.Fatalf("append: posts=%d y=%v pages=%v", len(a.rendered), a.scroll.Y, tr.pages)
	}
	a.appendNextPage()
	if len(a.rendered) != 60 {
		t.Fatal("second append missing")
	}
	for i := 0; i < 20; i++ {
		a.appendNextPage()
	}
	if len(a.rendered) != 200 || len(tr.pages) != 10 || !a.continuationLimited() {
		t.Fatalf("unbounded segment: posts=%d requests=%d", len(a.rendered), len(tr.pages))
	}
}
func TestContinuousFailureDoesNotLoopAndExplicitRetryWorks(t *testing.T) {
	a, tr := continuousReader(t)
	tr.fail = true
	a.appendNextPage()
	if a.continuation.Error == nil || len(a.rendered) != 20 {
		t.Fatal("failed append hid old posts")
	}
	a.observeContinuation(ui.Rect{W: 500, H: 400}, ui.Rect{Y: 300, H: 80})
	if len(tr.pages) != 2 {
		t.Fatal("automatic failure retried without permission")
	}
	tr.fail = false
	a.appendNextPage()
	if len(a.rendered) != 40 || a.continuation.Error != nil {
		t.Fatal("explicit retry did not recover")
	}
}
func TestContinuousRepeatsAndEmptyEndWithoutDuplicates(t *testing.T) {
	for _, kind := range []string{"repeat", "empty"} {
		t.Run(kind, func(t *testing.T) {
			a, tr := continuousReader(t)
			tr.repeat = kind == "repeat"
			tr.empty = kind == "empty"
			a.appendNextPage()
			a.appendNextPage()
			if len(a.rendered) != 20 || !a.continuation.Ended || len(tr.pages) != 2 {
				t.Fatal("repeat/empty did not stop")
			}
		})
	}
}
func TestContinuousResponseInvalidAfterTargetOrAccountChange(t *testing.T) {
	for _, kind := range []string{"topic", "account", "author", "closed"} {
		t.Run(kind, func(t *testing.T) {
			a, _ := continuousReader(t)
			var tasks []func()
			a.runReadTask = func(f func()) { tasks = append(tasks, f) }
			a.appendNextPage()
			a.appendNextPage()
			if len(tasks) != 1 {
				t.Fatal("duplicate in-flight append")
			}
			switch kind {
			case "topic":
				a.selected.ID = 2002
			case "account":
				a.client.SetCookies([]*http.Cookie{{Name: "ngaPassportUid", Value: "321"}, {Name: "ngaPassportCid", Value: "synthetic"}})
			case "author":
				a.onlyAuthor = true
			case "closed":
				a.closing = true
			}
			tasks[0]()
			if len(a.rendered) != 20 {
				t.Fatal("stale response appended")
			}
		})
	}
}
func TestContinuousVisiblePagePersistsAndRestoresAcrossReflow(t *testing.T) {
	a, _ := continuousReader(t)
	a.appendNextPage()
	a.state.Theme = "NGA经典屎黄色"
	tt := ui.NewTester(a.view, 1280, 820)
	tt.Frame()
	target := a.rendered[22].post
	a.reading.Pending = &readingRestore{TopicID: 1001, Page: 2, Position: local.ReadingPosition{PID: target.ID, Floor: 22, Fraction: .25}}
	for i := 0; i < 5; i++ {
		tt.Frame()
	}
	tt.Advance(2 * time.Second)
	r, _ := local.FindReading(a.state.History, 1001, 0)
	if r.Page != 2 || r.Position == nil || r.Position.Floor != 22 {
		t.Fatalf("wrong saved appended page %+v", r)
	}
	a.state.FontSize = 22
	for i := 0; i < 6; i++ {
		tt.Frame()
	}
	tt.Advance(2 * time.Second)
	r, _ = local.FindReading(a.state.History, 1001, 0)
	if r.Page != 2 || r.Position == nil || r.Position.Floor != 22 {
		t.Fatalf("reflow lost appended anchor %+v", r)
	}
	writePreview(t, tt, "ux-continuous-page-two")
}
func TestContinuousRefreshRetainsBodyUntilSuccessAndOnFailure(t *testing.T) {
	a, tr := continuousReader(t)
	a.scroll.Y = 500
	var task func()
	a.runReadTask = func(f func()) { task = f }
	a.loadThread(1)
	if len(a.rendered) != 20 || a.scroll.Y != 500 {
		t.Fatal("same-page refresh cleared body or scroll")
	}
	tt := ui.NewTester(a.view, 1280, 820)
	if !tt.HasText("正在刷新正文，已有内容仍可阅读…") {
		t.Fatal("missing refresh feedback")
	}
	tr.fail = true
	task()
	if len(a.rendered) != 20 || a.threadErr != nil || a.refreshError == nil {
		t.Fatal("refresh error replaced usable article")
	}
	tr.fail = false
	a.loadThread(1)
	task()
	if a.refreshError != nil || len(a.rendered) != 20 {
		t.Fatal("refresh retry failed")
	}
	a.runReadTask = func(func()) {}
	a.selected = nga.Topic{ID: 2002}
	a.loadThread(1)
	if len(a.rendered) != 0 {
		t.Fatal("old topic body leaks into new target")
	}
}

func TestContinuousWaitsForScrollAndNeverDropsPartOfPage(t *testing.T) {
	a, tr := continuousReader(t)
	a.observeContinuation(ui.Rect{W: 500, H: 400}, ui.Rect{Y: 300, H: 80})
	if len(tr.pages) != 1 {
		t.Fatal("opening a short page started auto pagination")
	}
	a.scroll.Y = 100
	a.observeContinuation(ui.Rect{W: 500, H: 400}, ui.Rect{Y: 300, H: 80})
	if len(tr.pages) != 2 {
		t.Fatal("scroll to bottom did not append")
	}
	for len(a.rendered) < 195 {
		r := a.rendered[0]
		r.post.ID = int64(1000 + len(a.rendered))
		r.post.Floor = 1000 + len(a.rendered)
		a.rendered = append(a.rendered, r)
	}
	a.appendNextPage()
	if len(a.rendered) != 195 || !a.continuationLimited() || a.continuationLast() != 2 {
		t.Fatal("part of next page was silently dropped at segment bound")
	}
}
