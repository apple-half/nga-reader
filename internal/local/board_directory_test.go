package local

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestPublicBoardDirectoryCacheRoundTripAndExpiry(t *testing.T) {
	dir := t.TempDir()
	now := time.Date(2026, 11, 12, 8, 0, 0, 0, time.UTC)
	raw := []byte(`{"data":{"0":{"all":{"other":{"name":"社区","content":[{"fid":-7,"name":"网事杂谈"}]}}}}}`)
	if err := SaveBoardDirectoryCache(dir, raw, now); err != nil {
		t.Fatal(err)
	}
	entries, at, err := LoadBoardDirectoryCache(dir, now.Add(time.Hour))
	if err != nil || len(entries) != 1 || entries[0].ID != -7 || !at.Equal(now) || !BoardDirectoryFresh(at, now.Add(time.Hour)) {
		t.Fatalf("cache: %+v %v %v", entries, at, err)
	}
	if BoardDirectoryFresh(at, now.Add(25*time.Hour)) || BoardDirectoryFresh(now.Add(time.Hour), now) {
		t.Fatal("expired/future cache considered fresh")
	}
	if err := SaveBoardDirectoryCache(dir, []byte("<html>challenge</html>"), now.Add(time.Hour)); err == nil {
		t.Fatal("invalid cache replaced valid directory")
	}
	if entries, at, err = LoadBoardDirectoryCache(dir, now.Add(time.Hour)); err != nil || len(entries) != 1 || !at.Equal(now) {
		t.Fatal("valid fallback lost")
	}
}

func TestPublicBoardDirectoryCacheRejectsCorruptionAndBadTime(t *testing.T) {
	dir := t.TempDir()
	now := time.Now()
	for _, raw := range []string{`not json`, `{"version":2}`, `{"version":1,"fetchedAt":"2099-01-01T00:00:00Z","data":"e30="}`, string(make([]byte, 2<<20))} {
		if err := os.WriteFile(filepath.Join(dir, "board-directory.json"), []byte(raw), 0600); err != nil {
			t.Fatal(err)
		}
		if _, _, err := LoadBoardDirectoryCache(dir, now); err == nil {
			t.Fatal("accepted corrupt/unbounded/future cache")
		}
	}
	if _, _, err := LoadBoardDirectoryCache(t.TempDir(), now); !os.IsNotExist(err) {
		t.Fatalf("missing file: %v", err)
	}
}

func TestManualBoardLinkRejectsCollectionParent(t *testing.T) {
	for _, link := range []string{"https://bbs.nga.cn/thread.php?fid=-7&stid=39827852", "https://ngabbs.com/thread.php?fid=414&stid="} {
		if board, err := ParseBoard(link); err == nil {
			t.Fatalf("collection added as parent %+v", board)
		}
	}
}
