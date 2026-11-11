package nga

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/url"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"golang.org/x/net/html"
	"golang.org/x/text/encoding/simplifiedchinese"
)

var postContentID = regexp.MustCompile(`^postcontent([0-9]+)$`)
var postAnchorID = regexp.MustCompile(`^pid([0-9]+)Anchor$`)
var htmlDocumentEnd = regexp.MustCompile(`(?i)</html\s*>\s*$`)

// This recognizes only NGA's literal pagination initializer. It is not a
// JavaScript parser/evaluator. The path is validated against the requested tid.
var pageLiteral = regexp.MustCompile(`\bvar\s+__PAGE\s*=\s*\{\s*0\s*:\s*['"]([^'"]+)['"]\s*,\s*1\s*:\s*(\d+)\s*,\s*2\s*:\s*(\d+)\s*,\s*3\s*:\s*(\d+)\s*\}`)

// ParseThreadHTML reads NGA's server-rendered noBBCode response as data.
// It never executes scripts or loads linked scripts, images, or other resources.
func ParseThreadHTML(raw []byte, tid int64, page int) (ThreadPage, error) {
	fail := func() (ThreadPage, error) {
		return ThreadPage{}, &APIError{Kind: "parse", Message: "论坛正文暂时无法读取；请稍后重试或打开原帖"}
	}
	if len(raw) > MaxResponseBytes || tid <= 0 || page < 1 {
		return fail()
	}
	// html.Parse repairs missing closing tags. Check the actual response end
	// first so a truncated HTML fallback cannot silently omit later replies.
	if !htmlDocumentEnd.Match(raw) {
		return fail()
	}
	if !utf8.Valid(raw) {
		var err error
		raw, err = simplifiedchinese.GB18030.NewDecoder().Bytes(raw)
		if err != nil {
			return fail()
		}
	}
	doc, err := html.Parse(bytes.NewReader(raw))
	if err != nil {
		return fail()
	}
	ids := map[string]*html.Node{}
	var scripts []*html.Node
	visitHTML(doc, func(n *html.Node) bool {
		if n.Type != html.ElementNode {
			return true
		}
		if id := nodeAttr(n, "id"); id != "" {
			if ids[id] == nil {
				ids[id] = n
			}
		}
		if n.Data == "script" {
			// Only page metadata outside posted content can supply names/pages.
			if !insidePost(n) {
				scripts = append(scripts, n)
			}
			return false
		}
		return true
	})
	result := ThreadPage{Topic: Topic{ID: tid}, Page: page, TotalPages: 1}
	// Require the real forum breadcrumb, not just an arbitrary HTML document
	// with a postcontent-like ID (e.g. a login, challenge, or error page).
	matched := false
	if nav := ids["m_nav"]; nav != nil {
		visitHTML(nav, func(n *html.Node) bool {
			if n.Type != html.ElementNode || n.Data != "a" {
				return true
			}
			u, e := url.Parse(nodeAttr(n, "href"))
			if e != nil {
				return true
			}
			if !sameForumURL(u) {
				return true
			}
			switch strings.TrimPrefix(u.Path, "/") {
			case "read.php":
				if u.Query().Get("tid") == strconv.FormatInt(tid, 10) {
					matched = true
					result.Topic.Title = strings.TrimSpace(nodeText(n))
				}
			case "thread.php":
				if fid, e := strconv.ParseInt(u.Query().Get("fid"), 10, 64); e == nil {
					result.Topic.FID = fid
				}
			}
			return true
		})
	}
	if !matched {
		return fail()
	}
	users := map[string]any{}
	per, foundPages := 20, false
	for _, script := range scripts {
		s := nodeText(script)
		if p := pageLiteral.FindStringSubmatch(s); p != nil {
			u, e := url.Parse(p[1])
			if e != nil || !sameForumURL(u) || u.Path != "/read.php" || u.Query().Get("tid") != strconv.FormatInt(tid, 10) {
				return fail()
			}
			total, _ := strconv.Atoi(p[2])
			current, _ := strconv.Atoi(p[3])
			size, _ := strconv.Atoi(p[4])
			if total < page || current != page || size < 1 || size > 100 || total > 1000000 {
				return fail()
			}
			result.TotalPages, per, foundPages = total, size, true
		}
		const init = "commonui.userInfo.setAll("
		if at := strings.Index(s, init); at >= 0 {
			// Decode only a JSON value, never the surrounding JavaScript.
			d := json.NewDecoder(bytes.NewReader(normalizeJSON([]byte(s[at+len(init):]))))
			d.UseNumber()
			var data map[string]any
			if d.Decode(&data) == nil {
				for key, user := range data {
					users[key] = user
				}
			}
		}
	}
	if page > 1 && !foundPages {
		return fail()
	}
	var indexes []int
	for id, node := range ids {
		m := postContentID.FindStringSubmatch(id)
		if m == nil || !hasClass(node, "postcontent") {
			continue
		}
		i, e := strconv.Atoi(m[1])
		if e != nil || i < 0 || i > 1000000 {
			continue
		}
		container := ids[fmt.Sprintf("postcontainer%d", i)]
		if container != nil && descendantOf(node, container) {
			indexes = append(indexes, i)
		}
	}
	sort.Ints(indexes)
	for _, i := range indexes {
		suffix := strconv.Itoa(i)
		container, body := ids["postcontainer"+suffix], ids["postcontent"+suffix]
		author := ids["postauthor"+suffix]
		if author == nil {
			return fail()
		}
		u, e := url.Parse(nodeAttr(author, "href"))
		if e != nil || !sameForumURL(u) {
			return fail()
		}
		uid, e := strconv.ParseInt(u.Query().Get("uid"), 10, 64)
		if e != nil || uid == 0 {
			return fail()
		}
		name := plain(str(object(users[strconv.FormatInt(uid, 10)])["username"]))
		if name == "" {
			name = strings.TrimSpace(nodeText(author))
		}
		if name == "" {
			name = fmt.Sprintf("UID %d", uid)
		}
		if strings.HasPrefix(name, "#anony_") {
			name = "匿名用户"
		}
		post := Post{Floor: (page-1)*per + i, AuthorID: uid, Author: name, Content: strings.TrimSpace(postText(body))}
		foundPID := false
		visitHTML(container, func(n *html.Node) bool {
			if n == body {
				return false
			}
			if m := postAnchorID.FindStringSubmatch(nodeAttr(n, "id")); m != nil {
				post.ID, _ = strconv.ParseInt(m[1], 10, 64)
				foundPID = true
			}
			if n.Type == html.ElementNode && n.Data == "a" && strings.HasPrefix(nodeAttr(n, "name"), "l") {
				if floor, e := strconv.Atoi(strings.TrimPrefix(nodeAttr(n, "name"), "l")); e == nil && floor >= 0 {
					post.Floor = floor
				}
			}
			return n.Type != html.ElementNode || n.Data != "script"
		})
		if !foundPID {
			return fail()
		}
		if date := ids["postdate"+suffix]; date != nil {
			value := strings.TrimSpace(nodeText(date))
			post.PostDate = stamp(value)
			if post.PostDate.IsZero() {
				post.PostDate, _ = time.ParseInLocation("2006-01-02 15:04", value, time.Local)
			}
		}
		result.Posts = append(result.Posts, post)
		if post.Floor == 0 {
			result.MainAuthorID, result.Topic.AuthorID, result.Topic.Author, result.Topic.PostDate = uid, uid, name, post.PostDate
		}
	}
	if len(result.Posts) == 0 {
		return fail()
	}
	// Static HTML supplies total pages, but not an exact total reply count.
	// Keep TotalRows unknown (0); the UI displays the number on this page.
	return result, nil
}

func sameForumURL(u *url.URL) bool {
	return u.User == nil && ((u.Scheme == "" && u.Host == "") || (u.Scheme == "https" && u.Host == "bbs.nga.cn"))
}
func nodeAttr(n *html.Node, name string) string {
	for _, a := range n.Attr {
		if a.Key == name {
			return a.Val
		}
	}
	return ""
}
func hasClass(n *html.Node, class string) bool {
	for _, c := range strings.Fields(nodeAttr(n, "class")) {
		if c == class {
			return true
		}
	}
	return false
}
func visitHTML(n *html.Node, visit func(*html.Node) bool) {
	if !visit(n) {
		return
	}
	for p := n.FirstChild; p != nil; p = p.NextSibling {
		visitHTML(p, visit)
	}
}
func descendantOf(n, parent *html.Node) bool {
	for n = n.Parent; n != nil; n = n.Parent {
		if n == parent {
			return true
		}
	}
	return false
}
func insidePost(n *html.Node) bool {
	for n = n.Parent; n != nil; n = n.Parent {
		if strings.HasPrefix(nodeAttr(n, "id"), "postcontainer") || hasClass(n, "postcontent") {
			return true
		}
	}
	return false
}
func nodeText(n *html.Node) string {
	var b strings.Builder
	visitHTML(n, func(p *html.Node) bool {
		if p.Type == html.TextNode {
			b.WriteString(p.Data)
		}
		return true
	})
	return b.String()
}
func postText(n *html.Node) string {
	var b strings.Builder
	var walk func(*html.Node)
	walk = func(p *html.Node) {
		if p.Type == html.TextNode {
			b.WriteString(p.Data)
			return
		}
		if p.Type == html.ElementNode {
			switch p.Data {
			case "script", "style", "iframe", "object", "embed", "noscript":
				return
			case "br":
				b.WriteByte('\n')
				return
			case "img":
				if address, e := ImageURL(nodeAttr(p, "src")); e == nil {
					b.WriteString("[img]" + address + "[/img]")
				}
				return
			}
		}
		tag := map[string]string{"b": "b", "strong": "b", "i": "i", "em": "i", "u": "u", "del": "del", "s": "del"}[p.Data]
		if tag != "" {
			b.WriteString("[" + tag + "]")
		}
		for c := p.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
		if tag != "" {
			b.WriteString("[/" + tag + "]")
		}
	}
	walk(n)
	return b.String()
}
