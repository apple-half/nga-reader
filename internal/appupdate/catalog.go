package appupdate

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"
)

type Asset struct {
	Name   string `json:"name"`
	URL    string `json:"browser_download_url"`
	Size   int64  `json:"size"`
	Digest string `json:"digest"`
}
type Release struct {
	TagName    string  `json:"tag_name"`
	Draft      bool    `json:"draft"`
	Prerelease bool    `json:"prerelease"`
	Assets     []Asset `json:"assets"`
}
type Offer struct {
	Manifest   Manifest
	Raw        []byte
	Prerelease bool
}
type Client struct {
	http       *http.Client
	pub        ed25519.PublicKey
	mu         sync.Mutex
	apiRetryAt time.Time
}

func NewClient(client *http.Client, pub ed25519.PublicKey) *Client {
	if client == nil {
		client = &http.Client{Timeout: 90 * time.Second}
	}
	clone := *client
	clone.Jar = nil
	if clone.Timeout == 0 {
		clone.Timeout = 90 * time.Second
	}
	clone.CheckRedirect = func(r *http.Request, via []*http.Request) error {
		if len(via) > 5 || r.URL.Scheme != "https" || r.URL.User != nil {
			return fmt.Errorf("更新下载跳转被拒绝")
		}
		switch r.URL.Host {
		case "api.github.com", "github.com", "release-assets.githubusercontent.com", "objects.githubusercontent.com", "raw.githubusercontent.com":
			return nil
		}
		return fmt.Errorf("更新下载跳转到了未知地址")
	}
	return &Client{http: &clone, pub: append(ed25519.PublicKey(nil), pub...)}
}
func (c *Client) get(ctx context.Context, address string, limit int64) ([]byte, error) {
	body, err := c.open(ctx, address)
	if err != nil {
		return nil, err
	}
	defer body.Close()
	raw, err := io.ReadAll(io.LimitReader(body, limit+1))
	if err != nil {
		return nil, err
	}
	if int64(len(raw)) > limit {
		return nil, fmt.Errorf("更新响应超过大小限制")
	}
	return raw, nil
}
func (c *Client) open(ctx context.Context, address string) (io.ReadCloser, error) {
	if address != "https://api.github.com/repos/"+Repository+"/releases?per_page=30" && !SafeAssetURL(address) && !safeFeedURL(address) {
		return nil, fmt.Errorf("更新地址不属于指定GitHub仓库")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, address, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "NGAReader-Updater")
	req.Header.Set("Accept", "application/vnd.github+json")
	if req.URL.Host == "api.github.com" {
		c.mu.Lock()
		retryAt := c.apiRetryAt
		c.mu.Unlock()
		if time.Now().Before(retryAt) {
			return nil, &HTTPError{Status: 403, RetryAt: retryAt, RateLimited: true}
		}
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("连接GitHub失败：%w", err)
	}
	if resp.StatusCode != 200 {
		resp.Body.Close()
		failure := responseError(resp, time.Now())
		if req.URL.Host == "api.github.com" && failure.RateLimited {
			c.mu.Lock()
			c.apiRetryAt = failure.RetryAt
			c.mu.Unlock()
		}
		return nil, failure
	}
	return resp.Body, nil
}
func (c *Client) latestAPI(ctx context.Context, current, target string) (*Offer, error) {
	if _, err := parseVersion(current); err != nil {
		return nil, err
	}
	validTarget := false
	for _, t := range Targets {
		if target == t {
			validTarget = true
		}
	}
	if !validTarget {
		return nil, fmt.Errorf("当前平台暂不提供更新")
	}
	raw, err := c.get(ctx, "https://api.github.com/repos/"+Repository+"/releases?per_page=30", 4<<20)
	if err != nil {
		return nil, err
	}
	var releases []Release
	if err := json.Unmarshal(raw, &releases); err != nil {
		return nil, fmt.Errorf("GitHub版本列表格式异常")
	}
	eligible := releases[:0]
	for _, release := range releases {
		n, err := CompareVersions(release.TagName, current)
		if !release.Draft && err == nil && n > 0 && strings.HasPrefix(release.TagName, "v") {
			eligible = append(eligible, release)
		}
	}
	releases = eligible
	sort.SliceStable(releases, func(i, j int) bool {
		n, err := CompareVersions(releases[i].TagName, releases[j].TagName)
		return err == nil && n > 0
	})
	for _, release := range releases {
		version := strings.TrimPrefix(release.TagName, "v")
		wanted := map[string]Asset{}
		duplicate := false
		for _, a := range release.Assets {
			if a.Name == ManifestName(target) || a.Name == TarName(version, target) {
				if _, ok := wanted[a.Name]; ok {
					duplicate = true
				}
				wanted[a.Name] = a
			}
		}
		if duplicate || len(wanted) != 2 {
			continue
		}
		meta, archive := wanted[ManifestName(target)], wanted[TarName(version, target)]
		if meta.URL != AssetURL(version, meta.Name) || archive.URL != AssetURL(version, archive.Name) || meta.Size <= 0 || meta.Size > MaxManifestBytes || archive.Size <= 0 || archive.Size > MaxArchiveBytes {
			return nil, fmt.Errorf("更新资产地址或大小异常")
		}
		raw, err := c.get(ctx, meta.URL, MaxManifestBytes)
		if err != nil {
			return nil, err
		}
		sum := sha256.Sum256(raw)
		if int64(len(raw)) != meta.Size || meta.Digest != "sha256:"+hex.EncodeToString(sum[:]) {
			return nil, fmt.Errorf("更新说明下载不完整或被更改")
		}
		manifest, err := DecodeManifest(raw, c.pub, target)
		if err != nil {
			return nil, err
		}
		if manifest.Version != version || archive.Size != manifest.Size || archive.Digest != "sha256:"+manifest.SHA256 {
			return nil, fmt.Errorf("更新资产与签名说明不一致")
		}
		return &Offer{Manifest: manifest, Raw: append([]byte(nil), raw...), Prerelease: release.Prerelease}, nil
	}
	return nil, nil
}

// Source pins the already verified manifest and sole archive of this offer.
// The MyGo Update retains this source even if a later check finds a new offer.
type Source struct {
	Client *Client
	Offer  Offer
}

func (s Source) Manifest(context.Context, bool) (io.ReadCloser, error) {
	return io.NopCloser(bytes.NewReader(s.Offer.Raw)), nil
}
func (s Source) Open(ctx context.Context, address string) (io.ReadCloser, error) {
	if address != s.Offer.Manifest.URL {
		return nil, fmt.Errorf("拒绝下载未授权的更新文件")
	}
	return s.Client.open(ctx, address)
}
