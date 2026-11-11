package appupdate

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"
)

// Feed wraps the original signed metadata unchanged. Prerelease is a display
// label; all installation decisions remain bound by the signed Manifest.
type Feed struct {
	Manifest   json.RawMessage `json:"manifest"`
	Prerelease bool            `json:"prerelease"`
}

func FeedURL(target string) string {
	return "https://raw.githubusercontent.com/" + Repository + "/main/updates/" + ManifestName(target)
}
func safeFeedURL(address string) bool {
	for _, target := range Targets {
		if address == FeedURL(target) {
			return true
		}
	}
	return false
}
func (c *Client) Latest(ctx context.Context, current, target string) (*Offer, error) {
	if _, err := parseVersion(current); err != nil {
		return nil, err
	}
	valid := false
	for _, t := range Targets {
		valid = valid || t == target
	}
	if !valid {
		return nil, fmt.Errorf("当前平台暂不提供更新")
	}
	feedCtx, cancel := context.WithTimeout(ctx, 8*time.Second)
	defer cancel()
	body, err := c.open(feedCtx, FeedURL(target))
	if err != nil {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		return c.latestAPI(ctx, current, target)
	}
	defer body.Close()
	raw, err := io.ReadAll(io.LimitReader(body, MaxManifestBytes+1))
	if err != nil {
		return nil, fmt.Errorf("读取签名更新索引失败：%w", err)
	}
	if int64(len(raw)) > MaxManifestBytes {
		return nil, fmt.Errorf("签名更新索引超过大小限制")
	}
	var feed Feed
	d := json.NewDecoder(bytes.NewReader(raw))
	d.DisallowUnknownFields()
	if d.Decode(&feed) != nil {
		return nil, fmt.Errorf("签名更新索引格式无效")
	}
	var extra any
	if d.Decode(&extra) != io.EOF {
		return nil, fmt.Errorf("签名更新索引格式无效")
	}
	m, err := DecodeManifest(feed.Manifest, c.pub, target)
	if err != nil {
		return nil, err
	}
	n, err := CompareVersions(m.Version, current)
	if err != nil {
		return nil, err
	}
	if n <= 0 {
		return nil, nil
	}
	return &Offer{Manifest: m, Raw: append([]byte(nil), feed.Manifest...), Prerelease: feed.Prerelease}, nil
}

type HTTPError struct {
	Status      int
	RetryAt     time.Time
	RateLimited bool
}

func (e *HTTPError) Error() string {
	if e.RateLimited {
		return "GitHub暂时限制请求，请在" + e.RetryAt.Local().Format("15:04") + "后重新检查，也可打开下载页手动更新。"
	}
	return fmt.Sprintf("GitHub返回HTTP%d；可稍后重试或打开下载页手动更新。", e.Status)
}
func responseError(r *http.Response, now time.Time) *HTTPError {
	e := &HTTPError{Status: r.StatusCode, RateLimited: r.StatusCode == 429 || r.StatusCode == 403 && (r.Header.Get("X-RateLimit-Remaining") == "0" || r.Header.Get("Retry-After") != "")}
	if !e.RateLimited {
		return e
	}
	e.RetryAt = now.Add(time.Minute)
	if seconds, err := strconv.ParseInt(r.Header.Get("Retry-After"), 10, 64); err == nil && seconds > 0 && seconds <= 86400 {
		e.RetryAt = now.Add(time.Duration(seconds) * time.Second)
	}
	if epoch, err := strconv.ParseInt(r.Header.Get("X-RateLimit-Reset"), 10, 64); err == nil {
		reset := time.Unix(epoch, 0)
		if reset.After(e.RetryAt) && reset.Before(now.Add(24*time.Hour)) {
			e.RetryAt = reset
		}
	}
	return e
}
