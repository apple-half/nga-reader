package main

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
)

func TestAutomaticLoginMonitorStopsAfterSuccessOrClose(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	checks := 0
	pollLogin(ctx, time.Millisecond, func() bool { checks++; return false })
	if checks != 0 {
		t.Fatal("closed login still reads the WebView session")
	}
	ctx, cancel = context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	pollLogin(ctx, time.Millisecond, func() bool { checks++; return checks == 3 })
	if checks != 3 || ctx.Err() != nil {
		t.Fatalf("monitor did not finish when the session became ready: checks=%d err=%v", checks, ctx.Err())
	}
}

func TestIncompleteLoginPreservesExistingSession(t *testing.T) {
	a := newReader(t.TempDir())
	defer a.cancel()
	if err := a.client.ImportCookieHeader("ngaPassportUid=123; ngaPassportCid=existing-test-session"); err != nil {
		t.Fatal(err)
	}
	if err := a.adoptLoginCookies([]*http.Cookie{{Name: "ngaPassportUid", Value: "456", Domain: "bbs.nga.cn", Path: "/", Secure: true}}); err == nil {
		t.Fatal("incomplete login was accepted")
	}
	for _, c := range a.client.Cookies() {
		if c.Name == "ngaPassportUid" && c.Value != "123" {
			t.Fatal("failed login replaced the active account")
		}
	}
	if !a.client.HasSession() {
		t.Fatal("failed login erased the active session")
	}
}

func TestLoginCanFinishInsideLoginWindow(t *testing.T) {
	a := newReader(t.TempDir())
	defer a.cancel()
	tt := ui.NewTester(a.loginView, 940, 820)
	if err := tt.Click("完成登录并返回"); err != nil {
		t.Fatal("login window has no completion action:", err)
	}
	if !tt.HasText("请先打开官方登录窗口") {
		t.Fatalf("completion result is hidden on the main window: %q", tt.Texts())
	}
	writePreview(t, tt, "login-controls")
}
