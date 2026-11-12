package main

import (
	"context"
	"errors"
	"github.com/egoist/mygo/ui"
	"ngareader/internal/nga"
	"sort"
)

type continuationState struct {
	First, Last, Count int
	Loading, Ended     bool
	Error              error
	Capped, AutoReady  bool
	Observed           bool
	LastY              float32
}

func (a *reader) resetContinuousPage() {
	page := max(1, a.thread.Page)
	a.continuation = continuationState{First: page, Last: page, Count: 1}
	a.loadedOnlyAuthor = a.onlyAuthor
	a.refreshError = nil
}
func (a *reader) continuationLast() int {
	if a.continuation.First == max(1, a.thread.Page) && a.continuation.Last >= a.thread.Page {
		return a.continuation.Last
	}
	return max(1, a.thread.Page)
}
func (a *reader) hasLoadedPage(page int) bool {
	return page >= max(1, a.thread.Page) && page <= a.continuationLast()
}
func (a *reader) visibleThreadPage() int {
	if a.onlyAuthor {
		return max(1, a.reading.VisiblePage, a.thread.Page)
	}
	if a.reading.VisiblePage > 0 && a.hasLoadedPage(a.reading.VisiblePage) {
		return a.reading.VisiblePage
	}
	return max(1, a.thread.Page)
}
func (a *reader) continuationLimited() bool {
	return a.continuation.Capped || a.continuation.Count >= 10 || len(a.rendered) >= 200
}
func (a *reader) continuationHasNext() bool {
	if a.continuation.Ended || len(a.rendered) == 0 {
		return false
	}
	if total := a.threadPageLimit(); total > 0 {
		return a.continuationLast() < total
	}
	n := 0
	last := a.continuationLast()
	for _, r := range a.rendered {
		if max(1, r.page) == last {
			n++
		}
	}
	return n >= 20
}
func (a *reader) observeContinuation(viewport, last ui.Rect) {
	if !a.continuation.Observed {
		a.continuation.Observed = true
		a.continuation.LastY = a.scroll.Y
		return
	}
	if a.scroll.Y > a.continuation.LastY+.5 {
		a.continuation.AutoReady = true
	}
	a.continuation.LastY = a.scroll.Y
	if a.scroll.Y <= 1 {
		a.continuation.AutoReady = false
	}
	if a.demo || !a.state.AutoNextPage || a.search.Open || a.modalOpen() || a.navigationBusy() || a.loadingThread || a.continuation.Loading || a.continuation.Error != nil || a.continuationLimited() || a.reading.Pending != nil || a.reading.Settling > 0 || !a.continuationHasNext() {
		return
	}
	if a.continuation.AutoReady && viewport.H > 0 && last.H > 0 && last.Y+last.H >= viewport.Y && last.Y+last.H <= viewport.Y+viewport.H+180 {
		a.appendNextPage()
	}
}
func (a *reader) appendNextPage() {
	if a.demo || a.navigationBusy() || a.loadingThread || a.continuation.Loading || a.continuationLimited() || !a.continuationHasNext() {
		return
	}
	if a.continuation.Count == 0 {
		a.resetContinuousPage()
	}
	ctx, token := a.threadGate.Begin(a.ctx)
	topic, uid, author := a.selected, a.readingUID(), a.onlyAuthor
	authorID := int64(0)
	if author {
		authorID = a.thread.MainAuthorID
	}
	page := a.continuationLast() + 1
	a.continuation.Loading = true
	a.continuation.AutoReady = false
	a.continuation.Error = nil
	a.readTask(func() {
		result, err := a.client.Thread(ctx, topic.ID, page, authorID)
		a.update(func() {
			if !a.threadGate.Accept(token) || a.closing || topic.ID != a.selected.ID || uid != a.readingUID() || author != a.onlyAuthor {
				return
			}
			a.continuation.Loading = false
			if err != nil {
				if !errors.Is(err, context.Canceled) {
					a.continuation.Error = err
				}
				return
			}
			if result.Topic.ID != 0 && result.Topic.ID != topic.ID || result.Page != page {
				a.continuation.Error = errors.New("论坛返回的页码不一致，请重试")
				return
			}
			seenID := map[int64]bool{}
			seenFloor := map[int]bool{}
			for _, r := range a.rendered {
				if r.post.ID > 0 {
					seenID[r.post.ID] = true
				}
				seenFloor[r.post.Floor] = true
			}
			added := 0
			for _, p := range result.Posts {
				if p.ID > 0 && seenID[p.ID] || p.ID <= 0 && seenFloor[p.Floor] {
					continue
				}
				added++
			}
			if len(a.rendered)+added > 200 {
				a.continuation.Capped = true
				return
			}
			added = 0
			for _, p := range result.Posts {
				if p.ID > 0 && seenID[p.ID] || p.ID <= 0 && seenFloor[p.Floor] {
					continue
				}
				if len(a.rendered) >= 200 {
					break
				}
				source := nga.MarkdownOnDemand(p.Content)
				a.rendered = append(a.rendered, renderedPost{post: p, source: source, animated: containsAnimatedEmote(source), page: page})
				a.thread.Posts = append(a.thread.Posts, p)
				seenID[p.ID] = true
				seenFloor[p.Floor] = true
				added++
			}
			if added == 0 {
				a.continuation.Ended = true
				return
			}
			a.continuation.Last = page
			a.continuation.Count++
			if result.PagesKnown || result.TotalRows > 0 {
				a.thread.PagesKnown = result.PagesKnown
				a.thread.TotalPages = result.TotalPages
				a.thread.TotalRows = result.TotalRows
				a.selected.Replies = result.Topic.Replies
				a.thread.Topic.Replies = result.Topic.Replies
			}
		})
	})
}

func (a *reader) replaceLoadedPage(result nga.ThreadPage) {
	var posts []renderedPost
	for _, r := range a.rendered {
		if max(1, r.page) != result.Page {
			posts = append(posts, r)
		}
	}
	for _, p := range result.Posts {
		source := nga.MarkdownOnDemand(p.Content)
		posts = append(posts, renderedPost{post: p, source: source, animated: containsAnimatedEmote(source), page: result.Page})
	}
	sort.SliceStable(posts, func(i, j int) bool {
		if posts[i].page != posts[j].page {
			return posts[i].page < posts[j].page
		}
		return posts[i].post.Floor < posts[j].post.Floor
	})
	a.rendered = posts
	a.thread.Posts = nil
	for _, r := range posts {
		a.thread.Posts = append(a.thread.Posts, r.post)
	}
	a.thread.Topic = result.Topic
	a.thread.TotalPages = result.TotalPages
	a.thread.TotalRows = result.TotalRows
	a.thread.PagesKnown = result.PagesKnown
	a.continuation.Ended = false
	a.continuation.Error = nil
}
