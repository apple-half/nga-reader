package main

import (
	"fmt"
	"github.com/egoist/mygo/ui"
	"math"
	"net/http"
	"ngareader/internal/local"
	"ngareader/internal/nga"
	"strings"
	"testing"
	"time"
)

func TestLeavingAuthorFilterRestoresFullThreadPage(t *testing.T) {
	a := newReader(t.TempDir())
	defer a.cancel()
	a.selected = nga.Topic{ID: 1001, Title: "筛选模式切换", Replies: 79}
	a.state.History = []local.Reading{{Topic: a.selected, Page: 3, Scoped: true, Position: &local.ReadingPosition{PID: 103, Floor: 40, Fraction: .25}}}
	longReadingFixture(a)
	a.onlyAuthor = true
	transport := &lastPageReadingTransport{}
	a.client = nga.NewClient(&http.Client{Transport: transport})
	a.runReadTask = func(fn func()) { fn() }
	tt := ui.NewTester(a.view, 980, 620)
	if err := tt.Click("显示全部"); err != nil {
		t.Fatal(err)
	}
	r, _ := local.FindReading(a.state.History, 1001, 0)
	if fmt.Sprint(transport.pages) != "[3]" || a.thread.Page != 3 || r.Page != 3 {
		t.Fatalf("filter exit lost full-thread page: requests=%v thread=%d reading=%+v", transport.pages, a.thread.Page, r)
	}
}

func TestActiveReadingReflowKeepsLogicalPosition(t *testing.T) {
	for _, change := range []string{"font", "width"} {
		t.Run(change, func(t *testing.T) {
			a := newReader(t.TempDir())
			defer a.cancel()
			a.demo = true
			topic := nga.Topic{ID: 1001, Title: "阅读时重排", Replies: 79}
			a.state.History = []local.Reading{{Topic: topic, Page: 1, Scoped: true, Position: &local.ReadingPosition{PID: 107, Floor: 7, Fraction: .25}}}
			a.selectTopic(topic)
			longReadingFixture(a)
			for i := range a.thread.Posts {
				a.thread.Posts[i].Content = strings.Repeat(strings.Repeat("用于检查调整字号和窗口宽度后阅读楼层保持不变。", 5)+"\n\n", 12)
			}
			a.resetMarkdown()
			a.demo = false
			tt := ui.NewTester(a.view, 1280, 820)
			tt.Frame()
			before, _ := local.FindReading(a.state.History, 1001, 0)
			oldY := a.scroll.Y
			if before.Position == nil {
				t.Fatal("fixture has no reading anchor")
			}
			if change == "font" {
				a.state.FontSize = 22
				tt.Frame()
			} else {
				tt = ui.NewTester(a.view, 980, 620)
				tt.Frame()
			}
			tt.Advance(2 * time.Second)
			after, _ := local.FindReading(a.state.History, 1001, 0)
			if after.Position == nil || after.Position.PID != before.Position.PID || after.Position.Floor != before.Position.Floor || math.Abs(float64(after.Position.Fraction-before.Position.Fraction)) > .025 {
				t.Fatalf("%s reflow replaced anchor: before=%+v after=%+v", change, before.Position, after.Position)
			}
			if math.Abs(float64(a.scroll.Y-oldY)) < 200 {
				t.Fatal("reflow did not adjust native scroll offset")
			}
			reflowY := a.scroll.Y
			x := float32(1100)
			if change == "width" {
				x = 850
			}
			tt.Scroll(x, 350, 0, 420)
			tt.Advance(2 * time.Second)
			if a.scroll.Y <= reflowY+100 || a.reading.Pending != nil {
				t.Fatal("manual scroll after reflow snapped back")
			}
		})
	}
}

type failedReadingTransport struct{}

func (failedReadingTransport) RoundTrip(*http.Request) (*http.Response, error) {
	return nil, fmt.Errorf("synthetic offline failure")
}

func TestStartReadingFailureKeepsStoredProgressAndSuccessCommitsReset(t *testing.T) {
	a := newReader(t.TempDir())
	defer a.cancel()
	a.selected = nga.Topic{ID: 1001, Title: "从头阅读失败", Replies: 79}
	a.thread = nga.ThreadPage{Topic: a.selected, Page: 3, TotalPages: 4, TotalRows: 80}
	a.state.History = []local.Reading{{Topic: a.selected, Page: 3, Scoped: true, Position: &local.ReadingPosition{PID: 140, Floor: 40, Fraction: .4}}}
	if err := local.Save(a.dir, a.state); err != nil {
		t.Fatal(err)
	}
	a.client = nga.NewClient(&http.Client{Transport: failedReadingTransport{}})
	a.runReadTask = func(fn func()) { fn() }
	a.startReadingFromBeginning()
	saved, err := local.Load(a.dir)
	if err != nil {
		t.Fatal(err)
	}
	r, _ := local.FindReading(saved.History, 1001, 0)
	if r.Page != 3 || r.Position == nil || r.Position.Floor != 40 {
		t.Fatalf("failed restart destroyed saved progress: %+v", r)
	}
	a.client = nga.NewClient(&http.Client{Transport: &lastPageReadingTransport{}})
	a.startReadingFromBeginning()
	saved, err = local.Load(a.dir)
	if err != nil {
		t.Fatal(err)
	}
	r, _ = local.FindReading(saved.History, 1001, 0)
	if r.Page != 1 || r.Position != nil {
		t.Fatalf("successful restart did not reset progress: %+v", r)
	}
}

func TestCancelledStartAndAuthorOnlyStartKeepFullThreadProgress(t *testing.T) {
	a := newReader(t.TempDir())
	defer a.cancel()
	a.selected = nga.Topic{ID: 1001, Title: "取消从头阅读", Replies: 79}
	a.thread = nga.ThreadPage{Topic: a.selected, Page: 3, TotalPages: 4, TotalRows: 80}
	a.state.History = []local.Reading{{Topic: a.selected, Page: 3, Scoped: true, Position: &local.ReadingPosition{PID: 140, Floor: 40, Fraction: .4}}}
	a.client = nga.NewClient(&http.Client{Transport: &lastPageReadingTransport{}})
	a.onlyAuthor = true
	a.runReadTask = func(fn func()) { fn() }
	a.startReadingFromBeginning()
	r, _ := local.FindReading(a.state.History, 1001, 0)
	if r.Page != 3 || r.Position == nil {
		t.Fatal("author-only start reset full-thread progress")
	}
	a.onlyAuthor = false
	a.thread.Page = 3
	var queued []func()
	a.runReadTask = func(fn func()) { queued = append(queued, fn) }
	a.startReadingFromBeginning()
	a.selectTopicPage(nga.Topic{ID: 2002, Title: "另一个帖子"}, 1)
	queued[0]()
	r, _ = local.FindReading(a.state.History, 1001, 0)
	if r.Page != 3 || r.Position == nil || r.Position.Floor != 40 {
		t.Fatal("cancelled start reset the old progress")
	}
}
