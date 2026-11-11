package nga

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
	"unicode/utf8"

	"golang.org/x/text/encoding/simplifiedchinese"
)

const LoginURL = BaseURL + "/nuke.php?__lib=login&__act=account&login"

var loginMeta = regexp.MustCompile(`(?i)<meta\b[^>]*>`)
var loginCharset = regexp.MustCompile(`(?i)(charset\s*=\s*['"]?)(?:gbk|gb2312|gb18030|utf-?8)\b`)

// LoginDocument fetches only the public official login wrapper, without a
// session. NGA currently labels its GBK wrapper as HTTP UTF-8. Normalize that
// one document before showing it at its original HTTPS URL; the official
// iframe and all credential submissions still go directly through the WebView.
func (c *Client) LoginDocument(ctx context.Context) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.base+"/nuke.php?__lib=login&__act=account&login", nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Accept", "text/html")
	req.Header.Set("User-Agent", "NGAReader/0.1")
	response, err := c.http.Do(req)
	if err != nil {
		return "", err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return "", fmt.Errorf("官方登录页返回 HTTP %d", response.StatusCode)
	}
	raw, err := io.ReadAll(io.LimitReader(response.Body, (1<<20)+1))
	if err != nil {
		return "", err
	}
	if len(raw) > 1<<20 {
		return "", fmt.Errorf("官方登录页响应过大")
	}
	if !utf8.Valid(raw) {
		raw, err = simplifiedchinese.GB18030.NewDecoder().Bytes(raw)
		if err != nil || bytes.Contains(raw, []byte("\ufffd")) {
			return "", fmt.Errorf("无法解码官方登录页")
		}
	}
	document := string(raw)
	if !strings.Contains(strings.ToLower(document), "<html") {
		return "", fmt.Errorf("官方登录页返回了无法显示的内容")
	}
	return loginMeta.ReplaceAllStringFunc(document, func(meta string) string {
		return loginCharset.ReplaceAllString(meta, "${1}utf-8")
	}), nil
}
