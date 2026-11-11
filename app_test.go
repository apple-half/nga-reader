package main

import (
	"image/png"
	"ngareader/internal/local"
	"ngareader/internal/nga"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/egoist/mygo/ui"
)

func TestLoggedOutViewOffersLogin(t *testing.T) {
	a := newReader(t.TempDir())
	tt := ui.NewTester(a.view, 1280, 820)
	writePreview(t, tt, "logged-out")
	if !tt.HasText("登录 NGA") || !tt.HasText("大漩涡") || a.demo {
		t.Fatalf("logged-out state hides login action: %q", tt.Texts())
	}
	if err := tt.Click("导入收藏板块"); err != nil {
		t.Fatal(err)
	}
	if !a.importAfterLogin || a.loadingFavorites {
		t.Fatal("import did not defer until the user's login")
	}
}

func TestHelpOpensAndClosesInsideReader(t *testing.T) {
	a := newReader(t.TempDir())
	defer a.cancel()
	a.state.Theme = "NGA经典屎黄色"
	tt := ui.NewTester(a.view, 980, 620)
	if err := tt.Click("使用说明"); err != nil {
		t.Fatal(err)
	}
	if !tt.HasText("快速开始") {
		t.Fatal("built-in manual did not open")
	}
	writePreview(t, tt, "built-in-help")
	if err := tt.Click("关闭说明"); err != nil {
		t.Fatal(err)
	}
	if tt.HasText("快速开始") {
		t.Fatal("manual did not close")
	}
}

func TestDemoTopicSelectionUpdatesReader(t *testing.T) {
	a := newReader(t.TempDir())
	a.useDemo()
	tt := ui.NewTester(a.view, 1280, 820)
	writePreview(t, tt, "three-column")
	if os.Getenv("NGA_READER_PREVIEW") != "" {
		preview := newReader(t.TempDir())
		defer preview.cancel()
		preview.useDemo()
		preview.state.Theme = "NGA经典屎黄色"
		classic := ui.NewTester(preview.view, 1280, 820)
		writePreview(t, classic, "nga-classic")
	}
	if err := tt.Click("桌面收拾干净以后，工作心情真的会变好"); err != nil {
		t.Fatal(err)
	}
	if a.thread.Topic.ID != 1002 || len(a.thread.Posts) == 0 || !strings.Contains(a.thread.Posts[0].Content, "少放东西") {
		t.Fatalf("clicked topic did not change the article: %+v", a.thread)
	}
	if err := tt.Click("收藏帖子"); err != nil {
		t.Fatal(err)
	}
	if !a.isFavorite(1002) {
		t.Fatal("bookmark action did not update local state")
	}
}

func TestImagesNeverAutoloadInEitherLegacyMode(t *testing.T) {
	a := newReader(t.TempDir())
	defer a.cancel()
	for _, textOnly := range []bool{true, false} {
		a.state.TextOnly = textOnly
		a.resetMarkdown()
		if a.md.LoadImage != nil {
			t.Fatalf("unopened images still load in legacy textOnly=%t", textOnly)
		}
	}
}

func TestImageHasExplicitShowAction(t *testing.T) {
	a := newReader(t.TempDir())
	defer a.cancel()
	a.useDemo()
	a.state.Theme = "NGA经典屎黄色"
	a.thread.Posts = []nga.Post{{Floor: 0, Author: "图片示例", Content: "图片默认不加载。\n[img]./mon_202611/10/-7Q31-fcoxK23T3cSk0-zk.jpg[/img]"}}
	a.resetMarkdown()
	tt := ui.NewTester(a.view, 1280, 820)
	if !tt.HasText("展示图片") {
		t.Fatalf("no click-to-show image action: %q", tt.Texts())
	}
	writePreview(t, tt, "on-demand-image")
	if a.pictureAddress != "" || a.pictureWeb != nil {
		t.Fatal("picture preview was created without a click")
	}
	if err := tt.Click("展示图片"); err != nil {
		t.Fatal(err)
	}
	if a.pictureAddress != "https://img.nga.cn/attachments/mon_202611/10/-7Q31-fcoxK23T3cSk0-zk.jpg" {
		t.Fatalf("image action did not choose the clicked attachment: %q", a.pictureAddress)
	}
}

func TestOnlyAuthorCanBeTurnedOffWithSinglePost(t *testing.T) {
	a := newReader(t.TempDir())
	a.useDemo()
	tt := ui.NewTester(a.view, 1280, 820)
	if err := tt.Click("只看楼主"); err != nil {
		t.Fatal(err)
	}
	if !a.onlyAuthor {
		t.Fatal("author filter was not enabled")
	}
	if err := tt.Click("显示全部"); err != nil {
		t.Fatal("single-post author filter has no way back:", err)
	}
	if a.onlyAuthor {
		t.Fatal("author filter was not cleared")
	}
}

func TestSelectBoardClearsCancelledLoading(t *testing.T) {
	a := newReader(t.TempDir())
	a.demo = true
	a.loadingThread = true
	a.selectBoard(local.Board{ID: 123, Name: "另一个板块"})
	if a.loadingThread {
		t.Fatal("cancelled thread leaves a permanent loading status")
	}
}

func TestDemoExitRestoresBoard(t *testing.T) {
	a := newReader(t.TempDir())
	a.state.Boards = append(a.state.Boards, local.Board{ID: 123, Name: "我的板块"})
	a.state.LastBoard = 123
	a.board = a.state.Boards[1]
	a.useDemo()
	a.cancel()
	a.returnToLive()
	if a.board.ID != 123 {
		t.Fatalf("demo changed live board selection to %d", a.board.ID)
	}
}

// Optional offline QA artifact from the app's own native renderer. This does
// not capture or control the desktop, and does not run in ordinary test runs.
func writePreview(t *testing.T, tt *ui.Tester, name string) {
	t.Helper()
	dir := os.Getenv("NGA_READER_PREVIEW")
	if dir == "" {
		return
	}
	if err := os.MkdirAll(dir, 0700); err != nil {
		t.Fatal(err)
	}
	file, err := os.Create(filepath.Join(dir, name+".png"))
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	if err := png.Encode(file, tt.Image()); err != nil {
		t.Fatal(err)
	}
}
