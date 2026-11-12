// Package emotes contains the fixed public NGA emoticon catalog and original
// image resources. It has no network, browser, account or filesystem loader.
package emotes

import (
	"embed"
	"encoding/json"
	"net/url"
	"sort"
	"strings"
)

//go:embed catalog.json assets.json assets/* NOTICE.txt
var files embed.FS

type Emote struct{ Group, Name, File string }
type AssetInfo struct {
	Width, Height, Frames, Bytes int
	SHA256                       string
}

var catalog, byFile, infos = load()

func load() (map[string]map[string]string, map[string]Emote, map[string]AssetInfo) {
	var groups map[string]map[string]string
	var metadata map[string]AssetInfo
	raw, _ := files.ReadFile("catalog.json")
	if err := json.Unmarshal(raw, &groups); err != nil {
		panic("invalid built-in emoticon catalog")
	}
	raw, _ = files.ReadFile("assets.json")
	if err := json.Unmarshal(raw, &metadata); err != nil {
		panic("invalid built-in emoticon metadata")
	}
	reverse := map[string]Emote{}
	for group, names := range groups {
		for name, file := range names {
			reverse[file] = Emote{group, name, file}
		}
	}
	return groups, reverse, metadata
}

func Entries() []Emote {
	entries := make([]Emote, 0, len(byFile))
	for _, entry := range byFile {
		entries = append(entries, entry)
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].File < entries[j].File })
	return entries
}

// Lookup accepts both [s:1] legacy numbers and [s:ac:哭笑] named groups.
func Lookup(argument string) (Emote, bool) {
	if len(argument) > 100 || strings.ContainsAny(argument, "\r\n\t") {
		return Emote{}, false
	}
	group, name, found := strings.Cut(argument, ":")
	if !found {
		name = group
		group = "0"
	}
	if group == "" {
		group = "0"
	}
	file, ok := catalog[group][name]
	if !ok {
		return Emote{}, false
	}
	return Emote{group, name, file}, true
}

const destinationPrefix = "ngareader-emote://asset/"

func Destination(e Emote) string {
	if _, ok := byFile[e.File]; !ok {
		return ""
	}
	return destinationPrefix + e.File
}
func FromDestination(address string) (Emote, bool) {
	if !strings.HasPrefix(address, destinationPrefix) {
		return Emote{}, false
	}
	e, ok := byFile[strings.TrimPrefix(address, destinationPrefix)]
	return e, ok
}

func FromOfficialURL(address string) (Emote, bool) {
	if strings.HasPrefix(address, "//") {
		address = "https:" + address
	}
	u, err := url.Parse(address)
	if err != nil || u.User != nil || u.Scheme != "https" && u.Scheme != "http" || u.Host != "img4.nga.cn" && u.Host != "img4.nga.178.com" {
		return Emote{}, false
	}
	const prefix = "/ngabbs/post/smile/"
	if !strings.HasPrefix(u.EscapedPath(), prefix) {
		return Emote{}, false
	}
	e, ok := byFile[strings.TrimPrefix(u.EscapedPath(), prefix)]
	return e, ok
}

func Info(file string) (AssetInfo, bool) { info, ok := infos[file]; return info, ok }
func Notice() string                     { raw, _ := files.ReadFile("NOTICE.txt"); return string(raw) }
func Asset(file string) (AssetInfo, []byte, bool) {
	info, ok := infos[file]
	if !ok {
		return AssetInfo{}, nil, false
	}
	if _, ok := byFile[file]; !ok {
		return AssetInfo{}, nil, false
	}
	raw, err := files.ReadFile("assets/" + file)
	return info, raw, err == nil
}

func (e Emote) Label() string {
	if e.Group != "0" {
		return e.Name
	}
	if label, ok := map[string]string{"1": "微笑", "2": "呲牙", "3": "疑问", "4": "眨眼", "5": "脸红", "6": "难过", "7": "酷", "8": "抓狂"}[e.Name]; ok {
		return label
	}
	return "默认表情 " + e.Name
}
