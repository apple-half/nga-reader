package main

import (
	"github.com/egoist/mygo/ui"
	"ngareader/internal/local"
	"strings"
	"unicode/utf8"
)

type boardManagerState struct {
	Open        bool
	ID          int64
	Name, Error string
}

func (a *reader) openBoardManager() {
	if a.navigationBusy() || a.compose.Sending {
		return
	}
	a.manager = boardManagerState{Open: true, ID: a.board.ID, Name: a.board.Name}
}
func (a *reader) editManagedBoard(action string) {
	s := &a.manager
	if !s.Open || a.navigationBusy() || a.compose.Sending {
		return
	}
	candidate := a.state
	candidate.Boards = append([]local.Board(nil), a.state.Boards...)
	i := -1
	for k, b := range candidate.Boards {
		if b.ID == s.ID {
			i = k
			break
		}
	}
	if i < 0 {
		s.Error = "板块已变化，请重新选择"
		return
	}
	removed := false
	switch action {
	case "rename":
		name := strings.TrimSpace(s.Name)
		if name == "" || utf8.RuneCountInString(name) > 40 {
			s.Error = "名称应为1至40字"
			return
		}
		candidate.Boards[i].Name = name
		candidate.Boards[i].CustomName = true
	case "up":
		if i == 0 {
			return
		}
		candidate.Boards[i-1], candidate.Boards[i] = candidate.Boards[i], candidate.Boards[i-1]
	case "down":
		if i+1 >= len(candidate.Boards) {
			return
		}
		candidate.Boards[i+1], candidate.Boards[i] = candidate.Boards[i], candidate.Boards[i+1]
	case "remove":
		if len(candidate.Boards) <= 1 {
			s.Error = "至少保留一个板块"
			return
		}
		candidate.Boards = append(candidate.Boards[:i], candidate.Boards[i+1:]...)
		removed = true
		if a.board.ID == s.ID {
			candidate.LastBoard = candidate.Boards[min(i, len(candidate.Boards)-1)].ID
		}
	default:
		return
	}
	if !a.demo {
		if err := local.Save(a.dir, candidate); err != nil {
			s.Error = "修改未保存：" + err.Error()
			return
		}
	}
	a.state = candidate
	s.Error = ""
	if removed && a.board.ID == s.ID {
		for _, b := range candidate.Boards {
			if b.ID == candidate.LastBoard {
				a.selectBoard(b)
				break
			}
		}
	} else {
		for _, b := range candidate.Boards {
			if b.ID == a.board.ID {
				a.board = b
			}
		}
	}
	if removed {
		b := candidate.Boards[min(i, len(candidate.Boards)-1)]
		s.ID = b.ID
		s.Name = b.Name
	}
}
func (a *reader) boardManagerView(c *ui.Context) {
	s := &a.manager
	dialog(c, &s.Open, "管理我的板块", func() {
		ui.Text(c, "重命名、调整顺序或移除本地板块，不改变 NGA 账号收藏。").FontSize(12).TextColor(c.Theme().TextMuted)
		ui.Scroll(c.Key("managed-boards")).Height(150).FillWidth().Children(func() {
			for _, b := range a.state.Boards {
				b := b
				ui.Button(c, b.Name).Label("管理：" + b.Name).FillWidth().OnClick(func() { s.ID = b.ID; s.Name = b.Name; s.Error = "" })
			}
		})
		ui.TextInput(c, &s.Name).Label("板块显示名称").MaxLength(80).FillWidth()
		ui.Row(c).Gap(7).Wrap().Children(func() {
			ui.Button(c, "保存名称").OnClick(func() { a.editManagedBoard("rename") })
			ui.Button(c, "上移").Label("上移板块").OnClick(func() { a.editManagedBoard("up") })
			ui.Button(c, "下移").Label("下移板块").OnClick(func() { a.editManagedBoard("down") })
			ui.Button(c, "移除").Label("移除板块").Disabled(len(a.state.Boards) <= 1).OnClick(func() { a.editManagedBoard("remove") })
		})
		if s.Error != "" {
			ui.Text(c, s.Error).FontSize(12).TextColor(c.Theme().Danger)
		}
	})
}
