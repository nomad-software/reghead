package ui

import (
	"fmt"
	"html"
	"slices"
	"strings"

	"github.com/nomad-software/qute/qtgui"
	"github.com/nomad-software/qute/qtwidgets"

	"github.com/nomad-software/reghead/internal/engine"
)

// render draws all that is coloured by the result, as the theme also changes.
func (w *Window) render() {
	if w.result == nil {
		return
	}
	base := w.Palette().Color(qtgui.QPaletteBase)
	colours := syntaxFor(base.Lightness() < 128)
	w.colours = colours

	w.errorLabel.SetStyleSheet("color: " + colours.errorText)
	w.inlineFlags.SetStyleSheet(tokenStyle(colours.token(engine.Token{Kind: engine.TokenModifier})))
	w.explanation.SetHtml(explanationHTML(w.result.Explanation, colours))
	// The rows are replaced by this, including any lit up.
	w.litRows = nil
	w.listMatches(w.result, colours)
	w.paintHighlights(colours)
}

// paintHighlights draws what changes with what the mouse is over.
func (w *Window) paintHighlights(colours syntax) {
	// The result is of text that has since changed, and its positions may not fit. The
	// new result paints when it comes.
	if w.running {
		return
	}
	w.paint(colours)
	w.paintExplanation(colours)
	w.paintRegex(colours)
	w.paintReplacement(colours)
	w.paintRows(colours)
}

// paintRegex highlights the pattern in the regex field. Literals are left plain.
func (w *Window) paintRegex(colours syntax) {
	text := w.regex.Text()
	offsets := engine.NewOffsets(text)
	var spans []paintedSpan
	at := 0
	for _, token := range w.result.Tokens {
		end := at + len(token.Text)
		// The tokens are of the pattern that was evaluated, and a change since
		// then has a new result on its way.
		if end > len(text) || text[at:end] != token.Text {
			return
		}
		if c := colours.token(token); c.text != "" {
			spans = append(spans, paintedSpan{engine.Span{Start: at, End: end}, c.background, c.text})
		}
		at = end
	}
	if g := w.hover.group; g >= 0 && g < len(w.result.GroupRanges) {
		spans = append(spans, paintedSpan{w.result.GroupRanges[g], colours.edge(g), colours.hoverText})
	}
	highlight(w.regex, spans, offsets)
}

// explanationHTML sets out the steps of an explanation, indented by depth: each
// fragment, highlighted as a pattern, with what it does after it.
func explanationHTML(nodes []engine.Node, colours syntax) string {
	var b strings.Builder
	family := html.EscapeString(qtgui.QFontDatabaseGetSystemFont(qtgui.QFontDatabaseFixedFont).Family())
	var walk func(nodes []engine.Node, depth int)
	walk = func(nodes []engine.Node, depth int) {
		for _, n := range nodes {
			fmt.Fprintf(&b, `<p style="margin-left:%dpx;margin-top:0;margin-bottom:6px"><span style="font-family:'%s'">`, depth*20, family)
			for _, token := range n.Tokens {
				text := strings.ReplaceAll(html.EscapeString(token.Text), " ", "&nbsp;")
				fmt.Fprintf(&b, `<span style="%s">%s</span>`, tokenStyle(colours.token(token)), text)
			}
			fmt.Fprintf(&b, `</span>&nbsp;&nbsp;%s</p>`, html.EscapeString(n.Text))
			walk(n.Children, depth+1)
		}
	}
	walk(nodes, 0)
	return b.String()
}

// paintExplanation lights up the lines of the explanation in what is hovered.
func (w *Window) paintExplanation(colours syntax) {
	var spans []paintedSpan
	if g := w.hover.group; g >= 0 {
		document := w.explanation.Document()
		for i, groups := range explanationLines(w.result.Explanation) {
			block := document.FindBlockByNumber(i)
			if block.IsValid() && slices.Contains(groups, g) {
				// A block's length counts the line break at its end.
				start := block.Position()
				spans = append(spans, paintedSpan{engine.Span{Start: start, End: start + block.Length() - 1}, colours.edge(g), colours.hoverText})
			}
		}
	}
	highlightUTF16(w.explanation, spans)
}

// paint colours the matches and their groups in the text, and the replacements
// in the output, in the colours of the list of matches.
func (w *Window) paint(colours syntax) {
	// Later ones are drawn over earlier ones, so a group shows over its match, one
	// nested in it over that, and what is hovered over everything.
	var spans, lit []paintedSpan
	for i, m := range w.result.Matches[:min(len(w.result.Matches), maxShown)] {
		for g, span := range m.Groups {
			if span.Start < 0 {
				continue
			}
			spans = append(spans, paintedSpan{span, colours.fill(g, i), ""})
			if w.hovered(i, g) {
				lit = append(lit, paintedSpan{span, colours.edge(g), colours.hoverText})
			}
		}
	}
	highlight(w.text, append(spans, lit...), w.result.Offsets)
	if w.result.OutputOffsets != nil {
		highlight(w.output, w.outputSpans(colours), w.result.OutputOffsets)
	}
}

// outputSpans returns the colours of the output: as in the text, each replacement
// in the colour of its match, with the text of each group in the group's colour.
func (w *Window) outputSpans(colours syntax) []paintedSpan {
	replaced := w.result.Replaced[:min(len(w.result.Replaced), maxShown)]
	var spans, lit []paintedSpan
	for i, span := range replaced {
		spans = append(spans, paintedSpan{span, colours.fill(0, i), ""})
		if w.hovered(i, 0) {
			lit = append(lit, paintedSpan{span, colours.edge(0), colours.hoverText})
		}
	}
	for _, r := range w.result.ReplacedGroups {
		if r.Match >= len(replaced) {
			break
		}
		if r.Group > 0 {
			spans = append(spans, paintedSpan{r.Span, colours.fill(r.Group, r.Match), ""})
		}
		if r.Group > 0 && w.hovered(r.Match, r.Group) {
			lit = append(lit, paintedSpan{r.Span, colours.edge(r.Group), colours.hoverText})
		}
	}
	return append(spans, lit...)
}

// paintReplacement highlights the replacement field: references to groups as groups
// are in the pattern, and what is lit up by the hover.
func (w *Window) paintReplacement(colours syntax) {
	text := w.replacement.Text()
	var spans, lit []paintedSpan
	at := 0
	for _, part := range w.result.ReplacementParts {
		end := at + len(part.Source)
		// The parts are of the template that was evaluated, and a change since then
		// has a new result on its way.
		if end > len(text) || text[at:end] != part.Source {
			return
		}
		span := engine.Span{Start: at, End: end}
		kind := engine.TokenLiteral
		switch {
		case part.IsRef || part.Before || part.After:
			kind = engine.TokenGroup
		case part.Escaped:
			kind = engine.TokenEscaped
		}
		if c := colours.token(engine.Token{Kind: kind}); c.text != "" {
			spans = append(spans, paintedSpan{span, c.background, c.text})
		}
		if g := w.hover.group; g >= 0 && part.IsRef && part.GroupOf(w.result.GroupNames) == g {
			lit = append(lit, paintedSpan{span, colours.edge(g), colours.hoverText})
		}
		at = end
	}
	// The whole match is what the whole of the replacement takes the place of.
	if w.hover.group == 0 && text != "" {
		lit = []paintedSpan{{engine.Span{Start: 0, End: len(text)}, colours.edge(0), colours.hoverText}}
	}
	highlight(w.replacement, append(spans, lit...), engine.NewOffsets(text))
}

// paintedSpan is a span of an editor and how it is coloured.
type paintedSpan struct {
	engine.Span
	// background is its colour, and text that of its text, if it has its own.
	background, text string
}

// highlighter is a text pane that can colour parts of its text.
type highlighter interface {
	TextCursor() *qtgui.QTextCursor
	SetExtraSelections([]*qtwidgets.QTextEditExtraSelection)
}

// highlight colours spans of an editor, given in bytes of its text.
func highlight(edit highlighter, spans []paintedSpan, offsets *engine.Offsets) {
	converted := make([]paintedSpan, len(spans))
	for i, span := range spans {
		span.Start, span.End = offsets.UTF16(span.Start), offsets.UTF16(span.End)
		converted[i] = span
	}
	highlightUTF16(edit, converted)
}

// highlightUTF16 colours spans of an editor, given in Qt's positions.
func highlightUTF16(edit highlighter, spans []paintedSpan) {
	formats := map[[2]string]*qtgui.QTextCharFormat{}
	for _, span := range spans {
		key := [2]string{span.background, span.text}
		if formats[key] == nil {
			formats[key] = newSpanFormat(span.background, span.text)
		}
	}
	// Each is copied into the editor, so what is made here is freed at once, as
	// there can be thousands.
	cursor := edit.TextCursor()
	selections := make([]*qtwidgets.QTextEditExtraSelection, 0, len(spans))
	for _, span := range spans {
		cursor.SetPosition(span.Start)
		cursor.SetPositionFull(span.End, qtgui.QTextCursorKeepAnchor)
		selection := qtwidgets.NewQTextEditExtraSelection()
		selection.SetCursor(cursor)
		selection.SetFormat(formats[[2]string{span.background, span.text}])
		selections = append(selections, selection)
	}
	edit.SetExtraSelections(selections)
	for _, selection := range selections {
		selection.Delete()
	}
	for _, format := range formats {
		format.Delete()
	}
	cursor.Delete()
}

// newSpanFormat returns the format of a span.
func newSpanFormat(background, text string) *qtgui.QTextCharFormat {
	format := qtgui.NewQTextCharFormat()
	if background != "" {
		format.SetBackground(qtgui.NewQBrushWithColor(colour(background)))
	}
	if text != "" {
		format.SetForeground(qtgui.NewQBrushWithColor(colour(text)))
	}
	return format
}

// colour returns the colour of a "#rrggbb" name, as syntax.go and theme.go give
// them.
func colour(name string) qtgui.QColor {
	c, err := qtgui.ParseColor(name)
	if err != nil {
		panic(err)
	}
	return c
}
