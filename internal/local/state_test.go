package local

import (
	"context"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"ngareader/internal/nga"
)

func TestClassicThemeSurvivesRestart(t *testing.T) {
	dir := t.TempDir()
	s := Defaults()
	s.Theme = "NGA经典屎黄色"
	if err := Save(dir, s); err != nil {
		t.Fatal(err)
	}
	got, err := Load(dir)
	if err != nil || got.Theme != "NGA经典屎黄色" {
		t.Fatalf("selected appearance was lost after restart: %q %v", got.Theme, err)
	}
}

func TestStateRoundTrip(t *testing.T) {
	dir := t.TempDir()
	s := Defaults()
	s.Theme = "浅色"
	s.FontSize = 17
	s.Favorites = []nga.Topic{{ID: 123, Title: "收藏标题"}}
	if err := Save(dir, s); err != nil {
		t.Fatal(err)
	}
	got, err := Load(dir)
	if err != nil || got.Theme != "浅色" || got.FontSize != 17 || len(got.Favorites) != 1 || got.Favorites[0].ID != 123 {
		t.Fatalf("settings lost: %+v %v", got, err)
	}
}
func TestCorruptSettingsPreserved(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "settings.json"), []byte("{corrupt"), 0600)
	got, err := Load(dir)
	if err == nil || len(got.Boards) != 1 || got.Boards[0].ID != -7 {
		t.Fatalf("bad recovery: %+v %v", got, err)
	}
	backups, _ := filepath.Glob(filepath.Join(dir, "settings.invalid-*.json"))
	if len(backups) != 1 {
		t.Fatal("corrupt file was not preserved")
	}
}
func TestBoardParsing(t *testing.T) {
	for _, value := range []string{"-7", "https://bbs.nga.cn/thread.php?fid=-7"} {
		b, err := ParseBoard(value)
		if err != nil || b.ID != -7 {
			t.Fatalf("bad board: %+v %v", b, err)
		}
	}
	for _, value := range []string{"0", "https://evil.test/thread.php?fid=-7", "https://bbs.nga.cn/read.php?tid=1"} {
		if _, err := ParseBoard(value); err == nil {
			t.Errorf("accepted %s", value)
		}
	}
}

func TestMergeFavoriteBoardsPreservesLocalNames(t *testing.T) {
	original := []Board{{ID: -7, Name: "自定义水区", CustomName: true}}
	merged, added := MergeBoards(original, []Board{{ID: -7, Name: "大漩涡"}, {ID: 406, Name: "英雄联盟"}, {ID: 406, Name: "重复"}})
	if added != 1 || len(merged) != 2 || merged[0].Name != "自定义水区" || !merged[0].CustomName || merged[1].ID != 406 {
		t.Fatalf("wrong merge: %+v %d", merged, added)
	}
	if len(original) != 1 || original[0].Name != "自定义水区" {
		t.Fatal("input state was changed")
	}
}

type memoryKeys struct {
	data map[string][]byte
	fail bool
}

func (k *memoryKeys) Get(key string) ([]byte, error) {
	if k.fail {
		return nil, errors.New("locked")
	}
	if k.data[key] == nil {
		return nil, ErrNoSecret
	}
	return k.data[key], nil
}
func (k *memoryKeys) Set(key string, value []byte) error {
	if k.fail {
		return errors.New("locked")
	}
	k.data[key] = value
	return nil
}
func (k *memoryKeys) Delete(key string) error { delete(k.data, key); return nil }
func TestSessionEncryptedRoundTrip(t *testing.T) {
	dir := t.TempDir()
	keys := &memoryKeys{data: map[string][]byte{}}
	cookies := []*http.Cookie{{Name: "ngaPassportUid", Value: "123", Domain: "bbs.nga.cn", Path: "/", Secure: true}, {Name: "ngaPassportCid", Value: "SENSITIVE_SESSION_VALUE", Domain: "bbs.nga.cn", Path: "/", Secure: true}}
	if err := SaveSession(dir, cookies, keys); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(filepath.Join(dir, "session.enc"))
	if err != nil || strings.Contains(string(raw), "SENSITIVE_SESSION_VALUE") {
		t.Fatalf("session was not encrypted: %v", err)
	}
	got, err := LoadSession(dir, keys)
	if err != nil || len(got) != 2 || got[1].Value != "SENSITIVE_SESSION_VALUE" {
		t.Fatalf("session did not restore: %v", err)
	}
	if err := ClearSession(dir, keys); err != nil {
		t.Fatal(err)
	}
	got, err = LoadSession(dir, keys)
	if err != nil || len(got) != 0 {
		t.Fatal("signout did not clear session")
	}
}
func TestSecretStoreFailureWritesNoSession(t *testing.T) {
	dir := t.TempDir()
	keys := &memoryKeys{fail: true}
	if err := SaveSession(dir, []*http.Cookie{{Name: "x", Value: "secret"}}, keys); err == nil {
		t.Fatal("secret-store failure was ignored")
	}
	if _, err := os.Stat(filepath.Join(dir, "session.enc")); !os.IsNotExist(err) {
		t.Fatal("session file written without protected key")
	}
}
func TestCorruptSessionFailsClosed(t *testing.T) {
	dir := t.TempDir()
	keys := &memoryKeys{data: map[string][]byte{}}
	SaveSession(dir, []*http.Cookie{{Name: "x", Value: "secret"}}, keys)
	path := filepath.Join(dir, "session.enc")
	raw, _ := os.ReadFile(path)
	raw[len(raw)-1] ^= 1
	os.WriteFile(path, raw, 0600)
	got, err := LoadSession(dir, keys)
	if err == nil || got != nil {
		t.Fatal("tampered credential file was accepted")
	}
}
func TestRequestGenerationRejectsStaleResults(t *testing.T) {
	var gate RequestGate
	old, first := gate.Begin(context.Background())
	_, second := gate.Begin(context.Background())
	if gate.Accept(first) || !gate.Accept(second) || old.Err() != context.Canceled {
		t.Fatal("late request can overwrite current view")
	}
	gate.Close()
	if gate.Accept(second) {
		t.Fatal("closed view still accepts requests")
	}
}
