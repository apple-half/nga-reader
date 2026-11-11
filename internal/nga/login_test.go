package nga

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"golang.org/x/text/encoding/simplifiedchinese"
)

func TestLoginDocumentCorrectsMislabeledGBK(t *testing.T) {
	const original = "<!doctype html><html><head><title>账号操作</title><meta http-equiv='Content-Type' content='text/html; charset=GBK'/></head><body><script>confirm('登录成功 是否返回首页')</script><iframe src='/nuke/account_copy.html?login'></iframe></body></html>"
	gbk, err := simplifiedchinese.GBK.NewEncoder().String(original)
	if err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" || r.URL.Path != "/nuke.php" || r.URL.Query().Get("__lib") != "login" || r.URL.Query().Get("__act") != "account" {
			t.Errorf("wrong official login request: %s %s", r.Method, r.URL)
		}
		if r.Header.Get("Cookie") != "" {
			t.Error("public login bootstrap received an account cookie")
		}
		w.Header().Set("Content-Type", "text/html; charset=UTF-8")
		w.Write([]byte(gbk))
	}))
	defer srv.Close()
	c := NewClient(srv.Client())
	c.base = srv.URL
	c.ImportCookieHeader("ngaPassportUid=123; ngaPassportCid=existing-test-session")
	document, err := c.LoginDocument(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(document, "<title>账号操作</title>") || !strings.Contains(document, "confirm('登录成功 是否返回首页')") || strings.Contains(document, "\ufffd") {
		t.Fatalf("official title or script was corrupted: %q", document)
	}
	if !strings.Contains(document, "charset=utf-8") || !strings.Contains(document, "src='/nuke/account_copy.html?login'") {
		t.Fatal("document encoding or official relative iframe URL changed")
	}
}

func TestLoginDocumentKeepsUTF8(t *testing.T) {
	const original = "<!doctype html><html><head><meta charset='UTF-8'><title>账号操作</title></head><body><script>var message='登录成功';</script></body></html>"
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.Write([]byte(original)) }))
	defer srv.Close()
	c := NewClient(srv.Client())
	c.base = srv.URL
	document, err := c.LoginDocument(context.Background())
	if err != nil || !strings.Contains(document, "var message='登录成功';") {
		t.Fatalf("valid UTF-8 was re-decoded: %q %v", document, err)
	}
}

func TestLoginDocumentRejectsFailedOrOversizedPage(t *testing.T) {
	for _, tc := range []struct {
		name   string
		status int
		body   string
	}{
		{"service error", 503, "<html>unavailable</html>"},
		{"oversized", 200, "<html>" + strings.Repeat("x", 1<<20) + "</html>"},
		{"unexpected response", 200, `{"error":["please try later"]}`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(tc.status); w.Write([]byte(tc.body)) }))
			defer srv.Close()
			c := NewClient(srv.Client())
			c.base = srv.URL
			if _, err := c.LoginDocument(context.Background()); err == nil {
				t.Fatal("unusable login page was accepted")
			}
		})
	}
}
