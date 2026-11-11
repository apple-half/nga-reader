package main

import (
	"context"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/egoist/mygo/ui"
	"ngareader/internal/local"
	"ngareader/internal/nga"
)

type fakePostingAPI struct {
	uid                    int64
	reads, writes          int
	prepareErr, publishErr error
	target                 nga.PostTarget
}

func (f *fakePostingAPI) SessionUID() int64 { return f.uid }
func (f *fakePostingAPI) PreparePost(_ context.Context, target nga.PostTarget) (nga.PostPreparation, error) {
	f.reads++
	f.target = target
	if target.Action != "new" {
		target.FID = 436
	}
	content := ""
	if target.Action == "quote" {
		content = "[quote]论坛生成的引用[/quote]\n"
	}
	return nga.PostPreparation{UID: f.uid, Target: target, Content: content}, f.prepareErr
}
func (f *fakePostingAPI) PublishPost(_ context.Context, p nga.PostPreparation, subject, content string) (nga.PublishResult, error) {
	f.writes++
	return nga.PublishResult{TID: p.Target.TID, Message: "发贴完毕"}, f.publishErr
}
func (f *fakePostingAPI) TopicCategories(context.Context, int64) ([]string, error) {
	return []string{"[讨论]", "[求助]"}, nil
}
func postingReader(t *testing.T) (*reader, *fakePostingAPI) {
	t.Helper()
	a := newReader(t.TempDir())
	t.Cleanup(a.cancel)
	f := &fakePostingAPI{uid: 123}
	a.posting = f
	a.runPosting = func(fn func()) { fn() }
	return a, f
}

func TestComposerNativeEntryAndDraftRestoration(t *testing.T) {
	a, f := postingReader(t)
	tt := ui.NewTester(a.view, 1280, 820)
	if err := tt.Click("发帖"); err != nil {
		t.Fatal(err)
	}
	if !a.compose.Open || !a.compose.Ready || f.target.Action != "new" || !tt.HasText("发表帖子") {
		t.Fatalf("new composer missing: %+v", a.compose)
	}
	a.compose.Draft.Subject = "自己的标题"
	a.compose.Draft.Content = "未完成的正文"
	if !a.closeComposer() {
		t.Fatal("could not save/close")
	}
	a.openComposer(nga.PostTarget{Action: "new", FID: -7}, "大漩涡")
	if a.compose.Draft.Subject != "自己的标题" || a.compose.Draft.Content != "未完成的正文" {
		t.Fatal("new topic draft lost")
	}
	writePreview(t, ui.NewTester(a.view, 1280, 820), "compose-new")
}

func TestComposerQuotesUsePreparedScopeAndPreserveDraft(t *testing.T) {
	a, _ := postingReader(t)
	a.openComposer(nga.PostTarget{Action: "quote", FID: -7, TID: 9, PID: 42}, "某帖 #2")
	if a.compose.Draft.Target.FID != 436 || a.compose.Draft.Content != "[quote]论坛生成的引用[/quote]\n" {
		t.Fatal("prepared quote/scope not used")
	}
	a.compose.Draft.Content += "我的回复"
	a.closeComposer()
	a.openComposer(nga.PostTarget{Action: "quote", TID: 9, PID: 42}, "某帖 #2")
	if a.compose.Draft.Content != "[quote]论坛生成的引用[/quote]\n我的回复" {
		t.Fatal("prepare replaced restored text")
	}
}

func TestComposerDemoNeverCallsPostingAPIOrWritesDraft(t *testing.T) {
	a, f := postingReader(t)
	a.useDemo()
	a.openComposer(nga.PostTarget{Action: "new", FID: -7}, "演示")
	a.compose.Draft.Subject = "演示标题"
	a.compose.Draft.Content = "演示正文"
	a.sendComposer()
	a.closeComposer()
	if f.reads != 0 || f.writes != 0 {
		t.Fatal("demo called NGA")
	}
	if _, err := os.Stat(filepath.Join(a.dir, "drafts.json")); !os.IsNotExist(err) {
		t.Fatal("demo persisted real draft")
	}
}

func TestComposerUncertainOutcomeSurvivesRestartAndRequiresAcknowledgement(t *testing.T) {
	a, f := postingReader(t)
	a.openComposer(nga.PostTarget{Action: "reply", TID: 9}, "主题")
	a.compose.Draft.Content = "我的回复"
	f.publishErr = &nga.PublishError{Err: errors.New("连接中断"), Uncertain: true}
	a.sendComposer()
	a.sendComposer()
	if f.writes != 1 || !a.compose.Draft.Uncertain {
		t.Fatal("uncertain result was resent")
	}
	b := newReader(a.dir)
	defer b.cancel()
	b.posting = f
	b.runPosting = a.runPosting
	b.openComposer(nga.PostTarget{Action: "reply", TID: 9}, "主题")
	b.sendComposer()
	if f.writes != 1 || !b.compose.Draft.Uncertain {
		t.Fatal("restart cleared duplicate protection")
	}
	b.compose.RetryAcknowledged = true
	b.acknowledgeComposer()
	f.publishErr = nil
	b.sendComposer()
	if f.writes != 2 || b.compose.Open || len(b.drafts) != 0 {
		t.Fatalf("explicit retry failed: writes=%d open=%t drafts=%d", f.writes, b.compose.Open, len(b.drafts))
	}
}

func TestComposerPersistsPendingBeforePostAndLocksEverySessionPath(t *testing.T) {
	a, f := postingReader(t)
	a.openComposer(nga.PostTarget{Action: "reply", TID: 9}, "主题")
	a.compose.Draft.Content = "正文"
	var queued func()
	a.runPosting = func(fn func()) { queued = fn }
	a.sendComposer()
	a.sendComposer()
	stored, err := local.LoadDrafts(a.dir)
	if err != nil || len(stored) != 1 || !stored[0].Uncertain || !a.compose.Sending || f.writes != 0 {
		t.Fatal("pending record was not durable before network")
	}
	if a.closeComposer() {
		t.Fatal("closed while sending")
	}
	if err := a.adoptLoginCookies([]*http.Cookie{{Name: "ngaPassportUid", Value: "999"}, {Name: "ngaPassportCid", Value: "synthetic"}}); err == nil {
		t.Fatal("session replaced while sending")
	}
	a.signOut()
	a.useDemo()
	a.cookieInput = "ngaPassportUid=999; ngaPassportCid=synthetic"
	a.importSession()
	if a.demo || !a.compose.Sending || a.client.SessionUID() != 0 {
		t.Fatal("session-changing path bypassed send lock")
	}
	if queued == nil {
		t.Fatal("submission was not queued")
	}
	queued()
	if f.writes != 1 {
		t.Fatal("queued submission count wrong")
	}
}

func TestComposerDoesNotSendWhenPersistenceFails(t *testing.T) {
	a, f := postingReader(t)
	a.openComposer(nga.PostTarget{Action: "reply", TID: 9}, "主题")
	a.compose.Draft.Content = "正文"
	blocker := filepath.Join(t.TempDir(), "file")
	if err := os.WriteFile(blocker, []byte("x"), 0600); err != nil {
		t.Fatal(err)
	}
	a.dir = blocker
	a.sendComposer()
	if f.writes != 0 || a.compose.Sending || a.compose.Error == "" {
		t.Fatal("post went out without durable draft")
	}
}

func TestComposerAccountChangeCannotLeakOrSubmitDraft(t *testing.T) {
	a, f := postingReader(t)
	a.openComposer(nga.PostTarget{Action: "reply", TID: 9}, "主题")
	a.compose.Draft.Content = "账号123的草稿"
	f.uid = 999
	a.sendComposer()
	if f.writes != 0 {
		t.Fatal("different account submitted old draft")
	}
	a.closeComposer()
	a.openComposer(nga.PostTarget{Action: "reply", TID: 9}, "主题")
	if a.compose.Draft.Content != "" || a.compose.Draft.UID != 999 {
		t.Fatal("another account saw old draft")
	}
}

func TestComposerEnterOnlyAddsNewlineAndMinimumWindowIsUsable(t *testing.T) {
	a, _ := postingReader(t)
	a.useDemo()
	a.openComposer(nga.PostTarget{Action: "new", FID: -7}, "大漩涡")
	for _, theme := range []string{"深色", "浅色", "NGA经典屎黄色"} {
		a.state.Theme = theme
		a.compose.Draft.Content = ""
		tt := ui.NewTester(a.view, 980, 620)
		if err := tt.Click("正文"); err != nil {
			t.Fatal(err)
		}
		tt.Type("第一行")
		tt.Key(0, ui.KeyEnter)
		tt.Type("第二行")
		if !strings.Contains(a.compose.Draft.Content, "第一行\n第二行") || a.compose.Sending {
			t.Fatal("Enter submitted instead of editing")
		}
		if !tt.HasText("发表帖子") || !tt.HasText("保存并关闭") {
			t.Fatal("composer actions missing in small window")
		}
		writePreview(t, tt, "compose-"+theme)
	}
}

func TestComposerConfirmedExistingPostCanClearUncertainDraft(t *testing.T) {
	a, f := postingReader(t)
	a.openComposer(nga.PostTarget{Action: "reply", TID: 9}, "主题")
	a.compose.Draft.Content = "已经发表的内容"
	a.compose.Draft.Uncertain = true
	a.saveComposer()
	a.confirmComposerPublished()
	if a.compose.Open || len(a.drafts) != 0 || f.writes != 0 {
		t.Fatal("confirming existing post did not clear only its draft")
	}
}

func TestComposerCorruptDraftFileCannotBeOverwrittenByDiscard(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "drafts.json")
	original := []byte("broken draft file")
	if err := os.WriteFile(path, original, 0600); err != nil {
		t.Fatal(err)
	}
	a := newReader(dir)
	defer a.cancel()
	a.posting = &fakePostingAPI{uid: 123}
	a.runPosting = func(fn func()) { fn() }
	a.openComposer(nga.PostTarget{Action: "new", FID: -7}, "板块")
	a.discardComposer()
	got, _ := os.ReadFile(path)
	if !a.compose.Open || string(got) != string(original) {
		t.Fatal("discard overwrote a corrupt drafts file")
	}
}

func TestComposerReadActionsBindTopicAndMainFloor(t *testing.T) {
	a, f := postingReader(t)
	a.useDemo()
	tt := ui.NewTester(a.view, 1280, 820)
	if err := tt.Click("回复"); err != nil {
		t.Fatal(err)
	}
	if a.compose.Draft.Target.Action != "reply" || a.compose.Draft.Target.TID != 1001 {
		t.Fatal("reply entry lost topic")
	}
	a.closeComposer()
	tt.Frame()
	if err := tt.Click("引用回复第0楼"); err != nil {
		t.Fatal(err)
	}
	if a.compose.Draft.Target.Action != "quote" || a.compose.Draft.Target.PID != 0 || f.reads != 0 {
		t.Fatal("main-floor quote changed scope or made network call")
	}
	writePreview(t, tt, "compose-quote")
}

func TestComposerEditorPreservesValidSupplementaryCharacters(t *testing.T) {
	a, _ := postingReader(t)
	a.useDemo()
	a.openComposer(nga.PostTarget{Action: "new", FID: -7}, "大漩涡")
	tt := ui.NewTester(a.view, 1280, 820)
	title := strings.Repeat("😀", 65)
	if err := tt.Click("帖子标题"); err != nil {
		t.Fatal(err)
	}
	tt.Type(title)
	if a.compose.Draft.Subject != title {
		t.Fatalf("valid emoji title was cut to %d runes", len([]rune(a.compose.Draft.Subject)))
	}
	body := strings.Repeat("😀", 10001)
	if err := tt.Click("正文"); err != nil {
		t.Fatal(err)
	}
	tt.Type(body)
	if a.compose.Draft.Content != body {
		t.Fatalf("valid emoji body was cut to %d runes", len([]rune(a.compose.Draft.Content)))
	}
}

func TestReaderSecondOwnerCannotLoadOrOverwritePendingDrafts(t *testing.T) {
	dir := t.TempDir()
	a, closeOwner, err := openOwnedReader(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer a.cancel()
	defer closeOwner()
	draft := local.Draft{UID: 123, Target: nga.PostTarget{Action: "reply", TID: 9, FID: -7}, Content: "待确认内容", Uncertain: true}
	if err := local.SaveDrafts(dir, []local.Draft{draft}); err != nil {
		t.Fatal(err)
	}
	if second, release, err := openOwnedReader(dir); err == nil {
		second.cancel()
		release()
		t.Fatal("second startup loaded a writable stale snapshot")
	}
	closeOwner()
	b, closeSecond, err := openOwnedReader(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer b.cancel()
	defer closeSecond()
	if len(b.drafts) != 1 || !b.drafts[0].Uncertain {
		t.Fatal("pending protection disappeared across owners")
	}
}
