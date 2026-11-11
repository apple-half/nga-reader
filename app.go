package main

import (
	"context"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/netproxy"
	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/markdown"

	"ngareader/internal/local"
	"ngareader/internal/nga"
)

type renderedPost struct {
	post   nga.Post
	source string
}
type reader struct {
	dir                                           string
	state                                         local.State
	liveState                                     *local.State
	client                                        *nga.Client
	keys                                          local.KeyStore
	ctx                                           context.Context
	cancel                                        context.CancelFunc
	win                                           *mygo.Window
	tray                                          *mygo.Tray
	authWindow                                    *mygo.Window
	authView                                      *mygo.WebView
	authSession                                   *mygo.Session
	authCancel                                    context.CancelFunc
	authBusy                                      bool
	authLoading                                   bool
	authError                                     string
	pictureWindow                                 *mygo.Window
	pictureWeb                                    *mygo.WebView
	pictureAddress, pictureError                  string
	demo, closing, focused, onlyAuthor            bool
	mode, filter                                  string
	hot                                           bool
	board                                         local.Board
	topics                                        nga.TopicPage
	thread                                        nga.ThreadPage
	selected                                      nga.Topic
	loadingTopics, loadingThread                  bool
	topicErr, threadErr                           error
	topicGate, threadGate                         local.RequestGate
	composeGate                                   local.RequestGate
	compose                                       composerState
	posting                                       postingAPI
	runPosting                                    func(func())
	postAfterLogin                                *postIntent
	drafts                                        []local.Draft
	draftsError                                   error
	favoriteGate                                  local.RequestGate
	loadingFavorites, importAfterLogin            bool
	listState                                     ui.ListState
	scroll                                        ui.ScrollState
	md                                            *markdown.Options
	rendered                                      []renderedPost
	settingsOpen, addOpen, linkOpen, cookieOpen   bool
	helpOpen                                      bool
	boardInput, boardName, linkInput, cookieInput string
	formError, message                            string
	messageUntil                                  time.Time
}

func newReader(dir string) *reader {
	state, err := local.Load(dir)
	ctx, cancel := context.WithCancel(context.Background())
	a := &reader{dir: dir, state: state, ctx: ctx, cancel: cancel, mode: "board", client: nga.NewClient(&http.Client{Transport: netproxy.Transport(), Timeout: 25 * time.Second})}
	a.drafts, a.draftsError = local.LoadDrafts(dir)
	a.board = state.Boards[0]
	for _, b := range state.Boards {
		if b.ID == state.LastBoard {
			a.board = b
		}
	}
	if err != nil {
		a.notify(err.Error())
	}
	a.resetMarkdown()
	return a
}

func (a *reader) update(fn func()) {
	if a.ctx.Err() != nil {
		return
	}
	if a.win != nil {
		if !a.win.IsDestroyed() {
			a.win.Update(func() {
				if !a.closing && a.ctx.Err() == nil {
					fn()
				}
			})
		}
	} else {
		fn()
	}
}
func (a *reader) notify(message string) {
	a.message = message
	a.messageUntil = time.Now().Add(8 * time.Second)
}
func (a *reader) save() {
	if a.demo {
		return
	}
	if err := local.Save(a.dir, a.state); err != nil {
		a.notify(err.Error())
	}
}
func (a *reader) isFavorite(id int64) bool {
	for _, t := range a.state.Favorites {
		if t.ID == id {
			return true
		}
	}
	return false
}
func (a *reader) toggleFavorite() {
	if a.selected.ID <= 0 {
		return
	}
	if a.isFavorite(a.selected.ID) {
		for i, t := range a.state.Favorites {
			if t.ID == a.selected.ID {
				a.state.Favorites = append(a.state.Favorites[:i], a.state.Favorites[i+1:]...)
				break
			}
		}
		a.notify("已取消本地收藏")
	} else {
		a.state.Favorites = append([]nga.Topic{a.selected}, a.state.Favorites...)
		a.notify("已加入本地收藏")
	}
	a.save()
}
func (a *reader) rememberReading() {
	if a.selected.ID <= 0 {
		return
	}
	history := []local.Reading{{Topic: a.selected, Page: max(1, a.thread.Page), At: time.Now()}}
	for _, h := range a.state.History {
		if h.Topic.ID != a.selected.ID && len(history) < 100 {
			history = append(history, h)
		}
	}
	a.state.History = history
	a.state.LastTopic = a.selected.ID
	a.save()
}
func (a *reader) displayTopics() []nga.Topic {
	items := a.topics.Topics
	if a.mode == "favorites" {
		items = a.state.Favorites
	}
	if a.mode == "history" {
		items = nil
		for _, h := range a.state.History {
			items = append(items, h.Topic)
		}
	}
	out := make([]nga.Topic, 0, len(items))
	query := strings.ToLower(strings.TrimSpace(a.filter))
	for _, t := range items {
		if query == "" || strings.Contains(strings.ToLower(t.Title+" "+t.Author), query) {
			out = append(out, t)
		}
	}
	if a.hot {
		sort.SliceStable(out, func(i, j int) bool { return out[i].Replies > out[j].Replies })
	}
	return out
}
func (a *reader) resetMarkdown() {
	// No LoadImage callback: the reader never requests unopened images,
	// including raw Markdown images that appear in user-provided post text.
	a.md = &markdown.Options{OnLink: a.openLink, WrapCode: true, Block: a.imageLinkBlock}
	a.rendered = nil
	for _, p := range a.thread.Posts {
		a.rendered = append(a.rendered, renderedPost{post: p, source: nga.MarkdownOnDemand(p.Content)})
	}
}
func (a *reader) shutdown() {
	if a.closing {
		return
	}
	a.closing = true
	a.saveComposer()
	a.cancel()
	a.composeGate.Close()
	a.topicGate.Close()
	a.threadGate.Close()
	a.favoriteGate.Close()
	a.save()
	if !a.demo && a.state.RememberLogin && a.keys != nil && a.client.HasSession() {
		_ = local.SaveSession(a.dir, a.client.Cookies(), a.keys)
	}
	if a.authWindow != nil && !a.authWindow.IsDestroyed() {
		a.authWindow.Destroy()
	}
	if a.pictureWindow != nil && !a.pictureWindow.IsDestroyed() {
		a.pictureWindow.Destroy()
	}
	if a.tray != nil {
		a.tray.Destroy()
	}
	mygo.GlobalShortcut.Unregister("Ctrl+Shift+Space")
}
