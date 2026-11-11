package main

import (
	"context"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"ngareader/internal/local"
	"ngareader/internal/nga"
)

type postingAPI interface {
	SessionUID() int64
	PreparePost(context.Context, nga.PostTarget) (nga.PostPreparation, error)
	PublishPost(context.Context, nga.PostPreparation, string, string) (nga.PublishResult, error)
	TopicCategories(context.Context, int64) ([]string, error)
}

type postIntent struct {
	target nga.PostTarget
	label  string
}

type composerState struct {
	Open, Preparing, Ready, Sending bool
	Draft                           local.Draft
	Prepared                        nga.PostPreparation
	Categories                      []string
	CategoryChoice, BoardChoice     string
	Error, SaveError, Info          string
	RetryAcknowledged               bool
}

func (a *reader) postClient() postingAPI {
	if a.posting != nil {
		return a.posting
	}
	return a.client
}
func (a *reader) postTask(fn func()) {
	if a.runPosting != nil {
		a.runPosting(fn)
	} else {
		go fn()
	}
}

func (a *reader) openComposer(target nga.PostTarget, label string) {
	if a.updates.Installing || a.updates.Restarting || a.closing {
		a.notify("正在安装更新，请等待完成后再发帖")
		return
	}
	if target.Validate() != nil {
		a.notify("无法确定发帖目标，请先打开板块或帖子")
		return
	}
	if a.compose.Sending {
		a.notify("正在发送，请等待论坛返回结果")
		return
	}
	if a.compose.Open && !a.closeComposer() {
		return
	}
	uid := a.postClient().SessionUID()
	if !a.demo && uid == 0 {
		a.postAfterLogin = &postIntent{target: target, label: label}
		a.notify("请先登录，完成后会返回编辑器")
		a.openLogin()
		return
	}
	if a.demo {
		uid = 0
	}
	label = string([]rune(label)[:min(256, utf8.RuneCountInString(label))])
	draft := local.Draft{UID: uid, Target: target, Label: label}
	if !a.demo {
		for _, saved := range a.drafts {
			if saved.Key() == draft.Key() {
				draft = saved
				break
			}
		}
	}
	a.compose = composerState{Open: true, Draft: draft, CategoryChoice: "不加分类", BoardChoice: a.composeBoardLabel(draft.Target.FID)}
	if draft.Category != "" {
		a.compose.CategoryChoice = draft.Category
	}
	if draft.Uncertain {
		a.compose.Info = "上次发送结果待确认。请先检查原帖或板块，确认没有发表后再解除保护。"
	}
	a.prepareComposer()
}

func (a *reader) composeBoardLabel(fid int64) string {
	for _, board := range a.state.Boards {
		if board.ID == fid {
			return fmt.Sprintf("%s · %d", board.Name, board.ID)
		}
	}
	return fmt.Sprintf("板块 %d", fid)
}

func (a *reader) prepareComposer() {
	if !a.compose.Open || a.compose.Sending {
		return
	}
	if a.demo {
		a.compose.Ready = true
		a.compose.Prepared = nga.PostPreparation{Target: a.compose.Draft.Target}
		if a.compose.Draft.Target.Action == "quote" && a.compose.Draft.Content == "" {
			a.compose.Draft.Content = "[quote]这里是离线演示的引用内容。[/quote]\n"
		}
		a.compose.Categories = []string{"[讨论]", "[求助]"}
		a.compose.Info = "演示模式：可以体验编辑器，内容不会发送或保存。"
		return
	}
	api := a.postClient()
	if api.SessionUID() != a.compose.Draft.UID {
		a.compose.Error = "账号已变化，请保存并关闭后重新打开"
		a.compose.Ready = false
		return
	}
	ctx, token := a.composeGate.Begin(a.ctx)
	target, uid := a.compose.Draft.Target, a.compose.Draft.UID
	a.compose.Preparing = true
	a.compose.Ready = false
	a.compose.Error = ""
	a.postTask(func() {
		prepared, err := api.PreparePost(ctx, target)
		var categories []string
		var categoryErr error
		if err == nil && target.Action == "new" {
			categories, categoryErr = api.TopicCategories(ctx, target.FID)
		}
		a.update(func() {
			if !a.composeGate.Accept(token) || !a.compose.Open {
				return
			}
			a.compose.Preparing = false
			if api.SessionUID() != uid || a.compose.Draft.UID != uid {
				a.compose.Error = "账号已变化，请重新打开编辑器"
				return
			}
			if err != nil {
				a.compose.Error = err.Error()
				return
			}
			if prepared.UID != uid || prepared.Target.Action != target.Action || prepared.Target.TID != target.TID || prepared.Target.PID != target.PID || (target.Action == "new" && prepared.Target.FID != target.FID) {
				a.compose.Error = "论坛准备信息与当前目标不一致，已停止"
				return
			}
			a.compose.Prepared = prepared
			a.compose.Draft.Target = prepared.Target
			a.compose.Ready = true
			a.compose.BoardChoice = a.composeBoardLabel(prepared.Target.FID)
			if a.compose.Draft.Content == "" && prepared.Content != "" {
				a.compose.Draft.Content = prepared.Content
			}
			if target.Action == "new" && a.compose.Draft.Subject == "" {
				a.compose.Draft.Subject = prepared.Subject
			}
			a.compose.Categories = categories
			if categoryErr != nil {
				a.compose.Info = "分类暂时读取失败，可在标题中手动填写板块要求的分类前缀。"
			}
			a.saveComposer()
		})
	})
}

func (a *reader) saveComposer() bool {
	if !a.compose.Open || a.demo {
		return true
	}
	if a.draftsError != nil {
		a.compose.SaveError = a.draftsError.Error()
		return false
	}
	if utf8.RuneCountInString(a.compose.Draft.Subject) > nga.MaxPostSubjectRunes || utf8.RuneCountInString(a.compose.Draft.Content) > nga.MaxPostContentRunes {
		a.compose.SaveError = "草稿超过字数上限，请删减后保存；当前内容仍保留在编辑器中。"
		return false
	}
	a.compose.Draft.UpdatedAt = time.Now()
	next := append([]local.Draft(nil), a.drafts...)
	found := false
	for i := range next {
		if next[i].Key() == a.compose.Draft.Key() {
			next[i] = a.compose.Draft
			found = true
			break
		}
	}
	if !found {
		next = append(next, a.compose.Draft)
	}
	if err := local.SaveDrafts(a.dir, next); err != nil {
		a.compose.SaveError = "草稿未能保存到本机：" + err.Error()
		return false
	}
	a.drafts = next
	a.compose.SaveError = ""
	return true
}

func (a *reader) closeComposer() bool {
	if a.compose.Sending {
		a.compose.Open = true
		a.compose.Error = "正在发送，收到结果后才能关闭编辑器"
		return false
	}
	if !a.saveComposer() {
		a.compose.Open = true
		return false
	}
	a.composeGate.Begin(a.ctx)
	a.compose.Open = false
	return true
}

func (a *reader) allowSessionChange() bool {
	if a.updates.Installing || a.updates.Restarting || a.closing {
		a.notify("正在安装更新，请等待完成后再切换账号")
		return false
	}
	if a.compose.Sending {
		a.notify("正在发送，请等待论坛返回结果后再切换账号")
		return false
	}
	return !a.compose.Open || a.closeComposer()
}

func (a *reader) resumeComposerAfterLogin() {
	if a.postAfterLogin == nil {
		return
	}
	intent := *a.postAfterLogin
	a.postAfterLogin = nil
	a.openComposer(intent.target, intent.label)
}

func (a *reader) loginForComposer() {
	if a.compose.Sending {
		return
	}
	a.postAfterLogin = &postIntent{target: a.compose.Draft.Target, label: a.compose.Draft.Label}
	a.openLogin()
}

func (a *reader) acknowledgeComposer() {
	if a.compose.Sending || !a.compose.RetryAcknowledged || !a.compose.Draft.Uncertain {
		return
	}
	a.compose.Draft.AcknowledgeUncertain()
	if !a.saveComposer() {
		a.compose.Draft.Uncertain = true
		return
	}
	a.compose.RetryAcknowledged = false
	a.compose.Error = ""
	a.compose.Info = "已解除重复发送保护；发送前仍可修改内容。"
	a.prepareComposer()
}

func (a *reader) removeComposerDraft() bool {
	if a.demo {
		return true
	}
	if a.draftsError != nil {
		a.compose.SaveError = a.draftsError.Error()
		return false
	}
	next := make([]local.Draft, 0, len(a.drafts))
	for _, d := range a.drafts {
		if d.Key() != a.compose.Draft.Key() {
			next = append(next, d)
		}
	}
	if err := local.SaveDrafts(a.dir, next); err != nil {
		a.compose.SaveError = "草稿未能清理：" + err.Error()
		return false
	}
	a.drafts = next
	return true
}

func (a *reader) discardComposer() {
	if a.compose.Sending || a.compose.Draft.Uncertain {
		return
	}
	if a.removeComposerDraft() {
		a.composeGate.Begin(a.ctx)
		a.compose.Open = false
	}
}

func (a *reader) confirmComposerPublished() {
	if !a.compose.Draft.Uncertain || a.compose.Sending {
		return
	}
	if a.removeComposerDraft() {
		a.composeGate.Begin(a.ctx)
		a.compose.Open = false
		a.notify("已清理这份已发表草稿")
	}
}

func (a *reader) sendComposer() {
	if a.updates.Installing || a.updates.Restarting || a.closing {
		return
	}
	if a.demo || !a.compose.Open || a.compose.Sending || a.compose.Preparing || !a.compose.Ready {
		return
	}
	if a.compose.Prepared.Target != a.compose.Draft.Target {
		a.compose.Error = "目标已变化，请重新准备"
		return
	}
	if err := a.compose.Draft.BeginSend(a.postClient().SessionUID()); err != nil {
		a.compose.Error = err.Error()
		return
	}
	// This write is mandatory before POST, including before the worker starts.
	if !a.saveComposer() {
		a.compose.Draft.Uncertain = false
		a.compose.Error = "未能保存发送记录，已停止发送"
		return
	}
	a.compose.Sending = true
	a.compose.Error = ""
	a.compose.Info = "正在发送，请等待结果。"
	api, prepared, sent := a.postClient(), a.compose.Prepared, a.compose.Draft
	a.postTask(func() {
		result, err := api.PublishPost(a.ctx, prepared, sent.PostSubject(), sent.Content)
		a.update(func() {
			a.compose.Sending = false
			if a.compose.Draft.EndSend(err) {
				a.compose.Ready = false
				if !a.removeComposerDraft() {
					a.compose.Info = "论坛已确认发表，但本机草稿清理失败。请勿再次发送。"
					a.compose.Draft.Uncertain = true
					return
				}
				a.compose.Open = false
				a.composeGate.Begin(a.ctx)
				message := "已发表，正在刷新帖子"
				if result.UnderReview {
					message = "已提交论坛审核，审核通过后才会显示"
				}
				a.notify(message)
				if a.win == nil || a.win.IsDestroyed() {
					return
				}
				if sent.Target.Action == "new" {
					a.selectBoard(local.Board{ID: sent.Target.FID, Name: strings.Split(a.compose.BoardChoice, " · ")[0]})
					if result.TID > 0 && !result.UnderReview {
						a.selectTopic(nga.Topic{ID: result.TID, FID: sent.Target.FID, Title: sent.PostSubject()})
					}
				} else {
					a.onlyAuthor = false
					a.loadThread(max(1, a.thread.TotalPages))
				}
				return
			}
			a.compose.Error = err.Error()
			a.compose.Info = "内容已保留。"
			if a.compose.Draft.Uncertain {
				a.compose.Info = "结果待确认：请先检查原帖或板块，避免重复发表。"
			} else {
				a.compose.Ready = false
			}
			a.saveComposer()
		})
	})
}

func (a *reader) checkComposerDestination() {
	if a.compose.Open && !a.compose.Sending && !a.demo && a.compose.Draft.UID != a.postClient().SessionUID() {
		if a.closeComposer() {
			a.notify("账号已变化，原账号草稿已保留")
		}
	}
}
