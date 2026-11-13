package ui

import (
	"github.com/nomad-software/qute/qtcore"
	"github.com/nomad-software/qute/qtgui"
	"github.com/nomad-software/qute/qtwidgets"
)

// titleLabel returns the bold label a pane has above it.
func titleLabel(text string) *qtwidgets.QLabel {
	label := qtwidgets.NewQLabelWithText(text, nil)
	font := label.Font()
	font.SetBold(true)
	label.SetFont(font)
	return label
}

// titled returns a widget under a title.
func titled(title *qtwidgets.QLabel, widget qtwidgets.QWidgetLike) *qtwidgets.QWidget {
	pane := qtwidgets.NewQWidget(nil)
	layout := qtwidgets.NewQVBoxLayout(pane)
	layout.SetContentsMargins(qtcore.QMargins{})
	layout.AddWidget(title)
	layout.AddWidget(widget)
	return pane
}

// rememberTextHeight keeps the height of the text pane, which reads as nothing
// while the substitution pane beneath it is hidden.
func (w *Window) rememberTextHeight() {
	if sizes := w.left.Sizes(); w.substitution.IsVisible() && len(sizes) == 2 && sizes[0] > 0 {
		w.config.TextHeight = sizes[0]
	}
}

// restoreTextHeight gives the text pane its kept height.
func (w *Window) restoreTextHeight() {
	setFirstPane(w.left, w.config.TextHeight)
}

// newPaneSplitter returns a splitter whose handles draw only a row of dots, in
// place of what the style draws, which on some is coloured. With nothing painted
// behind them, the window shows through, whether a handle is hovered or not.
func newPaneSplitter(orientation qtcore.Orientation) *qtwidgets.QSplitter {
	s := qtwidgets.NewQSplitterWithOrientation(orientation, nil)
	// Only a handle made here can have its painting replaced.
	s.OverrideCreateHandle(func(func() *qtwidgets.QSplitterHandle) *qtwidgets.QSplitterHandle {
		handle := qtwidgets.NewQSplitterHandleWithOrientation(s.Orientation(), s)
		handle.OverridePaintEvent(func(func(*qtgui.QPaintEvent), *qtgui.QPaintEvent) { paintGripDots(handle) })
		return handle
	})
	return s
}

// paintGripDots draws the dots at the centre of a handle.
func paintGripDots(handle *qtwidgets.QSplitterHandle) {
	palette := handle.Palette()
	text := palette.Color(qtgui.QPaletteWindowText)
	window := palette.Color(qtgui.QPaletteWindow)
	// Half way between the two shows in any theme, and has no hue of its own.
	mid := func(a, b uint8) uint8 { return uint8((int(a) + int(b)) / 2) }
	colour := qtgui.RGB(mid(text.R, window.R), mid(text.G, window.G), mid(text.B, window.B))

	span := gripDots*gripDotSize + (gripDots-1)*gripDotGap
	qtgui.PaintOn(handle, func(painter *qtgui.QPainter) {
		for i := range gripDots {
			along := i * (gripDotSize + gripDotGap)
			x := (handle.Width()-span)/2 + along
			y := (handle.Height() - gripDotSize) / 2
			if handle.Orientation() == qtcore.Horizontal {
				x = (handle.Width() - gripDotSize) / 2
				y = (handle.Height()-span)/2 + along
			}
			painter.FillRect(qtcore.QRect{X: x, Y: y, Width: gripDotSize, Height: gripDotSize}, colour)
		}
	})
}

// paneSpace returns the room for a splitter's two panes, without its handle.
func paneSpace(s *qtwidgets.QSplitter) int {
	if s.Orientation() == qtcore.Horizontal {
		return s.Width() - s.HandleWidth()
	}
	return s.Height() - s.HandleWidth()
}

// setFirstPane sizes the first pane of a splitter, or shares the room evenly if
// the size is unset or does not fit.
func setFirstPane(s *qtwidgets.QSplitter, size int) {
	total := paneSpace(s)
	if size <= 0 || size >= total {
		size = total / 2
	}
	s.SetSizes([]int{size, total - size})
}

// restoreLayout gives the panes their saved sizes.
func (w *Window) restoreLayout() {
	total := paneSpace(w.outer)
	side := w.config.SideWidth
	if side <= 0 || side >= total {
		side = total * 2 / 5
	}
	w.outer.SetSizes([]int{total - side, side})
	setFirstPane(w.side, w.config.ExplainHeight)
	w.restoreTextHeight()
}
