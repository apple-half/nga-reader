package appupdate

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestSignedFeedWorksWhileGitHubAPIIsRateLimited(t *testing.T) {
	pub, key := testKeys(t)
	m := testManifest(t, key, "windows-amd64")
	raw, _ := json.Marshal(m)
	feed, _ := json.Marshal(map[string]any{"manifest": json.RawMessage(raw), "prerelease": true})
	apiRequests := 0
	c := NewClient(&http.Client{Transport: testTransport(func(r *http.Request) (*http.Response, error) {
		if r.Header.Get("Cookie") != "" || r.Header.Get("Authorization") != "" {
			t.Error("credentials leaked")
		}
		if r.URL.Host == "raw.githubusercontent.com" {
			return &http.Response{StatusCode: 200, Header: make(http.Header), Body: ioCloser(feed)}, nil
		}
		apiRequests++
		return &http.Response{StatusCode: 403, Header: http.Header{"X-Ratelimit-Remaining": []string{"0"}}, Body: ioCloser(nil)}, nil
	})}, pub)
	offer, err := c.Latest(context.Background(), "0.1.7", "windows-amd64")
	if err != nil || offer == nil || offer.Manifest.Version != "0.1.8" || !offer.Prerelease || apiRequests != 0 {
		t.Fatalf("signed feed unavailable: offer=%v api=%d err=%v", offer, apiRequests, err)
	}
	if offer, err := c.Latest(context.Background(), "0.1.8", "windows-amd64"); err != nil || offer != nil {
		t.Fatal("same version offered", err)
	}
}

func TestPublishedFeedSeedsRetainValidSignatures(t *testing.T) {
	keyText, err := os.ReadFile("../../resources/update-public.txt")
	if err != nil {
		t.Fatal(err)
	}
	pub, err := PublicKey(string(keyText))
	if err != nil {
		t.Fatal(err)
	}
	for _, target := range Targets {
		raw, err := os.ReadFile(filepath.Join("../../updates", ManifestName(target)))
		if err != nil {
			t.Fatal(err)
		}
		var feed Feed
		if err = json.Unmarshal(raw, &feed); err != nil {
			t.Fatal(err)
		}
		if _, err := DecodeManifest(feed.Manifest, pub, target); err != nil {
			t.Fatal(target, err)
		}
	}
}

func TestHTTPFailureFallsBackButOversizedFeedDoesNot(t *testing.T) {
	pub, _ := testKeys(t)
	for _, oversize := range []bool{false, true} {
		api := 0
		c := NewClient(&http.Client{Transport: testTransport(func(r *http.Request) (*http.Response, error) {
			body := []byte("[]")
			code := 200
			if r.URL.Host == "raw.githubusercontent.com" {
				code = 404
				if oversize {
					code = 200
					body = make([]byte, MaxManifestBytes+1)
				}
			} else {
				api++
			}
			return &http.Response{StatusCode: code, Header: make(http.Header), Body: ioCloser(body)}, nil
		})}, pub)
		offer, err := c.Latest(context.Background(), "0.1.7", "windows-amd64")
		if oversize {
			if err == nil || api != 0 {
				t.Fatal("oversized feed accepted/fell back")
			}
		} else if err != nil || offer != nil || api != 1 {
			t.Fatal("HTTP fallback failed", err, api)
		}
	}
}

func TestInvalidSignedFeedFailsClosed(t *testing.T) {
	pub, key := testKeys(t)
	m := testManifest(t, key, "windows-amd64")
	m.Version = "9.9.9"
	raw, _ := json.Marshal(m)
	feed, _ := json.Marshal(map[string]any{"manifest": json.RawMessage(raw), "prerelease": true})
	api := 0
	c := NewClient(&http.Client{Transport: testTransport(func(r *http.Request) (*http.Response, error) {
		if r.URL.Host != "raw.githubusercontent.com" {
			api++
		}
		return &http.Response{StatusCode: 200, Header: make(http.Header), Body: ioCloser(feed)}, nil
	})}, pub)
	if _, err := c.Latest(context.Background(), "0.1.7", "windows-amd64"); err == nil || api != 0 {
		t.Fatal("invalid feed triggered fallback", err, api)
	}
}

func TestRateLimitBackoffDoesNotRepeatAPIRequests(t *testing.T) {
	pub, _ := testKeys(t)
	api := 0
	c := NewClient(&http.Client{Transport: testTransport(func(r *http.Request) (*http.Response, error) {
		if r.URL.Host == "raw.githubusercontent.com" {
			return &http.Response{StatusCode: 404, Header: make(http.Header), Body: ioCloser(nil)}, nil
		}
		api++
		return &http.Response{StatusCode: 403, Header: http.Header{"X-Ratelimit-Remaining": []string{"0"}, "X-Ratelimit-Reset": []string{time.Now().Add(time.Hour).Format("invalid")}, "Retry-After": []string{"60"}}, Body: ioCloser(nil)}, nil
	})}, pub)
	for range 2 {
		if _, err := c.Latest(context.Background(), "0.1.7", "windows-amd64"); err == nil || !strings.Contains(err.Error(), "限制") {
			t.Fatal("rate limit not explained", err)
		}
	}
	if api != 1 {
		t.Fatalf("API retried during cooldown: %d", api)
	}
}
