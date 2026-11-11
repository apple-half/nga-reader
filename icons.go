package main

import (
	"bytes"
	"image"
	"image/color"
	"image/png"

	"github.com/egoist/mygo/ui"
)

func svg(path string) *ui.SVG {
	return ui.MustParseSVG([]byte(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.6" stroke-linecap="round" stroke-linejoin="round">` + path + `</svg>`))
}

var icons = map[string]*ui.SVG{
	"app":      svg(`<rect x="3" y="4" width="18" height="16" rx="2"/><path d="M3 9h18M9 9v11"/>`),
	"topics":   svg(`<path d="M21 11a7 7 0 0 1-7 7H8l-5 3V9a7 7 0 0 1 7-7h4a7 7 0 0 1 7 7Z"/>`),
	"bookmark": svg(`<path d="M6 21V5a2 2 0 0 1 2-2h8a2 2 0 0 1 2 2v16l-6-4Z"/>`),
	"clock":    svg(`<circle cx="12" cy="12" r="9"/><path d="M12 7v6l4 2"/>`),
	"help":     svg(`<circle cx="12" cy="12" r="9"/><path d="M9.7 8.8a2.5 2.5 0 0 1 4.8 1c0 1.8-2.5 2-2.5 4M12 17h.01"/>`),
	"settings": svg(`<path d="M4 6h16M4 12h16M4 18h16"/><circle cx="8" cy="6" r="2" fill="currentColor"/><circle cx="16" cy="12" r="2" fill="currentColor"/><circle cx="10" cy="18" r="2" fill="currentColor"/>`),
	"plus":     svg(`<path d="M12 5v14M5 12h14"/>`),
	"refresh":  svg(`<path d="M20 7v5h-5M4 17v-5h5"/><path d="M6 6a8 8 0 0 1 13 4M18 18a8 8 0 0 1-13-4"/>`),
	"left":     svg(`<path d="m14 6-6 6 6 6"/>`), "right": svg(`<path d="m10 6 6 6-6 6"/>`),
	"link":  svg(`<path d="m10 13 4-4M9 7l2-2a5 5 0 0 1 7 7l-2 2M15 17l-2 2a5 5 0 0 1-7-7l2-2"/>`),
	"focus": svg(`<path d="M8 3H3v5M16 3h5v5M3 16v5h5M21 16v5h-5"/>`),
	"text":  svg(`<path d="M4 5h16M12 5v15M8 20h8"/>`),
	"user":  svg(`<circle cx="12" cy="8" r="4"/><path d="M4 21v-2a8 8 0 0 1 16 0v2"/>`),
}

func windowIcon() []byte {
	img := image.NewNRGBA(image.Rect(0, 0, 64, 64))
	bg := color.NRGBA{R: 45, G: 49, B: 43, A: 255}
	fg := color.NRGBA{R: 215, G: 226, B: 207, A: 255}
	for y := range 64 {
		for x := range 64 {
			if x >= 4 && x < 60 && y >= 4 && y < 60 {
				img.SetNRGBA(x, y, bg)
			}
			if (x >= 15 && x <= 48 && (y == 16 || y == 17 || y == 27 || y == 28 || y == 47 || y == 48)) || (y >= 16 && y <= 48 && (x == 15 || x == 16 || x == 47 || x == 48)) || (x >= 26 && x <= 27 && y >= 28 && y <= 47) {
				img.SetNRGBA(x, y, fg)
			}
		}
	}
	var b bytes.Buffer
	_ = png.Encode(&b, img)
	return b.Bytes()
}
