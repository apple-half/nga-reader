package main

import (
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/egoist/mygo/ui"
)

func legacyUpdateNotes(t *testing.T) string {
	t.Helper()
	raw, err := os.ReadFile("testdata/update-notes-0.1.8.md")
	if err != nil {
		t.Fatal(err)
	}
	return string(raw)
}

func requireUpdateControlVisible(t *testing.T, tt *ui.Tester, label string, width, height float32) {
	t.Helper()
	r, ok := tt.Find(label)
	if !ok || r.W <= 0 || r.H <= 0 || r.X < 0 || r.Y < 0 || r.X+r.W > width+1 || r.Y+r.H > height+1 {
		t.Fatalf("update control clipped or missing: %.60s %+v", label, r)
	}
}

func TestUpdateLegacyNotesAreFormattedAndFocused(t *testing.T) {
	for i, theme := range []string{"浅色", "深色", "NGA经典屎黄色"} {
		t.Run(theme, func(t *testing.T) {
			a, _, offer := updateTestReader(t)
			a.state.Theme = theme
			offer.info.Notes = legacyUpdateNotes(t)
			a.checkUpdates(true)
			tt := ui.NewTester(a.view, 980, 620)
			if tt.HasText("## 下载免安装版") || tt.HasText("**更新检查403**") || tt.HasText("| 你的电脑 |") {
				t.Fatal("full GitHub release Markdown shown as raw update text")
			}
			if !tt.HasText("本次更新") || !tt.HasText("更新检查403") {
				t.Fatal("actual changes are not visible")
			}
			if tt.HasText("NGAReader-0.1.8-Windows-x64-portable.zip") || tt.HasText("开发者附件说明") {
				t.Fatal("web download instructions crowd the in-app changelog")
			}
			for _, label := range []string{"完整更新说明", "下载并安装", "启动时自动检查更新", "关闭更新"} {
				requireUpdateControlVisible(t, tt, label, 980, 620)
			}
			writePreview(t, tt, fmt.Sprintf("update-dialog-theme-%d", i))
		})
	}
}

func TestUpdateStatusAndActionsStayVisibleWithLongNotes(t *testing.T) {
	for _, state := range []string{"installing", "restart", "error", "manual"} {
		t.Run(state, func(t *testing.T) {
			a, _, offer := updateTestReader(t)
			offer.info.Notes = strings.Repeat("很长的历史说明和下载内容。\n\n", 600)
			a.checkUpdates(true)
			status, action := "", "下载并安装"
			switch state {
			case "installing":
				a.updates.Installing = true
				a.updates.Message = "正在下载并校验更新，请不要退出软件。"
				a.updates.Total = 4 << 20
				a.updates.Downloaded = 2 << 20
				status, action = "2.0 / 4.0 MB", "正在安装…"
			case "restart":
				a.updates.ReadyToRestart = true
				a.updates.Message = "安装完成，请重启"
				status, action = a.updates.Message, "重启使用新版"
			case "error":
				a.updates.Error = "下载失败：" + strings.Repeat("暂时无法连接更新服务。", 45)
				status = a.updates.Error
			case "manual":
				offer.info.Installable = false
				offer.info.Reason = "当前程序目录不可写，请手动下载便携包。"
				status, action = offer.info.Reason, "手动下载"
			}
			tt := ui.NewTester(a.view, 760, 520)
			requireUpdateControlVisible(t, tt, status, 760, 520)
			requireUpdateControlVisible(t, tt, action, 760, 520)
			writePreview(t, tt, "update-dialog-"+state)
		})
	}
}
