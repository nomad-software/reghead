package ui

import (
	"strings"

	"github.com/nomad-software/qute/qtcore"
	"github.com/nomad-software/qute/qtgui"
	"github.com/nomad-software/qute/qtwidgets"
)

// lineFieldMargin is the space around the line of text in the regex field.
const lineFieldMargin = 6

// lineField is a field for one line that can be syntax highlighted, as the pattern
// and the replacement are. A QLineEdit cannot colour parts of its text, so this is
// a text editor made to behave as a single line.
type lineField struct {
	*qtwidgets.QPlainTextEdit
	// copyAll, if set, gives what copying the whole field copies.
	copyAll func() string
}

// newLineField returns an empty field in the font.
func newLineField(font *qtgui.QFont) *lineField {
	f := &lineField{QPlainTextEdit: qtwidgets.NewQPlainTextEdit(nil)}
	f.SetFont(font)
	f.SetLineWrapMode(qtwidgets.QPlainTextEditNoWrap)
	f.SetHorizontalScrollBarPolicy(qtcore.ScrollBarAlwaysOff)
	f.SetVerticalScrollBarPolicy(qtcore.ScrollBarAlwaysOff)
	f.SetTabChangesFocus(true)
	f.Document().SetDocumentMargin(lineFieldMargin)
	// The space kept for the inline flags has the colour of the field.
	f.SetBackgroundRole(qtgui.QPaletteBase)
	f.SetAutoFillBackground(true)

	f.SetFixedHeight(f.FontMetrics().LineSpacing() + 2*lineFieldMargin + 2*f.FrameWidth())

	f.OverrideKeyPressEvent(func(super func(*qtgui.QKeyEvent), e *qtgui.QKeyEvent) {
		// It is one line, which has nothing to do with the Enter key.
		if key := qtcore.Key(e.Key()); key == qtcore.Key_Return || key == qtcore.Key_Enter {
			return
		}
		if f.copyAll != nil && e.Matches(qtgui.QKeySequenceCopy) && f.Text() != "" && f.SelectedText() == f.Text() {
			qtgui.QGuiApplicationClipboard().SetText(f.copyAll())
			return
		}
		super(e)
	})

	// What is pasted or dropped stays on the one line.
	f.OverrideInsertFromMimeData(func(_ func(*qtcore.QMimeData), source *qtcore.QMimeData) {
		f.InsertPlainText(singleLine(source.Text()))
	})
	return f
}

// singleLine takes the line breaks out of text.
func singleLine(text string) string {
	return strings.NewReplacer("\r", "", "\n", "", " ", "", " ", "").Replace(text)
}

// Text returns the pattern in the field.
func (f *lineField) Text() string { return f.ToPlainText() }

// SetText replaces the pattern in the field, with the cursor after it.
func (f *lineField) SetText(text string) {
	f.SetPlainText(singleLine(text))
	cursor := f.TextCursor()
	cursor.MovePosition(qtgui.QTextCursorEnd)
	f.SetTextCursor(cursor)
}

// setLeftMargin keeps room for the inline flags before the text.
func (f *lineField) setLeftMargin(left int) {
	f.Protected().SetViewportMargins(qtcore.QMargins{Left: left})
}

// SelectedText returns what is selected.
func (f *lineField) SelectedText() string {
	return f.TextCursor().SelectedText()
}

// CursorPosition returns where the cursor is.
func (f *lineField) CursorPosition() int {
	return f.TextCursor().Position()
}

// SetCursorPosition moves the cursor.
func (f *lineField) SetCursorPosition(position int) {
	cursor := f.TextCursor()
	cursor.SetPosition(position)
	f.SetTextCursor(cursor)
}
