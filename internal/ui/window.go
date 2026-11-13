// Package ui is the window of the program.
package ui

import (
	"fmt"
	"os"

	"github.com/nomad-software/qute"
	"github.com/nomad-software/qute/qtcore"
	"github.com/nomad-software/qute/qtgui"
	"github.com/nomad-software/qute/qtwidgets"

	"github.com/nomad-software/reghead/internal/engine"
	"github.com/nomad-software/reghead/internal/settings"
)

const (
	// submenuArrowRoom follows the title of a submenu, to keep its arrow clear of it.
	submenuArrowRoom = "      "
	// maxShown is how many matches are listed and highlighted, as Qt slows
	// badly with many more. Substitution always covers them all.
	maxShown = 5000
	// maxValue is how many characters of a match the list shows.
	maxValue = 200
	// autosaveDelay is how many milliseconds after a change the settings are saved,
	// so that typing does not write them on every key.
	autosaveDelay = 1000
	// matchColumns is how many columns the list of matches has.
	matchColumns = 3
	// matchDotSize is the size of the dots in the list of matches.
	matchDotSize = 12
	// A handle's grip is gripDots dots of gripDotSize pixels, gripDotGap apart.
	gripDots    = 5
	gripDotSize = 2
	gripDotGap  = 2
	// windowMargin is the space between each edge of the window and the panes.
	windowMargin = 20
	// paneHandleWidth is the thickness of the handles between panes, which also
	// serves as the gap between them.
	paneHandleWidth = 10
	// inlineFlagsLeft is where the inline flags start in the regex field.
	inlineFlagsLeft = 5
)

// Window is the main window.
type Window struct {
	*qtwidgets.QMainWindow
	path   string
	config settings.Settings
	engine engine.Engine

	regex, replacement *lineField
	errorLabel         *qtwidgets.QLabel
	regexTitle         *qtwidgets.QLabel
	text, output       *qtwidgets.QPlainTextEdit
	substitution       *qtwidgets.QWidget
	explanation        *qtwidgets.QTextBrowser
	matches            *qtwidgets.QTreeWidget
	matchesTitle       *qtwidgets.QLabel
	outer, left, side  *qtwidgets.QSplitter
	flagsMenu          *qtwidgets.QMenu
	examplesMenu       *qtwidgets.QMenu
	quitAction         *qtgui.QAction
	// exampleActions are the menu's actions, in the order of examples.
	exampleActions []*qtgui.QAction
	modeActions    []*qtgui.QAction
	engineActions  []*qtgui.QAction
	flagsButton    *qtwidgets.QToolButton
	copyButton     *qtwidgets.QToolButton
	// inlineFlags shows, in the regex field but out of reach of editing, what
	// the flags add to the pattern.
	inlineFlags *qtwidgets.QLabel

	// running says an evaluation is under way, and pending that what it
	// evaluates has changed since. Only the UI's thread touches them.
	running, pending bool
	result           *engine.Result
	// autosave saves the settings a moment after a change.
	autosave *qtcore.QTimer
	// colours are those of the theme, as of the last render.
	colours syntax
	// hover is what the mouse is over, and litRows the rows lit up for it.
	hover   hoverTarget
	litRows []*qtwidgets.QTreeWidgetItem
}

// NewWindow makes the window, with the settings at path.
func NewWindow(path string) *Window {
	config := settings.Load(path)
	w := &Window{
		QMainWindow: qtwidgets.NewQMainWindow(nil),
		path:        path,
		config:      config,
		engine:      engine.ByName(config.Engine),
		hover:       noHover,
	}
	w.config.Flags, w.config.Pattern = takeInlineFlags(w.engine, w.offeredFlags(config.Flags), config.Pattern)
	w.SetWindowTitle("RegHead")
	w.SetWindowIcon(appIcon())
	w.Resize(config.Width, config.Height)
	if config.Theme != "" {
		applyTheme(config.Theme)
	}
	w.makeWidgets()
	w.makeMenus()

	// Connected after the text is set, so that it is evaluated only once.
	w.regex.OnTextChanged(w.regexChanged)
	w.replacement.OnTextChanged(w.refresh)
	w.text.OnTextChanged(w.refresh)
	w.matches.OnCurrentItemChanged(func(current, _ *qtwidgets.QTreeWidgetItem) { w.selectMatch(current) })

	w.watchHover()

	w.OverrideChangeEvent(func(super func(*qtcore.QEvent), e *qtcore.QEvent) {
		super(e)
		if t := e.Type(); t == qtcore.QEventPaletteChange || t == qtcore.QEventApplicationPaletteChange {
			w.render()
		}
	})
	// What is entered is saved soon after it changes, so that it survives the program
	// ending without its window being closed, as in a crash.
	w.autosave = qtcore.NewQTimer(w)
	w.autosave.SetSingleShot(true)
	w.autosave.OnTimeout(w.saveSettings)

	w.OverrideCloseEvent(func(super func(*qtgui.QCloseEvent), e *qtgui.QCloseEvent) {
		w.autosave.Stop()
		w.saveSettings()
		super(e)
	})
	return w
}

// Start shows the window and lays out its panes, which can only be sized once
// it is shown.
func (w *Window) Start() {
	if w.config.Maximized {
		w.ShowMaximized()
	} else {
		w.Show()
	}
	qute.Post(w.restoreLayout)
	w.refresh()
}

// makeWidgets makes the widgets and sets them out.
func (w *Window) makeWidgets() {
	fixed := qtgui.QFontDatabaseGetSystemFont(qtgui.QFontDatabaseFixedFont)

	w.regex = newLineField(fixed)
	w.regex.SetText(w.config.Pattern)
	w.makeInlineFlags(fixed)
	w.flagsMenu = qtwidgets.NewQMenu(nil)
	// A flag is ticked without closing the menu, so that several can be set.
	w.flagsMenu.OverrideMouseReleaseEvent(func(super func(*qtgui.QMouseEvent), e *qtgui.QMouseEvent) {
		if a := w.flagsMenu.ActiveAction(); a != nil {
			a.Trigger()
			return
		}
		super(e)
	})
	w.flagsButton = qtwidgets.NewQToolButton(nil)
	w.flagsButton.SetMenu(w.flagsMenu)
	w.flagsButton.SetPopupMode(qtwidgets.QToolButtonInstantPopup)
	w.flagsButton.SetToolTip("Flags")
	regexRow := qtwidgets.NewQHBoxLayout(nil)
	regexRow.AddWidget(w.regex)
	regexRow.AddWidget(w.flagsButton)
	w.copyButton = qtwidgets.NewQToolButton(nil)
	w.copyButton.SetText("Copy")
	w.copyButton.SetToolTip("Copy the regular expression, with its flags")
	w.copyButton.SetEnabled(w.regex.Text() != "")
	w.copyButton.OnClicked(func(bool) {
		qtgui.QGuiApplicationClipboard().SetText(w.fullPattern())
	})
	regexRow.AddWidget(w.copyButton)
	w.fillFlagsMenu()
	w.errorLabel = qtwidgets.NewQLabel(nil)
	w.errorLabel.SetTextFormat(qtcore.PlainText)
	w.errorLabel.SetWordWrap(true)
	w.errorLabel.Hide()

	w.text = qtwidgets.NewQPlainTextEdit(nil)
	w.text.SetFont(fixed)
	w.text.SetPlainText(w.config.Text)

	w.replacement = newLineField(fixed)
	w.replacement.SetPlaceholderText(w.engine.ReplacementHint())
	w.replacement.SetText(w.config.Replacement)
	w.output = qtwidgets.NewQPlainTextEdit(nil)
	w.output.SetFont(fixed)
	w.output.SetReadOnly(true)

	w.substitution = qtwidgets.NewQWidget(nil)
	subLayout := qtwidgets.NewQVBoxLayout(w.substitution)
	subLayout.SetContentsMargins(qtcore.QMargins{})
	subLayout.AddWidget(titleLabel("Replacement"))
	subLayout.AddWidget(w.replacement)
	subLayout.AddWidget(titleLabel("Result"))
	subLayout.AddWidget(w.output)
	w.substitution.SetVisible(w.config.Mode == settings.ModeSubstitute)

	w.left = newPaneSplitter(qtcore.Vertical)
	w.left.SetChildrenCollapsible(false)
	w.left.AddWidget(titled(titleLabel("Text"), w.text))
	w.left.AddWidget(w.substitution)

	main := qtwidgets.NewQWidget(nil)
	mainLayout := qtwidgets.NewQVBoxLayout(main)
	mainLayout.SetContentsMargins(qtcore.QMargins{})
	w.regexTitle = titleLabel(regexTitleText(w.engine))
	mainLayout.AddWidget(w.regexTitle)
	mainLayout.AddLayout(regexRow)
	mainLayout.AddWidget(w.errorLabel)
	mainLayout.AddWidgetFull(w.left, 1, 0)

	w.explanation = qtwidgets.NewQTextBrowser(nil)
	w.explanation.SetOpenLinks(false)
	w.explanation.SetPlaceholderText("Enter a regular expression to have it explained here.")

	w.matches = qtwidgets.NewQTreeWidget(nil)
	w.matches.SetColumnCount(matchColumns)
	w.matches.SetHeaderLabels([]string{"Match", "Position", "Value"})
	w.matches.SetUniformRowHeights(true)
	w.matches.SetIconSize(qtcore.QSize{Width: matchDotSize, Height: matchDotSize})
	w.matches.Header().SetStretchLastSection(true)
	// The first column has room for a group's name, unless it was resized.
	w.matches.Header().ResizeSection(0, 170)
	for i, width := range w.config.MatchColumns {
		if i < 2 && width > 0 {
			w.matches.Header().ResizeSection(i, width)
		}
	}
	w.matchesTitle = titleLabel("Matches")

	w.side = newPaneSplitter(qtcore.Vertical)
	w.side.SetChildrenCollapsible(false)
	w.side.AddWidget(titled(titleLabel("Explanation"), w.explanation))
	w.side.AddWidget(titled(w.matchesTitle, w.matches))

	w.outer = newPaneSplitter(qtcore.Horizontal)
	w.outer.SetChildrenCollapsible(false)
	w.outer.AddWidget(main)
	w.outer.AddWidget(w.side)
	// Growing the window grows the main pane, and the side pane keeps its size.
	w.outer.SetStretchFactor(0, 1)
	w.outer.SetStretchFactor(1, 0)

	for _, splitter := range []*qtwidgets.QSplitter{w.left, w.side, w.outer} {
		splitter.SetHandleWidth(paneHandleWidth)
	}

	central := qtwidgets.NewQWidget(nil)
	layout := qtwidgets.NewQVBoxLayout(central)
	layout.SetContentsMargins(qtcore.QMargins{Left: windowMargin, Top: windowMargin, Right: windowMargin, Bottom: windowMargin})
	layout.AddWidget(w.outer)
	w.SetCentralWidget(central)
	// A status bar along the bottom, which shows no text and is there for the
	// handle in its corner, to drag the window bigger or smaller by.
	w.StatusBar().SetSizeGripEnabled(true)
}

// saveSettings writes the settings, with the size of the window and its panes.
func (w *Window) saveSettings() {
	w.config.Maximized = w.IsMaximized()
	if !w.config.Maximized {
		w.config.Width, w.config.Height = w.Width(), w.Height()
	}
	if sizes := w.outer.Sizes(); len(sizes) == 2 && sizes[1] > 0 {
		w.config.SideWidth = sizes[1]
	}
	if sizes := w.side.Sizes(); len(sizes) == 2 && sizes[0] > 0 {
		w.config.ExplainHeight = sizes[0]
	}
	w.rememberTextHeight()
	header := w.matches.Header()
	w.config.MatchColumns = []int{header.SectionSize(0), header.SectionSize(1)}
	w.config.Engine = w.engine.Name()
	w.config.Pattern = w.regex.Text()
	w.config.Text = w.text.ToPlainText()
	w.config.Replacement = w.replacement.Text()
	if err := w.config.Save(w.path); err != nil {
		fmt.Fprintln(os.Stderr, "reghead: the settings could not be saved:", err)
	}
}

// refresh evaluates what is entered, away from the UI's thread so that a slow
// engine cannot freeze the window. One evaluation runs at a time; changes made
// meanwhile are evaluated together when it ends, and its stale result dropped.
func (w *Window) refresh() {
	// The first run is of what was just loaded, which has nothing new to save.
	if w.result != nil {
		w.autosave.StartWithInterval(autosaveDelay)
	}
	if w.running {
		w.pending = true
		return
	}
	w.running = true
	request := engine.Request{
		Engine:      w.engine,
		Pattern:     w.regex.Text(),
		Flags:       w.config.Flags,
		Text:        w.text.ToPlainText(),
		Substitute:  w.config.Mode == settings.ModeSubstitute,
		Replacement: w.replacement.Text(),
	}
	go func() {
		result := engine.Evaluate(request)
		qute.Post(func() {
			w.running = false
			if w.pending {
				w.pending = false
				w.refresh()
				return
			}
			w.show(&result, request.Substitute)
		})
	}()
}

// show puts a result on screen.
func (w *Window) show(r *engine.Result, substitute bool) {
	w.result = r
	w.hover = noHover
	problem := r.Error
	if problem == "" {
		problem = r.Warning
	}
	w.errorLabel.SetText(problem)
	w.errorLabel.SetVisible(problem != "")
	if substitute {
		w.output.SetPlainText(r.Output)
	}
	w.render()
}
