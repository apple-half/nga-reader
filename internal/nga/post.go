package nga

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"html"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"unicode/utf8"

	htmlparse "golang.org/x/net/html"
	"golang.org/x/net/html/atom"
	"golang.org/x/text/encoding/simplifiedchinese"
)

const MaxPostSubjectRunes = 128
const MaxPostContentRunes = 20000

type PostTarget struct {
	Action string `json:"action"`
	FID    int64  `json:"fid"`
	TID    int64  `json:"tid"`
	PID    int64  `json:"pid"`
}

func (t PostTarget) Validate() error {
	switch t.Action {
	case "new":
		if t.FID != 0 && t.TID == 0 && t.PID == 0 {
			return nil
		}
	case "reply", "quote":
		if t.TID > 0 && t.PID >= 0 {
			return nil
		}
	}
	return fmt.Errorf("无效的发帖操作或目标")
}

func ValidatePostText(target PostTarget, subject, content string) error {
	if err := target.Validate(); err != nil {
		return err
	}
	if target.Action == "new" && strings.TrimSpace(subject) == "" {
		return fmt.Errorf("请填写帖子标题")
	}
	if strings.TrimSpace(content) == "" {
		return fmt.Errorf("请填写正文")
	}
	if !utf8.ValidString(subject) || !utf8.ValidString(content) || strings.ContainsRune(subject+content, 0) {
		return fmt.Errorf("标题或正文包含无效字符")
	}
	if utf8.RuneCountInString(subject) > MaxPostSubjectRunes || utf8.RuneCountInString(content) > MaxPostContentRunes {
		return fmt.Errorf("标题最多%d字，正文最多%d字；论坛可能有更严格限制", MaxPostSubjectRunes, MaxPostContentRunes)
	}
	return nil
}

type PostPreparation struct {
	Target           PostTarget
	UID              int64
	Subject, Content string
	owner            *Client
	session          [32]byte
	cookies          []*http.Cookie
}

type PublishResult struct {
	TID, PID    int64
	Message     string
	UnderReview bool
}

// Uncertain means the server may already have committed the post. Callers must
// retain their durable pending record and must not automatically retry.
type PublishError struct {
	Err       error
	Uncertain bool
}

func (e *PublishError) Error() string { return e.Err.Error() }
func (e *PublishError) Unwrap() error { return e.Err }

func sessionIdentity(cookies []*http.Cookie) (int64, [32]byte) {
	var uid int64
	var cid string
	for _, cookie := range cookies {
		if cookie.Name == "ngaPassportUid" {
			uid, _ = strconv.ParseInt(cookie.Value, 10, 64)
		}
		if cookie.Name == "ngaPassportCid" {
			cid = cookie.Value
		}
	}
	if uid <= 0 || cid == "" {
		return 0, [32]byte{}
	}
	return uid, sha256.Sum256([]byte(strconv.FormatInt(uid, 10) + "\x00" + cid))
}

func (c *Client) SessionUID() int64 { uid, _ := sessionIdentity(c.Cookies()); return uid }

func (t PostTarget) query() url.Values {
	q := url.Values{"action": {t.Action}, "tid": {strconv.FormatInt(t.TID, 10)}, "pid": {strconv.FormatInt(t.PID, 10)}, "__output": {"8"}}
	if t.FID != 0 {
		q.Set("fid", strconv.FormatInt(t.FID, 10))
	}
	return q
}

func (c *Client) frozen(cookies []*http.Cookie) *Client {
	copy := NewClient(c.http)
	copy.base = c.base
	copy.SetCookies(cookies)
	return copy
}

func (c *Client) PreparePost(ctx context.Context, target PostTarget) (PostPreparation, error) {
	if err := target.Validate(); err != nil {
		return PostPreparation{}, err
	}
	cookies := c.Cookies()
	uid, identity := sessionIdentity(cookies)
	if uid == 0 {
		return PostPreparation{}, businessError("请先登录 NGA 后再发帖", 0)
	}
	snapshot := c.frozen(cookies)
	raw, err := snapshot.request(ctx, http.MethodGet, "/post.php", target.query(), nil)
	if err != nil {
		return PostPreparation{}, err
	}
	root, err := postDocument(raw)
	if err != nil {
		return PostPreparation{}, err
	}
	if err = postRejection(root); err != nil {
		return PostPreparation{}, err
	}
	data := object(root["data"])
	if integer(object(data["__CU"])["uid"]) != uid {
		return PostPreparation{}, businessError("无法确认发帖账号；请重新登录后再试", 0)
	}
	if target.Action == "new" {
		if integer(data["fid"]) != target.FID {
			return PostPreparation{}, fmt.Errorf("发帖准备返回了不同板块，已停止")
		}
	} else {
		if integer(data["tid"]) != target.TID || integer(data["pid"]) != target.PID || integer(data["fid"]) == 0 {
			return PostPreparation{}, fmt.Errorf("发帖准备返回了不同帖子或楼层，已停止")
		}
		target.FID = integer(data["fid"])
	}
	return PostPreparation{Target: target, UID: uid, Subject: html.UnescapeString(str(data["subject"])), Content: html.UnescapeString(lineBreak.ReplaceAllString(str(data["content"]), "\n")), owner: c, session: identity, cookies: snapshot.Cookies()}, nil
}

func postDocument(raw []byte) (map[string]any, error) {
	if !utf8.Valid(raw) {
		var err error
		raw, err = simplifiedchinese.GB18030.NewDecoder().Bytes(raw)
		if err != nil {
			return nil, fmt.Errorf("无法解码发帖响应")
		}
	}
	raw = bytes.TrimPrefix(raw, []byte{0xef, 0xbb, 0xbf})
	d := json.NewDecoder(bytes.NewReader(normalizeJSON(raw)))
	d.UseNumber()
	var root map[string]any
	if err := d.Decode(&root); err != nil || root == nil {
		return nil, fmt.Errorf("论坛返回了无法识别的发帖结果")
	}
	var extra any
	if err := d.Decode(&extra); err != io.EOF {
		return nil, fmt.Errorf("论坛发帖响应不完整或含有额外内容")
	}
	return root, nil
}

func postIndex(value any, index int) any {
	switch x := value.(type) {
	case []any:
		if index < len(x) {
			return x[index]
		}
	case map[string]any:
		return x[strconv.Itoa(index)]
	}
	return nil
}

func postRejection(root map[string]any) error {
	message := plain(str(postIndex(root["error"], 0)))
	if message == "" {
		message = plain(str(root["error"]))
	}
	if message != "" {
		return businessError(message, 0)
	}
	message = plain(str(postIndex(object(root["data"])["__MESSAGE"], 1)))
	if definitelyRejected(message) {
		return businessError(message, 0)
	}
	return nil
}

func definitelyRejected(message string) bool {
	for _, text := range []string{"没有登录", "未登录", "请先登录", "重新登录", "权限不足", "没有权限", "无权限", "禁止发言", "禁言", "禁止回复", "锁定", "发帖过快", "发贴过快", "太频繁", "频率", "稍后再试", "验证码", "验证失败", "标题不能为空", "正文不能为空", "字数不足", "超过字数", "审核未通过", "审核不通过"} {
		if strings.Contains(message, text) {
			return true
		}
	}
	return false
}

func (c *Client) PublishPost(ctx context.Context, prepared PostPreparation, subject, content string) (PublishResult, error) {
	fail := func(err error, uncertain bool) (PublishResult, error) {
		return PublishResult{}, &PublishError{Err: err, Uncertain: uncertain}
	}
	if err := ValidatePostText(prepared.Target, subject, content); err != nil {
		return fail(err, false)
	}
	uid, identity := sessionIdentity(c.Cookies())
	if uid == 0 || prepared.owner != c || uid != prepared.UID || identity != prepared.session {
		return fail(fmt.Errorf("登录会话已变化，请重新准备后发送"), false)
	}
	if err := ctx.Err(); err != nil {
		return fail(err, false)
	}
	form := url.Values{"step": {"2"}, "post_subject": {subject}, "post_content": {content}}
	query := prepared.Target.query()
	query.Set("__inchst", "UTF8")
	// A non-replayable body and a redirect refusal prevent a 307/308 (or a
	// transport retry) from silently submitting the same user content twice.
	encoded := form.Encode()
	body := io.NopCloser(strings.NewReader(encoded))
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.base+"/post.php?"+query.Encode(), body)
	if err != nil {
		return fail(err, false)
	}
	req.ContentLength = int64(len(encoded))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Referer", BaseURL+"/post.php?"+prepared.Target.query().Encode())
	req.Header.Set("Origin", BaseURL)
	req.Header.Set("User-Agent", "NGAReader/0.1")
	client := *c.http
	client.Jar = c.frozen(prepared.cookies).jar
	client.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	response, err := client.Do(req)
	if err != nil {
		return fail(fmt.Errorf("发送连接中断，结果待确认：%w", err), true)
	}
	defer response.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(response.Body, MaxResponseBytes+1))
	if err != nil || len(raw) > MaxResponseBytes {
		return fail(fmt.Errorf("未能完整读取发送结果，请先检查原帖"), true)
	}
	root, err := postDocument(raw)
	if err != nil {
		return fail(err, true)
	}
	if err = postRejection(root); err != nil {
		return fail(err, response.StatusCode >= 500 || !definitelyRejected(err.Error()))
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return fail(fmt.Errorf("论坛返回HTTP%d，发送结果待确认", response.StatusCode), true)
	}
	messageData := object(root["data"])["__MESSAGE"]
	message := plain(str(postIndex(messageData, 1)))
	success := false
	for _, prefix := range []string{"发贴完毕", "发帖完毕", "发表成功", "发帖成功", "回复成功", "主题发布成功", "已提交审核"} {
		if strings.HasPrefix(message, prefix) {
			success = true
			break
		}
	}
	if !success {
		return fail(fmt.Errorf("未收到明确的发表完成结果，请先检查原帖"), true)
	}
	result := PublishResult{TID: prepared.Target.TID, Message: message, UnderReview: strings.Contains(message, "审核")}
	// Navigation is data only. A link never makes an otherwise unknown result
	// successful, and non-NGA links are never opened or fetched.
	fragment, _ := htmlparse.Parse(strings.NewReader(str(postIndex(messageData, 2))))
	var visit func(*htmlparse.Node)
	visit = func(n *htmlparse.Node) {
		if n.Type == htmlparse.ElementNode && n.DataAtom == atom.A {
			for _, attr := range n.Attr {
				if attr.Key == "href" {
					u, e := url.Parse(attr.Val)
					if e != nil || u.User != nil || (u.Host != "" && u.Host != "bbs.nga.cn") || (u.Scheme != "" && u.Scheme != "https") || (u.Path != "read.php" && u.Path != "/read.php") {
						continue
					}
					tid, _ := strconv.ParseInt(u.Query().Get("tid"), 10, 64)
					pid, _ := strconv.ParseInt(u.Query().Get("pid"), 10, 64)
					if tid > 0 && (prepared.Target.TID == 0 || tid == prepared.Target.TID) {
						result.TID = tid
						if pid > 0 {
							result.PID = pid
						}
					}
				}
			}
		}
		for child := n.FirstChild; child != nil; child = child.NextSibling {
			visit(child)
		}
	}
	if fragment != nil {
		visit(fragment)
	}
	return result, nil
}

func (c *Client) TopicCategories(ctx context.Context, fid int64) ([]string, error) {
	if fid == 0 {
		return nil, fmt.Errorf("无效的板块")
	}
	raw, err := c.get(ctx, "/nuke.php", url.Values{"__lib": {"topic_key"}, "__act": {"get"}, "fid": {strconv.FormatInt(fid, 10)}, "__output": {"8"}})
	if err != nil {
		return nil, err
	}
	root, err := postDocument(raw)
	if err != nil {
		return nil, err
	}
	if err = postRejection(root); err != nil {
		return nil, err
	}
	data := object(root["data"])
	if data == nil {
		return nil, fmt.Errorf("无法识别板块分类")
	}
	var out []string
	seen := map[string]bool{}
	for _, item := range collection(data["0"]) {
		text := plain(str(item["0"]))
		if text != "" && utf8.RuneCountInString(text) <= 64 && !seen[text] && len(out) < 100 {
			out = append(out, text)
			seen[text] = true
		}
	}
	return out, nil
}
