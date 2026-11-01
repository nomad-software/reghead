package ui

import (
	"runtime"
	"strings"

	qt "github.com/mappu/miqt/qt6"
)

// lineFieldMargin is the space around the line of text in the regex field.
const lineFieldMargin = 6

// lineField is a field for one line that can be syntax highlighted, as the pattern
// and the replacement are. A QLineEdit cannot colour parts of its text, so this is
// a text editor made to behave as a single line.
type lineField struct {
	*qt.QPlainTextEdit
	// copyAll, if set, gives what copying the whole field copies.
	copyAll func() string
}

// newLineField returns an empty field in the font.
func newLineField(font *qt.QFont) *lineField {
	f := &lineField{QPlainTextEdit: qt.NewQPlainTextEdit2()}
	f.SetFont(font)
	f.SetLineWrapMode(qt.QPlainTextEdit__NoWrap)
	f.SetHorizontalScrollBarPolicy(qt.ScrollBarAlwaysOff)
	f.SetVerticalScrollBarPolicy(qt.ScrollBarAlwaysOff)
	f.SetTabChangesFocus(true)
	f.Document().SetDocumentMargin(lineFieldMargin)
	// The space kept for the inline flags has the colour of the field.
	f.SetBackgroundRole(qt.QPalette__Base)
	f.SetAutoFillBackground(true)

	metrics := f.FontMetrics()
	f.SetFixedHeight(metrics.LineSpacing() + 2*lineFieldMargin + 2*f.FrameWidth())
	runtime.KeepAlive(metrics)

	f.OnKeyPressEvent(func(super func(*qt.QKeyEvent), e *qt.QKeyEvent) {
		// It is one line, which has nothing to do with the Enter key.
		if key := qt.Key(e.Key()); key == qt.Key_Return || key == qt.Key_Enter {
			return
		}
		if f.copyAll != nil && e.Matches(qt.QKeySequence__Copy) && f.Text() != "" && f.SelectedText() == f.Text() {
			qt.QGuiApplication_Clipboard().SetText(f.copyAll())
			return
		}
		super(e)
	})

	// What is pasted or dropped stays on the one line.
	f.OnInsertFromMimeData(func(_ func(*qt.QMimeData), source *qt.QMimeData) {
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
	defer runtime.KeepAlive(cursor)
	cursor.MovePosition(qt.QTextCursor__End)
	f.SetTextCursor(cursor)
}

// SelectedText returns what is selected.
func (f *lineField) SelectedText() string {
	cursor := f.TextCursor()
	defer runtime.KeepAlive(cursor)
	return cursor.SelectedText()
}

// CursorPosition returns where the cursor is.
func (f *lineField) CursorPosition() int {
	cursor := f.TextCursor()
	defer runtime.KeepAlive(cursor)
	return cursor.Position()
}

// SetCursorPosition moves the cursor.
func (f *lineField) SetCursorPosition(position int) {
	cursor := f.TextCursor()
	defer runtime.KeepAlive(cursor)
	cursor.SetPosition(position)
	f.SetTextCursor(cursor)
}
