package main

import (
	"fmt"
	"github.com/egoist/mygo/ui"
	"io"
	"net/http"
	"ngareader/internal/nga"
	"strconv"
	"strings"
	"testing"
)

func TestPageInputRejectsInvalidBoundsAndAcceptsFullWidthDigits(t *testing.T) {
	for _, value := range []string{"", "0", "-1", "1.5", "1e3", "+2", "1000001", "5"} {
		if _, err := parsePageInput(value, 4); err == nil {
			t.Fatalf("invalid page accepted: %q", value)
		}
	}
	page, err := parsePageInput(" ３ ", 4)
	if err != nil || page != 3 {
		t.Fatal("full width input failed", page, err)
	}
	if page, err := parsePageInput("999", 0); err != nil || page != 999 {
		t.Fatal("unknown total blocked valid jump")
	}
}

func TestNativePageJumpKeyboardAndSamePage(t *testing.T) {
	a := newReader(t.TempDir())
	defer a.cancel()
	a.useDemo()
	longReadingFixture(a)
	a.demo = false
	a.client = nga.NewClient(&http.Client{Transport: offlineReadingTransport{}})
	tt := ui.NewTester(a.view, 980, 620)
	if err := tt.Click("跳转回复页码"); err != nil {
		t.Fatal(err)
	}
	if err := tt.Click("页码"); err != nil {
		t.Fatal(err)
	}
	tt.Key(ui.Cmd, ui.KeyA)
	tt.Type("0")
	tt.Key(0, ui.KeyEnter)
	if !a.jump.Open || a.jump.Error == "" {
		t.Fatal("invalid page closed dialog")
	}
	tt.Key(ui.Cmd, ui.KeyA)
	tt.Type("1")
	tt.Key(0, ui.KeyEnter)
	if a.jump.Open || a.loadingThread {
		t.Fatal("same page caused another request")
	}
	if err := tt.Click("跳转回复页码"); err != nil {
		t.Fatal(err)
	}
	writePreview(t, tt, "page-jump-dialog")
	tt.Key(0, ui.KeyEscape)
	if a.jump.Open {
		t.Fatal("Escape did not close jump dialog")
	}
}

func TestPageJumpRevalidatesTargetAndUnknownTotal(t *testing.T) {
	a := newReader(t.TempDir())
	defer a.cancel()
	a.useDemo()
	longReadingFixture(a)
	a.demo = false
	a.openPageJump(true)
	a.jump.Input = "2"
	a.selected.ID++
	a.applyPageJump()
	if a.thread.Page != 1 || a.jump.Error == "" {
		t.Fatal("stale target navigated")
	}
	a.thread.TotalRows = 0
	a.thread.TotalPages = 0
	if a.threadPageLimit() != 0 {
		t.Fatal("unknown total guessed last page")
	}
}

func TestNativeOpenLinkShortcutAndRestoreStart(t *testing.T) {
	a := newReader(t.TempDir())
	defer a.cancel()
	a.useDemo()
	tt := ui.NewTester(a.view, 980, 620)
	tt.Key(ui.Cmd, ui.KeyL)
	if !a.linkOpen || !tt.HasText("NGA 帖子链接") {
		t.Fatal("open link shortcut missing")
	}
	tt.Key(0, ui.KeyEscape)
	if a.linkOpen {
		t.Fatal("Escape did not close link dialog")
	}
}

func TestOpenLinkHonorsExplicitPageInsteadOfSavedResume(t *testing.T) {
	a := newReader(t.TempDir())
	defer a.cancel()
	a.client = nga.NewClient(&http.Client{Transport: offlineReadingTransport{}})
	a.openLink("https://bbs.nga.cn/read.php?tid=1001&page=3")
	if a.thread.Page != 3 {
		t.Fatalf("explicit link page ignored: %d", a.thread.Page)
	}
}

type lastPageReadingTransport struct{ pages []int }

func (s *lastPageReadingTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	s.pages = append(s.pages, page)
	body := fmt.Sprintf(`{"data":{"__T":{"tid":1001,"fid":-7,"subject":"末页链接","replies":79},"__R":[{"pid":%d,"lou":%d,"authorid":2,"content":"正文"}],"__R__ROWS":80,"__R__ROWS_PAGE":20}}`, 100+page, (page-1)*20)
	return &http.Response{StatusCode: 200, Header: http.Header{"Content-Type": []string{"application/json"}}, Body: io.NopCloser(strings.NewReader(body)), Request: r}, nil
}

func TestLastPageLinkResolvesMetadataOnce(t *testing.T) {
	a := newReader(t.TempDir())
	defer a.cancel()
	transport := &lastPageReadingTransport{}
	a.client = nga.NewClient(&http.Client{Transport: transport})
	a.runReadTask = func(fn func()) { fn() }
	a.openLink("https://bbs.nga.cn/read.php?tid=1001&page=e")
	if fmt.Sprint(transport.pages) != "[1 4]" || a.thread.Page != 4 || a.linkLastTopic != 0 {
		t.Fatalf("last-page resolution: requests=%v page=%d pending=%d", transport.pages, a.thread.Page, a.linkLastTopic)
	}
	a.startReadingFromBeginning()
	if a.thread.Page != 1 {
		t.Fatal("from-beginning did not return to page one")
	}
}

func TestTopicRefreshRetainsRowsOnlyForSameBoardAndPage(t *testing.T) {
	a := newReader(t.TempDir())
	defer a.cancel()
	a.client = nga.NewClient(&http.Client{Transport: offlineReadingTransport{}})
	a.topicSourceFID = a.board.ID
	a.topics = nga.TopicPage{Page: 1, Topics: []nga.Topic{{ID: 1001, Title: "已有列表"}}}
	a.loadTopics(1)
	if len(a.topics.Topics) != 1 {
		t.Fatal("refresh cleared the current list")
	}
	a.board.ID = 999
	a.loadTopics(1)
	if len(a.topics.Topics) != 0 {
		t.Fatal("another board displayed old rows")
	}
}

func TestNativeInteractionAuditPreviews(t *testing.T) {
	for i, theme := range []string{"深色", "浅色", "NGA经典屎黄色"} {
		a := newReader(t.TempDir())
		defer a.cancel()
		a.useDemo()
		longReadingFixture(a)
		a.demo = false
		a.state.Theme = theme
		a.client = nga.NewClient(&http.Client{Transport: offlineReadingTransport{}})
		a.selected.Title = "这是一条较长的帖子标题，用于检查窄窗口中阅读和分页操作是否仍然清晰可用"
		tt := ui.NewTester(a.view, 980, 620)
		for _, label := range []string{"跳转回复页码", "回复末页", "从头阅读", "回到本页顶部"} {
			r, ok := tt.Find(label)
			if !ok || r.X < 0 || r.Y < 0 || r.X+r.W > 981 || r.Y+r.H > 621 {
				t.Fatalf("%s control clipped: %s %+v", theme, label, r)
			}
		}
		writePreview(t, tt, fmt.Sprintf("interaction-theme-%d", i))
		a.topicErr = fmt.Errorf("网络暂时不可用")
		tt.Frame()
		if !tt.HasText("重试") {
			t.Fatal("list error has no retry action")
		}
	}
	a, engine, _ := updateTestReader(t)
	engine.err = fmt.Errorf("GitHub暂时限制请求，请稍后检查或手动下载。")
	a.checkUpdates(true)
	tt := ui.NewTester(a.view, 980, 620)
	if !tt.HasText("打开下载页") {
		t.Fatal("update failure lacks manual download")
	}
	writePreview(t, tt, "update-error-compact")
}
