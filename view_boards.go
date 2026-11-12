package main

import (
	"fmt"
	"strings"

	"github.com/egoist/mygo/ui"
)

func (a *reader) boardPickerView(c *ui.Context) {
	p := &a.picker
	ui.DialogBase(c.Key("board-picker"), &a.addOpen, func(backdrop, panel ui.Element) {
		backdrop.Background(ui.RGBA(0, 0, 0, .5))
		panel.Width(620).MaxWidthPercent(94).Height(540).MaxHeightPercent(92).Padding(22).Radius(14).Gap(12).Background(c.Theme().Background).Border(1, c.Theme().Border)
		ui.Row(c).AlignItems(ui.Center).Gap(10).Children(func() {
			ui.Text(c, "添加板块").FontSize(19).FontWeight(500).Grow(1)
			ui.Button(c, "关闭").Label("关闭添加板块").OnClick(func() { a.addOpen = false })
		})
		ui.Row(c).Gap(8).Children(func() {
			search := ui.Button(c, "搜索板块").OnClick(func() { p.Manual = false; a.formError = "" })
			manual := ui.Button(c, "链接或编号").OnClick(func() { p.Manual = true; a.formError = "" })
			if p.Manual {
				manual.Background(c.Theme().Accent).TextColor(c.Theme().AccentText)
			} else {
				search.Background(c.Theme().Accent).TextColor(c.Theme().AccentText)
			}
		})
		if p.Manual {
			ui.Text(c, "支持首页未列出的普通板块；合集暂不支持添加。").FontSize(12).TextColor(c.Theme().TextMuted)
			input := ui.TextInput(c.Key("manual-board"), &a.boardInput).Label("板块 ID 或链接").Placeholder("例如 -7 或 NGA 官方板块链接").MaxLength(500).AutoFocus().FillWidth().OnChange(func() { a.formError = "" })
			ui.TextInput(c, &a.boardName).Label("板块名称").Placeholder("板块名称（可选）").MaxLength(30).FillWidth()
			ui.Spacer(c).Grow(1)
			if a.formError != "" {
				ui.Text(c, a.formError).FontSize(12).TextColor(c.Theme().Danger).MaxLines(3)
			}
			ui.Row(c).Justify(ui.End).Children(func() { ui.PrimaryButton(c, "添加并打开").OnClick(a.addBoard) })
			if input.Submitted() {
				a.addBoard()
			}
			return
		}
		input := ui.TextInput(c.Key("search-board"), &p.Query).Label("搜索板块名称").Placeholder("搜索名称或别名，如 汽车、数码、魔兽").MaxLength(120).AutoFocus().FillWidth().OnChange(func() { a.formError = "" })
		results := a.boardSearchResults()
		ui.Row(c).AlignItems(ui.Center).Gap(8).Children(func() {
			ui.Textf(c, "首页公开目录 · %d 个结果", len(results)).FontSize(11).TextColor(c.Theme().TextMuted).Grow(1)
			if p.Query != "" {
				ui.Button(c, "清空").FontSize(11).Padding(4, 7).OnClick(func() { p.Query = ""; a.formError = "" })
			}
		})
		if len(results) == 0 {
			ui.Column(c).Grow(1).MinHeight(0).Justify(ui.Center).Gap(10).Children(func() {
				ui.Text(c, "没有找到这个板块").FontSize(15)
				ui.Text(c, "试试简称或其他名称，也可以用链接或编号添加。").FontSize(12).TextColor(c.Theme().TextMuted)
			})
		} else {
			p.list.Key = func(i int) any { return [2]int64{results[i].ID, results[i].CollectionID} }
			p.list.Label = func(i int) string { return results[i].Name }
			ui.List(c.Key("board-results"), &p.list, len(results), func(i int) {
				entry := results[i]
				row := c.Key(fmt.Sprintf("board-%d-%d", entry.ID, entry.CollectionID))
				ui.Row(row).FillWidth().Height(73).Padding(10, 9).Gap(12).AlignItems(ui.Center).BorderWidth(0, 0, 1, 0).BorderColor(c.Theme().Border).Children(func() {
					ui.Column(row).Grow(1).MinWidth(0).Gap(5).Children(func() {
						ui.Row(row).AlignItems(ui.Center).Gap(8).Children(func() {
							ui.Text(row, entry.Name).FontSize(14).FontWeight(500).SingleLine().Grow(1)
							if entry.CollectionID != 0 {
								ui.Text(row, "合集").FontSize(10).Padding(2, 5).Radius(4).Background(c.Theme().Surface).TextColor(c.Theme().TextMuted)
							} else if a.hasBoard(entry.ID) {
								ui.Text(row, "已添加").FontSize(10).TextColor(c.Theme().TextMuted)
							}
						})
						meta := entry.Category
						if entry.Description != "" {
							meta = strings.Trim(meta+" · "+entry.Description, " ·")
						}
						if entry.CollectionID != 0 {
							meta = "暂不支持添加 · " + meta
						}
						ui.Text(row, meta).FontSize(11).TextColor(c.Theme().TextMuted).SingleLine().FillWidth()
					})
					if entry.CollectionID != 0 {
						ui.Button(row, "官网查看").Label("官网查看 " + entry.Name).FontSize(11).Disabled(a.demo).OnClick(func() { a.openBoardCollection(entry) })
					} else {
						label, access := "添加并打开", "添加 "+entry.Name
						if a.hasBoard(entry.ID) {
							label = "打开"
							access = "打开 " + entry.Name
						}
						ui.Button(row, label).Label(access).FontSize(11).OnClick(func() { a.addDirectoryBoard(entry) })
					}
				})
			}).Grow(1).MinHeight(0).FillWidth()
		}
		if a.formError != "" {
			ui.Text(c, a.formError).FontSize(12).TextColor(c.Theme().Danger).MaxLines(2)
		}
		ui.Divider(c)
		ui.Row(c).AlignItems(ui.Center).Gap(10).Children(func() {
			status := p.Source + " · 本地搜索，无需登录"
			if p.Loading {
				status = p.Source + " · 正在刷新，搜索可继续"
			} else if p.Error != "" {
				status = p.Error
			}
			ui.Text(c, status).FontSize(11).TextColor(c.Theme().TextMuted).Grow(1).MaxLines(2)
			ui.Button(c, "刷新目录").FontSize(11).Disabled(a.demo || p.Loading).OnClick(a.refreshBoardDirectory)
		})
		if input.Submitted() {
			a.submitBoardSearch()
		}
	})
}
