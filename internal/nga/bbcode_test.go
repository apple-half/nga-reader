package nga

import (
	"strings"
	"testing"
)

func TestBBCodeNestedQuoteAndImage(t *testing.T) {
	input := "[quote]外层\n[quote][b]内层[/b][/quote][/quote]\n[img]./mon_202611/09/demo.png[/img]"
	got := Markdown(input, false)
	if !strings.Contains(got, "> > **内层**") || !strings.Contains(got, "https://img.nga.cn/attachments/mon_202611/09/demo.png") {
		t.Fatalf("bad nested conversion: %q", got)
	}
	plain := Markdown(input, true)
	if strings.Contains(plain, "![") || strings.Contains(plain, "https://img.") || !strings.Contains(plain, "图片") {
		t.Fatalf("text mode still fetches image: %q", plain)
	}
}

func TestBBCodeUnsafeLinks(t *testing.T) {
	input := "[url=javascript:alert(1)]点我[/url] [img]http://127.0.0.1/x[/img] [url=file:///C:/secret]本地[/url] [url=https://example.com/a]外链[/url]"
	got := Markdown(input, false)
	if strings.Contains(got, "javascript:") || strings.Contains(got, "127.0.0.1") || strings.Contains(got, "file:") || !strings.Contains(got, "点我") || !strings.Contains(got, "https://example.com/a") {
		t.Fatalf("unsafe rendering: %q", got)
	}
	for _, s := range []string{"http://localhost/x", "https://img.nga.178.com.evil.test/x", "https://img.nga.178.com:8443/x"} {
		if _, err := ImageURL(s); err == nil {
			t.Errorf("accepted image address %s", s)
		}
	}
}

func TestBBCodeDepthIsBounded(t *testing.T) {
	input := strings.Repeat("[quote]", 200) + strings.Repeat("一行文字\n", 200) + strings.Repeat("[/quote]", 200)
	if got := Markdown(input, true); len(got) > 64000 {
		t.Fatalf("nested quoting amplified a small post to %d bytes", len(got))
	}
}

func TestCodeFenceAvoidsDelimiterAmplification(t *testing.T) {
	content := strings.Repeat("`", 10000)
	got := Markdown("[code]"+content+"[/code]", true)
	if len(got) > len(content)+100 {
		t.Fatalf("code delimiter amplified a %d-byte block to %d bytes", len(content), len(got))
	}
	if !strings.Contains(got, content) {
		t.Fatal("code body was not preserved")
	}
}
