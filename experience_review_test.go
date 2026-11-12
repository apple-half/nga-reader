package main

import (
	"fmt"
	"github.com/egoist/mygo/ui"
	"io"
	"math"
	"net/http"
	"ngareader/internal/local"
	"ngareader/internal/nga"
	"strconv"
	"strings"
	"testing"
)

type filteredExperienceTransport struct {
	hideAuthor bool
	requests   []string
}

func (s *filteredExperienceTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	q := r.URL.Query()
	tid, _ := strconv.ParseInt(q.Get("tid"), 10, 64)
	main := 11
	if tid == 2002 {
		main = 22
	}
	s.requests = append(s.requests, fmt.Sprintf("%d/%s", tid, q.Get("authorid")))
	author := main
	if s.hideAuthor {
		author = 0
	}
	var posts []string
	for _, f := range []int{0, 20, 40, 60} {
		posts = append(posts, fmt.Sprintf(`{"pid":%d,"lou":%d,"authorid":%d,"content":"%s"}`, tid*100+int64(f), f, main, strings.Repeat("筛选阅读定位样例。", 100)))
	}
	body := fmt.Sprintf(`{"data":{"__T":{"tid":%d,"fid":-7,"authorid":%d,"subject":"筛选测试"},"__R":[%s],"__R__ROWS":4,"__R__ROWS_PAGE":20}}`, tid, author, strings.Join(posts, ","))
	return &http.Response{StatusCode: 200, Header: http.Header{"Content-Type": []string{"application/json"}}, Body: io.NopCloser(strings.NewReader(body)), Request: r}, nil
}
func filteredExperienceReader(t *testing.T, hide bool) (*reader, *filteredExperienceTransport) {
	t.Helper()
	a := newReader(t.TempDir())
	t.Cleanup(a.cancel)
	tr := &filteredExperienceTransport{hideAuthor: hide}
	a.client = nga.NewClient(&http.Client{Transport: tr})
	a.runReadTask = func(f func()) { f() }
	a.selectTopic(nga.Topic{ID: 1001, FID: -7, Title: "A"})
	a.toggleAuthorFilter()
	return a, tr
}
func settleExperience(t *ui.Tester) {
	for i := 0; i < 7; i++ {
		t.Frame()
	}
}
func locateFilteredExperience(a *reader, t *ui.Tester, floor int) {
	a.reading.Pending = &readingRestore{TopicID: a.selected.ID, UID: a.readingUID(), Page: 1, Position: local.ReadingPosition{PID: a.selected.ID*100 + int64(floor), Floor: floor, Fraction: .2}, Quiet: true}
	settleExperience(t)
}

func TestReviewFilteredHistoryUsesTargetMainAuthorAndRestoresFloor(t *testing.T) {
	for _, hide := range []bool{false, true} {
		t.Run(fmt.Sprint(hide), func(t *testing.T) {
			a, tr := filteredExperienceReader(t, hide)
			tt := ui.NewTester(a.view, 1280, 820)
			locateFilteredExperience(a, tt, 40)
			a.selectTopic(nga.Topic{ID: 2002, FID: -7, Title: "B"})
			a.toggleAuthorFilter()
			settleExperience(tt)
			locateFilteredExperience(a, tt, 20)
			if err := tt.Click("阅读后退"); err != nil {
				t.Fatal(err)
			}
			settleExperience(tt)
			if tr.requests[len(tr.requests)-1] != "1001/11" || !a.onlyAuthor || a.reading.Anchor == nil || a.reading.Anchor.Floor != 40 {
				t.Fatalf("back wrong target: requests=%v anchor=%+v", tr.requests, a.reading.Anchor)
			}
			if err := tt.Click("阅读前进"); err != nil {
				t.Fatal(err)
			}
			settleExperience(tt)
			if tr.requests[len(tr.requests)-1] != "2002/22" || a.reading.Anchor == nil || a.reading.Anchor.Floor != 20 {
				t.Fatalf("forward wrong target: requests=%v anchor=%+v", tr.requests, a.reading.Anchor)
			}
		})
	}
}
func TestReviewFilteredRefreshUsesLiveViewAnchorAndKeepsFullProgress(t *testing.T) {
	for _, scroll := range []bool{false, true} {
		t.Run(fmt.Sprint(scroll), func(t *testing.T) {
			a, _ := filteredExperienceReader(t, false)
			a.state.History = []local.Reading{{Topic: a.selected, Page: 1, UID: 0, Scoped: true, Position: &local.ReadingPosition{PID: 999, Floor: 5, Fraction: .4}}}
			tt := ui.NewTester(a.view, 1280, 820)
			locateFilteredExperience(a, tt, 40)
			var task func()
			a.runReadTask = func(f func()) { task = f }
			a.loadThread(1)
			if scroll {
				tt.Scroll(1100, 350, 0, 600)
				settleExperience(tt)
			}
			before := clonePosition(a.reading.Anchor)
			if before == nil {
				t.Fatal("fixture has no live filtered anchor")
			}
			task()
			settleExperience(tt)
			after := a.reading.Anchor
			if after == nil || after.PID != before.PID || after.Floor != before.Floor || math.Abs(float64(after.Fraction-before.Fraction)) > .025 {
				t.Fatalf("refresh changed filtered view: before=%+v after=%+v", before, after)
			}
			r, _ := local.FindReading(a.state.History, 1001, 0)
			if r.Position == nil || r.Position.Floor != 5 || r.Position.PID != 999 || r.Position.Fraction != .4 {
				t.Fatal("filtered refresh changed full-thread history")
			}
		})
	}
}

func TestNativeContinuationAppendsOnceWithoutMovingView(t *testing.T) {
	a, tr := continuousReader(t)
	var pending []func()
	a.runReadTask = func(f func()) { pending = append(pending, f) }
	tt := ui.NewTester(a.view, 1280, 820)
	tt.Frame()
	if len(pending) != 0 {
		t.Fatal("initial view loaded further pages")
	}
	tt.Scroll(1100, 350, 0, 100000)
	for i := 0; i < 3; i++ {
		tt.Frame()
	}
	if len(pending) != 1 || !a.continuation.Loading {
		t.Fatalf("wheel did not request exactly one next page: %d, %+v", len(pending), a.continuation)
	}
	before := a.scroll.Y
	pending[0]()
	for i := 0; i < 4; i++ {
		tt.Frame()
	}
	if len(a.rendered) != 40 || len(tr.pages) != 2 || math.Abs(float64(a.scroll.Y-before)) > 1 {
		t.Fatalf("append moved scroll: before=%v after=%v posts=%d requests=%v", before, a.scroll.Y, len(a.rendered), tr.pages)
	}
	writePreview(t, tt, "experience-native-appended")
}
