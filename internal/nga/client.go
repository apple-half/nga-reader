package nga

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"
)

const BaseURL = "https://bbs.nga.cn"
const MaxResponseBytes = 8 << 20

type Client struct {
	base string
	http *http.Client
	mu   sync.RWMutex
	jar  *cookieStore
}

func NewClient(client *http.Client) *Client {
	if client == nil {
		client = &http.Client{Timeout: 25 * time.Second}
	}
	clone := *client
	if clone.Timeout == 0 {
		clone.Timeout = 25 * time.Second
	}
	clone.Jar = nil
	clone.CheckRedirect = func(req *http.Request, via []*http.Request) error {
		if len(via) >= 5 {
			return fmt.Errorf("论坛重定向过多")
		}
		if req.URL.Scheme != "https" || req.URL.Host != "bbs.nga.cn" {
			return fmt.Errorf("已阻止论坛跳转到其他地址")
		}
		return nil
	}
	return &Client{base: BaseURL, http: &clone, jar: newCookieStore()}
}

func (c *Client) Topics(ctx context.Context, fid int64, page int) (TopicPage, error) {
	if fid == 0 || page < 1 {
		return TopicPage{}, fmt.Errorf("无效的板块或页码")
	}
	raw, err := c.get(ctx, "/thread.php", url.Values{"fid": {strconv.FormatInt(fid, 10)}, "page": {strconv.Itoa(page)}})
	if err != nil {
		return TopicPage{}, err
	}
	return ParseTopics(raw, page)
}
func (c *Client) Thread(ctx context.Context, tid int64, page int, authorID int64) (ThreadPage, error) {
	if tid <= 0 || page < 1 {
		return ThreadPage{}, fmt.Errorf("无效的帖子或页码")
	}
	values := url.Values{"tid": {strconv.FormatInt(tid, 10)}, "page": {strconv.Itoa(page)}}
	if authorID != 0 {
		values.Set("authorid", strconv.FormatInt(authorID, 10))
	}
	raw, err := c.get(ctx, "/read.php", values)
	if err != nil {
		return ThreadPage{}, err
	}
	result, err := ParseThread(raw, page)
	var api *APIError
	if !errors.As(err, &api) || api.Kind != "parse" {
		return result, err
	}
	if err := ctx.Err(); err != nil {
		return ThreadPage{}, err
	}
	// Some NGA read.php JSON responses end in the middle of a reply. The
	// static noBBCode page still contains the complete posts. Fetch it once
	// with the same session and scope, without executing any page resources.
	values.Set("noBBCode", "")
	values.Del("__output")
	raw, err = c.get(ctx, "/read.php", values)
	if err != nil {
		return ThreadPage{}, err
	}
	return ParseThreadHTML(raw, tid, page)
}

func (c *Client) get(ctx context.Context, path string, query url.Values) ([]byte, error) {
	return c.request(ctx, http.MethodGet, path, query, nil)
}
func (c *Client) request(ctx context.Context, method, path string, query, form url.Values) ([]byte, error) {
	if !query.Has("noBBCode") {
		query.Set("__output", "11")
	}
	query.Set("__inchst", "UTF8")
	var body io.Reader
	if form != nil {
		body = strings.NewReader(form.Encode())
	}
	req, err := http.NewRequestWithContext(ctx, method, c.base+path+"?"+query.Encode(), body)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "NGAReader/0.1")
	if form != nil {
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	}
	req.Header.Set("Accept", "application/json")
	if query.Has("noBBCode") {
		req.Header.Set("Accept", "text/html")
	}
	req.Header.Set("Referer", BaseURL+"/")
	client := *c.http
	c.mu.RLock()
	client.Jar = c.jar
	c.mu.RUnlock()
	response, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("连接论坛失败：%w", err)
	}
	defer response.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(response.Body, MaxResponseBytes+1))
	if err != nil {
		return nil, fmt.Errorf("读取论坛响应失败：%w", err)
	}
	if len(raw) > MaxResponseBytes {
		return nil, fmt.Errorf("论坛响应过大，已停止加载")
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		if _, err := decode(raw); err != nil {
			if api, ok := err.(*APIError); ok && api.Kind != "parse" {
				api.Status = response.StatusCode
				return nil, api
			}
		}
		return nil, businessError(fmt.Sprintf("论坛返回 HTTP %d，请稍后重试", response.StatusCode), response.StatusCode)
	}
	return raw, nil
}

func (c *Client) SetCookies(cookies []*http.Cookie) {
	jar := newCookieStore()
	u, _ := url.Parse(BaseURL)
	jar.SetCookies(u, cookies)
	c.mu.Lock()
	c.jar = jar
	c.mu.Unlock()
}
func (c *Client) Cookies() []*http.Cookie {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.jar.snapshot()
}
func (c *Client) HasSession() bool {
	values := map[string]string{}
	for _, v := range c.Cookies() {
		values[v.Name] = v.Value
	}
	return values["ngaPassportUid"] != "" && values["ngaPassportCid"] != ""
}
func (c *Client) ImportCookieHeader(value string) error {
	value = strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(value), "Cookie:"))
	if value == "" || len(value) > 16384 || strings.ContainsAny(value, "\r\n") {
		return fmt.Errorf("Cookie 格式无效；请只粘贴 Cookie 值，不包含其他请求头")
	}
	var cookies []*http.Cookie
	hasUID, hasCID := false, false
	for _, part := range strings.Split(value, ";") {
		pair := strings.SplitN(strings.TrimSpace(part), "=", 2)
		if len(pair) != 2 {
			return fmt.Errorf("Cookie 格式无效")
		}
		cookie := &http.Cookie{Name: pair[0], Value: pair[1], Domain: "bbs.nga.cn", Path: "/", Secure: true}
		if cookie.Valid() != nil {
			return fmt.Errorf("Cookie 中有无效字符")
		}
		if cookie.Name == "ngaPassportUid" {
			id, _ := strconv.ParseInt(cookie.Value, 10, 64)
			hasUID = id > 0
		}
		if cookie.Name == "ngaPassportCid" {
			hasCID = cookie.Value != ""
		}
		cookies = append(cookies, cookie)
	}
	if !hasUID || !hasCID {
		return fmt.Errorf("缺少有效的 ngaPassportUid 或 ngaPassportCid；无需在聊天中发送这些内容")
	}
	c.SetCookies(cookies)
	return nil
}

func ParseLink(value string) (tid, fid int64, err error) {
	u, e := url.Parse(strings.TrimSpace(value))
	if e != nil || u == nil {
		return 0, 0, fmt.Errorf("链接格式不正确")
	}
	validHost := u.Host == "bbs.nga.cn" || u.Host == "ngabbs.com" || u.Host == "nga.178.com"
	if !validHost || u.Scheme != "https" || u.User != nil {
		return 0, 0, fmt.Errorf("请使用 NGA 官方 HTTPS 帖子或板块链接")
	}
	switch u.Path {
	case "/read.php":
		tid, _ = strconv.ParseInt(u.Query().Get("tid"), 10, 64)
		if tid > 0 {
			return tid, 0, nil
		}
	case "/thread.php":
		fid, _ = strconv.ParseInt(u.Query().Get("fid"), 10, 64)
		if fid != 0 {
			return 0, fid, nil
		}
	}
	return 0, 0, fmt.Errorf("链接中缺少有效的 tid 或 fid")
}

func ImageURL(value string) (string, error) {
	value = strings.TrimSpace(value)
	if strings.HasPrefix(value, "./") {
		value = "https://img.nga.cn/attachments/" + strings.TrimPrefix(value, "./")
	}
	if strings.HasPrefix(value, "//") {
		value = "https:" + value
	}
	u, err := url.Parse(value)
	if err != nil || u == nil {
		return "", fmt.Errorf("图片地址无效")
	}
	host := strings.ToLower(u.Host)
	switch host {
	case "img.nga.178.com":
		host = "img.nga.cn"
	case "img4.nga.178.com":
		host = "img4.nga.cn"
	case "img8.nga.178.com":
		host = "img8.nga.cn"
	}
	allowed := host == "img.nga.cn" || host == "img4.nga.cn" || host == "img8.nga.cn"
	if (u.Scheme != "https" && u.Scheme != "http") || !allowed || u.User != nil {
		return "", fmt.Errorf("仅加载 NGA 的 HTTPS 附件")
	}
	u.Scheme, u.Host = "https", host
	return u.String(), nil
}

func (c *Client) Image(ctx context.Context, address string) ([]byte, error) {
	safe, err := ImageURL(address)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, safe, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Referer", BaseURL+"/")
	client := *c.http
	client.Jar = nil
	client.CheckRedirect = func(req *http.Request, via []*http.Request) error {
		if len(via) > 3 {
			return fmt.Errorf("图片重定向过多")
		}
		_, err := ImageURL(req.URL.String())
		return err
	}
	response, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()
	if response.StatusCode != 200 {
		return nil, fmt.Errorf("图片加载失败 (%d)", response.StatusCode)
	}
	raw, err := io.ReadAll(io.LimitReader(response.Body, MaxResponseBytes+1))
	if len(raw) > MaxResponseBytes {
		return nil, fmt.Errorf("图片过大")
	}
	return raw, err
}
