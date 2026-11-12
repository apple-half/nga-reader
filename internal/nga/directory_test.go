package nga

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"

	"golang.org/x/text/encoding/simplifiedchinese"
)

const directoryFixture = `window.script_muti_get_var_store={"data":{"0":{"all":{"other":{"name":"网事杂谈","content":{"0":{"content":{"0":{"fid":-7,"name":"网事杂谈","nameS":"水区"},"1":{"fid":-343809,"name":"寂寞的车","info":"车迷俱乐部"},"2":{"fid":436,"name":"消费电子 IT新闻"},"3":{"fid":-7,"stid":39827852,"name":"考研讨论"}}}}},"new":{"name":"最近新增","content":[{"fid":436,"name":"数码板块"}]},"fast":{"name":"收藏版面","content":[{"fid":999,"name":"私人收藏占位"}]}}}}};`

func TestBoardDirectoryRejectsMalformedIdentities(t *testing.T) {
	for _, identity := range []string{
		`"fid":-7,"stid":"broken"`, `"fid":-7,"stid":null`, `"fid":-7,"stid":39827852.0`,
		`"fid":-7,"stid":true`, `"fid":-7,"stid":-1`, `"fid":-7,"stid":9223372036854775808`,
		`"fid":"broken"`, `"fid":null`, `"fid":true`, `"fid":9223372036854775808`, `"fid":-9223372036854775809`, `"fid":436.0`,
	} {
		t.Run(identity, func(t *testing.T) {
			raw := fmt.Sprintf(`{"data":{"0":{"all":{"other":{"content":[{"fid":436,"name":"消费电子"},{%s,"name":"不能变成父板块"}]}}}}}`, identity)
			if entries, err := ParseBoardDirectory([]byte(raw)); err == nil {
				t.Fatalf("malformed identity replaced catalog: %+v", entries)
			}
		})
	}
}

func TestBoardDirectoryDecodesPublicScriptAndSeparatesCollections(t *testing.T) {
	gbk, err := simplifiedchinese.GB18030.NewEncoder().Bytes([]byte(directoryFixture))
	if err != nil {
		t.Fatal(err)
	}
	for _, raw := range [][]byte{[]byte(directoryFixture), gbk} {
		entries, err := ParseBoardDirectory(raw)
		if err != nil || len(entries) != 4 {
			t.Fatalf("catalog: %+v %v", entries, err)
		}
		index := NewDirectoryIndex(entries)
		for query, id := range map[string]int64{"汽车": -343809, "车迷": -343809, "数码": 436, "ＩＴ": 436, "大漩涡": -7, "水区": -7, "-343809": -343809} {
			got := index.Search(query)
			if len(got) != 1 || got[0].ID != id || got[0].CollectionID != 0 {
				t.Fatalf("query %q: %+v", query, got)
			}
		}
		got := index.Search("考研")
		if len(got) != 1 || got[0].CollectionID != 39827852 {
			t.Fatalf("collection lost: %+v", got)
		}
		if len(index.Search("私人")) != 0 {
			t.Fatal("personal placeholder included")
		}
	}
}

func TestBoardDirectoryRejectsNonDataAndEmptyOrTruncatedResponses(t *testing.T) {
	for _, raw := range []string{"<html>verify</html>", directoryFixture + "alert(1)", "other=" + directoryFixture, `{"data":{"0":{"all":{}}}}`, directoryFixture[:len(directoryFixture)/2], strings.Repeat("x", MaxDirectoryBytes+1)} {
		if _, err := ParseBoardDirectory([]byte(raw)); err == nil {
			t.Fatalf("accepted unsafe or unusable directory, bytes=%d", len(raw))
		}
	}
}

func TestEmbeddedBoardDirectoryContainsHomepageBoards(t *testing.T) {
	entries := EmbeddedBoardDirectory()
	if len(entries) < 350 || len(entries) > 1500 {
		t.Fatalf("snapshot size %d", len(entries))
	}
	for _, query := range []string{"汽车", "数码", "魔兽", "英雄联盟", "考研"} {
		if len(SearchBoardDirectory(entries, query)) == 0 {
			t.Fatalf("missing homepage query %q", query)
		}
	}
	got := SearchBoardDirectory(entries, "  消费电子   ＩＴ  ")
	if len(got) != 1 || got[0].ID != 436 {
		t.Fatalf("Unicode/token search: %+v", got)
	}
}

type directoryTransport func(*http.Request) (*http.Response, error)

func (f directoryTransport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestBoardDirectoryPublicRequestDoesNotSendSession(t *testing.T) {
	c := NewClient(&http.Client{Transport: directoryTransport(func(r *http.Request) (*http.Response, error) {
		if r.URL.String() != BoardDirectoryURL || r.Method != "GET" || r.Header.Get("Cookie") != "" || r.Header.Get("Authorization") != "" || r.Header.Get("Referer") != "" {
			t.Errorf("not an anonymous fixed-source request: %s %v", r.URL, r.Header)
		}
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(directoryFixture)), Header: make(http.Header), Request: r}, nil
	})})
	// A forum session must never be attached to the public CDN request.
	c.SetCookies([]*http.Cookie{{Name: "ngaPassportUid", Value: "123"}, {Name: "ngaPassportCid", Value: "private-test-token"}})
	raw, err := c.BoardDirectory(context.Background())
	if err != nil || len(raw) == 0 {
		t.Fatalf("fetch: %v", err)
	}
}

func TestBoardDirectoryRequestBoundsRedirectsStatusAndCancellation(t *testing.T) {
	for _, tc := range []struct {
		name           string
		status         int
		body, location string
	}{
		{"foreign redirect", 302, "", "https://example.com/steal"},
		{"downgrade", 302, "", "http://img4.nga.cn/proxy/cache_attach/bbs_index_data.js"},
		{"too big", 200, strings.Repeat("x", MaxDirectoryBytes+1), ""},
		{"forbidden", 403, directoryFixture, ""},
		{"html", 200, "<html>challenge</html>", ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			calls := 0
			c := NewClient(&http.Client{Transport: directoryTransport(func(r *http.Request) (*http.Response, error) {
				calls++
				h := make(http.Header)
				h.Set("Location", tc.location)
				return &http.Response{StatusCode: tc.status, Header: h, Body: io.NopCloser(strings.NewReader(tc.body)), Request: r}, nil
			})})
			if _, err := c.BoardDirectory(context.Background()); err == nil {
				t.Fatal("accepted invalid response")
			}
			if calls != 1 {
				t.Fatalf("followed an unsafe redirect: %d", calls)
			}
		})
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	c := NewClient(&http.Client{Transport: directoryTransport(func(r *http.Request) (*http.Response, error) { return nil, r.Context().Err() })})
	if _, err := c.BoardDirectory(ctx); err == nil {
		t.Fatal("ignored cancellation")
	}
}
