package nga

import (
	"strings"
	"testing"

	"golang.org/x/net/html"
)

func TestBBCodeNamedAndLegacyEmotesDoNotConsumeFollowingText(t *testing.T) {
	source := "前[s:ac:哭笑]中[s:1]后[s::1]尾[s:0:1]结束"
	got := MarkdownOnDemand(source)
	if strings.Count(got, "ngareader-emote://asset/") != 4 || !strings.Contains(got, "结束") || strings.Contains(got, "[表情]") {
		t.Fatalf("emotes/body lost: %q", got)
	}
	if !strings.Contains(got, "NGA 表情：哭笑") {
		t.Fatal("missing readable/accessibility name", got)
	}
	plain := Markdown(source, true)
	if strings.Contains(plain, "ngareader-emote:") || !strings.Contains(plain, "哭笑") || !strings.Contains(plain, "结束") {
		t.Fatal("plain-text fallback lost meaning", plain)
	}
}

func TestUnknownEmotesCodeAndStrikethroughRemainReadable(t *testing.T) {
	source := "[s:ac:未知表情]后文 [s:9999]后文2 [s]删除[/s] [code][s:ac:哭笑][/code]"
	got := MarkdownOnDemand(source)
	for _, text := range []string{"[s:ac:未知表情]后文", "[s:9999]后文2", "~~删除~~", "[s:ac:哭笑]"} {
		if !strings.Contains(got, text) {
			t.Fatalf("lost %q in %q", text, got)
		}
	}
	if strings.Contains(got, "ngareader-emote:") {
		t.Fatal("code/unknown tag turned into image", got)
	}
	quote := MarkdownOnDemand("[quote]引用[s:ac:哭笑][/quote]正常[img]./mon_202611/12/photo.jpg[/img]")
	if !strings.Contains(quote, "> 引用[](<ngareader-emote:") || !strings.Contains(quote, "展示图片") || strings.Contains(quote, "![") {
		t.Fatal("quote or attachment policy broken", quote)
	}
}

func TestOfficialSmileImageInHTMLStaysInlineAndAttachmentStillFolds(t *testing.T) {
	root, err := html.Parse(strings.NewReader(`<div>前<img class="smile_ac" src="https://img4.nga.cn/ngabbs/post/smile/ac15.png">后<img src="https://img.nga.cn/attachments/mon_202611/photo.jpg"></div>`))
	if err != nil {
		t.Fatal(err)
	}
	var div *html.Node
	visitHTML(root, func(n *html.Node) bool {
		if n.Type == html.ElementNode && n.Data == "div" {
			div = n
			return false
		}
		return true
	})
	if div == nil {
		t.Fatal("no body")
	}
	text := postText(div)
	if !strings.Contains(text, "前[s:ac:哭笑]后") {
		t.Fatal("HTML smile lost inline placement", text)
	}
	got := MarkdownOnDemand(text)
	if strings.Count(got, "ngareader-emote:") != 1 || strings.Count(got, "展示图片") != 1 {
		t.Fatal("HTML smile/attachment confused", got)
	}
}
