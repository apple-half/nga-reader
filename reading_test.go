package main

import (
	"context"
	"fmt"
	"github.com/egoist/mygo/ui"
	"net/http"
	"ngareader/internal/local"
	"ngareader/internal/nga"
	"strings"
	"testing"
	"time"
)

type offlineReadingTransport struct{}

func (offlineReadingTransport) RoundTrip(*http.Request) (*http.Response, error) {
	return nil, context.Canceled
}

func longReadingFixture(a *reader) {
	a.thread = nga.ThreadPage{Topic: a.selected, Page: 1, TotalPages: 4, TotalRows: 80, MainAuthorID: 100}
	for i := 0; i < 20; i++ {
		a.thread.Posts = append(a.thread.Posts, nga.Post{ID: int64(100 + i), Floor: i, AuthorID: 100, Author: "样例作者", Content: strings.Repeat("这是一段用于验证原生阅读位置的样例内容。\n\n", 12)})
	}
	a.resetMarkdown()
}
func TestReadMarkersOnlyAfterSuccessfulLoadAndPreserveFullThreadPosition(t *testing.T) {
	a := newReader(t.TempDir())
	defer a.cancel()
	a.demo = true
	a.selected = nga.Topic{ID: 1001, Title: "已读话题", Replies: 30}
	a.thread = nga.ThreadPage{Topic: a.selected, Page: 3, TotalRows: 31}
	a.rememberReading()
	seen, _ := local.ReadStatus(a.state.History, a.selected, 0)
	if !seen {
		t.Fatal("successful load not marked read")
	}
	r, _ := local.FindReading(a.state.History, 1001, 0)
	r.Position = &local.ReadingPosition{PID: 300, Floor: 45, Fraction: .4}
	a.state.History = local.PutReading(a.state.History, r)
	a.onlyAuthor = true
	a.thread.Page = 1
	a.rememberReading()
	r, _ = local.FindReading(a.state.History, 1001, 0)
	if r.Page != 3 || r.Position == nil || r.Position.Floor != 45 {
		t.Fatal("filtered reading overwrote full-thread position")
	}
	a.onlyAuthor = false
	a.selected.Replies = 31
	_, changed := local.ReadStatus(a.state.History, a.selected, 0)
	if !changed {
		t.Fatal("new reply marker missing")
	}
}

func TestNativeReadingRestoresFloorOnceAndPersistsScroll(t *testing.T) {
	a := newReader(t.TempDir())
	defer a.cancel()
	a.demo = true
	topic := nga.Topic{ID: 1001, Title: "长帖阅读", Replies: 79}
	a.state.History = local.PutReading(a.state.History, local.Reading{Topic: topic, Page: 1, Scoped: true, RepliesKnown: true, Position: &local.ReadingPosition{PID: 107, Floor: 7, Fraction: .25}})
	if err := local.Save(a.dir, a.state); err != nil {
		t.Fatal(err)
	}
	a.selectTopic(topic)
	longReadingFixture(a)
	a.demo = false
	tt := ui.NewTester(a.view, 1280, 820)
	tt.Frame()
	if a.scroll.Y < 1000 {
		t.Fatalf("saved floor was not restored: y=%f", a.scroll.Y)
	}
	before := a.scroll.Y
	tt.Scroll(1000, 350, 0, 420)
	tt.Advance(2 * time.Second)
	if a.scroll.Y <= before {
		t.Fatal("manual scrolling snapped back to saved location")
	}
	state, err := local.Load(a.dir)
	if err != nil {
		t.Fatal(err)
	}
	r, ok := local.FindReading(state.History, 1001, 0)
	if !ok || r.Position == nil || r.Position.Floor < 7 {
		t.Fatalf("scroll progress not saved: %+v", r)
	}
	writePreview(t, tt, "reading-resumed")
}

func TestReadMarkerVisibleInNativeTopicList(t *testing.T) {
	a := newReader(t.TempDir())
	defer a.cancel()
	a.useDemo()
	topic := a.topics.Topics[1]
	a.state.History = local.PutReading(a.state.History, local.Reading{Topic: topic, Page: 1, Scoped: true, RepliesKnown: true})
	a.topics.Topics[1].Replies++
	tt := ui.NewTester(a.view, 1280, 820)
	if !tt.HasText("有新回复") {
		t.Fatal("native topic list has no new-reply marker")
	}
	writePreview(t, tt, "reading-markers")
}

func TestReadingRestoresAcrossWidthAndMissingPIDAndFlushesBeforeNavigation(t *testing.T) {
	a := newReader(t.TempDir())
	defer a.cancel()
	a.demo = true
	topic := nga.Topic{ID: 1001, Title: "长帖阅读", Replies: 79}
	a.state.History = local.PutReading(a.state.History, local.Reading{Topic: topic, Page: 1, Scoped: true, Position: &local.ReadingPosition{PID: 9999, Floor: 5, Fraction: .3}})
	a.selectTopic(topic)
	longReadingFixture(a)
	a.demo = false
	a.state.FontSize = 20
	a.state.Theme = "NGA经典屎黄色"
	tt := ui.NewTester(a.view, 980, 620)
	tt.Frame()
	if a.scroll.Y < 1000 || a.reading.Pending != nil {
		t.Fatal("floor fallback did not restore at narrow width")
	}
	tt.Scroll(850, 300, 0, 260)
	before, _ := local.FindReading(a.state.History, 1001, 0)
	a.client = nga.NewClient(&http.Client{Transport: offlineReadingTransport{}})
	a.selectBoard(local.Board{ID: -7, Name: "大漩涡"})
	saved, err := local.Load(a.dir)
	if err != nil {
		t.Fatal(err)
	}
	after, _ := local.FindReading(saved.History, 1001, 0)
	if !sameReadingPosition(before.Position, after.Position) {
		t.Fatal("navigation lost the current anchor")
	}
	writePreview(t, tt, "reading-narrow-classic")
}

func TestReadingFailedOrCancelledThreadDoesNotMarkRead(t *testing.T) {
	a := newReader(t.TempDir())
	defer a.cancel()
	a.selected = nga.Topic{ID: 123, Title: "尚未加载"}
	a.thread = nga.ThreadPage{Topic: a.selected, Page: 2}
	a.loadingThread = true
	a.rememberReading()
	if len(a.state.History) != 0 {
		t.Fatal("loading thread was marked read")
	}
	a.loadingThread = false
	a.threadErr = fmt.Errorf("读取失败")
	a.rememberReading()
	if len(a.state.History) != 0 {
		t.Fatal("failed thread was marked read")
	}
}

func TestAccountChangeClearsPreviousReaderAndCancelsRequests(t *testing.T) {
	a := newReader(t.TempDir())
	defer a.cancel()
	if err := a.client.ImportCookieHeader("ngaPassportUid=123; ngaPassportCid=synthetic"); err != nil {
		t.Fatal(err)
	}
	a.selected = nga.Topic{ID: 99, Title: "之前账号的帖子"}
	a.thread = nga.ThreadPage{Topic: a.selected, Posts: []nga.Post{{Content: "旧内容"}}}
	a.resetMarkdown()
	_, token := a.threadGate.Begin(a.ctx)
	a.loadingThread = true
	values := []*http.Cookie{{Name: "ngaPassportUid", Value: "456", Domain: "bbs.nga.cn", Path: "/"}, {Name: "ngaPassportCid", Value: "synthetic-new", Domain: "bbs.nga.cn", Path: "/"}}
	if err := a.adoptLoginCookies(values); err != nil {
		t.Fatal(err)
	}
	if a.selected.ID != 0 || len(a.rendered) != 0 || a.loadingThread || a.threadGate.Accept(token) {
		t.Fatal("old account reading remained active")
	}
}
