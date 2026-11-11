package local

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"ngareader/internal/nga"
)

func textDraft(uid, fid, tid, pid int64, action string) Draft {
	return Draft{UID: uid, Target: nga.PostTarget{Action: action, FID: fid, TID: tid, PID: pid}, Subject: "测试标题", Content: "未发送的中文\n内容"}
}

func TestDraftAccountAndTargetIsolation(t *testing.T) {
	drafts := []Draft{textDraft(1, -7, 0, 0, "new"), textDraft(2, -7, 0, 0, "new"), textDraft(1, 436, 9, 0, "reply"), textDraft(1, 436, 9, 1, "quote")}
	dir := t.TempDir()
	if err := SaveDrafts(dir, drafts); err != nil {
		t.Fatal(err)
	}
	loaded, err := LoadDrafts(dir)
	if err != nil || len(loaded) != 4 {
		t.Fatalf("drafts=%v err=%v", loaded, err)
	}
	keys := map[string]bool{}
	for _, d := range loaded {
		if keys[d.Key()] {
			t.Fatal("account/target collided")
		}
		keys[d.Key()] = true
	}
	if textDraft(1, 0, 9, 0, "reply").Key() != textDraft(1, 436, 9, 0, "reply").Key() {
		t.Fatal("canonical reply board split the draft")
	}
}

func TestDraftPendingRecordSurvivesRestartAndCannotRepeat(t *testing.T) {
	d := textDraft(1, -7, 9, 0, "reply")
	if err := d.BeginSend(2); err == nil {
		t.Fatal("different account was allowed")
	}
	if err := d.BeginSend(1); err != nil {
		t.Fatal(err)
	}
	if err := d.BeginSend(1); err == nil {
		t.Fatal("double send was allowed")
	}
	dir := t.TempDir()
	if err := SaveDrafts(dir, []Draft{d}); err != nil {
		t.Fatal(err)
	}
	loaded, err := LoadDrafts(dir)
	if err != nil || !loaded[0].Uncertain {
		t.Fatalf("pending record lost: %v %v", loaded, err)
	}
	if err := loaded[0].BeginSend(1); err == nil {
		t.Fatal("restart made resend automatic")
	}
	loaded[0].AcknowledgeUncertain()
	if err := loaded[0].BeginSend(1); err != nil {
		t.Fatal("explicit acknowledgement did not unlock:", err)
	}
}

func TestDraftSubmissionOutcomesRetainText(t *testing.T) {
	for _, uncertain := range []bool{false, true} {
		d := textDraft(1, -7, 9, 0, "reply")
		_ = d.BeginSend(1)
		if d.EndSend(&nga.PublishError{Err: errors.New("服务端提示"), Uncertain: uncertain}) {
			t.Fatal("failed send deleted draft")
		}
		if d.Uncertain != uncertain || d.Content != "未发送的中文\n内容" {
			t.Fatal("failed send lost state")
		}
	}
	d := textDraft(1, -7, 9, 0, "reply")
	_ = d.BeginSend(1)
	if !d.EndSend(nil) {
		t.Fatal("confirmed success did not permit removal")
	}
}

func TestDraftCorruptionAndSizeLimitsPreserveFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "drafts.json")
	raw := []byte(`{"broken":true}`)
	if err := os.WriteFile(path, raw, 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadDrafts(dir); err == nil {
		t.Fatal("corrupt draft accepted")
	}
	got, _ := os.ReadFile(path)
	if string(got) != string(raw) {
		t.Fatal("load overwrote corrupt source")
	}
	d := textDraft(1, -7, 0, 0, "new")
	d.Content = strings.Repeat("字", nga.MaxPostContentRunes+1)
	if err := SaveDrafts(dir, []Draft{d}); err == nil {
		t.Fatal("oversized draft saved")
	}
	got, _ = os.ReadFile(path)
	if string(got) != string(raw) {
		t.Fatal("invalid save overwrote existing drafts")
	}
}

func TestDraftCategoryAndBlankEditingState(t *testing.T) {
	d := textDraft(1, -7, 0, 0, "new")
	d.Category = "[讨论]"
	if d.PostSubject() != "[讨论] 测试标题" {
		t.Fatal("category missing")
	}
	d.Subject = "[讨论] 自己写的前缀"
	if d.PostSubject() != d.Subject {
		t.Fatal("duplicate category prefix")
	}
	d.Subject = ""
	d.Content = ""
	d.Category = ""
	if err := SaveDrafts(t.TempDir(), []Draft{d}); err != nil {
		t.Fatal("unfinished draft rejected:", err)
	}
	if err := d.BeginSend(1); err == nil {
		t.Fatal("empty draft was sent")
	}
}
