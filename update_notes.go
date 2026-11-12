package main

import "strings"

// compactUpdateNotes keeps legacy web release notes focused on the changes.
// New signed manifests carry a short app-specific changelog already.
func compactUpdateNotes(raw string) string {
	lines := strings.Split(strings.ReplaceAll(raw, "\r\n", "\n"), "\n")
findChanges:
	for i, line := range lines {
		level, title := updateNotesHeading(line)
		switch title {
		case "本版改进", "本次更新", "更新内容":
			end := len(lines)
			for j := i + 1; j < len(lines); j++ {
				next, _ := updateNotesHeading(lines[j])
				if next > 0 && (level == 0 || next <= level) || strings.HasPrefix(strings.TrimSpace(lines[j]), "</details") {
					end = j
					break
				}
			}
			lines = lines[i+1 : end]
			break findChanges
		}
	}
	body := strings.TrimSpace(strings.Join(lines, "\n"))
	if body == "" {
		return "此版本未提供更新摘要，请查看完整更新说明。"
	}
	runes := []rune(body)
	if len(runes) > 2400 {
		body = string(runes[:2400]) + "…\n\n更多内容请查看完整更新说明。"
	}
	return body
}

func updateNotesHeading(line string) (int, string) {
	line = strings.TrimSpace(line)
	level := len(line) - len(strings.TrimLeft(line, "#"))
	if level > 0 {
		if level > 6 || len(line) <= level || line[level] != ' ' {
			return 0, ""
		}
		line = strings.TrimSpace(line[level:])
	}
	return level, strings.TrimSpace(strings.TrimRight(line, "#"))
}
