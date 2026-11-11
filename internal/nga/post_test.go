package nga

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"golang.org/x/text/encoding/simplifiedchinese"
)

func postingTestClient(t *testing.T, handler http.HandlerFunc) *Client {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	c := NewClient(srv.Client())
	c.base = srv.URL
	if err := c.ImportCookieHeader("ngaPassportUid=123; ngaPassportCid=synthetic"); err != nil {
		t.Fatal(err)
	}
	return c
}

func readyPost(t *testing.T, c *Client, target PostTarget) PostPreparation {
	t.Helper()
	p, err := c.PreparePost(context.Background(), target)
	if err != nil {
		t.Fatal(err)
	}
	return p
}

func TestPostingUsesUTF8InputAndGB18030Output(t *testing.T) {
	for _, action := range []string{"new", "reply", "quote"} {
		t.Run(action, func(t *testing.T) {
			posts := 0
			c := postingTestClient(t, func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != "/post.php" || r.URL.Query().Get("__output") != "8" || r.URL.Query().Get("__inchst") != "UTF8" {
					t.Errorf("wrong posting protocol: %s", r.URL)
				}
				if r.Method == "GET" {
					fmt.Fprintf(w, `{"data":{"__CU":{"uid":123},"fid":-7,"tid":%s,"pid":%s,"content":"[quote]原文 &amp; 引用[/quote]"}}`, r.URL.Query().Get("tid"), r.URL.Query().Get("pid"))
					return
				}
				posts++
				_ = r.ParseForm()
				if r.PostForm.Get("post_content") != "中文 & + 换行\n[s:ac:茶]" || r.PostForm.Get("step") != "2" || r.URL.Query().Get("action") != action {
					t.Errorf("form changed content: %#v", r.PostForm)
				}
				if r.PostForm.Has("attachments") || r.PostForm.Has("anony") {
					t.Error("unexpected upload/anonymous option")
				}
				data, _ := simplifiedchinese.GB18030.NewEncoder().Bytes([]byte(`{"data":{"__MESSAGE":{"0":1,"1":"发贴完毕","2":"<a href='read.php?tid=999&amp;pid=456'>查看</a>"}}}`))
				_, _ = w.Write(data)
			})
			target := PostTarget{Action: action, FID: -7, TID: 999, PID: 0}
			if action == "new" {
				target.TID = 0
			}
			p := readyPost(t, c, target)
			if action == "quote" && p.Content != "[quote]原文 & 引用[/quote]" {
				t.Fatalf("quote changed: %q", p.Content)
			}
			result, err := c.PublishPost(context.Background(), p, "标题", "中文 & + 换行\n[s:ac:茶]")
			if err != nil || result.TID != 999 || result.PID != 456 || posts != 1 {
				t.Fatalf("result=%+v posts=%d err=%v", result, posts, err)
			}
		})
	}
}

func TestPostingPreparationRejectsWrongAccountAndScope(t *testing.T) {
	for _, body := range []string{
		`{"data":{"__CU":{"uid":321},"fid":-7,"tid":9,"pid":0}}`,
		`{"data":{"__CU":{"uid":123},"fid":-7,"tid":99,"pid":0}}`,
		`{"data":{"__CU":{"uid":123},"fid":-7,"tid":9,"pid":88}}`,
		`{"data":{"fid":-7,"tid":9,"pid":0}}`,
	} {
		c := postingTestClient(t, func(w http.ResponseWriter, r *http.Request) { fmt.Fprint(w, body) })
		if _, err := c.PreparePost(context.Background(), PostTarget{Action: "reply", TID: 9}); err == nil {
			t.Fatalf("accepted scope/account mismatch: %s", body)
		}
	}
}

func TestPostingObjectErrorIsLoginError(t *testing.T) {
	c := postingTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"error":{"0":"2048:你没有登录","1":"","2":""},"data":{"__MESSAGE":{"0":2048,"1":"你没有登录","3":200}},"encode":"gbk"}`)
	})
	_, err := c.PreparePost(context.Background(), PostTarget{Action: "new", FID: -7})
	var api *APIError
	if !errors.As(err, &api) || api.Kind != "login" {
		t.Fatalf("object error not classified as login: %v", err)
	}
}

func TestPostingReplyUsesPreparedBoardAndSessionCannotChange(t *testing.T) {
	posts := 0
	c := postingTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "POST" {
			posts++
			fmt.Fprint(w, `{"data":{"__MESSAGE":[1,"发贴完毕"]}}`)
			return
		}
		fmt.Fprint(w, `{"data":{"__CU":{"uid":123},"fid":436,"tid":9,"pid":0}}`)
	})
	p := readyPost(t, c, PostTarget{Action: "reply", FID: -7, TID: 9})
	if p.Target.FID != 436 {
		t.Fatal("sidebar board replaced prepared board")
	}
	_ = c.ImportCookieHeader("ngaPassportUid=123; ngaPassportCid=replaced")
	_, err := c.PublishPost(context.Background(), p, "", "回复")
	if err == nil || posts != 0 {
		t.Fatalf("changed session sent a post: %d %v", posts, err)
	}
}

func TestPostingFailureClassificationAndNoAutomaticResubmit(t *testing.T) {
	cases := []struct {
		name, body        string
		uncertain, review bool
	}{
		{"permission", `{"error":{"0":"权限不足，禁止发言"}}`, false, false},
		{"rate", `{"error":["发帖过快，请稍后再试"]}`, false, false},
		{"captcha", `{"error":{"0":"请输入验证码"}}`, false, false},
		{"HTML", `<html>temporary error</html>`, true, false},
		{"empty data", `{"data":{}}`, true, false},
		{"only URL", `{"data":{"url":"read.php?tid=999"}}`, true, false},
		{"trailing data", `{"data":{"__MESSAGE":[1,"发贴完毕"]}}garbage`, true, false},
		{"review", `{"data":{"__MESSAGE":[1,"发贴完毕，等待审核"]}}`, false, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			posts := 0
			c := postingTestClient(t, func(w http.ResponseWriter, r *http.Request) {
				if r.Method == "GET" {
					fmt.Fprint(w, `{"data":{"__CU":{"uid":123},"fid":-7,"tid":9,"pid":0}}`)
					return
				}
				posts++
				fmt.Fprint(w, tc.body)
			})
			p := readyPost(t, c, PostTarget{Action: "reply", TID: 9})
			result, err := c.PublishPost(context.Background(), p, "", "内容")
			if tc.review {
				if err != nil || !result.UnderReview {
					t.Fatalf("review not recognized: %+v %v", result, err)
				}
				return
			}
			var failure *PublishError
			if !errors.As(err, &failure) || failure.Uncertain != tc.uncertain || posts != 1 {
				t.Fatalf("uncertain=%t, posts=%d err=%#v", tc.uncertain, posts, err)
			}
		})
	}
}

func TestPostingRedirectDoesNotReplayAndLostResponseIsUncertain(t *testing.T) {
	for _, mode := range []string{"redirect", "disconnect"} {
		t.Run(mode, func(t *testing.T) {
			posts := 0
			c := postingTestClient(t, func(w http.ResponseWriter, r *http.Request) {
				if r.Method == "GET" {
					fmt.Fprint(w, `{"data":{"__CU":{"uid":123},"fid":-7,"tid":9,"pid":0}}`)
					return
				}
				posts++
				if mode == "redirect" {
					http.Redirect(w, r, "/replay", 307)
					return
				}
				conn, _, _ := w.(http.Hijacker).Hijack()
				_ = conn.Close()
			})
			p := readyPost(t, c, PostTarget{Action: "reply", TID: 9})
			_, err := c.PublishPost(context.Background(), p, "", "正文")
			var failure *PublishError
			if !errors.As(err, &failure) || !failure.Uncertain || posts != 1 {
				t.Fatalf("post replayed or lost outcome: %d %v", posts, err)
			}
		})
	}
}

func TestPostingValidationBeforeNetwork(t *testing.T) {
	requests := 0
	c := postingTestClient(t, func(w http.ResponseWriter, r *http.Request) { requests++ })
	for _, target := range []PostTarget{{Action: "modify", TID: 1}, {Action: "new"}, {Action: "reply", TID: -1}, {Action: "quote", TID: 2, PID: -1}} {
		if _, err := c.PreparePost(context.Background(), target); err == nil {
			t.Fatalf("invalid target accepted: %+v", target)
		}
	}
	if requests != 0 {
		t.Fatal("invalid target made request")
	}
	if err := ValidatePostText(PostTarget{Action: "new", FID: -7}, "  ", "正文"); err == nil {
		t.Fatal("empty new title")
	}
	if err := ValidatePostText(PostTarget{Action: "reply", TID: 1}, "", strings.Repeat("字", MaxPostContentRunes+1)); err == nil {
		t.Fatal("oversized post")
	}
}

func TestPostingCategoriesDecodeNestedObject(t *testing.T) {
	c := postingTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/nuke.php" || r.URL.Query().Get("__lib") != "topic_key" || r.URL.Query().Get("fid") != "-7" {
			t.Errorf("wrong category URL: %s", r.URL)
		}
		fmt.Fprint(w, `{"data":{"0":{"0":{"0":"[讨论]","1":1},"1":{"0":"[求助]","1":1},"2":{"0":"[讨论]","1":0}}}}`)
	})
	items, err := c.TopicCategories(context.Background(), -7)
	if err != nil || strings.Join(items, ",") != "[讨论],[求助]" {
		t.Fatalf("categories=%v err=%v", items, err)
	}
}
