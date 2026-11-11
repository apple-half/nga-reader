package nga

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"golang.org/x/text/encoding/simplifiedchinese"
)

// Static noBBCode markup as returned by NGA, before its scripts populate
// author names and page buttons. All names and IDs here are synthetic.
const threadHTMLFixture = `<!doctype html><html><head><title>兼容 &amp; 测试 NGA玩家社区</title></head><body>
<div id="m_nav"><a href="/thread.php?fid=-7">网事杂谈</a><a href="/read.php?tid=100">兼容 &amp; 测试</a></div>
<div id="pagebtop"><a href="/read.php?tid=100&amp;page=2">下一页(2)</a><script>var __PAGE = {0:'/read.php?tid=100',1:3,2:1,3:20};commonui.pageBtn(document.getElementById('pageBtnHere').parentNode,__PAGE,true)</script></div>
<script>commonui.userInfo.setAll({"42":{"uid":42,"username":"测试楼主"},"-1":{"uid":-1,"username":"匿名甲"}});</script>
<table><tr><td><a id="postauthor0" href="nuke.php?func=ucp&amp;uid=42" class="author b"></a></td><td id="postcontainer0"><a id="pid0Anchor"></a><span id="postdate0" title="reply time">2026-11-08 15:59</span><h3 id="postsubject0">兼容 &amp; 测试</h3><span id="postcontent0" class="postcontent ubbcode">第一行<br>第二行 &amp; [b]加粗[/b]<script>neverExecute()</script><iframe src="https://example.invalid/"></iframe></span></td></tr>
<tr><td><a id="postauthor2" href="nuke.php?func=ucp&amp;uid=-1" class="author b"></a></td><td id="postcontainer2"><a id="pid900Anchor"></a><span id="postdate2" title="reply time">2026-11-08 16:02</span><h3 id="postsubject2"></h3><span id="postcontent2" class="postcontent ubbcode">[quote]引用[/quote]<br>[img]./mon_202611/10/test.jpg[/img]</span></td></tr></table></body></html>`

func TestTruncatedThreadFallsBackToStaticHTML(t *testing.T) {
	calls := 0
	raw, _ := simplifiedchinese.GB18030.NewEncoder().Bytes([]byte(threadHTMLFixture))
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		q := r.URL.Query()
		if r.URL.Path != "/read.php" || q.Get("tid") != "100" || q.Get("page") != "1" {
			t.Error("thread scope changed", r.URL)
		}
		if calls == 1 {
			http.SetCookie(w, &http.Cookie{Name: "testReadSession", Value: "synthetic", Path: "/read.php"})
			w.Write([]byte(`{"data":{"__R":{"0":{"content":"中途截断`))
			return
		}
		if calls > 2 {
			t.Error("unbounded retry")
		}
		if !q.Has("noBBCode") || q.Has("__output") {
			t.Error("fallback must request static noBBCode HTML", r.URL)
		}
		if session, e := r.Cookie("testReadSession"); e != nil || session.Value != "synthetic" {
			t.Error("fallback lost request session")
		}
		w.Header().Set("Content-Type", "text/html;charset=gbk")
		w.Write(raw)
	}))
	defer srv.Close()
	c := NewClient(srv.Client())
	c.base = srv.URL
	p, err := c.Thread(context.Background(), 100, 1, 0)
	if err != nil {
		t.Fatal(err)
	}
	if calls != 2 || len(p.Posts) != 2 || p.Topic.ID != 100 || p.Topic.Title != "兼容 & 测试" || p.Topic.FID != -7 || p.TotalPages != 3 {
		t.Fatalf("bad fallback page: %+v calls=%d", p, calls)
	}
	if p.Posts[0].Author != "测试楼主" || p.MainAuthorID != 42 || p.Posts[1].Author != "匿名甲" || p.Posts[1].ID != 900 || p.Posts[1].Floor != 2 {
		t.Fatalf("lost author/floor metadata: %+v", p)
	}
	if p.Posts[0].PostDate.IsZero() || p.Posts[0].Content != "第一行\n第二行 & [b]加粗[/b]" || !strings.Contains(MarkdownOnDemand(p.Posts[1].Content), "展示图片") {
		t.Fatalf("lost body/date or media policy: %+v", p.Posts)
	}
}

func TestThreadFallbackPreservesPageAndAuthorFilter(t *testing.T) {
	calls := 0
	htmlPage := strings.ReplaceAll(threadHTMLFixture, "1:3,2:1,3:20", "1:3,2:2,3:20")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if r.URL.Query().Get("page") != "2" || r.URL.Query().Get("authorid") != "42" {
			t.Error("lost filter", r.URL)
		}
		if calls == 1 {
			w.Write([]byte(`{"data":`))
			return
		}
		w.Write([]byte(htmlPage))
	}))
	defer srv.Close()
	c := NewClient(srv.Client())
	c.base = srv.URL
	p, e := c.Thread(context.Background(), 100, 2, 42)
	if e != nil || calls != 2 || p.Page != 2 {
		t.Fatalf("fallback failed: %+v %v calls=%d", p, e, calls)
	}
}

func TestThreadBusinessErrorsDoNotFallback(t *testing.T) {
	for _, tt := range []struct {
		body, kind string
		status     int
	}{{`{"error":["未登录"]}`, "login", 403}, {`{"error":["访问频繁"]}`, "rate", 429}, {`{"error":["没有权限"]}`, "permission", 200}} {
		t.Run(tt.kind, func(t *testing.T) {
			calls := 0
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls++
				w.WriteHeader(tt.status)
				w.Write([]byte(tt.body))
			}))
			defer srv.Close()
			c := NewClient(srv.Client())
			c.base = srv.URL
			_, e := c.Thread(context.Background(), 100, 1, 0)
			var api *APIError
			if !errors.As(e, &api) || api.Kind != tt.kind || calls != 1 {
				t.Fatalf("business error retried: %v calls=%d", e, calls)
			}
		})
	}
}

func TestThreadFallbackRejectsNonThreadAndMismatchedPages(t *testing.T) {
	for _, body := range []string{`<html>请登录后访问</html>`, strings.ReplaceAll(threadHTMLFixture, "tid=100", "tid=999"), strings.ReplaceAll(threadHTMLFixture, "1:3,2:1,3:20", "1:3,2:2,3:20")} {
		calls := 0
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			calls++
			if calls == 1 {
				w.Write([]byte(`{"data":`))
				return
			}
			w.Write([]byte(body))
		}))
		c := NewClient(srv.Client())
		c.base = srv.URL
		_, e := c.Thread(context.Background(), 100, 1, 0)
		srv.Close()
		if e == nil || calls != 2 {
			t.Fatalf("accepted invalid HTML or repeated fallback: %v calls=%d", e, calls)
		}
		if strings.Contains(e.Error(), "请登录后重试") {
			t.Fatal("parse error still blames login")
		}
	}
}

func TestThreadHTMLRejectsTruncatedDocument(t *testing.T) {
	// HTML parsers normally auto-close missing tags. A damaged response must
	// not silently turn into a successful page containing only some replies.
	partial := strings.Split(threadHTMLFixture, `<tr><td><a id="postauthor2"`)[0]
	if _, e := ParseThreadHTML([]byte(partial), 100, 1); e == nil {
		t.Fatal("accepted incomplete HTML after the first reply")
	}
}

func TestThreadFallbackHonorsCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if calls == 1 {
			w.Write([]byte(`{"data":`))
			return
		}
		cancel()
		<-r.Context().Done()
	}))
	defer srv.Close()
	c := NewClient(srv.Client())
	c.base = srv.URL
	_, e := c.Thread(ctx, 100, 1, 0)
	if !errors.Is(e, context.Canceled) || calls != 2 {
		t.Fatalf("fallback ignored cancel: %v calls=%d", e, calls)
	}
}
