package nga

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// A 403 with NGA's JSON body must lead to a login action, not an empty list.
func TestHTTP403ReturnsLoginError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("fid") != "-7" || r.URL.Query().Get("__output") != "11" {
			t.Error("wrong API query", r.URL)
		}
		w.WriteHeader(403)
		w.Write([]byte(`{"error":["1:未登录","<a>登录</a>",null],"data":{"__MESSAGE":[1,"未登录",null,403]}}`))
	}))
	defer srv.Close()
	c := NewClient(srv.Client())
	c.base = srv.URL
	_, err := c.Topics(context.Background(), -7, 1)
	var api *APIError
	if !errors.As(err, &api) || api.Kind != "login" || !strings.Contains(api.Message, "未登录") {
		t.Fatalf("want login error, got %#v", err)
	}
}

func TestNumericTopicsAreSorted(t *testing.T) {
	page, err := ParseTopics([]byte(`{"data":{"__T":{"10":{"tid":"300","fid":-7,"subject":"第三 &amp; 条","author":"丙","replies":2},"2":{"tid":200,"fid":-7,"subject":"第二条","author":"乙","replies":"1"},"0":{"tid":100,"fid":-7,"subject":"第一条","author":"甲","postdate":1700000000,"replies":0}},"__T__ROWS":3,"__ROWS":120,"__T__ROWS_PAGE":50,"__PAGE":1}}`), 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(page.Topics) != 3 || page.Topics[0].ID != 100 || page.Topics[1].ID != 200 || page.Topics[2].Title != "第三 & 条" {
		t.Fatalf("wrong topic order/content: %+v", page)
	}
	if page.TotalPages != 3 || !page.HasNext {
		t.Fatalf("wrong pages: %+v", page)
	}
}

func TestTopicPaginationUsesTotalRows(t *testing.T) {
	page, err := ParseTopics([]byte(`{"data":{"__T":[{"tid":123,"subject":"本页一条"}],"__T__ROWS":1,"__T__ROWS_PAGE":50,"__ROWS":120}}`), 1)
	if err != nil || page.TotalPages != 3 || !page.HasNext {
		t.Fatalf("per-page row count disabled next page: %+v %v", page, err)
	}
}

func TestMalformedCollectionIsError(t *testing.T) {
	if _, err := ParseTopics([]byte(`{"data":{"__T":"invalid"}}`), 1); err == nil {
		t.Fatal("malformed topic list became empty success")
	}
	if _, err := ParseThread([]byte(`{"data":{"__T":{"tid":1},"__R":null}}`), 1); err == nil {
		t.Fatal("malformed reply list became empty success")
	}
	if page, err := ParseTopics([]byte(`{"data":{"__T":[]}}`), 1); err != nil || len(page.Topics) != 0 {
		t.Fatal("genuinely empty board was rejected")
	}
}

func TestSessionCookiesPreservePathAndExpiry(t *testing.T) {
	c := NewClient(nil)
	future := time.Now().Add(12 * time.Hour).Truncate(time.Second)
	c.SetCookies([]*http.Cookie{{Name: "ngaPassportUid", Value: "123", Domain: "bbs.nga.cn", Path: "/", Secure: true, Expires: future}, {Name: "ngaPassportCid", Value: "token", Domain: "bbs.nga.cn", Path: "/", Secure: true, Expires: future}, {Name: "readVerifier", Value: "narrow", Domain: "bbs.nga.cn", Path: "/read.php", Secure: true, Expires: future}})
	cookies := c.Cookies()
	if len(cookies) != 3 {
		t.Fatalf("path-specific session cookie lost: %+v", cookies)
	}
	for _, cookie := range cookies {
		if !cookie.Expires.Equal(future) {
			t.Fatalf("expiry lost for %s", cookie.Name)
		}
		if cookie.Name == "readVerifier" && cookie.Path != "/read.php" {
			t.Fatal("cookie path widened")
		}
	}
}

func TestThreadIncludesZeroPIDAndUsers(t *testing.T) {
	page, err := ParseThread([]byte(`{"data":{"__T":{"tid":100,"fid":-7,"subject":"主题","authorid":42,"author":"楼主","replies":20},"__R":{"1":{"pid":123,"lou":1,"authorid":-2,"content":"回复","postdate":1700000001},"0":{"pid":0,"lou":0,"authorid":42,"content":"主楼<br/>正文","postdate":1700000000}},"__U":{"42":{"username":"楼主"},"-2":{"username":"匿名甲"}},"__R__ROWS":21,"__R__ROWS_PAGE":20}}`), 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(page.Posts) != 2 || page.Posts[0].ID != 0 || page.Posts[0].Floor != 0 || page.Posts[1].Author != "匿名甲" || page.MainAuthorID != 42 || page.TotalPages != 2 {
		t.Fatalf("wrong thread: %+v", page)
	}
}

func TestPageCounts(t *testing.T) {
	page, err := ParseThread([]byte(`{"data":{"__T":{"tid":100,"subject":"数组回复","replies":40},"__R":[{"pid":101,"lou":40,"authorid":2,"content":"最后一页"}],"__U":{"2":{"username":"用户"}},"__R__ROWS_PAGE":20}}`), 3)
	if err != nil || page.TotalPages != 3 || page.Page != 3 || page.Posts[0].Floor != 40 {
		t.Fatalf("wrong last page: %+v %v", page, err)
	}
	if _, err = ParseTopics([]byte(`<html>service unavailable</html>`), 1); err == nil {
		t.Fatal("HTML response silently accepted")
	}
}

func TestNonStrictNumericKeysPreservePostText(t *testing.T) {
	raw := []byte(`{"data":{"__T":{"tid":1,"subject":"兼容测试","replies":0},"__R":{0:{"pid":0,"lou":0,"authorid":2,"content":"时间 10:00,2: 保持原样"}},"__U":{2:{"username":"读者"}}}}`)
	page, err := ParseThread(raw, 1)
	if err != nil || len(page.Posts) != 1 || page.Posts[0].Content != "时间 10:00,2: 保持原样" {
		t.Fatalf("numeric object keys corrupted content: %+v %v", page, err)
	}
}

func TestCookieImportRejectsInjection(t *testing.T) {
	c := NewClient(&http.Client{})
	if err := c.ImportCookieHeader("ngaPassportUid=123; ngaPassportCid=abc"); err != nil {
		t.Fatal(err)
	}
	if !c.HasSession() {
		t.Fatal("session not loaded")
	}
	for _, s := range []string{"x=1\r\nHost: evil.test", "not a cookie", "ngaPassportUid=123"} {
		if err := c.ImportCookieHeader(s); err == nil {
			t.Errorf("accepted invalid/incomplete session %q", s)
		}
	}
}

func TestOnlyAllowedNGAURLs(t *testing.T) {
	for _, s := range []string{"https://evil.test/read.php?tid=1", "file:///read.php?tid=1", "https://bbs.nga.cn.evil.test/read.php?tid=1", "https://user@bbs.nga.cn/read.php?tid=1"} {
		if _, _, err := ParseLink(s); err == nil {
			t.Errorf("accepted %s", s)
		}
	}
	tid, fid, err := ParseLink("https://bbs.nga.cn/read.php?tid=123")
	if err != nil || tid != 123 || fid != 0 {
		t.Fatalf("valid topic rejected: %d %d %v", tid, fid, err)
	}
	tid, fid, err = ParseLink("https://bbs.nga.cn/thread.php?fid=-7")
	if err != nil || tid != 0 || fid != -7 {
		t.Fatalf("valid board rejected: %d %d %v", tid, fid, err)
	}
}

func TestResponseLimitAndCancellation(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("page") == "2" {
			<-r.Context().Done()
			return
		}
		w.Write([]byte(strings.Repeat("x", MaxResponseBytes+1)))
	}))
	defer srv.Close()
	c := NewClient(srv.Client())
	c.base = srv.URL
	if _, err := c.Topics(context.Background(), -7, 1); err == nil || !strings.Contains(err.Error(), "过大") {
		t.Fatalf("want size error, got %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Millisecond)
	defer cancel()
	if _, err := c.Topics(ctx, -7, 2); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("want cancellation, got %v", err)
	}
}
