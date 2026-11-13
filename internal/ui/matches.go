package ui

import (
	"fmt"
	"math"
	"strings"
	"unicode/utf8"

	"github.com/nomad-software/qute/qtcore"
	"github.com/nomad-software/qute/qtgui"
	"github.com/nomad-software/qute/qtwidgets"

	"github.com/nomad-software/reghead/internal/engine"
)

// listMatches fills the list of matches.
func (w *Window) listMatches(r *engine.Result, colours syntax) {
	w.matches.Clear()
	title := fmt.Sprintf("Matches (%d)", len(r.Matches))
	if len(r.Matches) > maxShown {
		title = fmt.Sprintf("Matches (%d, first %d shown)", len(r.Matches), maxShown)
	}
	w.matchesTitle.SetText(title)
	for i, m := range r.Matches[:min(len(r.Matches), maxShown)] {
		item := qtwidgets.NewQTreeWidgetItemInWithStrings(w.matches, spanRow(fmt.Sprintf("Match %d", i+1), r, m.Groups[0]))
		item.SetIcon(0, w.matchDot(colours.fill(0, i), colours.edge(0)))
		for g, span := range m.Groups[1:] {
			name := fmt.Sprintf("Group %d", g+1)
			if g+1 < len(r.GroupNames) && r.GroupNames[g+1] != "" {
				name += " " + r.GroupNames[g+1]
			}
			child := qtwidgets.NewQTreeWidgetItemUnderWithStrings(item, spanRow(name, r, span))
			if span.Start >= 0 {
				child.SetIcon(0, w.matchDot(colours.fill(g+1, i), colours.edge(g+1)))
			} else {
				// Nothing to show, but in line with the rows that have a dot.
				child.SetIcon(0, w.matchDot("", ""))
			}
		}
	}
	w.matches.ExpandAll()
}

// matchDot returns a dot in the colour a match or group has in the text, or a
// blank of the same size if there is no colour. Dots are kept, as a list can have
// thousands of rows but only a few colours.
func (w *Window) matchDot(fill, edge string) *qtgui.QIcon {
	ratio := w.AsQPaintDevice().DevicePixelRatioF()
	key := fmt.Sprintf("%s %s %g", fill, edge, ratio)
	if icon := matchDots[key]; icon != nil {
		return icon
	}
	size := int(math.Ceil(matchDotSize * ratio))
	pixmap := qtgui.NewQPixmapWithW(size, size)
	pixmap.FillFull(qtgui.RGBAColor(0, 0, 0, 0))
	if fill != "" {
		qtgui.PaintOn(pixmap, func(painter *qtgui.QPainter) {
			painter.SetRenderHint(qtgui.QPainterAntialiasing)
			pen := qtgui.NewQPenWithColor(colour(edge))
			pen.SetWidthF(ratio)
			painter.SetPen(pen)
			painter.SetBrushColor(colour(fill))
			inset := ratio / 2
			painter.DrawEllipseF(qtcore.QRectF{X: inset, Y: inset, Width: float64(size) - ratio, Height: float64(size) - ratio})
		})
	}
	pixmap.SetDevicePixelRatio(ratio)
	icon := qtgui.NewQIconWithPixmap(pixmap)
	matchDots[key] = icon
	return icon
}

// matchDots are the dots made so far.
var matchDots = map[string]*qtgui.QIcon{}

// spanRow returns the columns of a row of the list of matches.
func spanRow(name string, r *engine.Result, s engine.Span) []string {
	if s.Start < 0 {
		return []string{name, "", "(no part in the match)"}
	}
	position := fmt.Sprintf("%d-%d", r.Offsets.Rune(s.Start), r.Offsets.Rune(s.End))
	return []string{name, position, displayValue(r.Text[s.Start:s.End])}
}

// displayValue fits matched text on one row of the list.
func displayValue(s string) string {
	if s == "" {
		return "(empty)"
	}
	if utf8.RuneCountInString(s) > maxValue {
		s = string([]rune(s)[:maxValue]) + "…"
	}
	return strings.NewReplacer("\n", "↵", "\t", "→", "\r", "").Replace(s)
}

// selectMatch selects in the text the match or group picked from the list.
func (w *Window) selectMatch(item *qtwidgets.QTreeWidgetItem) {
	if item == nil || w.result == nil {
		return
	}
	parent := item.Parent()
	var span engine.Span
	if parent == nil {
		span = w.result.Matches[w.matches.IndexOfTopLevelItem(item)].Groups[0]
	} else {
		span = w.result.Matches[w.matches.IndexOfTopLevelItem(parent)].Groups[parent.IndexOfChild(item)+1]
	}
	// The text may have changed since the result, with a new one on its way.
	if span.Start < 0 || w.running {
		return
	}
	cursor := w.text.TextCursor()
	cursor.SetPosition(w.result.Offsets.UTF16(span.Start))
	cursor.SetPositionFull(w.result.Offsets.UTF16(span.End), qtgui.QTextCursorKeepAnchor)
	w.text.SetTextCursor(cursor)
}
