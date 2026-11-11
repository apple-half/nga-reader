package main

import (
	"fmt"
	"ngareader/internal/local"
)

func (a *reader) importFavoriteBoards() {
	if a.demo {
		a.returnToLive()
	}
	if !a.client.HasSession() {
		a.importAfterLogin = true
		a.notify("请先登录 NGA，完成后将导入收藏板块")
		a.openLogin()
		return
	}
	ctx, token := a.favoriteGate.Begin(a.ctx)
	a.loadingFavorites = true
	go func() {
		boards, err := a.client.FavoriteBoards(ctx)
		a.update(func() {
			if !a.favoriteGate.Accept(token) {
				return
			}
			a.loadingFavorites = false
			if err != nil {
				a.notify("导入收藏板块失败：" + errorMessage(err))
				return
			}
			incoming := make([]local.Board, 0, len(boards))
			skipped := 0
			for _, b := range boards {
				if b.CollectionID != 0 {
					skipped++
					continue
				}
				incoming = append(incoming, local.Board{ID: b.ID, Name: b.Name})
			}
			merged, added := local.MergeBoards(a.state.Boards, incoming)
			a.state.Boards = merged
			a.save()
			message := fmt.Sprintf("收藏板块已导入：新增 %d 个，已有 %d 个", added, len(incoming)-added)
			if len(boards) == 0 {
				message = "这个 NGA 账号尚未收藏板块"
			}
			if skipped > 0 {
				message += fmt.Sprintf("；%d 个合集暂不支持", skipped)
			}
			a.notify(message)
		})
	}()
}
