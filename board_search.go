package main

import (
	"fmt"
	"strings"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
	"ngareader/internal/local"
	"ngareader/internal/nga"
)

type boardPickerState struct {
	Query, Source, Error string
	Manual, Loading      bool
	FetchedAt            time.Time
	index, resultsIndex  *nga.DirectoryIndex
	lastQuery            string
	results              []nga.DirectoryEntry
	list                 ui.ListState
}

func (a *reader) resetBoardPicker() {
	a.directoryGate.Begin(a.ctx)
	a.picker = boardPickerState{}
	a.addOpen = false
}

func (a *reader) openBoardPicker() {
	if a.navigationBusy() {
		return
	}
	// Repeated calls while open must neither steal focus nor start more work.
	if a.addOpen {
		return
	}
	a.formError = ""
	a.boardInput = ""
	a.boardName = ""
	p := &a.picker
	p.Manual = false
	p.Query = ""
	p.list = ui.ListState{}
	if p.index == nil {
		entries := nga.EmbeddedBoardDirectory()
		p.Source = "内置目录"
		if !a.demo {
			if cached, at, err := local.LoadBoardDirectoryCache(a.dir, time.Now()); err == nil {
				entries = cached
				p.FetchedAt = at
				p.Source = "本地目录"
			}
		}
		p.index = nga.NewDirectoryIndex(entries)
	}
	a.addOpen = true
	if !local.BoardDirectoryFresh(p.FetchedAt, time.Now()) {
		a.refreshBoardDirectory()
	}
}

func (a *reader) refreshBoardDirectory() {
	p := &a.picker
	if a.demo || a.navigationBusy() || p.Loading {
		return
	}
	p.Loading = true
	p.Error = ""
	ctx, token := a.directoryGate.Begin(a.ctx)
	client := a.client
	task := func() {
		raw, err := client.BoardDirectory(ctx)
		var entries []nga.DirectoryEntry
		if err == nil {
			entries, err = nga.ParseBoardDirectory(raw)
		}
		a.update(func() {
			if !a.directoryGate.Accept(token) || a.demo {
				return
			}
			p := &a.picker
			p.Loading = false
			if err != nil {
				p.Error = "刷新失败，仍可使用" + p.Source + "搜索"
				return
			}
			at := time.Now()
			p.index = nga.NewDirectoryIndex(entries)
			p.Source = "最新目录"
			p.FetchedAt = at
			p.list = ui.ListState{}
			if saveErr := local.SaveBoardDirectoryCache(a.dir, raw, at); saveErr != nil {
				p.Error = "目录已刷新，但本地缓存未保存"
			}
		})
	}
	if a.runDirectoryTask != nil {
		a.runDirectoryTask(task)
	} else {
		go task()
	}
}

func (a *reader) boardSearchResults() []nga.DirectoryEntry {
	p := &a.picker
	if p.index != p.resultsIndex || p.Query != p.lastQuery {
		p.results = p.index.Search(p.Query)
		p.resultsIndex = p.index
		p.lastQuery = p.Query
		p.list = ui.ListState{}
	}
	return p.results
}

func (a *reader) submitBoardSearch() {
	if !a.addOpen || a.navigationBusy() {
		return
	}
	results := a.boardSearchResults()
	if strings.TrimSpace(a.picker.Query) != "" && len(results) == 1 && results[0].CollectionID == 0 {
		a.addDirectoryBoard(results[0])
		return
	}
	if len(results) == 1 && results[0].CollectionID != 0 {
		a.formError = "这是合集，暂不支持添加；可点击“官网查看”"
		return
	}
	a.formError = "请进一步搜索，或点击结果旁的按钮选择板块"
}

func (a *reader) addDirectoryBoard(entry nga.DirectoryEntry) {
	if !a.addOpen || a.navigationBusy() {
		return
	}
	if entry.CollectionID != 0 {
		a.formError = "合集暂不支持添加，请在 NGA 官网查看"
		return
	}
	if entry.ID == 0 || strings.TrimSpace(entry.Name) == "" {
		a.formError = "板块信息无效，请刷新目录后重试"
		return
	}
	a.persistAddedBoard(local.Board{ID: entry.ID, Name: entry.Name}, false)
}

// Commit a separate candidate first. A failed write keeps the dialog, query,
// current board and live snapshot intact, including when trying manual input.
func (a *reader) persistAddedBoard(board local.Board, rename bool) {
	candidate := a.state
	candidate.Boards = append([]local.Board(nil), a.state.Boards...)
	found := false
	for i, existing := range candidate.Boards {
		if existing.ID == board.ID {
			found = true
			if rename {
				candidate.Boards[i] = board
			} else {
				board = existing
			}
			break
		}
	}
	if !found {
		candidate.Boards = append(candidate.Boards, board)
	}
	candidate.LastBoard = board.ID
	if !a.demo {
		if err := local.Save(a.dir, candidate); err != nil {
			a.formError = "添加未保存：" + err.Error()
			return
		}
	}
	a.state = candidate
	a.addOpen = false
	a.formError = ""
	a.boardInput = ""
	a.boardName = ""
	a.selectBoard(board)
}

func (a *reader) hasBoard(id int64) bool {
	for _, board := range a.state.Boards {
		if board.ID == id {
			return true
		}
	}
	return false
}

func (a *reader) openBoardCollection(entry nga.DirectoryEntry) {
	if a.demo || a.navigationBusy() || entry.CollectionID <= 0 || a.win == nil {
		return
	}
	address := fmt.Sprintf("https://bbs.nga.cn/thread.php?stid=%d", entry.CollectionID)
	if entry.ID != 0 {
		address += fmt.Sprintf("&fid=%d", entry.ID)
	}
	if err := mygo.Shell.OpenExternal(address); err != nil {
		a.formError = "无法打开 NGA 官网，请稍后重试"
	}
}
