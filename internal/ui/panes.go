package ui

import (
	"runtime"

	qt "github.com/mappu/miqt/qt6"
)

// titleLabel returns the bold label a pane has above it.
func titleLabel(text string) *qt.QLabel {
	label := qt.NewQLabel3(text)
	font := label.Font()
	font.SetBold(true)
	label.SetFont(font)
	runtime.KeepAlive(font)
	return label
}

// titled returns a widget under a title.
func titled(title *qt.QLabel, widget *qt.QWidget) *qt.QWidget {
	pane := qt.NewQWidget2()
	layout := qt.NewQVBoxLayout(pane)
	layout.SetContentsMargins(0, 0, 0, 0)
	layout.AddWidget(title.QWidget)
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
func newPaneSplitter(orientation qt.Orientation) *qt.QSplitter {
	s := qt.NewQSplitter3(orientation)
	// Only a handle made here can have its painting replaced.
	s.OnCreateHandle(func(func() *qt.QSplitterHandle) *qt.QSplitterHandle {
		handle := qt.NewQSplitterHandle(s.Orientation(), s)
		handle.OnPaintEvent(func(func(*qt.QPaintEvent), *qt.QPaintEvent) { paintGripDots(handle) })
		return handle
	})
	return s
}

// paintGripDots draws the dots at the centre of a handle.
func paintGripDots(handle *qt.QSplitterHandle) {
	palette := handle.Palette()
	text := palette.ColorWithCr(qt.QPalette__WindowText)
	window := palette.ColorWithCr(qt.QPalette__Window)
	// Half way between the two shows in any theme, and has no hue of its own.
	colour := qt.NewQColor3((text.Red()+window.Red())/2, (text.Green()+window.Green())/2, (text.Blue()+window.Blue())/2)
	defer colour.Delete()
	runtime.KeepAlive(palette)

	painter := qt.NewQPainter2(handle.QPaintDevice)
	defer painter.Delete()
	span := gripDots*gripDotSize + (gripDots-1)*gripDotGap
	for i := range gripDots {
		along := i * (gripDotSize + gripDotGap)
		x := (handle.Width()-span)/2 + along
		y := (handle.Height() - gripDotSize) / 2
		if handle.Orientation() == qt.Horizontal {
			x = (handle.Width() - gripDotSize) / 2
			y = (handle.Height()-span)/2 + along
		}
		painter.FillRect5(x, y, gripDotSize, gripDotSize, colour)
	}
	painter.End()
}

// paneSpace returns the room for a splitter's two panes, without its handle.
func paneSpace(s *qt.QSplitter) int {
	if s.Orientation() == qt.Horizontal {
		return s.Width() - s.HandleWidth()
	}
	return s.Height() - s.HandleWidth()
}

// setFirstPane sizes the first pane of a splitter, or shares the room evenly if
// the size is unset or does not fit.
func setFirstPane(s *qt.QSplitter, size int) {
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
