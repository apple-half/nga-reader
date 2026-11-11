package nga

import (
	"bytes"
	"encoding/json"
	"fmt"
	"html"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"golang.org/x/text/encoding/simplifiedchinese"
)

var htmlTag = regexp.MustCompile(`<[^>]*>`)
var lineBreak = regexp.MustCompile(`(?i)<br\s*/?>`)

func plain(s string) string {
	return strings.TrimSpace(html.UnescapeString(htmlTag.ReplaceAllString(lineBreak.ReplaceAllString(s, "\n"), "")))
}
func str(v any) string {
	switch x := v.(type) {
	case string:
		return x
	case json.Number:
		return string(x)
	case float64:
		return strconv.FormatFloat(x, 'f', -1, 64)
	}
	return ""
}
func integer(v any) int64         { n, _ := strconv.ParseInt(str(v), 10, 64); return n }
func object(v any) map[string]any { m, _ := v.(map[string]any); return m }
func stamp(v any) time.Time {
	if n := integer(v); n > 0 {
		return time.Unix(n, 0)
	}
	for _, f := range []string{"2006-01-02 15:04:05", time.RFC3339} {
		if t, err := time.ParseInLocation(f, str(v), time.Local); err == nil {
			return t
		}
	}
	return time.Time{}
}

func decode(raw []byte) (map[string]any, error) {
	if !utf8.Valid(raw) {
		var err error
		raw, err = simplifiedchinese.GB18030.NewDecoder().Bytes(raw)
		if err != nil {
			return nil, &APIError{Kind: "parse", Message: "无法解码论坛内容"}
		}
	}
	raw = bytes.TrimPrefix(raw, []byte{0xef, 0xbb, 0xbf})
	d := json.NewDecoder(bytes.NewReader(raw))
	d.UseNumber()
	var root map[string]any
	if err := d.Decode(&root); err != nil {
		d = json.NewDecoder(bytes.NewReader(normalizeJSON(raw)))
		d.UseNumber()
		if err = d.Decode(&root); err != nil {
			return nil, &APIError{Kind: "parse", Message: "论坛返回的数据不完整或格式异常；请重试或打开原帖"}
		}
	}
	if errs, ok := root["error"].([]any); ok && len(errs) > 0 && errs[0] != nil {
		message := plain(str(errs[0]))
		if message == "" {
			message = "论坛拒绝了这次请求"
		}
		return nil, businessError(message, 0)
	}
	data := object(root["data"])
	if data == nil {
		if array, ok := root["data"].([]any); ok && len(array) > 0 {
			if first := object(array[0]); first != nil {
				return first, nil
			}
		}
		data = root
	}
	return data, nil
}

// NGA occasionally emits unquoted numeric object keys. Repair only syntax
// outside strings, never numbers/colons in a post's text, and never eval JS.
func normalizeJSON(raw []byte) []byte {
	out := make([]byte, 0, len(raw)+32)
	quoted, escaped := false, false
	previous := byte(0)
	for i := 0; i < len(raw); i++ {
		b := raw[i]
		if quoted {
			if escaped {
				out = append(out, b)
				escaped = false
				continue
			}
			if b == '\\' {
				out = append(out, b)
				escaped = true
				continue
			}
			if b == '"' {
				quoted = false
			}
			if b < 32 {
				switch b {
				case '\n':
					out = append(out, '\\', 'n')
				case '\r':
					out = append(out, '\\', 'r')
				case '\t':
					out = append(out, '\\', 't')
				default:
					out = append(out, []byte(fmt.Sprintf("\\u%04x", b))...)
				}
				continue
			}
			out = append(out, b)
			continue
		}
		if b == '"' {
			quoted = true
			out = append(out, b)
			previous = b
			continue
		}
		if (previous == '{' || previous == ',') && (b == '-' || (b >= '0' && b <= '9')) {
			end := i
			if raw[end] == '-' {
				end++
			}
			for end < len(raw) && raw[end] >= '0' && raw[end] <= '9' {
				end++
			}
			colon := end
			for colon < len(raw) && (raw[colon] == ' ' || raw[colon] == '\n' || raw[colon] == '\r' || raw[colon] == '\t') {
				colon++
			}
			if end > i && colon < len(raw) && raw[colon] == ':' {
				out = append(out, '"')
				out = append(out, raw[i:end]...)
				out = append(out, '"')
				i = end - 1
				previous = '"'
				continue
			}
		}
		out = append(out, b)
		if b != ' ' && b != '\n' && b != '\r' && b != '\t' {
			previous = b
		}
	}
	return out
}

func businessError(message string, status int) *APIError {
	kind := "permission"
	switch {
	case status == 429 || strings.Contains(message, "频繁") || strings.Contains(message, "频率") || strings.Contains(message, "过快"):
		kind = "rate"
	case strings.Contains(message, "登录") || strings.Contains(message, "访客"):
		kind = "login"
	}
	return &APIError{Kind: kind, Message: message, Status: status}
}

func collection(v any) []map[string]any {
	var out []map[string]any
	switch x := v.(type) {
	case []any:
		for _, v := range x {
			if m := object(v); m != nil {
				out = append(out, m)
			}
		}
	case map[string]any:
		var keys []int
		for k := range x {
			if n, err := strconv.Atoi(k); err == nil {
				keys = append(keys, n)
			}
		}
		sort.Ints(keys)
		for _, k := range keys {
			if m := object(x[strconv.Itoa(k)]); m != nil {
				out = append(out, m)
			}
		}
	}
	return out
}

func validCollection(v any) bool {
	switch v.(type) {
	case []any, map[string]any:
		return true
	}
	return false
}

func topic(m map[string]any) Topic {
	return Topic{ID: integer(m["tid"]), FID: integer(m["fid"]), Title: plain(str(m["subject"])), Author: plain(str(m["author"])), AuthorID: integer(m["authorid"]), Replies: int(integer(m["replies"])), PostDate: stamp(m["postdate"]), LastPost: stamp(m["lastpost"])}
}

func ParseTopics(raw []byte, page int) (TopicPage, error) {
	data, err := decode(raw)
	if err != nil {
		return TopicPage{}, err
	}
	rows, exists := data["__T"]
	if !exists || !validCollection(rows) {
		return TopicPage{}, &APIError{Kind: "parse", Message: "论坛返回的数据中没有帖子列表"}
	}
	result := TopicPage{Page: max(1, page)}
	for _, m := range collection(rows) {
		t := topic(m)
		if t.ID != 0 {
			result.Topics = append(result.Topics, t)
		}
	}
	per := int(integer(data["__T__ROWS_PAGE"]))
	if per <= 0 {
		per = 50
	}
	total := int(integer(data["__ROWS"]))
	if total > 0 {
		result.TotalPages = max(1, (total+per-1)/per)
		result.HasNext = result.Page < result.TotalPages
	} else {
		result.HasNext = len(result.Topics) >= per
	}
	result.BoardName = plain(str(object(data["__F"])["name"]))
	return result, nil
}

func ParseThread(raw []byte, page int) (ThreadPage, error) {
	data, err := decode(raw)
	if err != nil {
		return ThreadPage{}, err
	}
	rows, exists := data["__R"]
	if !exists || !validCollection(rows) {
		return ThreadPage{}, &APIError{Kind: "parse", Message: "论坛返回的数据中没有正文；请检查账号权限"}
	}
	m := object(data["__T"])
	if _, ok := m["tid"]; !ok {
		if values := collection(m); len(values) > 0 {
			m = values[0]
		}
	}
	result := ThreadPage{Topic: topic(m), Page: max(1, page)}
	result.MainAuthorID = result.Topic.AuthorID
	users := object(data["__U"])
	per := int(integer(data["__R__ROWS_PAGE"]))
	if per <= 0 {
		per = 20
	}
	for i, m := range collection(rows) {
		authorID := integer(m["authorid"])
		name := plain(str(object(users[strconv.FormatInt(authorID, 10)])["username"]))
		if name == "" {
			name = plain(str(m["author"]))
		}
		if name == "" {
			name = fmt.Sprintf("UID %d", authorID)
		}
		if strings.HasPrefix(name, "#anony_") {
			name = "匿名用户 · " + strings.TrimPrefix(name, "#anony_")[:min(6, len(strings.TrimPrefix(name, "#anony_")))]
		}
		floor := (result.Page-1)*per + i
		if v, ok := m["lou"]; ok {
			floor = int(integer(v))
		}
		result.Posts = append(result.Posts, Post{ID: integer(m["pid"]), Floor: floor, AuthorID: authorID, Author: name, Content: str(m["content"]), PostDate: stamp(m["postdate"])})
		if floor == 0 && result.MainAuthorID == 0 {
			result.MainAuthorID = authorID
		}
	}
	result.TotalRows = max(int(integer(data["__R__ROWS"])), int(integer(data["__ROWS"])), result.Topic.Replies+1, len(result.Posts))
	result.TotalPages = max(1, (result.TotalRows+per-1)/per)
	return result, nil
}
