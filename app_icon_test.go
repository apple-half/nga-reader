package main

import (
	"bytes"
	"github.com/egoist/mygo/ui"
	"image/png"
	"os"
	"testing"
)

func TestCanonicalAppIconUsesHighResolutionTransparentPNG(t *testing.T) {
	raw := windowIcon()
	img, err := png.Decode(bytes.NewReader(raw))
	if err != nil {
		t.Fatal(err)
	}
	b := img.Bounds()
	if b.Dx() < 512 || b.Dy() != b.Dx() {
		t.Fatalf("low-resolution or non-square app icon: %v", b)
	}
	_, _, _, alpha := img.At(0, 0).RGBA()
	if alpha != 0 {
		t.Fatal("icon outside tile is not transparent")
	}
	canonical, err := os.ReadFile("packaging/icon.png")
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(raw, canonical) {
		t.Fatal("window/tray icon differs from platform packaging source")
	}
}
func TestAppIconNativeSizePreview(t *testing.T) {
	img, err := png.Decode(bytes.NewReader(windowIcon()))
	if err != nil {
		t.Fatal(err)
	}
	bitmap := ui.NewBitmap(img)
	tt := ui.NewTester(func(c *ui.Context) {
		ui.Row(c).Fill().Padding(20).Gap(24).AlignItems(ui.Center).Background(ui.Hex("#202020")).Children(func() {
			for _, size := range []float32{16, 32, 64, 128} {
				ui.Column(c).Gap(10).AlignItems(ui.Center).Children(func() {
					ui.Image(c, bitmap).Size(size, size)
					ui.Textf(c, "%.0f px", size).TextColor(ui.Hex("#fff3d2"))
				})
			}
		})
	}, 480, 210)
	writePreview(t, tt, "office-logo-sizes")
}
