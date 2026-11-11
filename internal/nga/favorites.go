package nga

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
)

type FavoriteBoard struct {
	ID           int64
	CollectionID int64
	Name         string
}

// FavoriteBoards only performs action=get. It never modifies account favorites.
// Schema reference: BugenZhao/MNGA logic/service/src/forum.rs.
func (c *Client) FavoriteBoards(ctx context.Context) ([]FavoriteBoard, error) {
	raw, err := c.request(ctx, http.MethodPost, "/nuke.php", url.Values{"__lib": {"forum_favor2"}, "__act": {"forum_favor"}}, url.Values{"action": {"get"}})
	if err != nil {
		return nil, err
	}
	return ParseFavoriteBoards(raw)
}
func ParseFavoriteBoards(raw []byte) ([]FavoriteBoard, error) {
	data, err := decode(raw)
	if err != nil {
		return nil, err
	}
	var source any = data
	if value, ok := data["0"]; ok {
		if m := object(value); m == nil || m["fid"] == nil && m["stid"] == nil {
			source = value
		}
	}
	if !validCollection(source) {
		return nil, &APIError{Kind: "parse", Message: "无法识别 NGA 收藏板块数据"}
	}
	rows := collection(source)
	var out []FavoriteBoard
	for _, m := range rows {
		board := FavoriteBoard{ID: integer(m["fid"]), CollectionID: integer(m["stid"]), Name: plain(str(m["name"]))}
		if strings.HasPrefix(str(m["fid"]), "t") {
			board.CollectionID, _ = strconv.ParseInt(strings.TrimPrefix(str(m["fid"]), "t"), 10, 64)
		}
		if board.ID == 0 && board.CollectionID == 0 {
			continue
		}
		if board.Name == "" {
			board.Name = fmt.Sprintf("板块 %d", board.ID)
		}
		out = append(out, board)
	}
	if len(rows) > 0 && len(out) == 0 {
		return nil, &APIError{Kind: "parse", Message: "收藏数据中没有可识别的板块 ID"}
	}
	return out, nil
}
