package main

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/gif"
	"image/png"
	"math"
	"strings"
	"time"

	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/markdown"
	"ngareader/internal/emotes"
)

const emoteFrameInterval = 125 * time.Millisecond
const maxEmoteFramePixels = 4 << 20

type emoteImage struct {
	frames        []*ui.Bitmap
	delays        []time.Duration
	cycle         time.Duration
	lightBackdrop bool
}
type emoticonRenderer struct {
	images  map[string]*emoteImage
	started time.Time
	now     func() time.Time
}

func emoteSize(width, height int, font float32) (float32, float32) {
	if width < 1 || height < 1 || width > 256 || height > 256 || font <= 0 || math.IsNaN(float64(font)) || math.IsInf(float64(font), 0) {
		return 0, 0
	}
	maxW, maxH := min(56, font*3), min(32, font*2)
	if width > 100 || height > 100 {
		maxW, maxH = min(72, font*4), min(48, font*3)
	}
	scale := min(float32(1), maxW/float32(width), maxH/float32(height))
	return float32(width) * scale, float32(height) * scale
}

func (r *emoticonRenderer) clock() time.Time {
	if r.now != nil {
		return r.now()
	}
	return time.Now()
}
func (r *emoticonRenderer) image(file string) (*emoteImage, error) {
	if r.images == nil {
		r.images = map[string]*emoteImage{}
	}
	if img, ok := r.images[file]; ok {
		if img == nil {
			return nil, fmt.Errorf("内置表情无法解码")
		}
		return img, nil
	}
	_, raw, ok := emotes.Asset(file)
	if !ok {
		return nil, fmt.Errorf("未知表情")
	}
	img, err := decodeEmote(raw)
	r.images[file] = img
	return img, err
}

func (r *emoticonRenderer) inline(address string, font float32) (markdown.InlineObject, bool) {
	e, ok := emotes.FromDestination(address)
	if !ok {
		return markdown.InlineObject{}, false
	}
	info, ok := emotes.Info(e.File)
	if !ok {
		return markdown.InlineObject{}, false
	}
	w, h := emoteSize(info.Width, info.Height, font)
	if w <= 0 {
		return markdown.InlineObject{}, false
	}
	img, err := r.image(e.File)
	if r.started.IsZero() {
		r.started = r.clock()
	}
	depth := min(h*.2, font*.2)
	return markdown.InlineObject{Width: w, Height: h - depth, Depth: depth, Draw: func(p *ui.Painter, box ui.Rect) {
		if err != nil || img == nil {
			p.Text(box.X, box.Y+(box.H-10)/2, "表情", 10, p.Theme().TextMuted)
			return
		}
		if p.Theme().Dark && img.lightBackdrop {
			p.Fill(box, ui.Hex("#fff6e4"), 3)
		}
		p.Image(img.frames[img.frameIndex(r.clock().Sub(r.started))], box, ui.Contain)
	}}, true
}

func (img *emoteImage) frameIndex(elapsed time.Duration) int {
	if len(img.frames) < 2 || img.cycle <= 0 || elapsed < 0 {
		return 0
	}
	at := elapsed % img.cycle
	for i, delay := range img.delays {
		if at < delay {
			return i
		}
		at -= delay
	}
	return 0
}

func (r *emoticonRenderer) nextFrameDelay() time.Duration {
	soon := time.Second
	elapsed := max(time.Duration(0), r.clock().Sub(r.started))
	for _, img := range r.images {
		if img == nil || len(img.frames) < 2 || img.cycle <= 0 {
			continue
		}
		at := elapsed % img.cycle
		for _, delay := range img.delays {
			if at < delay {
				soon = min(soon, delay-at)
				break
			}
			at -= delay
		}
	}
	return max(emoteFrameInterval, soon)
}

func decodeEmote(raw []byte) (*emoteImage, error) {
	bad := func() (*emoteImage, error) { return nil, fmt.Errorf("内置表情格式或尺寸无效") }
	if len(raw) == 0 || len(raw) > 256<<10 {
		return bad()
	}
	cfg, format, err := image.DecodeConfig(bytes.NewReader(raw))
	if err != nil || cfg.Width < 1 || cfg.Height < 1 || cfg.Width > 256 || cfg.Height > 256 {
		return bad()
	}
	if format == "png" {
		img, err := png.Decode(bytes.NewReader(raw))
		if err != nil {
			return nil, err
		}
		return &emoteImage{frames: []*ui.Bitmap{ui.NewBitmap(img)}, lightBackdrop: emoteNeedsBackdrop(img)}, nil
	}
	if format != "gif" {
		return bad()
	}
	if err := checkEmoteGIF(raw, cfg.Width, cfg.Height); err != nil {
		return nil, err
	}
	g, err := gif.DecodeAll(bytes.NewReader(raw))
	if err != nil {
		return nil, err
	}
	result := &emoteImage{}
	canvas := image.NewNRGBA(image.Rect(0, 0, cfg.Width, cfg.Height))
	clone := func() *image.NRGBA {
		copy := image.NewNRGBA(canvas.Bounds())
		copy.Pix = bytes.Clone(canvas.Pix)
		return copy
	}
	for i, frame := range g.Image {
		disposal := byte(gif.DisposalNone)
		if i < len(g.Disposal) {
			disposal = g.Disposal[i]
		}
		var prior *image.NRGBA
		if disposal == gif.DisposalPrevious {
			prior = clone()
		}
		draw.Draw(canvas, frame.Bounds(), frame, frame.Bounds().Min, draw.Over)
		result.frames = append(result.frames, ui.NewBitmap(clone()))
		delay := time.Duration(g.Delay[i]) * 10 * time.Millisecond
		if delay <= 0 {
			delay = 100 * time.Millisecond
		}
		result.delays = append(result.delays, delay)
		result.cycle += delay
		switch disposal {
		case gif.DisposalBackground:
			draw.Draw(canvas, frame.Bounds(), image.Transparent, image.Point{}, draw.Src)
		case gif.DisposalPrevious:
			canvas = prior
		}
	}
	return result, nil
}

// Preserve the original artwork. Transparent black line art needs a small
// neutral backing in the dark theme; opaque and colored faces keep their look.
func emoteNeedsBackdrop(img image.Image) bool {
	visible, dark, transparent := 0, 0, 0
	for y := img.Bounds().Min.Y; y < img.Bounds().Max.Y; y++ {
		for x := img.Bounds().Min.X; x < img.Bounds().Max.X; x++ {
			pixel := color.NRGBAModel.Convert(img.At(x, y)).(color.NRGBA)
			if pixel.A < 32 {
				transparent++
				continue
			}
			visible++
			if int(pixel.R)+int(pixel.G)+int(pixel.B) < 270 && int(max(pixel.R, pixel.G, pixel.B))-int(min(pixel.R, pixel.G, pixel.B)) < 60 {
				dark++
			}
		}
	}
	return visible > 0 && dark*2 > visible && transparent*8 > img.Bounds().Dx()*img.Bounds().Dy()
}

// Inspect descriptors before gif.DecodeAll can allocate frames. Both compressed
// input and the total decoded canvas budget are bounded for built-in assets.
func checkEmoteGIF(raw []byte, width, height int) error {
	bad := fmt.Errorf("表情GIF超过帧数／像素预算或数据损坏")
	if len(raw) < 13 {
		return bad
	}
	offset := 13
	if raw[10]&0x80 != 0 {
		offset += 3 * (1 << ((raw[10] & 7) + 1))
	}
	blocks := func() bool {
		for offset < len(raw) {
			n := int(raw[offset])
			offset++
			if n == 0 {
				return true
			}
			if n > len(raw)-offset {
				return false
			}
			offset += n
		}
		return false
	}
	frames := 0
	for offset < len(raw) {
		kind := raw[offset]
		offset++
		switch kind {
		case 0x3b:
			if frames > 0 {
				return nil
			}
			return bad
		case 0x21:
			if offset >= len(raw) {
				return bad
			}
			offset++
			if !blocks() {
				return bad
			}
		case 0x2c:
			if len(raw)-offset < 9 {
				return bad
			}
			x, y := int(binary.LittleEndian.Uint16(raw[offset:])), int(binary.LittleEndian.Uint16(raw[offset+2:]))
			w, h := int(binary.LittleEndian.Uint16(raw[offset+4:])), int(binary.LittleEndian.Uint16(raw[offset+6:]))
			packed := raw[offset+8]
			offset += 9
			frames++
			if frames > 128 || width*height*frames > maxEmoteFramePixels || w < 1 || h < 1 || x+w > width || y+h > height {
				return bad
			}
			if packed&0x80 != 0 {
				offset += 3 * (1 << ((packed & 7) + 1))
			}
			if offset >= len(raw) {
				return bad
			}
			offset++ // LZW minimum code size.
			if !blocks() {
				return bad
			}
		default:
			return bad
		}
	}
	return bad
}

func containsAnimatedEmote(source string) bool {
	const prefix = "ngareader-emote://asset/"
	for {
		_, tail, ok := strings.Cut(source, prefix)
		if !ok {
			return false
		}
		end := strings.IndexAny(tail, ">)\" \t\r\n")
		if end < 0 {
			return false
		}
		if info, ok := emotes.Info(tail[:end]); ok && info.Frames > 1 {
			return true
		}
		source = tail[end:]
	}
}

func animatedEmoteVisible(viewport ui.Rect, rows []readingRow) bool {
	if viewport.W <= 0 || viewport.H <= 0 {
		return false
	}
	for _, row := range rows {
		box := row.Bounds
		if row.Animated && box.W > 0 && box.H > 0 && box.X < viewport.X+viewport.W && box.X+box.W > viewport.X && box.Y < viewport.Y+viewport.H && box.Y+box.H > viewport.Y {
			return true
		}
	}
	return false
}
