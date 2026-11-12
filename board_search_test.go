package main

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
	"ngareader/internal/local"
	"ngareader/internal/nga"
)

func boardPickerReader(t *testing.T) *reader {
	t.Helper()
	a := newReader(t.TempDir())
	t.Cleanup(a.cancel)
	a.runDirectoryTask = func(func()) {} // Explicit offline native UI fixture.
	a.runReadTask = func(func()) {}
	return a
}

func TestNativeBoardSearchAddsByNameAndKeepsCustomDuplicates(t *testing.T) {
	a := boardPickerReader(t)
	tt := ui.NewTester(a.view, 980, 620)
	if err := tt.Click("添加板块"); err != nil {
		t.Fatal(err)
	}
	if err := tt.Click("搜索板块名称"); err != nil {
		t.Fatal(err)
	}
	tt.Type("汽车")
	if !tt.HasText("寂寞的车") || !tt.HasText("车迷俱乐部") {
		t.Fatalf("search hidden: %v", tt.Texts())
	}
	writePreview(t, tt, "board-search-car")
	if err := tt.Click("添加 寂寞的车"); err != nil {
		t.Fatal(err)
	}
	if a.addOpen || a.board.ID != -343809 || len(a.state.Boards) != 2 {
		t.Fatalf("not added/opened: %+v", a.state.Boards)
	}
	stored, err := local.Load(a.dir)
	if err != nil || len(stored.Boards) != 2 || stored.LastBoard != -343809 {
		t.Fatal("add not saved", err)
	}
	a.state.Boards[1].Name = "我的车版"
	a.state.Boards[1].CustomName = true
	a.openBoardPicker()
	a.picker.Query = "寂寞的车"
	a.submitBoardSearch()
	if len(a.state.Boards) != 2 || a.board.Name != "我的车版" || !a.board.CustomName {
		t.Fatal("duplicate overwrote custom name", a.board)
	}
}

func TestNativeBoardSearchKeyboardAmbiguityCollectionAndEscape(t *testing.T) {
	a := boardPickerReader(t)
	a.openBoardPicker()
	tt := ui.NewTester(a.view, 980, 620)
	if err := tt.Click("搜索板块名称"); err != nil {
		t.Fatal(err)
	}
	tt.Type("魔兽")
	tt.Key(0, ui.KeyEnter)
	if !a.addOpen || len(a.state.Boards) != 1 {
		t.Fatal("ambiguous query added a board")
	}
	tt.Key(ui.Cmd, ui.KeyA)
	tt.Type("考研讨论")
	tt.Key(0, ui.KeyEnter)
	if !a.addOpen || len(a.state.Boards) != 1 || !tt.HasText("合集") || !tt.HasText("官网查看") {
		t.Fatal("collection was added as parent or not explained")
	}
	tt.Key(ui.Cmd, ui.KeyA)
	tt.Type("寂寞的车")
	tt.Key(0, ui.KeyEnter)
	if a.addOpen || a.board.ID != -343809 {
		t.Fatal("unique Enter did not add/open")
	}
	a.openBoardPicker()
	tt.Frame()
	tt.Key(0, ui.KeyEscape)
	if a.addOpen {
		t.Fatal("Escape did not close")
	}
}

func TestBoardSearchPersistenceFailureKeepsInputAndDoesNotMutateState(t *testing.T) {
	a := boardPickerReader(t)
	file := filepath.Join(t.TempDir(), "not-a-directory")
	if err := os.WriteFile(file, []byte("occupied"), 0600); err != nil {
		t.Fatal(err)
	}
	a.dir = file
	a.openBoardPicker()
	a.picker.Query = "寂寞的车"
	a.submitBoardSearch()
	if !a.addOpen || a.formError == "" || a.picker.Query != "寂寞的车" || len(a.state.Boards) != 1 || a.board.ID != -7 {
		t.Fatal("failed save pretended success")
	}
	a.formError = ""
	a.addDirectoryBoard(nga.DirectoryEntry{ID: -7, CollectionID: 39827852, Name: "考研讨论"})
	if !a.addOpen || a.formError == "" || a.state.Boards[0].Name != "大漩涡" {
		t.Fatal("collection guard failed")
	}
	a.boardInput = "436"
	a.boardName = "自定义名称"
	a.addBoard()
	if !a.addOpen || a.boardInput != "436" || a.boardName != "自定义名称" || len(a.state.Boards) != 1 {
		t.Fatal("manual save failure lost input")
	}
}

func TestBoardSearchCorruptCacheFallsBackAndManualDuplicateKeepsCustomName(t *testing.T) {
	a := boardPickerReader(t)
	if err := os.WriteFile(filepath.Join(a.dir, "board-directory.json"), []byte("broken"), 0600); err != nil {
		t.Fatal(err)
	}
	a.openBoardPicker()
	a.picker.Query = "数码"
	if a.picker.Source != "内置目录" || len(a.boardSearchResults()) == 0 {
		t.Fatal("corrupt cache removed embedded fallback")
	}
	a.boardInput = "-7"
	a.state.Boards[0].Name = "我的水区"
	a.state.Boards[0].CustomName = true
	a.addBoard()
	if a.board.Name != "我的水区" || !a.board.CustomName || len(a.state.Boards) != 1 {
		t.Fatal("manual duplicate reset saved name")
	}
	for _, busy := range []string{"installing", "closing"} {
		a.addOpen = false
		a.closing = busy == "closing"
		a.updates.Installing = busy == "installing"
		a.openBoardPicker()
		if a.addOpen {
			t.Fatalf("opened picker during %s", busy)
		}
	}
}

type boardDirectoryTransport func(*http.Request) (*http.Response, error)

func (f boardDirectoryTransport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

const tinyDirectory = `{"data":{"0":{"all":{"other":{"name":"社区","content":[{"fid":436,"name":"消费电子 IT新闻"}]}}}}}`

func TestBoardSearchRefreshIsOncePerOpenNotPerKeystrokeAndCaches(t *testing.T) {
	a := boardPickerReader(t)
	calls := 0
	var pending []func()
	a.client = nga.NewClient(&http.Client{Transport: boardDirectoryTransport(func(r *http.Request) (*http.Response, error) {
		calls++
		return &http.Response{StatusCode: 200, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(tinyDirectory)), Request: r}, nil
	})})
	a.runDirectoryTask = func(fn func()) { pending = append(pending, fn) }
	a.openBoardPicker()
	for _, query := range []string{"数", "数码", "汽车", "魔兽"} {
		a.picker.Query = query
		a.boardSearchResults()
		a.openBoardPicker()
	}
	if len(pending) != 1 {
		t.Fatalf("duplicated requests: %d", len(pending))
	}
	a.addOpen = false
	a.formError = "another dialog owns this error"
	pending[0]()
	if calls != 1 || a.addOpen || a.formError != "another dialog owns this error" {
		t.Fatal("late refresh reopened/overwrote modal")
	}
	entries, _, err := local.LoadBoardDirectoryCache(a.dir, time.Now())
	if err != nil || len(entries) != 1 {
		t.Fatal("public directory not cached", err)
	}
	a.openBoardPicker()
	if len(pending) != 1 {
		t.Fatal("fresh directory requested again")
	}
	other := boardPickerReader(t)
	other.dir = a.dir
	other.openBoardPicker()
	other.picker.Query = "数码"
	if len(other.boardSearchResults()) != 1 || other.picker.Source != "本地目录" || other.picker.Loading {
		t.Fatal("fresh cache was not used")
	}
}

func TestBoardSearchMalformedCollectionRefreshPreservesCatalogAndCache(t *testing.T) {
	a := boardPickerReader(t)
	if err := local.SaveBoardDirectoryCache(a.dir, []byte(tinyDirectory), time.Now()); err != nil {
		t.Fatal(err)
	}
	a.openBoardPicker()
	priorIndex := a.picker.index
	path := filepath.Join(a.dir, "board-directory.json")
	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	a.runDirectoryTask = func(fn func()) { fn() }
	a.client = nga.NewClient(&http.Client{Transport: boardDirectoryTransport(func(r *http.Request) (*http.Response, error) {
		bad := `{"data":{"0":{"all":{"other":{"content":[{"fid":-7,"stid":"broken","name":"考研讨论"}]}}}}}`
		return &http.Response{StatusCode: 200, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(bad)), Request: r}, nil
	})})
	a.refreshBoardDirectory()
	after, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if a.picker.Error == "" || a.picker.index != priorIndex || string(before) != string(after) {
		t.Fatal("invalid refresh replaced valid catalog/cache")
	}
	a.picker.Query = "数码"
	got := a.boardSearchResults()
	if len(got) != 1 || got[0].ID != 436 || got[0].CollectionID != 0 {
		t.Fatal("prior local search lost", got)
	}
}

func TestBoardSearchFailureAndDemoCannotReplaceLiveStateOrWriteCache(t *testing.T) {
	a := boardPickerReader(t)
	var pending []func()
	a.runDirectoryTask = func(fn func()) { pending = append(pending, fn) }
	a.client = nga.NewClient(&http.Client{Transport: boardDirectoryTransport(func(r *http.Request) (*http.Response, error) { return nil, context.Canceled })})
	a.openBoardPicker()
	pending[0]()
	a.picker.Query = "汽车"
	if len(a.boardSearchResults()) == 0 || a.picker.Error == "" || a.picker.Loading {
		t.Fatal("failed refresh lost offline fallback")
	}
	a.refreshBoardDirectory()
	a.useDemo()
	pending[1]()
	a.openBoardPicker()
	a.picker.Query = "寂寞的车"
	a.submitBoardSearch()
	if len(pending) != 2 || a.liveState == nil || len(a.liveState.Boards) != 1 {
		t.Fatal("demo changed live boards or fetched directory")
	}
	if _, err := os.Stat(filepath.Join(a.dir, "board-directory.json")); !os.IsNotExist(err) {
		t.Fatal("demo/late task wrote public cache")
	}
	a.returnToLive()
	if len(a.state.Boards) != 1 || a.board.ID != -7 {
		t.Fatal("demo addition persisted")
	}
	a.openBoardPicker()
	a.shutdown()
	pending[len(pending)-1]()
	if _, err := os.Stat(filepath.Join(a.dir, "board-directory.json")); !os.IsNotExist(err) {
		t.Fatal("shutdown task wrote cache")
	}
}

func TestNativeBoardPickerThemesEmptyStateManualAndNarrowLayout(t *testing.T) {
	for _, theme := range []string{"深色", "浅色", "NGA经典屎黄色"} {
		for _, size := range [][2]int{{980, 620}, {760, 520}} {
			t.Run(fmt.Sprintf("%s-%d", theme, size[0]), func(t *testing.T) {
				a := boardPickerReader(t)
				a.state.Theme = theme
				a.openBoardPicker()
				tt := ui.NewTester(a.view, size[0], size[1])
				for _, label := range []string{"搜索板块名称", "关闭添加板块", "刷新目录", "链接或编号"} {
					r, ok := tt.Find(label)
					if !ok || r.X < 0 || r.Y < 0 || r.X+r.W > float32(size[0]) || r.Y+r.H > float32(size[1]) {
						t.Fatalf("control clipped %s %+v", label, r)
					}
				}
				writePreview(t, tt, fmt.Sprintf("board-picker-%s-%d", theme, size[0]))
				if err := tt.Click("搜索板块名称"); err != nil {
					t.Fatal(err)
				}
				tt.Type("不存在的板块zzzz")
				if !tt.HasText("没有找到") {
					t.Fatal("missing empty state")
				}
				if err := tt.Click("链接或编号"); err != nil {
					t.Fatal(err)
				}
				if err := tt.Click("板块 ID 或链接"); err != nil {
					t.Fatal(err)
				}
				tt.Type("436")
				if err := tt.Click("添加并打开"); err != nil {
					t.Fatal(err)
				}
				if a.board.ID != 436 || a.addOpen {
					t.Fatal("manual add regressed")
				}
			})
		}
	}
}
