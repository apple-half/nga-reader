package main

import (
	"bytes"
	"image"
	"image/color"
	"image/gif"
	"math"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/markdown"
	"ngareader/internal/emotes"
	"ngareader/internal/nga"
)

func TestEmoteSizeKeepsAspectRatioAndBoundsAtAllFontSizes(t *testing.T) {
	for _, font := range []float32{12, 15, 22} {
		for _, e := range emotes.Entries() {
			info, ok := emotes.Info(e.File)
			if !ok {
				t.Fatal(e)
			}
			w, h := emoteSize(info.Width, info.Height, font)
			if w <= 0 || h <= 0 || w > min(56, font*3)+.01 || h > min(32, font*2)+.01 || w > float32(info.Width) || h > float32(info.Height) || math.Abs(float64(w/h-float32(info.Width)/float32(info.Height))) > .001 {
				t.Fatalf("invalid %s font%.0f size%.2fx%.2f", e.File, font, w, h)
			}
		}
	}
	w, h := emoteSize(160, 120, 15)
	if w > 60 || h > 45 || w <= 0 || math.Abs(float64(w/h-4.0/3.0)) > .001 {
		t.Fatal("large sticker not contained", w, h)
	}
	for _, dims := range [][2]int{{0, 50}, {-1, 50}, {50000, 10}} {
		if w, h := emoteSize(dims[0], dims[1], 15); w != 0 || h != 0 {
			t.Fatal("invalid dimensions accepted", dims)
		}
	}
}

func TestNativeEmoteObjectUsesSharedBitmapsAndNeverEnablesExternalImages(t *testing.T) {
	a := newReader(t.TempDir())
	defer a.cancel()
	for _, e := range emotes.Entries() {
		o, ok := a.md.LinkIcon(emotes.Destination(e), 15)
		if !ok || o.Draw == nil || o.Width <= 0 || o.Height+o.Depth <= 0 {
			t.Fatal("missing native emoticon", e)
		}
	}
	count := len(a.emoteView.images)
	for i := 0; i < 20; i++ {
		e, _ := emotes.Lookup("ac:哭笑")
		a.md.LinkIcon(emotes.Destination(e), 15)
	}
	if len(a.emoteView.images) != count || count != 265 {
		t.Fatal("emote cache duplicated or incomplete", count, len(a.emoteView.images))
	}
	for _, address := range []string{"https://example.com/tracker.png", "file:///C:/private.png", "ngareader-emote://asset/../../private.png"} {
		if _, ok := a.md.LinkIcon(address, 15); ok {
			t.Fatal("external image privilege", address)
		}
	}
	if a.md.LoadImage != nil {
		t.Fatal("ordinary images may autoload")
	}
	a.openLink("ngareader-emote://asset/ac15.png")
	if a.message != "" {
		t.Fatal("emote click attempted external navigation", a.message)
	}
}

func gifPatchFixture(t *testing.T, disposal byte) []byte {
	t.Helper()
	palette := color.Palette{color.NRGBA{}, color.NRGBA{R: 255, A: 255}, color.NRGBA{G: 255, A: 255}, color.NRGBA{B: 255, A: 255}}
	one := image.NewPaletted(image.Rect(0, 0, 3, 1), palette)
	one.Pix = []byte{1, 1, 1}
	two := image.NewPaletted(image.Rect(1, 0, 2, 1), palette)
	two.Pix = []byte{2}
	three := image.NewPaletted(image.Rect(2, 0, 3, 1), palette)
	three.Pix = []byte{3}
	g := gif.GIF{Image: []*image.Paletted{one, two, three}, Delay: []int{10, 20, 30}, Disposal: []byte{gif.DisposalNone, disposal, gif.DisposalNone}, Config: image.Config{ColorModel: palette, Width: 3, Height: 1}}
	var b bytes.Buffer
	if err := gif.EncodeAll(&b, &g); err != nil {
		t.Fatal(err)
	}
	return b.Bytes()
}

func TestGIFFramesCompositePatchesDisposeAndRespectTime(t *testing.T) {
	for _, disposal := range []byte{gif.DisposalNone, gif.DisposalBackground, gif.DisposalPrevious} {
		decoded, err := decodeEmote(gifPatchFixture(t, disposal))
		if err != nil {
			t.Fatal(err)
		}
		if len(decoded.frames) != 3 || decoded.cycle != 600*time.Millisecond {
			t.Fatal("timing lost", decoded.cycle)
		}
		for elapsed, want := range map[time.Duration]int{0: 0, 99 * time.Millisecond: 0, 100 * time.Millisecond: 1, 299 * time.Millisecond: 1, 300 * time.Millisecond: 2, 600 * time.Millisecond: 0, -1: 0} {
			if got := decoded.frameIndex(elapsed); got != want {
				t.Fatalf("%s frame%d want%d", elapsed, got, want)
			}
		}
		// Draw the final composed frame at its natural size and inspect pixels.
		tt := ui.NewTester(func(c *ui.Context) {
			ui.Box(c).Size(3, 1).Background(ui.Hex("#222222")).Children(func() { ui.Image(c, decoded.frames[2]).Size(3, 1) })
		}, 3, 1)
		mid := tt.Image().RGBAAt(1, 0)
		if disposal == gif.DisposalNone && mid.G < 200 {
			t.Fatal("previous patch lost", mid)
		}
		if disposal == gif.DisposalPrevious && mid.R < 200 {
			t.Fatal("previous canvas not restored", mid)
		}
		if disposal == gif.DisposalBackground && (mid.R != 34 || mid.G != 34 || mid.B != 34) {
			t.Fatal("background disposal not cleared", mid)
		}
	}
	if _, err := decodeEmote([]byte("<html>challenge</html>")); err == nil {
		t.Fatal("nonimage accepted")
	}
}

func TestEmoteAnimationOnlyRequestsFramesForVisibleAnimatedRows(t *testing.T) {
	viewport := ui.Rect{X: 0, Y: 100, W: 400, H: 200}
	for _, tc := range []struct {
		rows []readingRow
		want bool
	}{
		{[]readingRow{{Bounds: ui.Rect{X: 0, Y: 110, W: 400, H: 80}, Animated: false}}, false},
		{[]readingRow{{Bounds: ui.Rect{X: 0, Y: 500, W: 400, H: 80}, Animated: true}}, false},
		{[]readingRow{{Bounds: ui.Rect{X: 0, Y: 110, W: 400, H: 80}, Animated: true}}, true},
		{nil, false},
	} {
		if got := animatedEmoteVisible(viewport, tc.rows); got != tc.want {
			t.Fatalf("animation visibility=%t want%t", got, tc.want)
		}
	}
	if !containsAnimatedEmote(nga.MarkdownOnDemand("[s:5]")) || containsAnimatedEmote(nga.MarkdownOnDemand("[s:ac:哭笑]")) || containsAnimatedEmote(nga.MarkdownOnDemand("[code][s:5][/code]")) {
		t.Fatal("static/code/animated classification wrong")
	}
}

func TestEmoteGIFDecodeRejectsFrameAndPixelBudgets(t *testing.T) {
	palette := color.Palette{color.Transparent, color.White}
	for _, tc := range []struct{ width, height, frames int }{{1, 1, 129}, {256, 256, 65}, {257, 1, 1}} {
		g := gif.GIF{Config: image.Config{ColorModel: palette, Width: tc.width, Height: tc.height}}
		for i := 0; i < tc.frames; i++ {
			g.Image = append(g.Image, image.NewPaletted(image.Rect(0, 0, 1, 1), palette))
			g.Delay = append(g.Delay, 1)
		}
		var data bytes.Buffer
		if err := gif.EncodeAll(&data, &g); err != nil {
			t.Fatal(err)
		}
		if _, err := decodeEmote(data.Bytes()); err == nil {
			t.Fatalf("unbounded GIF accepted %+v", tc)
		}
	}
	if _, err := decodeEmote(bytes.Repeat([]byte{'x'}, (256<<10)+1)); err == nil {
		t.Fatal("oversized input accepted")
	}
	valid := gifPatchFixture(t, gif.DisposalNone)
	for _, cut := range []int{0, 5, 13, len(valid) - 2} {
		if _, err := decodeEmote(valid[:cut]); err == nil {
			t.Fatal("truncated GIF accepted", cut)
		}
	}
}

func TestNativeEmoteParagraphWrapsInThemesWithoutLargeImages(t *testing.T) {
	for _, theme := range []string{"深色", "浅色", "NGA经典屎黄色"} {
		for _, font := range []int{12, 15, 22} {
			a := newReader(t.TempDir())
			a.useDemo()
			a.state.Theme = theme
			a.state.FontSize = font
			a.thread.Posts = []nga.Post{{Floor: 0, Author: "表情示例", Content: "同行测试：哭笑[s:ac:哭笑]后面继续文字。[s:a2:怒][s:ng:呲牙笑][s:pg:吃瓜]\n\n[quote]引用里也有[s:ac:哭笑][/quote]\n\n默认微笑[s:1]和动态脸红[s:5]，未知[s:ac:未来新表情]仍可读。\n\n[img]./mon_202611/12/photo.jpg[/img]"}}
			a.resetMarkdown()
			draws := 0
			maxHeight := float32(0)
			inline := a.md.LinkIcon
			a.md.LinkIcon = func(dest string, size float32) (markdown.InlineObject, bool) {
				o, ok := inline(dest, size)
				if ok {
					draw := o.Draw
					o.Draw = func(p *ui.Painter, r ui.Rect) { draws++; maxHeight = max(maxHeight, r.H); draw(p, r) }
				}
				return o, ok
			}
			tt := ui.NewTester(a.view, 980, 620)
			if draws == 0 || maxHeight > 32.1 || a.md.LoadImage != nil || !tt.HasText("展示图片") {
				t.Fatalf("inline sizing/attachment rule failed: draws%d h%.1f", draws, maxHeight)
			}
			if font == 15 {
				writePreview(t, tt, "emoticons-"+theme)
			}
			a.cancel()
		}
	}
}

func TestMonochromeEmoteRemainsReadableInDarkTheme(t *testing.T) {
	a := newReader(t.TempDir())
	defer a.cancel()
	a.state.Theme = "深色"
	var box ui.Rect
	inline := a.md.LinkIcon
	a.md.LinkIcon = func(dest string, size float32) (markdown.InlineObject, bool) {
		o, ok := inline(dest, size)
		if ok {
			draw := o.Draw
			o.Draw = func(p *ui.Painter, r ui.Rect) { box = r; draw(p, r) }
		}
		return o, ok
	}
	tt := ui.NewTester(func(c *ui.Context) {
		a.theme(c)
		ui.Column(c).Fill().Background(c.Theme().Background).Padding(12).Children(func() { markdown.View(c, nga.MarkdownOnDemand("前[s:ac:哭笑]后"), a.md) })
	}, 220, 90)
	bright, total := 0, 0
	for y := int(box.Y) + 3; y < int(box.Y+box.H)-3; y++ {
		for x := int(box.X) + 3; x < int(box.X+box.W)-3; x++ {
			pixel := tt.Image().RGBAAt(x, y)
			total++
			if pixel.R > 180 && pixel.G > 180 && pixel.B > 180 {
				bright++
			}
		}
	}
	if total == 0 || float64(bright)/float64(total) < .15 {
		t.Fatalf("black transparent line art disappears against dark background: bright%d/%d", bright, total)
	}
}
