package main

import (
	_ "embed"
	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/markdown"
)

//go:embed resources/usage.md
var builtInHelp string

var helpMarkdown = &markdown.Options{WrapCode: true}

func (a *reader) helpDialog(c *ui.Context) {
	ui.DialogBase(c.Key("built-in-help"), &a.helpOpen, func(backdrop, panel ui.Element) {
		backdrop.Background(ui.RGBA(0, 0, 0, .45))
		panel.Width(660).MaxWidthPercent(94).MaxHeightPercent(92).Padding(22).Radius(12).Gap(15).Background(c.Theme().Background).Border(1, c.Theme().Border)
		ui.Row(c).AlignItems(ui.Center).Gap(12).Children(func() {
			ui.Text(c, "使用说明").FontSize(18).FontWeight(500).Grow(1)
			ui.Button(c, "关闭说明").OnClick(func() { a.helpOpen = false })
		})
		ui.Scroll(c.Key("manual-content")).Height(420).MinHeight(0).FillWidth().Children(func() { markdown.View(c, builtInHelp, helpMarkdown).FillWidth() })
	})
}
