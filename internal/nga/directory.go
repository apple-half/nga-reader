package nga

import (
	"bytes"
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"html"
	"io"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"golang.org/x/text/encoding/simplifiedchinese"
	"golang.org/x/text/unicode/norm"
)

const BoardDirectoryURL = "https://img4.nga.cn/proxy/cache_attach/bbs_index_data.js"
const MaxDirectoryBytes = 1 << 20

// Snapshot of the public homepage directory, fetched 2026-11-12. This is data,
// never executable JavaScript; a failed refresh must not remove this fallback.
//
//go:embed directory_seed.json
var directorySeed []byte

type DirectoryEntry struct {
	ID           int64    `json:"fid"`
	CollectionID int64    `json:"stid,omitempty"`
	Name         string   `json:"name"`
	Description  string   `json:"description,omitempty"`
	Category     string   `json:"category,omitempty"`
	Aliases      []string `json:"aliases,omitempty"`
}

func EmbeddedBoardDirectory() []DirectoryEntry {
	entries, _ := ParseBoardDirectory(directorySeed)
	return entries
}

// BoardDirectory intentionally uses neither the forum session nor its request
// helper. The fixed CDN is public and cannot receive account cookies on redirects.
func (c *Client) BoardDirectory(ctx context.Context) ([]byte, error) {
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, BoardDirectoryURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "NGAReader/0.1")
	req.Header.Set("Accept", "application/javascript, application/json")
	client := *c.http
	client.Jar = nil
	client.CheckRedirect = func(req *http.Request, via []*http.Request) error {
		if len(via) >= 3 || req.URL.Scheme != "https" || req.URL.Host != "img4.nga.cn" || req.URL.User != nil {
			return fmt.Errorf("已阻止板块目录跳转到其他地址")
		}
		return nil
	}
	res, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("读取公开板块目录失败：%w", err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("板块目录返回 HTTP %d", res.StatusCode)
	}
	raw, err := io.ReadAll(io.LimitReader(res.Body, MaxDirectoryBytes+1))
	if err != nil {
		return nil, fmt.Errorf("读取板块目录失败：%w", err)
	}
	if _, err := ParseBoardDirectory(raw); err != nil {
		return nil, err
	}
	return raw, nil
}

// ParseBoardDirectory accepts the exact homepage assignment or its JSON data.
// No script, HTML, arbitrary assignment or trailing statement is evaluated.
func ParseBoardDirectory(raw []byte) ([]DirectoryEntry, error) {
	invalid := func() ([]DirectoryEntry, error) {
		return nil, fmt.Errorf("公开板块目录格式无效，继续使用本地目录")
	}
	if len(raw) == 0 || len(raw) > MaxDirectoryBytes {
		return invalid()
	}
	if !utf8.Valid(raw) {
		var err error
		raw, err = simplifiedchinese.GB18030.NewDecoder().Bytes(raw)
		if err != nil {
			return invalid()
		}
	}
	raw = bytes.TrimSpace(bytes.TrimPrefix(raw, []byte{0xef, 0xbb, 0xbf}))
	const prefix = "window.script_muti_get_var_store="
	if bytes.HasPrefix(raw, []byte(prefix)) {
		raw = bytes.TrimSpace(raw[len(prefix):])
		raw = bytes.TrimSpace(bytes.TrimSuffix(raw, []byte(";")))
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	var root map[string]any
	if decoder.Decode(&root) != nil {
		return invalid()
	}
	var trailing any
	if decoder.Decode(&trailing) != io.EOF {
		return invalid()
	}
	object := func(v any) map[string]any { m, _ := v.(map[string]any); return m }
	groups := object(object(object(root["data"])["0"])["all"])
	if groups == nil {
		return invalid()
	}
	entries := []DirectoryEntry{}
	seen := map[[2]int64]int{}
	nodes := 0
	valid := true
	var walk func(any, string, int)
	walk = func(value any, category string, depth int) {
		nodes++
		if depth > 12 || nodes > 8000 {
			valid = false
			return
		}
		switch node := value.(type) {
		case []any:
			for _, v := range node {
				if !valid {
					return
				}
				walk(v, category, depth+1)
			}
		case map[string]any:
			fid, cid := directoryNumber(node["fid"]), directoryNumber(node["stid"])
			name := directoryText(node["name"], 120)
			if name != "" && (fid != 0 || cid > 0) {
				entry := DirectoryEntry{ID: fid, CollectionID: cid, Name: name, Description: directoryText(node["info"], 240), Category: category}
				for _, k := range []string{"nameS", "infoS", "infoL"} {
					entry.Aliases = appendDirectoryAlias(entry.Aliases, directoryText(node[k], 240))
				}
				// Homepage collections share a parent fid. Their names and aliases
				// must not turn that parent into a different ordinary board.
				if cid == 0 {
					for _, alias := range directoryAliases[fid] {
						entry.Aliases = appendDirectoryAlias(entry.Aliases, alias)
					}
				}
				key := [2]int64{fid, cid}
				if at, ok := seen[key]; ok {
					existing := &entries[at]
					existing.Aliases = appendDirectoryAlias(existing.Aliases, entry.Name)
					for _, alias := range append(entry.Aliases, entry.Description, entry.Category) {
						existing.Aliases = appendDirectoryAlias(existing.Aliases, alias)
					}
				} else {
					seen[key] = len(entries)
					entries = append(entries, entry)
				}
				return
			}
			if child, ok := node["content"]; ok {
				if name != "" && name != category {
					category = category + " / " + name
				}
				walk(child, category, depth+1)
			} else {
				for _, k := range directoryKeys(node) {
					if !valid {
						return
					}
					walk(node[k], category, depth+1)
				}
			}
		}
	}
	for _, key := range directoryKeys(groups) {
		if key == "fast" || key == "follow" {
			continue
		}
		group := object(groups[key])
		if group == nil {
			continue
		}
		category := directoryText(group["name"], 100)
		walk(group["content"], category, 0)
	}
	if !valid || len(entries) == 0 || len(entries) > 2000 {
		return invalid()
	}
	return entries, nil
}

func directoryNumber(value any) int64 {
	var text string
	switch v := value.(type) {
	case json.Number:
		text = string(v)
	case string:
		text = v
	}
	n, _ := strconv.ParseInt(text, 10, 64)
	return n
}
func directoryText(value any, limit int) string {
	s, _ := value.(string)
	s = strings.Map(func(r rune) rune {
		if unicode.IsControl(r) {
			return ' '
		}
		return r
	}, html.UnescapeString(s))
	s = strings.Join(strings.Fields(s), " ")
	if utf8.RuneCountInString(s) > limit {
		s = string([]rune(s)[:limit])
	}
	return s
}
func appendDirectoryAlias(aliases []string, value string) []string {
	if value == "" || len(aliases) >= 24 {
		return aliases
	}
	for _, old := range aliases {
		if old == value {
			return aliases
		}
	}
	return append(aliases, value)
}
func directoryKeys(m map[string]any) []string {
	keys := make([]string, 0, len(m))
	for key := range m {
		keys = append(keys, key)
	}
	sort.Slice(keys, func(i, j int) bool {
		a, e1 := strconv.Atoi(keys[i])
		b, e2 := strconv.Atoi(keys[j])
		if e1 == nil && e2 == nil {
			return a < b
		}
		return keys[i] < keys[j]
	})
	return keys
}

var directoryAliases = map[int64][]string{
	-7: {"大漩涡", "水区"}, -343809: {"汽车", "汽车俱乐部", "车迷俱乐部"}, 436: {"数码", "消费电子"}, 7: {"魔兽", "WOW"},
}

type directorySearchEntry struct {
	entry     DirectoryEntry
	name, all string
	aliases   []string
}
type DirectoryIndex struct{ items []directorySearchEntry }

func normalizeDirectoryQuery(s string) string {
	return strings.ToLower(strings.Join(strings.Fields(norm.NFKC.String(s)), " "))
}
func NewDirectoryIndex(entries []DirectoryEntry) *DirectoryIndex {
	index := &DirectoryIndex{}
	for _, entry := range entries {
		item := directorySearchEntry{entry: entry, name: normalizeDirectoryQuery(entry.Name)}
		for _, alias := range entry.Aliases {
			item.aliases = append(item.aliases, normalizeDirectoryQuery(alias))
		}
		item.all = normalizeDirectoryQuery(strings.Join([]string{entry.Name, entry.Description, entry.Category, strings.Join(entry.Aliases, " "), strconv.FormatInt(entry.ID, 10)}, " "))
		index.items = append(index.items, item)
	}
	return index
}
func SearchBoardDirectory(entries []DirectoryEntry, query string) []DirectoryEntry {
	return NewDirectoryIndex(entries).Search(query)
}
func (index *DirectoryIndex) Search(query string) []DirectoryEntry {
	if index == nil {
		return nil
	}
	query = normalizeDirectoryQuery(query)
	terms := strings.Fields(query)
	type match struct {
		entry DirectoryEntry
		score int
	}
	matches := []match{}
	for _, item := range index.items {
		ok := true
		for _, term := range terms {
			if !strings.Contains(item.all, term) {
				ok = false
				break
			}
		}
		if !ok {
			continue
		}
		score := 4
		switch {
		case item.name == query:
			score = 0
		case strings.HasPrefix(item.name, query):
			score = 2
		case strings.Contains(item.name, query):
			score = 3
		}
		for _, alias := range item.aliases {
			if alias == query {
				score = min(score, 1)
			}
		}
		if query == strconv.FormatInt(item.entry.ID, 10) {
			score = 0
		}
		if item.entry.CollectionID != 0 {
			score += 10
		}
		// Put the common homepage boards at the front of the initial view.
		if query == "" {
			score = 2
			if _, common := directoryAliases[item.entry.ID]; common && item.entry.CollectionID == 0 {
				score = 0
			}
		}
		matches = append(matches, match{item.entry, score})
	}
	sort.SliceStable(matches, func(i, j int) bool { return matches[i].score < matches[j].score })
	result := make([]DirectoryEntry, len(matches))
	for i, m := range matches {
		result[i] = m.entry
	}
	return result
}
