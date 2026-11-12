package main

import (
	"github.com/egoist/mygo/ui"
	"net/http"
	"ngareader/internal/local"
	"ngareader/internal/nga"
	"testing"
)

func TestReadingNavigationBackForwardPreservesPageAndFloor(t *testing.T) {
	a, tr := continuousReader(t)
	a.appendNextPage()
	a.reading.VisiblePage = 2
	a.reading.Anchor = &local.ReadingPosition{PID: 123, Floor: 23, Fraction: .3}
	a.selectTopic(nga.Topic{ID: 2002, FID: -7, Title: "第二帖"})
	tt := ui.NewTester(a.view, 1280, 820)
	if err := tt.Click("阅读后退"); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 5; i++ {
		tt.Frame()
	}
	if a.selected.ID != 1001 || a.thread.Page != 2 || a.reading.Anchor == nil || a.reading.Anchor.Floor != 23 {
		t.Fatalf("back lost reading location: page=%d reading=%+v y=%v", a.thread.Page, a.reading, a.scroll.Y)
	}
	a.navigateHistory(1)
	if a.selected.ID != 2002 {
		t.Fatal("forward did not restore topic")
	}
	a.navigateHistory(-1)
	a.selectTopic(nga.Topic{ID: 3003, FID: -7, Title: "新分支"})
	if len(a.navigation.Forward) != 0 {
		t.Fatal("new navigation kept stale forward entries")
	}
	if len(tr.pages) < 4 {
		t.Fatal("history navigation did not use native client")
	}
}
func TestReadingNavigationBoundsAndAccountIsolation(t *testing.T) {
	a := newReader(t.TempDir())
	defer a.cancel()
	a.useDemo()
	for i := 0; i < 80; i++ {
		a.selected.ID = int64(1000 + i)
		a.recordNavigation()
	}
	if len(a.navigation.Back) != 50 {
		t.Fatal("history is unbounded")
	}
	a.client.SetCookies([]*http.Cookie{{Name: "ngaPassportUid", Value: "99"}, {Name: "ngaPassportCid", Value: "synthetic"}})
	a.ensureNavigationAccount()
	if len(a.navigation.Back) != 0 || len(a.navigation.Forward) != 0 {
		t.Fatal("another account can navigate prior account history")
	}
}
func TestContinueLastReadingOnlyUsesCurrentScopedAccount(t *testing.T) {
	a := newReader(t.TempDir())
	defer a.cancel()
	a.runReadTask = func(func()) {}
	a.state.LastTopic = 1001
	a.state.History = []local.Reading{{Topic: nga.Topic{ID: 1001, Title: "别人的记录"}, Page: 3, UID: 99, Scoped: true}, {Topic: nga.Topic{ID: 2002, Title: "我的记录"}, Page: 2, UID: 0, Scoped: true}}
	tt := ui.NewTester(a.view, 980, 620)
	if err := tt.Click("继续上次阅读"); err != nil {
		t.Fatal(err)
	}
	if a.selected.ID != 2002 || a.thread.Page != 2 {
		t.Fatal("continue used another account record")
	}
}
