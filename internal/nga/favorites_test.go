package nga

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestFavoriteBoardsReadOnlyRequest(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r.ParseForm()
		if r.Method != "POST" || r.URL.Path != "/nuke.php" || r.URL.Query().Get("__lib") != "forum_favor2" || r.URL.Query().Get("__act") != "forum_favor" || r.PostForm.Get("action") != "get" {
			t.Error("wrong or mutating favorites request", r.Method, r.URL, r.PostForm)
		}
		w.Write([]byte(`{"data":{"0":{"0":{"fid":-7,"name":"大漩涡"},"1":{"fid":"406","name":"英雄 &amp; 联盟"},"2":{"stid":12345,"name":"一个合集"}}}}`))
	}))
	defer srv.Close()
	c := NewClient(srv.Client())
	c.base = srv.URL
	boards, err := c.FavoriteBoards(context.Background())
	if err != nil || len(boards) != 3 || boards[0].ID != -7 || boards[1].Name != "英雄 & 联盟" || boards[2].CollectionID != 12345 {
		t.Fatalf("favorites lost: %+v %v", boards, err)
	}
}
func TestFavoriteBoardsArrayAndEmptyResponse(t *testing.T) {
	boards, err := ParseFavoriteBoards([]byte(`{"data":[{"0":{"fid":-7,"name":"大漩涡"}}]}`))
	if err != nil || len(boards) != 1 || boards[0].ID != -7 {
		t.Fatalf("array response failed: %+v %v", boards, err)
	}
	boards, err = ParseFavoriteBoards([]byte(`{"data":{"0":{}}}`))
	if err != nil || len(boards) != 0 {
		t.Fatal("empty favorites was rejected")
	}
	if _, err = ParseFavoriteBoards([]byte(`{"error":["1:未登录"]}`)); err == nil {
		t.Fatal("login failure became empty favorites")
	}
}
