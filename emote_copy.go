package main

import (
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/egoist/mygo/transfer"
	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/markdown"
	"github.com/egoist/mygo/ui/markdown/parser"
	"ngareader/internal/emotes"
)

type emoteCopyParagraph struct {
	Objects map[int]string
	Before  string
}

func (p emoteCopyParagraph) selected(part ui.SelectedText) string {
	runes := []rune(part.Text)
	lo := min(len(runes), max(0, part.Start))
	hi := min(len(runes), max(lo, part.End))
	var b strings.Builder
	for i := lo; i < hi; i++ {
		if code, ok := p.Objects[i]; ok && runes[i] == '\ufffc' {
			b.WriteString(code)
		} else {
			b.WriteRune(runes[i])
		}
	}
	return b.String()
}

type emoteCopyState struct {
	Source        string
	HasEmote      bool
	Paragraphs    map[uint64]emoteCopyParagraph
	NextSeparator string
}
type emoteCopyKey struct{}

func (s *emoteCopyState) copy(parts []ui.SelectedText) transfer.Data {
	var b strings.Builder
	for i, p := range parts {
		if i > 0 {
			separator := s.Paragraphs[p.ID].Before
			if separator == "" {
				separator = "\n\n"
			}
			b.WriteString(separator)
		}
		b.WriteString(s.Paragraphs[p.ID].selected(p))
	}
	return transfer.TextData(b.String())
}

// Keep default Markdown selection for ordinary documents. Emote documents use
// public native inline objects with explicit source mappings, so selecting an
// image has a meaningful NGA code rather than an empty decoration or U+FFFC.
func (a *reader) bodyMarkdown(c *ui.Context, source string) ui.Element {
	root := ui.Column(c).FillWidth()
	state := ui.Local(root, emoteCopyKey{}, func() emoteCopyState { return emoteCopyState{Paragraphs: map[uint64]emoteCopyParagraph{}} })
	if state.Source != source {
		state.Source = source
		state.HasEmote = hasRenderedEmote(source, false)
	}
	if !state.HasEmote {
		return root.Children(func() { markdown.View(c, source, a.md).FillWidth() })
	}
	root.Selectable().CopyData(state.copy).ContextMenu(func(m *ui.Menu) {
		m.EditItems()
		if m.Item("复制为NGA代码").Chosen() {
			c.Services().WriteClipboardData(state.copy(root.SelectedTexts()))
		}
	})
	if root.FocusWithin() && root.Shortcut(ui.Cmd|ui.Shift, ui.KeyC) {
		c.Services().WriteClipboardData(state.copy(root.SelectedTexts()))
	}
	// Context-menu choices and frame shortcuts read the committed selection.
	// Keep that frame's source mapping until those requests have been served.
	clear(state.Paragraphs)
	options := &markdown.Options{Theme: a.md.Theme, OnLink: a.openLink, WrapCode: true, ShareSelection: true}
	options.Block = func(c *ui.Context, d *parser.Document, i int) bool {
		if a.emoteTable(c, d, i, state) {
			return true
		}
		if a.emoteParagraph(c, d, i, state) {
			return true
		}
		return a.imageLinkBlock(c, d, i)
	}
	return root.Children(func() { markdown.View(c, source, options).FillWidth() })
}

func (a *reader) emoteParagraph(c *ui.Context, d *parser.Document, index int, state *emoteCopyState) bool {
	in := d.Inlines(index)
	if !inlineHasEmote(in, false) && d.Block(index).Kind != parser.TableCell {
		return false
	}
	font := float32(a.state.FontSize)
	color := c.Theme().Text
	for parent := d.Block(index).Parent; parent > 0; parent = d.Block(parent).Parent {
		if d.Block(parent).Kind == parser.Quote {
			color = c.Theme().TextMuted
			break
		}
	}
	base := ui.Span{Size: font, Color: color}
	if d.Block(index).Kind == parser.Heading {
		base.Size *= []float32{2, 1.5, 1.25, 1, 1, 1}[d.Block(index).Level-1]
		base.Weight = 500
	}
	mapping := emoteCopyParagraph{Objects: map[int]string{}, Before: state.NextSeparator}
	draws := map[int]func(*ui.Painter, ui.Rect){}
	offset := 0
	add := func(style ui.Span, text string) {
		if text != "" {
			style.Text = text
			ui.RichText(c, style)
			offset += utf8.RuneCountInString(text)
		}
	}
	var render func(int, int, ui.Span)
	render = func(lo, hi int, style ui.Span) {
		for k := lo; k < hi; k = in[k].Next {
			n := in[k]
			switch n.Kind {
			case parser.Strong, parser.Emphasis, parser.Strikethrough:
				child := style
				if n.Kind == parser.Strong {
					child.Weight = 500
				}
				if n.Kind == parser.Emphasis {
					child.Italic = true
				}
				if n.Kind == parser.Strikethrough {
					child.Strikethrough = true
				}
				render(k+1, n.Next, child)
			case parser.Link:
				if e, ok := emotes.FromDestination(n.Dest); ok && !n.Pending {
					makeObject := a.md.LinkIcon
					if makeObject == nil {
						makeObject = a.emoteView.inline
					}
					object, ok := makeObject(n.Dest, style.Size)
					if !ok {
						add(style, "[表情："+e.Label()+"]")
						continue
					}
					mapping.Objects[offset] = fmt.Sprintf("[s:%s:%s]", e.Group, e.Name)
					draws[offset] = object.Draw
					span := style
					span.Text = "\ufffc"
					span.ObjectWidth = object.Width
					span.ObjectHeight = object.Height
					span.ObjectDescent = object.Depth
					ui.RichText(c, span).Tooltip("NGA 表情：" + e.Label()).Description("NGA 表情：" + e.Label())
					offset++
				} else {
					linkStyle := style
					linkStyle.Color = a.md.Theme.Link
					link := ui.Link(c, "", "").TextColor(linkStyle.Color).Children(func() { render(k+1, n.Next, linkStyle) })
					if n.Title != "" {
						link.Tooltip(n.Title)
					} else {
						link.Tooltip(n.Dest)
					}
					link.OnClick(func() { a.openLink(n.Dest) })
				}
			case parser.Code:
				code := style
				code.Font = "monospace"
				code.Size *= .875
				code.Color = c.Theme().TextMuted
				code.Background = c.Theme().Surface
				code.Radius = 3
				add(code, n.Text)
			case parser.SoftBreak:
				add(style, " ")
			case parser.LineBreak:
				add(style, "\n")
			case parser.Image:
				add(style, "["+parser.PlainText(in[k+1:n.Next])+"]")
			case parser.HTML:
				switch strings.ToLower(n.Text) {
				case "<u>":
					style.Underline = true
				case "</u>":
					style.Underline = false
				case "<br>", "<br/>", "<br />":
					add(style, "\n")
				default:
					add(style, n.Text)
				}
			default:
				add(style, n.Text)
			}
		}
	}
	paragraph := ui.RichText(c).FontSize(base.Size).LineHeight(1.7).TextColor(color).FillWidth().Children(func() { render(0, len(in), base) })
	paragraph.DrawObjects(func(p *ui.Painter, at int, box ui.Rect) {
		if draw := draws[at]; draw != nil {
			draw(p, box)
		}
	})
	if d.Block(index).Kind == parser.Heading {
		paragraph.Role(ui.RoleHeading).Level(d.Block(index).Level)
	}
	state.Paragraphs[paragraph.ID()] = mapping
	return true
}

func (a *reader) emoteTable(c *ui.Context, d *parser.Document, index int, state *emoteCopyState) bool {
	b := d.Block(index)
	if b.Kind != parser.Table {
		return false
	}
	columns := len(b.Align)
	if columns == 0 {
		return false
	}
	widths := make([]float32, columns)
	hasEmote := false
	for row := range d.Children(index) {
		column := 0
		for cell := range d.Children(row) {
			if column >= columns {
				break
			}
			in := d.Inlines(cell)
			hasEmote = hasEmote || inlineHasEmote(in, false)
			w, _ := c.MeasureText(0, ui.Span{Text: parser.PlainText(in), Size: float32(a.state.FontSize)})
			widths[column] = max(widths[column], min(240, max(64, w+20)))
			column++
		}
	}
	if !hasEmote {
		return false
	}
	ui.ScrollHorizontal(c).FillWidth().Children(func() {
		ui.Column(c).Border(1, c.Theme().Border).Radius(5).Clip().Shrink(0).Role(ui.RoleTable).Children(func() {
			rowNumber := 0
			for row := range d.Children(index) {
				row := row
				number := rowNumber
				rowNumber++
				line := ui.Row(c).AlignItems(ui.Stretch).Role(ui.RoleRow)
				if d.Block(row).Header {
					line.Background(c.Theme().Surface)
				}
				if number > 0 {
					line.BorderWidth(1, 0, 0, 0).BorderColor(c.Theme().Border)
				}
				line.Children(func() {
					column := 0
					for cell := range d.Children(row) {
						if column >= columns {
							break
						}
						cell := cell
						col := column
						column++
						box := ui.Column(c).Width(widths[col]).Padding(6, 10).Role(ui.RoleCell)
						if col > 0 {
							box.BorderWidth(0, 0, 0, 1).BorderColor(c.Theme().Border)
						}
						if d.Block(row).Header {
							box.Role(ui.RoleColumnHeader).FontWeight(500)
						}
						box.Children(func() {
							state.NextSeparator = ""
							if col > 0 {
								state.NextSeparator = "\t"
							} else if number > 0 {
								state.NextSeparator = "\n"
							}
							a.emoteParagraph(c, d, cell, state)
							state.NextSeparator = ""
						})
					}
				})
			}
		})
	})
	return true
}
