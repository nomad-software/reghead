package ui

import (
	"fmt"
	"math"
	"runtime"
	"strings"
	"unicode/utf8"

	qt "github.com/mappu/miqt/qt6"

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
		item := qt.NewQTreeWidgetItem4(w.matches, spanRow(fmt.Sprintf("Match %d", i+1), r, m.Groups[0]))
		item.SetIcon(0, w.matchDot(colours.fill(0, i), colours.edge(0)))
		for g, span := range m.Groups[1:] {
			name := fmt.Sprintf("Group %d", g+1)
			if g+1 < len(r.GroupNames) && r.GroupNames[g+1] != "" {
				name += " " + r.GroupNames[g+1]
			}
			child := qt.NewQTreeWidgetItem7(item, spanRow(name, r, span))
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
func (w *Window) matchDot(fill, edge string) *qt.QIcon {
	ratio := w.DevicePixelRatioF()
	key := fmt.Sprintf("%s %s %g", fill, edge, ratio)
	if icon := matchDots[key]; icon != nil {
		return icon
	}
	size := int(math.Ceil(matchDotSize * ratio))
	pixmap := qt.NewQPixmap2(size, size)
	defer pixmap.Delete()
	transparent := qt.NewQColor2(qt.Transparent)
	defer transparent.Delete()
	pixmap.FillWithFillColor(transparent)
	if fill != "" {
		painter := qt.NewQPainter2(pixmap.QPaintDevice)
		painter.SetRenderHint(qt.QPainter__Antialiasing)
		edgeColour, fillColour := qt.NewQColor6(edge), qt.NewQColor6(fill)
		brush := qt.NewQBrush3(fillColour)
		pen := qt.NewQPen3(edgeColour)
		pen.SetWidthF(ratio)
		painter.SetPenWithPen(pen)
		painter.SetBrush(brush)
		inset := ratio / 2
		bounds := qt.NewQRectF4(inset, inset, float64(size)-ratio, float64(size)-ratio)
		painter.DrawEllipse(bounds)
		painter.End()
		painter.Delete()
		bounds.Delete()
		pen.Delete()
		brush.Delete()
		fillColour.Delete()
		edgeColour.Delete()
	}
	pixmap.SetDevicePixelRatio(ratio)
	icon := qt.NewQIcon2(pixmap)
	matchDots[key] = icon
	return icon
}

// matchDots are the dots made so far.
var matchDots = map[string]*qt.QIcon{}

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
func (w *Window) selectMatch(item *qt.QTreeWidgetItem) {
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
	defer runtime.KeepAlive(cursor)
	cursor.SetPosition(w.result.Offsets.UTF16(span.Start))
	cursor.SetPosition2(w.result.Offsets.UTF16(span.End), qt.QTextCursor__KeepAnchor)
	w.text.SetTextCursor(cursor)
}
