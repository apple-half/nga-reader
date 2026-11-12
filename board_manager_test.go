package main

import (
	"github.com/egoist/mygo/ui"
	"ngareader/internal/local"
	"os"
	"path/filepath"
	"testing"
)

func TestBoardManagerNativeRenameReorderRemoveAndLastProtection(t *testing.T) {
	a := newReader(t.TempDir())
	defer a.cancel()
	a.useDemo()
	a.state.Boards = []local.Board{{ID: -7, Name: "大漩涡"}, {ID: 123, Name: "汽车"}}
	a.board = a.state.Boards[0]
	tt := ui.NewTester(a.view, 980, 620)
	if tt.HasText("-7") || tt.HasText("123") {
		t.Fatal("technical IDs shown in sidebar")
	}
	for _, label := range []string{"管理板块", "管理：汽车"} {
		if err := tt.Click(label); err != nil {
			t.Fatal(err)
		}
	}
	a.manager.Name = "我的车"
	if err := tt.Click("保存名称"); err != nil {
		t.Fatal(err)
	}
	if a.state.Boards[1].Name != "我的车" || !a.state.Boards[1].CustomName {
		t.Fatal("rename failed")
	}
	if err := tt.Click("上移板块"); err != nil {
		t.Fatal(err)
	}
	if a.state.Boards[0].ID != 123 {
		t.Fatal("reorder failed")
	}
	if err := tt.Click("移除板块"); err != nil {
		t.Fatal(err)
	}
	a.editManagedBoard("remove")
	if len(a.state.Boards) != 1 || a.state.Boards[0].ID != -7 {
		t.Fatal("last board removed")
	}
	writePreview(t, tt, "ux-board-manager")
}
func TestBoardManagerFailedSaveIsAtomic(t *testing.T) {
	dir := t.TempDir()
	bad := filepath.Join(dir, "blocked")
	if err := os.WriteFile(bad, []byte("file"), 0600); err != nil {
		t.Fatal(err)
	}
	a := newReader(t.TempDir())
	defer a.cancel()
	a.dir = bad
	a.state.Boards = []local.Board{{ID: -7, Name: "大漩涡"}, {ID: 123, Name: "汽车"}}
	a.openBoardManager()
	a.manager.ID = -7
	a.manager.Name = "不能丢失"
	a.editManagedBoard("rename")
	if a.manager.Error == "" || a.state.Boards[0].Name != "大漩涡" || a.board.Name != "大漩涡" {
		t.Fatal("failed save mutated live board")
	}
	a.manager.ID = 123
	a.editManagedBoard("remove")
	if len(a.state.Boards) != 2 {
		t.Fatal("failed removal mutated list")
	}
}
func TestTopicFilterOffersClear(t *testing.T) {
	a := newReader(t.TempDir())
	defer a.cancel()
	a.useDemo()
	a.filter = "不存在的关键词"
	tt := ui.NewTester(a.view, 980, 620)
	if err := tt.Click("清除话题筛选"); err != nil {
		t.Fatal(err)
	}
	if a.filter != "" || len(a.displayTopics()) == 0 || !tt.HasText("本页按回复数") {
		t.Fatal("filter clear or sort scope missing")
	}
}
