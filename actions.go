package main

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"strings"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
	"ngareader/internal/emotes"
	"ngareader/internal/local"
	"ngareader/internal/nga"
)

func (a *reader) selectBoard(board local.Board) {
	a.search = readerSearchState{}
	a.recordNavigation()
	a.flushReadingProgress()
	a.reading = readingViewState{}
	a.linkLastTopic = 0
	a.mode = "board"
	a.board = board
	a.state.LastBoard = board.ID
	a.filter = ""
	a.selected = nga.Topic{}
	a.thread = nga.ThreadPage{}
	a.rendered = nil
	a.threadErr = nil
	a.loadingThread = false
	a.continuation = continuationState{}
	a.refreshError = nil
	a.refreshingThread = false
	a.resetMarkdown()
	a.onlyAuthor = false
	a.threadGate.Begin(a.ctx)
	a.listState = ui.ListState{}
	a.save()
	a.loadTopics(1)
}
func (a *reader) loadTopics(page int) {
	if a.demo {
		a.topics = demoTopics()
		return
	}
	ctx, token := a.topicGate.Begin(a.ctx)
	a.loadingTopics = true
	a.topicErr = nil
	if a.topicSourceFID != a.board.ID || a.topics.Page != page {
		a.topics = nga.TopicPage{Page: page}
	}
	fid := a.board.ID
	a.readTask(func() {
		result, err := a.client.Topics(ctx, fid, page)
		a.update(func() {
			if !a.topicGate.Accept(token) {
				return
			}
			a.loadingTopics = false
			if err != nil {
				if !errors.Is(err, context.Canceled) {
					a.topicErr = err
				}
				return
			}
			a.topics = result
			a.topicSourceFID = fid
			if result.BoardName != "" && !a.board.CustomName {
				a.board.Name = result.BoardName
				for i := range a.state.Boards {
					if a.state.Boards[i].ID == fid {
						a.state.Boards[i].Name = result.BoardName
					}
				}
				a.save()
			}
		})
	})
}
func (a *reader) selectTopic(t nga.Topic) {
	a.selectTopicPage(t, 0)
}
func (a *reader) selectTopicPage(t nga.Topic, page int) {
	a.search = readerSearchState{}
	if a.selected.ID != t.ID || page != 0 && page != a.visibleThreadPage() {
		a.recordNavigation()
	}
	a.linkLastTopic = 0
	if page == 0 {
		page = a.beginReadingTopic(t.ID)
	} else {
		a.flushReadingProgress()
		a.reading = readingViewState{}
		if page < 0 {
			page = 1
			a.linkLastTopic = t.ID
		}
	}
	a.selected = t
	a.onlyAuthor = false
	a.scroll = ui.ScrollState{}
	a.loadThread(page)
}
func (a *reader) loadThread(page int) {
	a.loadThreadAt(page, false)
}
func (a *reader) loadThreadAt(page int, fromStart bool) {
	if a.selected.ID <= 0 {
		a.notify("这个条目是板块或合集入口，请使用板块链接添加")
		return
	}
	keep := !fromStart && len(a.rendered) > 0 && a.thread.Topic.ID == a.selected.ID && a.hasLoadedPage(page) && page == a.visibleThreadPage() && a.loadedOnlyAuthor == a.onlyAuthor
	if !keep {
		a.prepareReadingLoad(page)
	} else {
		a.flushReadingProgress()
	}
	if fromStart {
		a.reading.Pending = nil
	}
	if !keep {
		a.scroll = ui.ScrollState{}
		a.continuation = continuationState{}
	}
	a.threadErr = nil
	a.refreshError = nil
	a.refreshingThread = keep
	if a.demo {
		a.thread = demoThread(a.selected.ID)
		a.loadingThread = false
		a.resetMarkdown()
		a.resetContinuousPage()
		if fromStart {
			a.clearReadingPosition()
		}
		a.rememberReading()
		return
	}
	ctx, token := a.threadGate.Begin(a.ctx)
	a.loadingThread = true
	selected := a.selected
	knownMainAuthorID := selected.AuthorID
	if a.thread.Topic.ID == selected.ID && a.thread.MainAuthorID != 0 {
		knownMainAuthorID = a.thread.MainAuthorID
	}
	authorID := int64(0)
	if a.onlyAuthor {
		authorID = knownMainAuthorID
	}
	if !keep {
		a.thread = nga.ThreadPage{Topic: selected, Page: page, MainAuthorID: authorID}
		a.resetMarkdown()
	}
	a.continuation.Loading = false
	a.readTask(func() {
		result, err := a.client.Thread(ctx, selected.ID, page, authorID)
		a.update(func() {
			if !a.threadGate.Accept(token) {
				return
			}
			a.loadingThread = false
			a.refreshingThread = false
			if err != nil {
				if !errors.Is(err, context.Canceled) {
					if keep {
						a.refreshError = err
					} else {
						a.threadErr = err
					}
				}
				return
			}
			if result.Topic.ID == 0 {
				result.Topic.ID = selected.ID
			}
			if result.Topic.Title == "" {
				result.Topic.Title = selected.Title
			}
			if result.Topic.FID == 0 {
				result.Topic.FID = a.board.ID
			}
			if result.MainAuthorID == 0 {
				result.MainAuthorID = knownMainAuthorID
			}
			if result.Topic.AuthorID == 0 {
				result.Topic.AuthorID = selected.AuthorID
				result.Topic.Author = selected.Author
			}
			if result.TotalRows == 0 {
				result.Topic.Replies = selected.Replies
			}
			if keep {
				if a.onlyAuthor {
					// Full-thread progress intentionally does not track filtered
					// scrolling. Take the latest live filtered anchor at completion,
					// including scrolling while the refresh was in flight.
					if a.reading.Anchor != nil {
						a.reading.Pending = &readingRestore{TopicID: a.selected.ID, UID: a.readingUID(), Page: a.visibleThreadPage(), Position: *a.reading.Anchor, Quiet: true}
						a.reading.Warm = 0
					}
				} else if r, ok := local.FindReading(a.state.History, a.selected.ID, a.readingUID()); ok && r.Position != nil {
					a.reading.Pending = &readingRestore{TopicID: a.selected.ID, UID: a.readingUID(), Page: r.Page, Position: *r.Position, Quiet: true}
					a.reading.Warm = 0
				}
				a.replaceLoadedPage(result)
				a.selected = result.Topic
				if a.reading.Pending != nil {
					a.reading.Settling = 1
				}
				return
			}
			a.thread = result
			a.selected = result.Topic
			if a.linkLastTopic == selected.ID {
				a.linkLastTopic = 0
				if (result.TotalRows > 0 || result.PagesKnown) && result.TotalPages > 1 {
					a.loadThread(result.TotalPages)
					return
				}
			}
			a.resetMarkdown()
			a.resetContinuousPage()
			if fromStart {
				a.clearReadingPosition()
			}
			a.rememberReading()
		})
	})
}
func (a *reader) openLink(address string) {
	address = strings.TrimSpace(address)
	if _, ok := emotes.FromDestination(address); ok {
		return
	}
	if safe, err := nga.ImageURL(address); err == nil {
		a.openPicture(safe)
		return
	}
	if strings.HasPrefix(address, "/") {
		address = nga.BaseURL + address
	}
	if tid, fid, err := nga.ParseLink(address); err == nil {
		if tid > 0 {
			page := 0
			if u, err := url.Parse(address); err == nil && u.Query().Get("page") != "" {
				value := u.Query().Get("page")
				if value == "e" {
					page = -1
				} else {
					parsed, err := parsePageInput(value, 0)
					if err != nil {
						a.notify(err.Error())
						return
					}
					page = parsed
				}
			}
			if fid == 0 {
				fid = a.board.ID
			}
			a.selectTopicPage(nga.Topic{ID: tid, FID: fid, Title: fmt.Sprintf("话题 #%d", tid)}, page)
			return
		}
		if fid != 0 {
			b, _ := local.ParseBoard(fmt.Sprint(fid))
			for _, existing := range a.state.Boards {
				if existing.ID == fid {
					b = existing
				}
			}
			a.selectBoard(b)
			return
		}
	}
	u, err := url.Parse(address)
	if err != nil || u.Scheme != "https" || u.Host == "" || u.User != nil {
		a.notify("仅支持打开 HTTPS 链接")
		return
	}
	if a.win != nil {
		if err := mygo.Shell.OpenExternal(address); err != nil {
			a.notify("无法打开链接：" + err.Error())
		}
	}
}
func (a *reader) addBoard() {
	if !a.addOpen || a.navigationBusy() {
		return
	}
	board, err := local.ParseBoard(a.boardInput)
	if err != nil {
		a.formError = err.Error()
		return
	}
	if name := strings.TrimSpace(a.boardName); name != "" {
		board.Name = name
		board.CustomName = true
	}
	a.persistAddedBoard(board, board.CustomName)
}
func (a *reader) returnToLive() {
	if !a.allowSessionChange() {
		return
	}
	a.resetBoardPicker()
	a.navigation = readingNavigation{}
	if a.liveState != nil {
		a.state = *a.liveState
		a.liveState = nil
	}
	a.board = a.state.Boards[0]
	for _, board := range a.state.Boards {
		if board.ID == a.state.LastBoard {
			a.board = board
			break
		}
	}
	a.applyAppearance()
	a.demo = false
	a.selected = nga.Topic{}
	a.thread = nga.ThreadPage{}
	a.resetMarkdown()
	a.mode = "board"
	a.filter = ""
	a.loadTopics(1)
}
func (a *reader) toggleHidden() {
	if a.win == nil {
		return
	}
	if a.win.IsVisible() {
		a.flushReadingProgress()
		a.win.Hide()
	} else {
		a.win.Show()
		a.win.Focus()
	}
}
