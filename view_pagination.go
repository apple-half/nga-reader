package main

import (
	"fmt"
	"github.com/egoist/mygo/ui"
)

func (a *reader) pageJumpView(c *ui.Context) {
	s := &a.jump
	dialog(c, &s.Open, "跳转页码", func() {
		total := a.topics.TotalPages
		current := max(1, a.topics.Page)
		kind := "话题列表"
		if s.Thread {
			total = a.threadPageLimit()
			current = a.visibleThreadPage()
			kind = "帖子回复"
		}
		if total > 0 {
			ui.Text(c, fmt.Sprintf("%s · 当前第%d页，共%d页", kind, current, total)).FontSize(12).TextColor(c.Theme().TextMuted)
		} else {
			ui.Text(c, kind+" · 总页数未知，输入页码直接跳转").FontSize(12).TextColor(c.Theme().TextMuted)
		}
		input := ui.TextInput(c, &s.Input).Label("页码").Placeholder("输入页码").MaxLength(7).AutoFocus().OnChange(func() { s.Error = "" })
		if s.Error != "" {
			ui.Text(c, s.Error).FontSize(12).TextColor(c.Theme().Danger)
		}
		ui.Row(c).Justify(ui.End).Children(func() { ui.PrimaryButton(c, "跳转").OnClick(a.applyPageJump) })
		if input.Submitted() {
			a.applyPageJump()
		}
	})
}

func (a *reader) readerNavigation(c *ui.Context, p palette) {
	if a.selected.ID <= 0 {
		ui.Row(c).Height(44).Padding(0, 14).BorderWidth(1, 0, 0, 0).BorderColor(c.Theme().Border).Children(func() { ui.Text(c, "选择一个话题，开始阅读").FontSize(11).TextColor(c.Theme().TextMuted) })
		return
	}
	ready := a.selected.ID > 0 && !a.loadingThread && !a.navigationBusy()
	page, total := a.visibleThreadPage(), a.threadPageLimit()
	ui.Column(c).Padding(5, 12).Gap(3).BorderWidth(1, 0, 0, 0).BorderColor(c.Theme().Border).Children(func() {
		ui.Row(c).Height(28).Gap(6).Children(func() {
			label := "选择一个话题，开始阅读"
			if a.selected.ID > 0 {
				if a.thread.TotalRows == 0 {
					label = fmt.Sprintf("第%d页 · 本页%d楼", page, len(a.rendered))
				} else {
					label = fmt.Sprintf("第%d页 · %d条回复", page, a.selected.Replies)
				}
			}
			ui.Text(c, label).FontSize(11).TextColor(c.Theme().TextMuted).SingleLine().Grow(1)
			ui.Button(c, "从头阅读").FontSize(11).Padding(3, 5).Disabled(!ready || page == 1 && a.scroll.Y <= 1 && a.threadErr == nil).OnClick(a.startReadingFromBeginning)
			label = "只看楼主"
			if a.onlyAuthor {
				label = "显示全部"
			}
			ui.Button(c, label).FontSize(11).Padding(3, 5).Disabled(!ready || a.thread.MainAuthorID == 0).OnClick(a.toggleAuthorFilter)
			iconButton(c, "thread-top", "top", "回到本页顶部", !ready || a.scroll.Y <= 1).OnClick(a.readingToTop)
			iconButton(c, "original", "link", "在浏览器打开原帖", a.selected.ID <= 0 || a.demo).OnClick(a.openOriginal)
		})
		ui.Row(c).Height(28).Gap(7).Justify(ui.Center).Children(func() {
			ui.Button(c, "首页").Label("回复首页").FontSize(11).Padding(4, 7).Disabled(!ready || page <= 1 || a.demo).OnClick(func() { a.navigateThreadPage(1) })
			iconButton(c, "thread-prev", "left", "上一页回复", !ready || page <= 1 || a.demo).OnClick(func() { a.navigateThreadPage(page - 1) })
			label := fmt.Sprintf("第%d页", page)
			if total > 0 {
				label = fmt.Sprintf("%d / %d 页", page, total)
			}
			ui.Button(c, label).Label("跳转回复页码").FontSize(11).Padding(4, 8).Tooltip("点击输入页码，Enter跳转").Disabled(!ready || a.demo).OnClick(func() { a.openPageJump(true) })
			hasNext := page < a.thread.TotalPages || total == 0 && a.continuationHasNext()
			iconButton(c, "thread-next", "right", "下一页回复", !ready || !hasNext || a.demo).OnClick(func() { a.navigateThreadPage(page + 1) })
			ui.Button(c, "末页").Label("回复末页").FontSize(11).Padding(4, 7).Disabled(!ready || total <= 0 || page >= total || a.demo).OnClick(func() { a.navigateThreadPage(total) })
		})
	})
}

func (a *reader) continuationFooter(c *ui.Context) {
	ui.Column(c).FillWidth().Center().Gap(10).Padding(18, 0).Children(func() {
		if a.continuation.Loading {
			ui.Text(c, "正在加载下一页…").FontSize(12).TextColor(c.Theme().TextMuted)
			return
		}
		if a.continuation.Error != nil {
			ui.Text(c, "下一页加载失败，已读内容仍保留。").FontSize(12).TextColor(c.Theme().TextMuted)
			ui.Button(c, "重试加载下一页").OnClick(a.appendNextPage)
			return
		}
		if !a.continuationHasNext() {
			ui.Text(c, "已经读到最后一页").FontSize(12).TextColor(c.Theme().TextMuted)
			return
		}
		if a.continuationLimited() {
			ui.Text(c, "本段已加载较多回复，下一段将从新页继续。").FontSize(12).TextColor(c.Theme().TextMuted)
			ui.Button(c, "继续下一段").Disabled(a.loadingThread).OnClick(func() { a.navigateThreadPage(a.continuationLast() + 1) })
			return
		}
		label := "加载下一页"
		if a.state.AutoNextPage {
			label = "继续向下滚动，自动加载下一页"
		}
		ui.Button(c, label).Label("加载下一页").Disabled(a.loadingThread).OnClick(a.appendNextPage)
	})
}
