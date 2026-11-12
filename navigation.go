package main

import (
	"github.com/egoist/mygo/ui"
	"ngareader/internal/local"
	"ngareader/internal/nga"
)

type readingLocation struct {
	Topic      nga.Topic
	Page       int
	OnlyAuthor bool
	Position   *local.ReadingPosition
}
type readingNavigation struct {
	Back, Forward          []readingLocation
	UID                    int64
	Initialized, Restoring bool
}

func clonePosition(p *local.ReadingPosition) *local.ReadingPosition {
	if p == nil {
		return nil
	}
	v := *p
	return &v
}
func (a *reader) ensureNavigationAccount() {
	if !a.navigation.Initialized || a.navigation.UID != a.readingUID() {
		a.navigation = readingNavigation{UID: a.readingUID(), Initialized: true}
	}
}
func (a *reader) captureLocation() readingLocation {
	loc := readingLocation{Topic: a.selected, Page: a.visibleThreadPage(), OnlyAuthor: a.onlyAuthor, Position: clonePosition(a.reading.Anchor)}
	if loc.Position == nil && !loc.OnlyAuthor {
		if r, ok := local.FindReading(a.state.History, loc.Topic.ID, a.readingUID()); ok && r.Page == loc.Page {
			loc.Position = clonePosition(r.Position)
		}
	}
	return loc
}
func boundedLocations(items []readingLocation, loc readingLocation) []readingLocation {
	items = append(items, loc)
	if len(items) > 50 {
		items = append([]readingLocation(nil), items[len(items)-50:]...)
	}
	return items
}
func (a *reader) recordNavigation() {
	a.ensureNavigationAccount()
	if a.navigation.Restoring || a.selected.ID <= 0 || a.loadingThread || a.threadErr != nil || len(a.rendered) == 0 {
		return
	}
	a.flushReadingProgress()
	a.navigation.Back = boundedLocations(a.navigation.Back, a.captureLocation())
	a.navigation.Forward = nil
}
func (a *reader) navigateHistory(delta int) {
	a.ensureNavigationAccount()
	if a.navigationBusy() || a.compose.Sending || a.loadingThread {
		return
	}
	var loc readingLocation
	if delta < 0 {
		if len(a.navigation.Back) == 0 {
			return
		}
		i := len(a.navigation.Back) - 1
		loc = a.navigation.Back[i]
		a.navigation.Back = a.navigation.Back[:i]
		if a.selected.ID > 0 {
			a.navigation.Forward = boundedLocations(a.navigation.Forward, a.captureLocation())
		}
	} else {
		if len(a.navigation.Forward) == 0 {
			return
		}
		i := len(a.navigation.Forward) - 1
		loc = a.navigation.Forward[i]
		a.navigation.Forward = a.navigation.Forward[:i]
		if a.selected.ID > 0 {
			a.navigation.Back = boundedLocations(a.navigation.Back, a.captureLocation())
		}
	}
	a.flushReadingProgress()
	a.navigation.Restoring = true
	a.search = readerSearchState{}
	defer func() { a.navigation.Restoring = false }()
	if loc.Topic.FID != 0 && loc.Topic.FID != a.board.ID {
		for _, b := range a.state.Boards {
			if b.ID == loc.Topic.FID {
				a.selectBoard(b)
				break
			}
		}
	}
	a.selected = loc.Topic
	a.onlyAuthor = loc.OnlyAuthor
	a.linkLastTopic = 0
	a.continuation = continuationState{}
	a.reading = readingViewState{}
	if loc.Position != nil {
		a.reading.Pending = &readingRestore{TopicID: loc.Topic.ID, UID: a.readingUID(), Page: max(1, loc.Page), Position: *loc.Position, Quiet: true}
	}
	a.scroll = ui.ScrollState{}
	a.loadThread(max(1, loc.Page))
}
func (a *reader) lastReading() (local.Reading, bool) {
	var latest local.Reading
	found := false
	for _, r := range a.state.History {
		if r.Scoped && r.UID == a.readingUID() && r.Topic.ID > 0 {
			if r.Topic.ID == a.state.LastTopic {
				return r, true
			}
			if !found {
				latest = r
				found = true
			}
		}
	}
	return latest, found
}
func (a *reader) continueLastReading() {
	if r, ok := a.lastReading(); ok && !a.navigationBusy() {
		a.selectTopic(r.Topic)
	}
}
