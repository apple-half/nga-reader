package main

import (
	"fmt"
	"ngareader/internal/local"
	"ngareader/internal/nga"
	"strconv"
	"strings"
)

type pageJumpState struct {
	Open, Thread, OnlyAuthor bool
	Input, Error             string
	TopicID, FID             int64
}

func parsePageInput(value string, total int) (int, error) {
	value = strings.TrimSpace(value)
	value = strings.Map(func(r rune) rune {
		if r >= '０' && r <= '９' {
			return r - '０' + '0'
		}
		return r
	}, value)
	if value == "" || len(value) > 7 {
		return 0, fmt.Errorf("请输入有效的正整数页码")
	}
	for _, r := range value {
		if r < '0' || r > '9' {
			return 0, fmt.Errorf("请输入有效的正整数页码")
		}
	}
	page, err := strconv.Atoi(value)
	if err != nil || page < 1 || page > 1000000 {
		return 0, fmt.Errorf("页码应在1至1000000之间")
	}
	if total > 0 && page > total {
		return 0, fmt.Errorf("页码应在1至%d之间", total)
	}
	return page, nil
}
func (a *reader) threadPageLimit() int {
	if a.thread.TotalRows > 0 || a.thread.PagesKnown {
		return max(0, a.thread.TotalPages)
	}
	return 0
}
func (a *reader) navigationBusy() bool {
	return a.closing || a.updates.Installing || a.updates.Restarting
}
func (a *reader) openPageJump(thread bool) {
	if a.navigationBusy() || a.demo {
		return
	}
	if thread && (a.selected.ID <= 0 || a.loadingThread) || !thread && (a.mode != "board" || a.loadingTopics) {
		return
	}
	a.jump = pageJumpState{Open: true, Thread: thread, TopicID: a.selected.ID, FID: a.board.ID, OnlyAuthor: a.onlyAuthor}
}
func (a *reader) applyPageJump() {
	s := &a.jump
	if !s.Open || a.navigationBusy() {
		return
	}
	if s.Thread && (s.TopicID != a.selected.ID || s.OnlyAuthor != a.onlyAuthor) || !s.Thread && (s.FID != a.board.ID || a.mode != "board") {
		s.Error = "当前阅读目标已变化，请关闭后重新打开页码输入。"
		return
	}
	if s.Thread && a.loadingThread || !s.Thread && a.loadingTopics {
		s.Error = "正在加载，请稍候。"
		return
	}
	total := a.topics.TotalPages
	if s.Thread {
		total = a.threadPageLimit()
	}
	page, err := parsePageInput(s.Input, total)
	if err != nil {
		s.Error = err.Error()
		return
	}
	s.Open = false
	if s.Thread {
		a.navigateThreadPage(page)
	} else if page != a.topics.Page {
		a.loadTopics(page)
	}
}
func (a *reader) navigateThreadPage(page int) {
	if a.navigationBusy() || a.demo || a.loadingThread || a.selected.ID <= 0 || page == a.thread.Page && a.threadErr == nil {
		return
	}
	if _, err := parsePageInput(strconv.Itoa(page), a.threadPageLimit()); err != nil {
		a.notify(err.Error())
		return
	}
	a.loadThread(page)
}
func (a *reader) startReadingFromBeginning() {
	if a.navigationBusy() || a.loadingThread || a.selected.ID <= 0 {
		return
	}
	a.flushReadingProgress()
	a.linkLastTopic = 0
	if r, ok := local.FindReading(a.state.History, a.selected.ID, a.readingUID()); ok {
		r.Page = 1
		r.Position = nil
		r.Scoped = true
		r.UID = a.readingUID()
		a.state.History = local.PutReading(a.state.History, r)
		a.save()
	}
	a.reading = readingViewState{Settling: 2}
	a.scroll.Y = 0
	if a.thread.Page != 1 || a.threadErr != nil {
		a.loadThread(1)
	}
}
func (a *reader) readingToTop() { a.reading.Pending = nil; a.reading.Settling = 2; a.scroll.Y = 0 }
func (a *reader) modalOpen() bool {
	return a.settingsOpen || a.addOpen || a.linkOpen || a.cookieOpen || a.helpOpen || a.compose.Open || a.updates.Open || a.jump.Open
}
func (a *reader) openLinkDialog() {
	if !a.navigationBusy() {
		a.linkOpen = true
		a.formError = ""
	}
}
func (a *reader) submitLink() {
	if _, _, err := nga.ParseLink(a.linkInput); err != nil {
		a.formError = err.Error()
		return
	}
	a.linkOpen = false
	a.formError = ""
	a.openLink(a.linkInput)
}
