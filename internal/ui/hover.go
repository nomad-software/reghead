package ui

import (
	"runtime"
	"slices"

	qt "github.com/mappu/miqt/qt6"

	"github.com/nomad-software/reghead/internal/engine"
)

// allMatches is the match of a hover on the pattern, which stands for a group in
// every match.
const allMatches = -1

// hoverTarget is what the mouse is over: a group, where 0 is the whole match, of
// one match or of all. The same is lit up in the pattern, the replacement, the text,
// the result, the explanation and the list, to show how they relate.
type hoverTarget struct{ match, group int }

// noHover is the mouse being over nothing.
var noHover = hoverTarget{match: allMatches, group: -1}

// watchHover has the fields, the text, the result, the explanation and the list
// report what the mouse is over.
func (w *Window) watchHover() {
	watchEditor(w, w.regex.QPlainTextEdit, w.hoverPatternAt)
	watchEditor(w, w.text, w.hoverTextAt)
	watchEditor(w, w.replacement.QPlainTextEdit, w.hoverReplacementAt)
	watchEditor(w, w.output, w.hoverOutputAt)
	watchEditor(w, w.explanation, w.hoverExplanationAt)

	w.matches.Viewport().SetMouseTracking(true)
	w.matches.OnMouseMoveEvent(func(super func(*qt.QMouseEvent), e *qt.QMouseEvent) {
		super(e)
		position := e.Position()
		point := position.ToPoint()
		w.hoverRow(w.matches.ItemAt(point))
		runtime.KeepAlive(position)
		runtime.KeepAlive(point)
	})
	w.matches.OnLeaveEvent(func(super func(*qt.QEvent), e *qt.QEvent) {
		super(e)
		w.setHover(noHover)
	})
}

// hoverEditor is a text pane the mouse can be followed over.
type hoverEditor interface {
	Viewport() *qt.QWidget
	OnMouseMoveEvent(func(super func(*qt.QMouseEvent), e *qt.QMouseEvent))
	OnLeaveEvent(func(super func(*qt.QEvent), e *qt.QEvent))
	CursorForPosition(*qt.QPoint) *qt.QTextCursor
	CursorRect(*qt.QTextCursor) *qt.QRect
}

// watchEditor calls hover with the position of the character under the mouse, or
// -1 if it is over none.
func watchEditor(w *Window, edit hoverEditor, hover func(position int)) {
	edit.Viewport().SetMouseTracking(true)
	edit.OnMouseMoveEvent(func(super func(*qt.QMouseEvent), e *qt.QMouseEvent) {
		super(e)
		position := e.Position()
		point := position.ToPoint()
		hover(characterAt(edit, point))
		runtime.KeepAlive(position)
		runtime.KeepAlive(point)
	})
	edit.OnLeaveEvent(func(super func(*qt.QEvent), e *qt.QEvent) {
		super(e)
		w.setHover(noHover)
	})
}

// characterAt returns the position of the character at a point of an editor, or
// -1 if there is none there. The nearest place for a cursor can be on either side
// of the character.
func characterAt(edit hoverEditor, point *qt.QPoint) int {
	cursor := edit.CursorForPosition(point)
	rect := edit.CursorRect(cursor)
	defer runtime.KeepAlive(cursor)
	defer runtime.KeepAlive(rect)
	if point.Y() < rect.Top() || point.Y() > rect.Bottom() {
		return -1
	}
	position := cursor.Position()
	if point.X() < rect.Left() {
		position--
	}
	block := cursor.Block()
	defer runtime.KeepAlive(block)
	// A block's length counts the line break at its end.
	if position < block.Position() || position >= block.Position()+block.Length()-1 {
		return -1
	}
	return position
}

// usable says the result is of what is on screen, so that positions can be
// looked up in it.
func (w *Window) usable() bool {
	return w.result != nil && w.result.Error == "" && !w.running
}

// hoverPatternAt lights up the innermost group at a position of the pattern, in
// all matches, or the whole match outside any group.
func (w *Window) hoverPatternAt(position int) {
	if !w.usable() || position < 0 || w.regex.Text() == "" {
		w.setHover(noHover)
		return
	}
	offsets := engine.NewOffsets(w.regex.Text())
	w.setHover(hoverTarget{allMatches, groupAt(w.result.GroupRanges, offsets.ByteAtUTF16(position))})
}

// groupAt returns the innermost group holding a byte of the pattern, or 0.
func groupAt(ranges []engine.Span, at int) int {
	best := 0
	for g, r := range ranges {
		if g > 0 && r.Start <= at && at < r.End && (best == 0 || r.End-r.Start < ranges[best].End-ranges[best].Start) {
			best = g
		}
	}
	return best
}

// hoverTextAt lights up the innermost group of the match at a position of the
// text.
func (w *Window) hoverTextAt(position int) {
	if !w.usable() || position < 0 {
		w.setHover(noHover)
		return
	}
	at := w.result.Offsets.ByteAtUTF16(position)
	for i, m := range w.result.Matches[:min(len(w.result.Matches), maxShown)] {
		if at < m.Groups[0].Start || at >= m.Groups[0].End {
			continue
		}
		w.setHover(hoverTarget{i, groupAt(m.Groups, at)})
		return
	}
	w.setHover(noHover)
}

// hoverReplacementAt lights up the group a reference in the replacement refers to,
// in all matches, or the whole match for the rest of it.
func (w *Window) hoverReplacementAt(position int) {
	text := w.replacement.Text()
	if !w.usable() || position < 0 || text == "" {
		w.setHover(noHover)
		return
	}
	byteAt := engine.NewOffsets(text).ByteAtUTF16(position)
	at := 0
	for _, part := range w.result.ReplacementParts {
		at += len(part.Source)
		if byteAt >= at {
			continue
		}
		if !part.IsRef {
			w.setHover(hoverTarget{allMatches, 0})
		} else if g := part.GroupOf(w.result.GroupNames); g >= 0 {
			w.setHover(hoverTarget{allMatches, g})
		} else {
			w.setHover(noHover)
		}
		return
	}
	w.setHover(noHover)
}

// hoverOutputAt lights up the group whose text is at a position of the result, or
// the whole match for the rest of a replacement.
func (w *Window) hoverOutputAt(position int) {
	if !w.usable() || position < 0 || w.result.OutputOffsets == nil {
		w.setHover(noHover)
		return
	}
	at := w.result.OutputOffsets.ByteAtUTF16(position)
	replaced := w.result.Replaced[:min(len(w.result.Replaced), maxShown)]
	for _, r := range w.result.ReplacedGroups {
		if r.Match < len(replaced) && r.Group > 0 && r.Start <= at && at < r.End {
			w.setHover(hoverTarget{r.Match, r.Group})
			return
		}
	}
	for i, span := range replaced {
		if span.Start <= at && at < span.End {
			w.setHover(hoverTarget{i, 0})
			return
		}
	}
	w.setHover(noHover)
}

// explanationLines returns, for each line of an explanation in order, the groups it
// lies in, innermost last. All that is in the pattern lies in the whole match, and
// what is not lies in nothing.
func explanationLines(nodes []engine.Node) [][]int {
	var lines [][]int
	var walk func(nodes []engine.Node, within []int)
	walk = func(nodes []engine.Node, within []int) {
		for _, n := range nodes {
			if !n.InPattern() {
				lines = append(lines, nil)
				continue
			}
			groups := within
			if g := n.CaptureGroup(); g > 0 {
				groups = append(slices.Clip(within), g)
			}
			lines = append(lines, groups)
			walk(n.Children, groups)
		}
	}
	walk(nodes, []int{0})
	return lines
}

// hoverExplanationAt lights up the innermost group of the line at a position of the
// explanation, in all matches, or the whole match outside any group.
func (w *Window) hoverExplanationAt(position int) {
	if !w.usable() || position < 0 {
		w.setHover(noHover)
		return
	}
	block := w.explanation.Document().FindBlock(position)
	defer runtime.KeepAlive(block)
	lines := explanationLines(w.result.Explanation)
	if n := block.BlockNumber(); n >= 0 && n < len(lines) && len(lines[n]) > 0 {
		w.setHover(hoverTarget{allMatches, lines[n][len(lines[n])-1]})
		return
	}
	w.setHover(noHover)
}

// hoverRow lights up what a row of the list stands for.
func (w *Window) hoverRow(item *qt.QTreeWidgetItem) {
	if !w.usable() || item == nil {
		w.setHover(noHover)
		return
	}
	parent := item.Parent()
	if parent == nil {
		w.setHover(hoverTarget{w.matches.IndexOfTopLevelItem(item), 0})
		return
	}
	w.setHover(hoverTarget{w.matches.IndexOfTopLevelItem(parent), parent.IndexOfChild(item) + 1})
}

// setHover lights up a target.
func (w *Window) setHover(target hoverTarget) {
	if target == w.hover {
		return
	}
	w.hover = target
	w.paintHighlights(w.colours)
}

// hovered says if a group of a match is what is lit up.
func (w *Window) hovered(match, group int) bool {
	return w.hover.group == group && (w.hover.match == allMatches || w.hover.match == match)
}

// paintRows lights up the rows of what is hovered, and returns those lit up before
// to how they were.
func (w *Window) paintRows(colours syntax) {
	plain := qt.NewQBrush()
	defer plain.Delete()
	for _, row := range w.litRows {
		for column := range matchColumns {
			row.SetBackground(column, plain)
		}
	}
	w.litRows = w.litRows[:0]
	if w.hover.group < 0 || w.result == nil {
		return
	}
	shown := min(len(w.result.Matches), maxShown)
	for i := range shown {
		groups := w.result.Matches[i].Groups
		// A group that took no part in this match has nothing here to light up.
		if !w.hovered(i, w.hover.group) || w.hover.group >= len(groups) || groups[w.hover.group].Start < 0 {
			continue
		}
		row := w.matches.TopLevelItem(i)
		if w.hover.group > 0 {
			row = row.Child(w.hover.group - 1)
		}
		colour := qt.NewQColor6(colours.fill(w.hover.group, i))
		brush := qt.NewQBrush3(colour)
		for column := range matchColumns {
			row.SetBackground(column, brush)
		}
		brush.Delete()
		colour.Delete()
		w.litRows = append(w.litRows, row)
	}
}
