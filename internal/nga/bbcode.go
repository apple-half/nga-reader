package nga

import (
	"html"
	"net/url"
	"regexp"
	"strings"
	"unicode/utf8"
)

var bbTag = regexp.MustCompile(`(?i)^(/?)([a-z*]+)(?:[= :](.*))?$`)

// Markdown parses BBCode as data. It never evaluates forum scripts or HTML.
func Markdown(source string, textOnly bool) string {
	return renderMarkdown(source, textOnly, false)
}

// MarkdownOnDemand exposes attachments as explicit native links. It never
// emits a Markdown image that a renderer could fetch before the user clicks.
func MarkdownOnDemand(source string) string {
	return renderMarkdown(source, false, true)
}

func renderMarkdown(source string, textOnly, onDemand bool) string {
	const limit = 1 << 20
	if len(source) > limit {
		source = clipUTF8(source, limit) + "\n[正文过长，可打开原帖查看完整内容]"
	}
	source = lineBreak.ReplaceAllString(source, "\n")
	source = html.UnescapeString(source)
	index := 0
	depth := 0
	var parse func(string) string
	parse = func(end string) string {
		depth++
		defer func() { depth-- }()
		if depth > 32 {
			tail := source[index:]
			index = len(source)
			return tail
		}
		var b strings.Builder
		for index < len(source) {
			if b.Len() > limit {
				index = len(source)
				return clipUTF8(b.String(), limit) + "\n[内容过长，可打开原帖查看完整内容]"
			}
			if source[index] != '[' {
				b.WriteByte(source[index])
				index++
				continue
			}
			close := strings.IndexByte(source[index:], ']')
			if close < 0 {
				b.WriteString(source[index:])
				index = len(source)
				break
			}
			literal := source[index : index+close+1]
			inner := source[index+1 : index+close]
			match := bbTag.FindStringSubmatch(inner)
			if match == nil {
				b.WriteString(literal)
				index += close + 1
				continue
			}
			name := strings.ToLower(match[2])
			closing := match[1] == "/"
			arg := match[3]
			if closing {
				index += close + 1
				if name == end {
					return b.String()
				}
				continue
			}
			index += close + 1
			if name == "s" && strings.Contains(arg, ":") {
				b.WriteString("[表情]")
				continue
			}
			if name == "*" {
				b.WriteString("\n- ")
				continue
			}
			if name == "br" {
				b.WriteByte('\n')
				continue
			}
			if name == "code" {
				tail := strings.ToLower(source[index:])
				n := strings.Index(tail, "[/code]")
				content := source[index:]
				if n >= 0 {
					content = source[index : index+n]
					index += n + len("[/code]")
				} else {
					index = len(source)
				}
				backticks, tildes := longestRun(content, '`'), longestRun(content, '~')
				marker := "`"
				run := backticks
				if tildes < backticks {
					marker = "~"
					run = tildes
				}
				fence := strings.Repeat(marker, max(3, run+1))
				b.WriteString("\n\n" + fence + "\n" + content + "\n" + fence + "\n\n")
				continue
			}
			// Self-contained NGA emoticons and unsupported standalone tags stay readable.
			known := map[string]bool{"b": true, "i": true, "u": true, "s": true, "del": true, "quote": true, "url": true, "img": true, "color": true, "size": true, "font": true, "collapse": true, "h": true, "list": true, "pid": true, "uid": true, "tid": true, "align": true, "center": true, "album": true, "flash": true}
			if !known[name] {
				b.WriteString(literal)
				continue
			}
			content := parse(name)
			switch name {
			case "b":
				b.WriteString("**" + content + "**")
			case "i":
				b.WriteString("*" + content + "*")
			case "u":
				b.WriteString("<u>" + content + "</u>")
			case "s", "del":
				b.WriteString("~~" + content + "~~")
			case "quote":
				b.WriteString("\n\n")
				for line := range strings.SplitSeq(strings.TrimSpace(content), "\n") {
					if b.Len()+len(line)+3 > limit {
						b.WriteString("\n[引用过长，可打开原帖查看完整内容]")
						break
					}
					b.WriteString("> " + line + "\n")
				}
				b.WriteByte('\n')
			case "url":
				dest := arg
				if dest == "" {
					dest = strings.TrimSpace(content)
				}
				if safeLink(dest) {
					b.WriteString("[" + strings.ReplaceAll(content, "]", "\\]") + "](<" + strings.ReplaceAll(dest, ">", "%3E") + ">)")
				} else {
					b.WriteString(content)
				}
			case "img":
				if address, err := ImageURL(content); err == nil && !textOnly {
					if onDemand {
						b.WriteString("\n\n[展示图片](<" + address + ">)\n\n")
					} else {
						b.WriteString("\n\n![图片](<" + address + ">)\n\n")
					}
				} else {
					b.WriteString("[图片已折叠]")
				}
			case "h":
				b.WriteString("\n\n### " + content + "\n\n")
			case "collapse":
				b.WriteString("\n\n**" + arg + "**\n\n" + content + "\n\n")
			case "list":
				b.WriteString("\n" + content + "\n")
			default:
				b.WriteString(content)
			}
		}
		if b.Len() > limit {
			return clipUTF8(b.String(), limit) + "\n[内容过长，可打开原帖查看完整内容]"
		}
		return b.String()
	}
	return strings.TrimSpace(parse(""))
}

func longestRun(s string, character byte) int {
	longest, current := 0, 0
	for i := 0; i < len(s); i++ {
		if s[i] == character {
			current++
			longest = max(longest, current)
		} else {
			current = 0
		}
	}
	return longest
}

func clipUTF8(s string, n int) string {
	if len(s) <= n {
		return s
	}
	for n > 0 && !utf8.RuneStart(s[n]) {
		n--
	}
	return s[:n]
}

func safeLink(s string) bool {
	u, err := url.Parse(strings.TrimSpace(s))
	return err == nil && u.Scheme == "https" && u.Host != "" && u.User == nil && !strings.ContainsAny(s, "\r\n")
}
