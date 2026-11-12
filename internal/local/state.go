package local

import (
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"ngareader/internal/nga"
)

type Board struct {
	ID         int64  `json:"id"`
	Name       string `json:"name"`
	CustomName bool   `json:"customName,omitempty"`
}
type Reading struct {
	Topic        nga.Topic        `json:"topic"`
	Page         int              `json:"page"`
	At           time.Time        `json:"at"`
	UID          int64            `json:"uid,omitempty"`
	Scoped       bool             `json:"scoped,omitempty"`
	RepliesKnown bool             `json:"repliesKnown,omitempty"`
	Position     *ReadingPosition `json:"position,omitempty"`
}
type State struct {
	Boards           []Board     `json:"boards"`
	Favorites        []nga.Topic `json:"favorites"`
	History          []Reading   `json:"history"`
	Theme            string      `json:"theme"`
	AppIcon          string      `json:"appIcon"`
	FontSize         int         `json:"fontSize"`
	TextOnly         bool        `json:"textOnly"`
	RememberLogin    bool        `json:"rememberLogin"`
	AutoCheckUpdates bool        `json:"autoCheckUpdates"`
	LastUpdateCheck  time.Time   `json:"lastUpdateCheck,omitempty"`
	LastBoard        int64       `json:"lastBoard"`
	LastTopic        int64       `json:"lastTopic"`
	SidebarWidth     float32     `json:"sidebarWidth"`
	TopicWidth       float32     `json:"topicWidth"`
	SidebarHidden    bool        `json:"sidebarHidden,omitempty"`
	WindowWidth      int         `json:"windowWidth,omitempty"`
	WindowHeight     int         `json:"windowHeight,omitempty"`
	AutoNextPage     bool        `json:"autoNextPage"`
}

// Treat only the optional icon preference tolerantly. An unsupported JSON
// value must not send otherwise valid boards, favorites and preferences through
// whole-file corruption recovery. Other fields retain their normal decoding.
func (s *State) UnmarshalJSON(raw []byte) error {
	type stateJSON State
	value := struct {
		*stateJSON
		AppIcon json.RawMessage `json:"appIcon"`
	}{stateJSON: (*stateJSON)(s)}
	if err := json.Unmarshal(raw, &value); err != nil {
		return err
	}
	s.AppIcon = "work"
	var icon string
	if json.Unmarshal(value.AppIcon, &icon) == nil && icon == "office" {
		s.AppIcon = "office"
	}
	return nil
}

func Defaults() State {
	return State{Boards: []Board{{ID: -7, Name: "大漩涡"}}, Theme: "深色", AppIcon: "work", FontSize: 15, TextOnly: true, LastBoard: -7, SidebarWidth: 180, TopicWidth: 310, WindowWidth: 1280, WindowHeight: 820, AutoCheckUpdates: true, AutoNextPage: true}
}

func Load(dir string) (State, error) {
	state := Defaults()
	path := filepath.Join(dir, "settings.json")
	raw, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return state, nil
	}
	if err != nil {
		return state, fmt.Errorf("读取设置失败：%w", err)
	}
	if len(raw) > 2<<20 || json.Unmarshal(raw, &state) != nil {
		backup := filepath.Join(dir, "settings.invalid-"+time.Now().Format("20060102-150405.000000000")+".json")
		if err := os.Rename(path, backup); err != nil {
			return Defaults(), fmt.Errorf("设置损坏且无法备份；请检查 data 目录权限")
		}
		return Defaults(), fmt.Errorf("设置文件损坏，已保留备份并恢复默认设置")
	}
	normalize(&state)
	return state, nil
}

func normalize(s *State) {
	if s.AppIcon != "office" {
		s.AppIcon = "work"
	}
	if len(s.Boards) == 0 {
		s.Boards = Defaults().Boards
	}
	s.FontSize = min(22, max(12, s.FontSize))
	if s.Theme != "浅色" && s.Theme != "深色" && s.Theme != "跟随系统" && s.Theme != "NGA经典屎黄色" {
		s.Theme = "深色"
	}
	if s.LastBoard == 0 {
		s.LastBoard = -7
	}
	s.SidebarWidth = min(280, max(160, s.SidebarWidth))
	s.TopicWidth = min(450, max(240, s.TopicWidth))
	s.WindowWidth = min(3840, max(980, s.WindowWidth))
	s.WindowHeight = min(2160, max(620, s.WindowHeight))
	s.History = normalizeReadings(s.History)
	if len(s.Favorites) > 300 {
		s.Favorites = s.Favorites[:300]
	}
}

func Save(dir string, state State) error {
	normalize(&state)
	raw, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return err
	}
	return atomicWrite(dir, "settings.json", raw)
}

func atomicWrite(dir, name string, raw []byte) error {
	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("无法创建 data 目录：%w", err)
	}
	file, err := os.CreateTemp(dir, ".write-*")
	if err != nil {
		return fmt.Errorf("data 目录不可写：%w", err)
	}
	path := file.Name()
	defer os.Remove(path)
	if err = file.Chmod(0600); err != nil {
		file.Close()
		return err
	}
	if _, err = file.Write(raw); err != nil {
		file.Close()
		return err
	}
	if err = file.Sync(); err != nil {
		file.Close()
		return err
	}
	if err = file.Close(); err != nil {
		return err
	}
	if err = os.Rename(path, filepath.Join(dir, name)); err != nil {
		return fmt.Errorf("保存文件失败：%w", err)
	}
	return nil
}

func ParseBoard(value string) (Board, error) {
	value = strings.TrimSpace(value)
	id, err := strconv.ParseInt(value, 10, 64)
	if err != nil {
		if u, e := url.Parse(value); e == nil && u.Query().Has("stid") {
			return Board{}, fmt.Errorf("这是合集链接，暂不支持添加；请在 NGA 官网查看")
		}
		tid, fid, e := nga.ParseLink(value)
		if e != nil || tid != 0 || fid == 0 {
			return Board{}, fmt.Errorf("请输入板块数字 ID 或 NGA 板块链接")
		}
		id = fid
	}
	if id == 0 {
		return Board{}, fmt.Errorf("板块 ID 不能为 0")
	}
	name := fmt.Sprintf("板块 %d", id)
	if id == -7 {
		name = "大漩涡"
	}
	return Board{ID: id, Name: name}, nil
}

func MergeBoards(existing, incoming []Board) ([]Board, int) {
	result := append([]Board(nil), existing...)
	seen := map[int64]bool{}
	for _, board := range existing {
		seen[board.ID] = true
	}
	added := 0
	for _, board := range incoming {
		if board.ID == 0 || seen[board.ID] {
			continue
		}
		seen[board.ID] = true
		result = append(result, board)
		added++
	}
	return result, added
}
