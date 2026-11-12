package main

import (
	"fmt"
	"github.com/egoist/mygo/ui"
	"math"
	"ngareader/internal/local"
	"ngareader/internal/nga"
	"time"
)

type readingRestore struct {
	TopicID, UID int64
	Page         int
	Position     local.ReadingPosition
	Quiet        bool
}
type readingRow struct {
	Post     nga.Post
	Bounds   ui.Rect
	Animated bool
	Page     int
}
type readingViewState struct {
	Pending                    *readingRestore
	TopicID, UID               int64
	Page, Font, Warm, Settling int
	Y, Width                   float32
	Dirty                      bool
	SaveAt                     time.Time
	VisiblePage                int
	Anchor                     *local.ReadingPosition
}

func (a *reader) readTask(fn func()) {
	if a.runReadTask != nil {
		a.runReadTask(fn)
	} else {
		go fn()
	}
}
func (a *reader) resetReadingSession() {
	a.navigation = readingNavigation{}
	a.flushReadingProgress()
	a.topicGate.Begin(a.ctx)
	a.threadGate.Begin(a.ctx)
	a.selected = nga.Topic{}
	a.thread = nga.ThreadPage{}
	a.topics = nga.TopicPage{}
	a.reading = readingViewState{}
	a.linkLastTopic = 0
	a.topicSourceFID = 0
	a.loadingTopics = false
	a.loadingThread = false
	a.continuation = continuationState{}
	a.refreshingThread = false
	a.refreshError = nil
	a.threadErr = nil
	a.topicErr = nil
	a.jump.Open = false
	a.resetMarkdown()
}

func (a *reader) readingUID() int64 { return a.client.SessionUID() }
func (a *reader) beginReadingTopic(id int64) int {
	a.flushReadingProgress()
	a.reading = readingViewState{}
	r, ok := local.FindReading(a.state.History, id, a.readingUID())
	if !ok {
		return 1
	}
	if r.Position != nil {
		a.reading.Pending = &readingRestore{TopicID: id, UID: a.readingUID(), Page: r.Page, Position: *r.Position}
	}
	return max(1, r.Page)
}
func (a *reader) prepareReadingLoad(page int) {
	a.flushReadingProgress()
	pending := a.reading.Pending
	if pending != nil && (pending.TopicID != a.selected.ID || pending.Page != page || pending.UID != a.readingUID()) {
		pending = nil
	}
	if pending == nil && !a.onlyAuthor && a.thread.Topic.ID == a.selected.ID && a.thread.Page == page {
		if r, ok := local.FindReading(a.state.History, a.selected.ID, a.readingUID()); ok && r.Page == page && r.Position != nil {
			pending = &readingRestore{TopicID: a.selected.ID, UID: a.readingUID(), Page: page, Position: *r.Position}
		}
	}
	a.reading = readingViewState{Pending: pending}
}
func (a *reader) rememberReading() {
	if a.selected.ID <= 0 || a.loadingThread || a.threadErr != nil || a.thread.Topic.ID != a.selected.ID {
		return
	}
	uid := a.readingUID()
	r, _ := local.FindReading(a.state.History, a.selected.ID, uid)
	r.Topic = a.selected
	r.At = time.Now()
	r.UID = uid
	r.Scoped = true
	r.RepliesKnown = a.thread.TotalRows > 0 || a.selected.Replies > 0
	if !a.onlyAuthor {
		page := max(1, a.thread.Page)
		if r.Page != page {
			r.Position = nil
		}
		r.Page = page
	}
	a.state.History = local.PutReading(a.state.History, r)
	a.state.LastTopic = a.selected.ID
	a.save()
}
func (a *reader) flushReadingProgress() bool {
	if !a.reading.Dirty {
		return true
	}
	if a.demo {
		a.reading.Dirty = false
		return true
	}
	if err := local.Save(a.dir, a.state); err != nil {
		a.notify("阅读位置未能保存：" + err.Error())
		return false
	}
	a.reading.Dirty = false
	return true
}
func sameReadingPosition(a, b *local.ReadingPosition) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}
	return a.PID == b.PID && a.Floor == b.Floor && math.Abs(float64(a.Fraction-b.Fraction)) < .0001
}
func (a *reader) observeReadingView(c *ui.Context, viewport ui.Rect, rows []readingRow) {
	s := &a.reading
	if a.loadingThread && !a.refreshingThread || a.threadErr != nil || a.thread.Topic.ID != a.selected.ID || len(rows) == 0 {
		return
	}
	uid := a.readingUID()
	if viewport.H <= 0 {
		if s.Warm < 2 {
			s.Warm++
			c.Invalidate()
		}
		return
	}
	samePage := s.TopicID == a.selected.ID && s.UID == uid && s.Page == a.thread.Page
	geometryChanged := samePage && s.Font > 0 && s.Width > 0 && (s.Font != a.state.FontSize || s.Width != viewport.W)
	if geometryChanged && s.Pending == nil {
		if a.onlyAuthor && s.Anchor != nil {
			s.Pending = &readingRestore{TopicID: a.selected.ID, UID: uid, Page: max(1, s.VisiblePage), Position: *s.Anchor, Quiet: true}
			s.Warm = 0
		}
		// Native TrackScroll retains pixels through reflow. Preserve the last
		// visible post instead, and restore it once after committed layout.
		if r, ok := local.FindReading(a.state.History, a.selected.ID, uid); !a.onlyAuthor && ok && a.hasLoadedPage(r.Page) && r.Position != nil {
			s.Pending = &readingRestore{TopicID: a.selected.ID, UID: uid, Page: r.Page, Position: *r.Position, Quiet: true}
			s.Warm = 0
		}
	}
	stable := samePage && !geometryChanged && math.Abs(float64(s.Y-a.scroll.Y)) < .25
	s.TopicID, s.UID, s.Page, s.Font, s.Width, s.Y = a.selected.ID, uid, a.thread.Page, a.state.FontSize, viewport.W, a.scroll.Y
	if !stable {
		c.Invalidate()
		return
	}
	if s.Settling > 0 {
		s.Settling--
		c.Invalidate()
		return
	}
	if p := s.Pending; p != nil {
		if p.TopicID != a.selected.ID || !a.hasLoadedPage(p.Page) || p.UID != uid {
			s.Pending = nil
			return
		}
		target := -1
		for i, r := range rows {
			if p.Position.PID > 0 && r.Post.ID == p.Position.PID {
				target = i
				break
			}
		}
		if target < 0 {
			for i, r := range rows {
				if r.Post.Floor == p.Position.Floor {
					target = i
					break
				}
			}
		}
		if target >= 0 && rows[target].Bounds.H <= 0 {
			if s.Warm < 4 {
				s.Warm++
				c.Invalidate()
			}
			return
		}
		s.Pending = nil
		s.Settling = 2
		if target >= 0 {
			r := rows[target]
			a.scroll.Y = max(0, a.scroll.Y+r.Bounds.Y-viewport.Y+p.Position.Fraction*r.Bounds.H)
			if !p.Quiet {
				a.notify(fmt.Sprintf("已继续阅读到第%d楼，可点击“从头阅读”重新开始。", r.Post.Floor))
			}
		} else {
			a.scroll.Y = 0
			a.notify("上次楼层已变化，已回到本页开头。")
		}
		c.Invalidate()
		return
	}
	var position *local.ReadingPosition
	page := max(1, a.thread.Page)
	if a.scroll.Y > 1 {
		probe := viewport.Y + 2
		for _, row := range rows {
			b := row.Bounds
			if b.H > 0 && b.Y+b.H > probe && b.Y < viewport.Y+viewport.H {
				page = max(1, row.Page, a.thread.Page)
				position = &local.ReadingPosition{PID: row.Post.ID, Floor: row.Post.Floor, Fraction: min(1, max(0, (probe-b.Y)/b.H))}
				break
			}
		}
		if position == nil {
			return
		}
	}
	s.VisiblePage = page
	s.Anchor = clonePosition(position)
	if a.onlyAuthor {
		return
	}
	r, ok := local.FindReading(a.state.History, a.selected.ID, uid)
	if !ok || !r.Scoped || r.UID != uid {
		return
	}
	if r.Page != page || !sameReadingPosition(r.Position, position) {
		r.Page = page
		r.Position = position
		r.At = c.Now()
		a.state.History = local.PutReading(a.state.History, r)
		s.Dirty = true
		s.SaveAt = c.Now().Add(time.Second)
	}
	if s.Dirty {
		if !c.Now().Before(s.SaveAt) {
			if !a.flushReadingProgress() {
				s.SaveAt = c.Now().Add(30 * time.Second)
			}
		}
		if s.Dirty {
			c.After(max(time.Millisecond, s.SaveAt.Sub(c.Now())))
		}
	}
}
