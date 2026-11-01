package ui

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	qt "github.com/mappu/miqt/qt6"
	"github.com/mappu/miqt/qt6/mainthread"

	"github.com/nomad-software/reghead/internal/engine"
	"github.com/nomad-software/reghead/internal/settings"
)

func TestMain(m *testing.M) {
	os.Setenv("QT_QPA_PLATFORM", "offscreen")
	qt.NewQApplication(os.Args[:1])
	qt.QGuiApplication_SetQuitOnLastWindowClosed(false)
	code := 0
	go func() {
		code = m.Run()
		mainthread.Start(qt.QCoreApplication_Quit)
	}()
	qt.QApplication_Exec()
	os.Exit(code)
}

func open(t *testing.T, s settings.Settings) (*Window, string) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "settings.json")
	if err := s.Save(path); err != nil {
		t.Fatal(err)
	}
	w := mainthread.Wait2(func() *Window {
		w := NewWindow(path)
		w.Start()
		return w
	})
	settle(t, w)
	return w, path
}

// settle waits for evaluation to finish and for what was queued to run.
func settle(t *testing.T, w *Window) {
	t.Helper()
	for range 500 {
		if mainthread.Wait2(func() bool { return !w.running && !w.pending && w.result != nil }) {
			// One more turn of the event loop, for the layout.
			mainthread.Wait(func() {})
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatal("evaluation did not finish")
}

// dotColour returns the colour in the middle of a row's dot.
func dotColour(row *qt.QTreeWidgetItem) string {
	image := row.Icon(0).PixmapWithExtent(matchDotSize).ToImage()
	return image.PixelColor(image.Width()/2, image.Height()/2).Name()
}

// dotAlpha returns how opaque the middle of a row's dot is.
func dotAlpha(row *qt.QTreeWidgetItem) int {
	image := row.Icon(0).PixmapWithExtent(matchDotSize).ToImage()
	return image.PixelColor(image.Width()/2, image.Height()/2).Alpha()
}

func TestMatchAndExplain(t *testing.T) {
	s := settings.Default()
	s.Pattern, s.Text = `(b)|😀`, "a😀b"
	w, _ := open(t, s)
	mainthread.Wait(func() {
		if n := w.matches.TopLevelItemCount(); n != 2 {
			t.Errorf("%d matches", n)
		}
		if got := w.matchesTitle.Text(); got != "Matches (2)" {
			t.Errorf("title %q", got)
		}
		if got := w.StatusBar().CurrentMessage(); got != "" {
			t.Errorf("status %q", got)
		}
		for _, splitter := range []*qt.QSplitter{w.left, w.side, w.outer} {
			if got := splitter.HandleWidth(); got != paneHandleWidth {
				t.Errorf("handle %d wide", got)
			}
		}
		if !w.StatusBar().IsSizeGripEnabled() {
			t.Error("no drag handle")
		}
		if got := w.explanation.ToPlainText(); !strings.Contains(got, "Either of these 2 alternatives") {
			t.Errorf("explanation %q", got)
		}
		// "b" is at UTF-16 position 3, after the surrogate pair.
		w.matches.SetCurrentItem(w.matches.TopLevelItem(1))
		cursor := w.text.TextCursor()
		if cursor.SelectionStart() != 3 || cursor.SelectionEnd() != 4 {
			t.Errorf("selected %d-%d", cursor.SelectionStart(), cursor.SelectionEnd())
		}
		if got := w.matches.TopLevelItem(1).Text(1); got != "2-3" {
			t.Errorf("position %q", got)
		}
		// Two matches and the group within one.
		selections := w.text.ExtraSelections()
		if got := len(selections); got != 3 {
			t.Fatalf("%d highlights", got)
		}
		// Each row of the list has the colour of what it lists in the text.
		colours := syntaxFor(false)
		row := w.matches.TopLevelItem(1).Child(0)
		if got := dotColour(row); got != colours.fill(1, 1) {
			t.Errorf("group dot %s", got)
		}
		if got := dotColour(w.matches.TopLevelItem(1)); got != colours.fill(0, 1) {
			t.Errorf("match dot %s", got)
		}
		// The row itself has no colour.
		for column := range 3 {
			brush := row.Background(column)
			if brush.Style() != qt.NoBrush {
				t.Errorf("column %d has a background", column)
			}
		}
		// A group that took no part has no dot, but keeps the same indent.
		blank := w.matches.TopLevelItem(0).Child(0)
		if blank.Text(1) != "" || blank.Icon(0).IsNull() {
			t.Errorf("unmatched group: position %q", blank.Text(1))
		}
		if got := dotAlpha(blank); got != 0 {
			t.Errorf("unmatched group has a dot, alpha %d", got)
		}
		format := selections[2].Format()
		background := format.Background()
		if got := background.Color().Name(); got != colours.fill(1, 1) {
			t.Errorf("group highlight %s", got)
		}
		w.regex.SetText("(")
	})
	settle(t, w)
	mainthread.Wait(func() {
		if !strings.Contains(w.errorLabel.Text(), "missing closing )") || w.errorLabel.IsHidden() {
			t.Errorf("error %q", w.errorLabel.Text())
		}
		if n := w.matches.TopLevelItemCount(); n != 0 {
			t.Errorf("%d matches after an error", n)
		}
		if got := w.StatusBar().CurrentMessage(); got != "" {
			t.Errorf("status %q", got)
		}
		w.Close()
	})
}

func TestSubstitution(t *testing.T) {
	s := settings.Default()
	s.Pattern, s.Text, s.Replacement = `(\w+)@`, "me@ you@", "<$1>"
	w, _ := open(t, s)
	mainthread.Wait(func() {
		if w.substitution.IsVisible() {
			t.Error("substitution shown in match mode")
		}
		w.setMode(settings.ModeSubstitute)
		if !w.substitution.IsVisible() {
			t.Error("substitution hidden")
		}
	})
	settle(t, w)
	mainthread.Wait(func() {
		if got := w.output.ToPlainText(); got != "<me> <you>" {
			t.Errorf("output %q", got)
		}
		w.replacement.SetText("[$1]")
	})
	settle(t, w)
	mainthread.Wait(func() {
		if got := w.output.ToPlainText(); got != "[me] [you]" {
			t.Errorf("output %q", got)
		}
		w.Close()
	})
}

// The window, its panes and everything entered come back as they were left.
func TestSettingsSaved(t *testing.T) {
	s := settings.Default()
	s.Width, s.Height = 1000, 700
	s.Mode = settings.ModeSubstitute
	w, path := open(t, s)
	mainthread.Wait(func() {
		w.outer.SetSizes([]int{650, 330})
		w.side.SetSizes([]int{200, 400})
		w.left.SetSizes([]int{150, 400})
		w.matches.Header().ResizeSection(0, 111)
		w.regex.SetText("x+")
		w.text.SetPlainText("xx")
		w.replacement.SetText("y")
	})
	settle(t, w)
	var side, explain, text int
	mainthread.Wait(func() {
		side, explain, text = w.outer.Sizes()[1], w.side.Sizes()[0], w.left.Sizes()[0]
		applyTheme(settings.ThemeDark)
		w.config.Theme = settings.ThemeDark
		w.Close()
	})
	got := settings.Load(path)
	if got.Width != 1000 || got.Height != 700 || got.SideWidth != side || got.ExplainHeight != explain || got.TextHeight != text {
		t.Errorf("sizes not saved: %+v", got)
	}
	if got.Theme != settings.ThemeDark || got.Mode != settings.ModeSubstitute || got.Pattern != "x+" || got.Text != "xx" || got.Replacement != "y" {
		t.Errorf("state not saved: %+v", got)
	}
	if got.MatchColumns[0] != 111 {
		t.Errorf("columns %v", got.MatchColumns)
	}

	again, _ := open(t, got)
	mainthread.Wait(func() {
		if s := again.outer.Sizes()[1]; s != side {
			t.Errorf("side width %d, want %d", s, side)
		}
		if s := again.side.Sizes()[0]; s != explain {
			t.Errorf("explanation height %d, want %d", s, explain)
		}
		if s := again.left.Sizes()[0]; s != text {
			t.Errorf("text height %d, want %d", s, text)
		}
		if s := again.matches.Header().SectionSize(0); s != 111 {
			t.Errorf("column %d", s)
		}
		again.Close()
		applyTheme("")
	})
}

func TestThemes(t *testing.T) {
	s := settings.Default()
	s.Pattern = "("
	w, _ := open(t, s)
	for _, c := range []struct{ theme, base, errorColour string }{
		{settings.ThemeDark, "#1e1e1e", "#ff7b7b"},
		{settings.ThemeLight, "#ffffff", "#c62828"},
	} {
		mainthread.Wait(func() { applyTheme(c.theme) })
		// The window hears of the new palette through the event loop.
		mainthread.Wait(func() {})
		mainthread.Wait(func() {
			palette := qt.QGuiApplication_Palette()
			if got := palette.ColorWithCr(qt.QPalette__Base).Name(); got != c.base {
				t.Errorf("%s base %s", c.theme, got)
			}
			if got := w.errorLabel.StyleSheet(); !strings.Contains(got, c.errorColour) {
				t.Errorf("%s error colour %q", c.theme, got)
			}
		})
	}
	mainthread.Wait(func() {
		applyTheme("")
		w.Close()
	})
}

func TestFlags(t *testing.T) {
	s := settings.Default()
	s.Pattern, s.Text, s.Flags = "(?s)A", "aa", "g"
	w, path := open(t, s)
	action := func(key string) *qt.QAction {
		for _, a := range w.flagsMenu.Actions() {
			if strings.HasSuffix(a.Text(), "("+key+")") {
				return a
			}
		}
		t.Fatalf("no flag %s", key)
		return nil
	}
	mainthread.Wait(func() {
		// Flags at the start of a saved pattern go to the dropdown.
		if got := w.regex.Text(); got != "A" {
			t.Errorf("field %q", got)
		}
		if got := w.inlineFlags.Text(); got != "(?s)" {
			t.Errorf("inline flags %q", got)
		}
		action("i").Trigger()
		if got := w.inlineFlags.Text(); got != "(?is)" {
			t.Errorf("inline flags %q", got)
		}
		if strings.TrimSpace(w.flagsButton.Text()) != "Flags (gis)" {
			t.Errorf("button %q", w.flagsButton.Text())
		}
		if got := w.fullPattern(); got != "(?is)A" {
			t.Errorf("full pattern %q", got)
		}
		// The copy button gives the regex in full.
		w.copyButton.Click()
		if got := qt.QGuiApplication_Clipboard().Text(); got != "(?is)A" {
			t.Errorf("copied %q", got)
		}
		if got := w.StatusBar().CurrentMessage(); got != "" {
			t.Errorf("status after copying %q", got)
		}
	})
	settle(t, w)
	mainthread.Wait(func() {
		if n := w.matches.TopLevelItemCount(); n != 2 {
			t.Errorf("%d matches with the flag", n)
		}
		if got := w.explanation.ToPlainText(); !strings.HasPrefix(got, "(?is)") {
			t.Errorf("explanation %q", got)
		}
		// Typed flags go to the dropdown, and are never in the field's text.
		w.regex.SetText("(?m)A")
		if got := w.regex.Text(); got != "A" {
			t.Errorf("field %q", got)
		}
		if !action("m").IsChecked() {
			t.Error("typed flag not ticked")
		}
		// A flag within the pattern is part of the regex, and stays.
		w.regex.SetText("a(?i)b")
		if got := w.regex.Text(); got != "a(?i)b" {
			t.Errorf("field %q", got)
		}
		action("i").Trigger()
		action("s").Trigger()
		action("m").Trigger()
		if !w.inlineFlags.IsHidden() {
			t.Error("inline flags shown with none on")
		}
		action("g").Trigger()
		if got := strings.TrimSpace(w.flagsButton.Text()); got != "Flags" {
			t.Errorf("button with no flags %q", got)
		}
		action("g").Trigger()
		w.regex.SetText("")
		if w.copyButton.IsEnabled() {
			t.Error("copy enabled with nothing to copy")
		}
		w.regex.SetText("a(?i)b")
	})
	settle(t, w)
	mainthread.Wait(func() { w.Close() })
	if got := settings.Load(path).Pattern; got != "a(?i)b" {
		t.Errorf("saved pattern %q", got)
	}
}

// The window follows the engine chosen: its flags, how they show, what is copied,
// the replacement syntax, and what results.
func TestChoosingEngine(t *testing.T) {
	s := settings.Default()
	s.Pattern, s.Text, s.Flags, s.Mode, s.Replacement = `(?<x>a)\w`, "ab", "giU", settings.ModeSubstitute, "[$&]"
	w, _ := open(t, s)
	choose := func(name string) {
		t.Helper()
		mainthread.Wait(func() {
			for i, e := range engine.All() {
				if e.Name() == name {
					w.engineActions[i].Trigger()
					return
				}
			}
			t.Fatalf("no engine %s", name)
		})
		settle(t, w)
	}
	offered := func() []string {
		var keys []string
		for _, a := range w.flagsMenu.Actions() {
			text := a.Text()
			keys = append(keys, text[len(text)-2:len(text)-1])
		}
		return keys
	}

	if got := mainthread.Wait2(func() string { return w.engine.Name() }); got != "Go" {
		t.Fatalf("engine %q", got)
	}
	mainthread.Wait(func() {
		if got := w.regexTitle.Text(); got != "Regular Expression (Go)" {
			t.Errorf("Go title %q", got)
		}
		if got := w.inlineFlags.Text(); got != "(?iU)" {
			t.Errorf("Go inline flags %q", got)
		}
	})

	choose("PCRE2")
	mainthread.Wait(func() {
		if got := w.regexTitle.Text(); got != "Regular Expression (PCRE2)" {
			t.Errorf("PCRE title %q", got)
		}
		if got := offered(); !slices.Equal(got, []string{"g", "i", "m", "s", "x", "U", "u"}) {
			t.Errorf("PCRE flags %q", got)
		}
		if w.inlineFlags.Text() != "" || !w.inlineFlags.IsHidden() {
			t.Errorf("PCRE inline flags %q", w.inlineFlags.Text())
		}
		if got := w.fullPattern(); got != `/(?<x>a)\w/iU` {
			t.Errorf("PCRE full pattern %q", got)
		}
		if got := w.replacement.PlaceholderText(); !strings.Contains(got, `\1`) {
			t.Errorf("PCRE hint %q", got)
		}
		if n := w.matches.TopLevelItemCount(); n != 1 {
			t.Errorf("PCRE %d matches", n)
		}
		if got := w.output.ToPlainText(); got != "[$&]" {
			t.Errorf("PCRE output %q", got)
		}
		if got := w.explanation.ToPlainText(); !strings.Contains(got, `named "x"`) {
			t.Errorf("PCRE explanation %q", got)
		}
	})

	choose("JavaScript")
	mainthread.Wait(func() {
		if got := w.regexTitle.Text(); got != "Regular Expression (JavaScript)" {
			t.Errorf("JavaScript title %q", got)
		}
		if got := offered(); !slices.Equal(got, []string{"g", "i", "m", "s", "u", "y"}) {
			t.Errorf("JavaScript flags %q", got)
		}
		// U is not one of its flags, so it has gone.
		if w.config.Flags != "gi" {
			t.Errorf("JavaScript kept flags %q", w.config.Flags)
		}
		if got := w.fullPattern(); got != `/(?<x>a)\w/gi` {
			t.Errorf("JavaScript full pattern %q", got)
		}
		if got := w.output.ToPlainText(); got != "[ab]" {
			t.Errorf("JavaScript output %q", got)
		}
		w.regex.SetText("(?i)a")
	})
	settle(t, w)
	mainthread.Wait(func() {
		// JavaScript has no such syntax, and the field does not take it as flags.
		if w.regex.Text() != "(?i)a" || w.errorLabel.IsHidden() {
			t.Errorf("field %q, error %q", w.regex.Text(), w.errorLabel.Text())
		}
		w.regex.SetText("(?<x>a)")
	})
	settle(t, w)

	choose("Go")
	mainthread.Wait(func() {
		if got := w.inlineFlags.Text(); got != "(?i)" {
			t.Errorf("Go inline flags %q", got)
		}
		w.Close()
	})
}

func TestSettingsDialog(t *testing.T) {
	w, path := open(t, settings.Default())
	mainthread.Wait(func() {
		d := newSettingsDialog(w)
		if got := d.tabs.TabText(0); got != "User interface" {
			t.Errorf("tab %q", got)
		}
		if got := d.heading.Text(); got != "Appearance" {
			t.Errorf("heading %q", got)
		}
		if got := d.theme.ItemText(0); got != "Follow the system" {
			t.Errorf("first theme %q", got)
		}
		if d.MinimumWidth() != settingsLeastWidth || d.MinimumHeight() != settingsLeastHeight {
			t.Errorf("minimum size %dx%d", d.MinimumWidth(), d.MinimumHeight())
		}
		if d.Width() < settingsWidth {
			t.Errorf("opens %d wide", d.Width())
		}
		d.theme.SetCurrentIndex(2)
		d.Reject()
		if w.config.Theme != "" {
			t.Errorf("cancelled, but theme is %q", w.config.Theme)
		}
		d.theme.SetCurrentIndex(2)
		d.buttons.Accepted()
		if w.config.Theme != settings.ThemeDark {
			t.Errorf("theme %q", w.config.Theme)
		}
		d.DeleteLater()
	})
	if got := settings.Load(path).Theme; got != settings.ThemeDark {
		t.Errorf("saved theme %q", got)
	}
	mainthread.Wait(func() {
		applyTheme("")
		w.Close()
	})
}

// A handle shows its dots, in a neutral colour, over nothing but the window.
func TestGripDots(t *testing.T) {
	s := settings.Default()
	s.Mode = settings.ModeSubstitute
	w, _ := open(t, s)
	mainthread.Wait(func() {
		for _, theme := range []string{settings.ThemeLight, settings.ThemeDark} {
			applyTheme(theme)
			for _, splitter := range []*qt.QSplitter{w.left, w.side, w.outer} {
				image := splitter.Handle(1).Grab().ToImage()
				centre := image.PixelColor(image.Width()/2, image.Height()/2)
				corner := image.PixelColor(0, 0)
				if centre.Name() == corner.Name() {
					t.Errorf("%s: no dot at the centre of the handle", theme)
				}
				if centre.Red() != centre.Green() || centre.Green() != centre.Blue() {
					t.Errorf("%s: coloured dot %s", theme, centre.Name())
				}
			}
		}
		applyTheme("")
		w.Close()
	})
}

// Each example runs on its engine, and finds what it sets out to.
func TestExampleResults(t *testing.T) {
	for _, e := range examples {
		r := engine.Evaluate(engine.Request{
			Engine: engine.ByName(e.engine), Pattern: e.pattern, Flags: e.flags, Text: e.text,
			Substitute: e.replacement != "", Replacement: e.replacement,
		})
		if r.Error != "" {
			t.Errorf("%s: %s", e.name, r.Error)
		}
		if len(r.Matches) != e.expected {
			t.Errorf("%s: %d matches, want %d", e.name, len(r.Matches), e.expected)
		}
		if flags, _ := engine.ByName(e.engine).SplitInlineFlags(e.pattern); flags != "" {
			t.Errorf("%s: flags belong in its flags, not its pattern", e.name)
		}
	}
}

// Each engine has its own submenu of examples, and every example is on one.
func TestExamplesByEngine(t *testing.T) {
	counts := map[string]int{}
	for _, e := range examples {
		counts[e.engine]++
	}
	w, _ := open(t, settings.Default())
	mainthread.Wait(func() {
		submenus := w.examplesMenu.Actions()
		if len(submenus) != len(engine.All()) {
			t.Fatalf("%d submenus", len(submenus))
		}
		for i, candidate := range engine.All() {
			if got := submenus[i].Text(); got != candidate.Name()+submenuArrowRoom {
				t.Errorf("submenu %d is %q", i, got)
			}
			if counts[candidate.Name()] < 5 {
				t.Errorf("%s has %d examples", candidate.Name(), counts[candidate.Name()])
			}
			delete(counts, candidate.Name())
		}
		if len(counts) != 0 {
			t.Errorf("examples for no engine: %v", counts)
		}
		for i, action := range w.exampleActions {
			if action == nil {
				t.Errorf("%s is on no menu", examples[i].name)
			}
		}
		w.Close()
	})
}

func TestExamplesMenu(t *testing.T) {
	s := settings.Default()
	s.Pattern, s.Text, s.Replacement, s.Flags = "old", "old", "old", "U"
	w, _ := open(t, s)
	for i, e := range examples {
		mainthread.Wait(func() { w.exampleActions[i].Trigger() })
		settle(t, w)
		mainthread.Wait(func() {
			if w.regex.Text() != e.pattern || w.text.ToPlainText() != e.text || w.replacement.Text() != e.replacement {
				t.Errorf("%s: fields not replaced", e.name)
			}
			if w.engine.Name() != e.engine {
				t.Errorf("%s: engine %q", e.name, w.engine.Name())
			}
			if w.config.Flags != e.flags {
				t.Errorf("%s: flags %q", e.name, w.config.Flags)
			}
			if w.config.Mode != e.mode() || w.substitution.IsVisible() != (e.replacement != "") {
				t.Errorf("%s: mode %q", e.name, w.config.Mode)
			}
			for j, mode := range w.modes {
				if w.modeActions[j].IsChecked() != (mode == e.mode()) {
					t.Errorf("%s: menu shows the wrong mode", e.name)
				}
			}
			if n := w.matches.TopLevelItemCount(); n != e.expected {
				t.Errorf("%s: %d matches listed", e.name, n)
			}
			if e.replacement != "" && w.output.ToPlainText() == e.text {
				t.Errorf("%s: output %q", e.name, w.output.ToPlainText())
			}
		})
	}
	mainthread.Wait(func() { w.Close() })
}

// Quitting from the menu keeps what was entered.
func TestQuitSaves(t *testing.T) {
	w, path := open(t, settings.Default())
	mainthread.Wait(func() {
		w.regex.SetText("kept")
		w.text.SetPlainText("kept text")
	})
	settle(t, w)
	mainthread.Wait(func() {
		w.quitAction.Trigger()
	})
	if got := settings.Load(path); got.Pattern != "kept" || got.Text != "kept text" {
		t.Errorf("saved %q and %q", got.Pattern, got.Text)
	}
}

// What is entered is saved soon after, with no need for the window to close.
func TestAutosave(t *testing.T) {
	w, path := open(t, settings.Default())
	mainthread.Wait(func() {
		w.regex.SetText("typed")
		w.text.SetPlainText("typed text")
		w.replacement.SetText("typed replacement")
		for i, e := range engine.All() {
			if e.Name() == "JavaScript" {
				w.engineActions[i].Trigger()
			}
		}
	})
	settle(t, w)
	saved := func() bool {
		got := settings.Load(path)
		return got.Pattern == "typed" && got.Text == "typed text" && got.Replacement == "typed replacement" && got.Engine == "JavaScript"
	}
	if saved() {
		t.Error("saved at once, on every change")
	}
	for range 100 {
		if saved() {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}
	if !saved() {
		t.Error("not saved after the delay")
	}
	mainthread.Wait(func() { w.Close() })

	// It all comes back, with the engine chosen in the menu and its title.
	again, _ := open(t, settings.Load(path))
	mainthread.Wait(func() {
		if got := again.engine.Name(); got != "JavaScript" {
			t.Errorf("engine %q", got)
		}
		if got := again.regexTitle.Text(); got != "Regular Expression (JavaScript)" {
			t.Errorf("title %q", got)
		}
		for i, e := range engine.All() {
			if again.engineActions[i].IsChecked() != (e.Name() == "JavaScript") {
				t.Errorf("menu shows %s wrongly", e.Name())
			}
		}
		if got := again.regex.Text(); got != "typed" {
			t.Errorf("pattern %q", got)
		}
		again.Close()
	})
}

func TestRegexHighlighting(t *testing.T) {
	s := settings.Default()
	s.Pattern, s.Flags = `(é\d)+`, "gi"
	w, _ := open(t, s)
	colours := syntaxFor(false)
	mainthread.Wait(func() {
		selections := w.regex.ExtraSelections()
		// Brackets, the escape and the quantifier; the literal is left plain.
		if len(selections) != 4 {
			t.Fatalf("%d highlighted", len(selections))
		}
		cursor := selections[1].Cursor()
		format := selections[1].Format()
		background, foreground := format.Background(), format.Foreground()
		if cursor.SelectedText() != `\d` || cursor.SelectionStart() != 2 {
			t.Errorf("highlighted %q at %d", cursor.SelectedText(), cursor.SelectionStart())
		}
		if background.Color().Name() != colours.meta.background || foreground.Color().Name() != colours.meta.text {
			t.Errorf("colours %s on %s", foreground.Color().Name(), background.Color().Name())
		}
		// It stays highlighted while it does not compile.
		w.regex.SetText(`(\d`)
	})
	settle(t, w)
	mainthread.Wait(func() {
		if got := len(w.regex.ExtraSelections()); got != 2 {
			t.Errorf("%d highlighted in an invalid pattern", got)
		}
		w.Close()
	})
}

// However text reaches it, the regex field holds one line.
func TestRegexFieldSingleLine(t *testing.T) {
	s := settings.Default()
	s.Pattern, s.Flags = "", "i"
	w, _ := open(t, s)
	mainthread.Wait(func() {
		qt.QGuiApplication_Clipboard().SetText("a\nb\r\nc")
		w.regex.Paste()
		if got := w.regex.Text(); got != "abc" {
			t.Errorf("pasted %q", got)
		}
		enter := qt.NewQKeyEvent(qt.QEvent__KeyPress, int(qt.Key_Return), qt.NoModifier)
		qt.QCoreApplication_SendEvent(w.regex.QObject, enter.QEvent)
		enter.Delete()
		if got := w.regex.Text(); got != "abc" {
			t.Errorf("after Enter %q", got)
		}
		w.regex.SetText("x\ny")
		if got := w.regex.Text(); got != "xy" {
			t.Errorf("set %q", got)
		}
		if got := w.regex.CursorPosition(); got != 2 {
			t.Errorf("cursor at %d", got)
		}
		if w.regex.Height() != w.regex.MinimumHeight() || w.regex.Height() != w.regex.MaximumHeight() {
			t.Errorf("height not fixed: %d", w.regex.Height())
		}
		w.Close()
	})
}

func spanColours(t *testing.T, edit *qt.QPlainTextEdit, colour string) []string {
	t.Helper()
	var found []string
	for _, selection := range edit.ExtraSelections() {
		format := selection.Format()
		background := format.Background()
		if background.Color().Name() == colour {
			cursor := selection.Cursor()
			found = append(found, cursor.SelectedText())
		}
	}
	return found
}

func TestHover(t *testing.T) {
	s := settings.Default()
	s.Pattern, s.Text, s.Flags = `x(a)(b)?`, "xa xab", "g"
	s.Theme = settings.ThemeLight
	w, _ := open(t, s)
	colours := syntaxFor(false)
	// A group in the pattern lights it up, in every match and in the list.
	mainthread.Wait(func() {
		w.hoverPatternAt(2) // The "a".
		if got := w.hover; got != (hoverTarget{allMatches, 1}) {
			t.Fatalf("hover %+v", got)
		}
		if got := spanColours(t, w.text, colours.edge(1)); !slices.Equal(got, []string{"a", "a"}) {
			t.Errorf("text %q", got)
		}
		if got := spanColours(t, w.regex.QPlainTextEdit, colours.edge(1)); !slices.Equal(got, []string{"(a)"}) {
			t.Errorf("pattern %q", got)
		}
		brush := w.matches.TopLevelItem(1).Child(0).Background(0)
		if got := brush.Color().Name(); brush.Style() == qt.NoBrush || got != colours.fill(1, 1) {
			t.Errorf("row %s", got)
		}
		// A group that took no part in a match is not lit up in that match.
		w.hoverPatternAt(5)
		if got := spanColours(t, w.text, colours.edge(2)); !slices.Equal(got, []string{"b"}) {
			t.Errorf("optional group %q", got)
		}
		// Outside any group is the whole match.
		w.hoverPatternAt(0)
		if got := spanColours(t, w.text, colours.edge(0)); !slices.Equal(got, []string{"xa", "xab"}) {
			t.Errorf("whole match %q", got)
		}
		// The text lights up its own match alone, and the group in the pattern.
		w.hoverTextAt(4) // The second "a".
		if got := w.hover; got != (hoverTarget{1, 1}) {
			t.Errorf("hover %+v", got)
		}
		if got := spanColours(t, w.text, colours.edge(1)); len(got) != 1 {
			t.Errorf("text %q", got)
		}
		if got := spanColours(t, w.regex.QPlainTextEdit, colours.edge(1)); !slices.Equal(got, []string{"(a)"}) {
			t.Errorf("pattern %q", got)
		}
		w.hoverTextAt(2) // The space between the matches.
		if w.hover != noHover {
			t.Errorf("hover %+v", w.hover)
		}
		// So does a row.
		w.hoverRow(w.matches.TopLevelItem(0).Child(0))
		if got := w.hover; got != (hoverTarget{0, 1}) {
			t.Errorf("hover %+v", got)
		}
		if got := spanColours(t, w.text, colours.edge(1)); len(got) != 1 {
			t.Errorf("text %q", got)
		}
		// Nothing stays lit up.
		w.setHover(noHover)
		if got := spanColours(t, w.text, colours.edge(1)); len(got) != 0 {
			t.Errorf("text %q", got)
		}
		for i := range 2 {
			brush := w.matches.TopLevelItem(i).Child(0).Background(0)
			if brush.Style() != qt.NoBrush {
				t.Errorf("row %d stays lit", i)
			}
		}
		// A new result starts unlit, without touching old rows.
		w.hoverRow(w.matches.TopLevelItem(0))
		w.regex.SetText("a")
	})
	settle(t, w)
	mainthread.Wait(func() {
		if w.hover != noHover {
			t.Errorf("hover after a change %+v", w.hover)
		}
		w.hoverPatternAt(0)
		applyTheme(settings.ThemeDark)
	})
	mainthread.Wait(func() {})
	mainthread.Wait(func() {
		w.setHover(noHover)
		applyTheme("")
		w.Close()
	})
}

func TestReplacementHighlighting(t *testing.T) {
	s := settings.Default()
	s.Mode, s.Theme = settings.ModeSubstitute, settings.ThemeLight
	s.Pattern, s.Text, s.Flags, s.Replacement = `(?P<a>\d)-(\d)`, "1-2 3-4", "g", "${a}/$2$$"
	w, _ := open(t, s)
	colours := syntaxFor(false)
	mainthread.Wait(func() {
		if got := w.output.ToPlainText(); got != "1/2$ 3/4$" {
			t.Fatalf("output %q", got)
		}
		// References and escapes are highlighted, and the rest left plain.
		if got := spanColours(t, w.replacement.QPlainTextEdit, colours.groups[0].background); !slices.Equal(got, []string{"${a}", "$2"}) {
			t.Errorf("references %q", got)
		}
		if got := spanColours(t, w.replacement.QPlainTextEdit, colours.escaped.background); !slices.Equal(got, []string{"$$"}) {
			t.Errorf("escapes %q", got)
		}
		// The result has the colours of the text, match by match.
		if got := spanColours(t, w.output, colours.fill(1, 0)); !slices.Equal(got, []string{"1"}) {
			t.Errorf("group 1 of match 1 %q", got)
		}
		if got := spanColours(t, w.output, colours.fill(2, 1)); !slices.Equal(got, []string{"4"}) {
			t.Errorf("group 2 of match 2 %q", got)
		}
		if got := spanColours(t, w.output, colours.fill(0, 1)); !slices.Equal(got, []string{"3/4$"}) {
			t.Errorf("match 2 %q", got)
		}

		// A reference lights up its group everywhere.
		w.hoverReplacementAt(5) // The "$2".
		if w.hover != (hoverTarget{allMatches, 2}) {
			t.Fatalf("hover %+v", w.hover)
		}
		if got := spanColours(t, w.replacement.QPlainTextEdit, colours.edge(2)); !slices.Equal(got, []string{"$2"}) {
			t.Errorf("reference %q", got)
		}
		if got := spanColours(t, w.output, colours.edge(2)); !slices.Equal(got, []string{"2", "4"}) {
			t.Errorf("result %q", got)
		}
		if got := spanColours(t, w.text, colours.edge(2)); !slices.Equal(got, []string{"2", "4"}) {
			t.Errorf("text %q", got)
		}
		if got := spanColours(t, w.regex.QPlainTextEdit, colours.edge(2)); !slices.Equal(got, []string{`(\d)`}) {
			t.Errorf("pattern %q", got)
		}
		// Literal parts stand for the whole match.
		w.hoverReplacementAt(4) // The "/".
		if w.hover != (hoverTarget{allMatches, 0}) {
			t.Errorf("hover %+v", w.hover)
		}
		if got := spanColours(t, w.replacement.QPlainTextEdit, colours.edge(0)); !slices.Equal(got, []string{"${a}/$2$$"}) {
			t.Errorf("whole replacement %q", got)
		}
		// The result lights up its own match alone.
		w.hoverOutputAt(7) // The "4".
		if w.hover != (hoverTarget{1, 2}) {
			t.Errorf("hover %+v", w.hover)
		}
		if got := spanColours(t, w.text, colours.edge(2)); !slices.Equal(got, []string{"4"}) {
			t.Errorf("text %q", got)
		}
		w.hoverOutputAt(6) // The "/" of the second.
		if w.hover != (hoverTarget{1, 0}) {
			t.Errorf("hover %+v", w.hover)
		}
		w.hoverOutputAt(4) // The space, which is not replaced.
		if w.hover != noHover {
			t.Errorf("hover %+v", w.hover)
		}
		// So does hovering elsewhere light up the result.
		w.hoverTextAt(0)
		if got := spanColours(t, w.output, colours.edge(1)); !slices.Equal(got, []string{"1"}) {
			t.Errorf("result from text %q", got)
		}
		// A reference to no group has nothing to light up.
		w.replacement.SetText("$9")
	})
	settle(t, w)
	mainthread.Wait(func() {
		w.hoverReplacementAt(0)
		if w.hover != noHover {
			t.Errorf("hover %+v", w.hover)
		}
		// Enter does nothing here either.
		enter := qt.NewQKeyEvent(qt.QEvent__KeyPress, int(qt.Key_Return), qt.NoModifier)
		qt.QCoreApplication_SendEvent(w.replacement.QObject, enter.QEvent)
		enter.Delete()
		if got := w.replacement.Text(); got != "$9" {
			t.Errorf("after Enter %q", got)
		}
		applyTheme("")
		w.Close()
	})
}

// A real movement of the mouse reaches the hover.
func TestHoverByMouse(t *testing.T) {
	s := settings.Default()
	s.Pattern, s.Text, s.Flags = `(a)`, "a", "g"
	w, _ := open(t, s)
	mainthread.Wait(func() {
		move := func(edit *qt.QPlainTextEdit, x, y float64) {
			point := qt.NewQPointF3(x, y)
			event := qt.NewQMouseEvent(qt.QEvent__MouseMove, point, qt.NoButton, qt.NoButton, qt.NoModifier)
			qt.QCoreApplication_SendEvent(edit.Viewport().QObject, event.QEvent)
			event.Delete()
			point.Delete()
		}
		cursor := w.regex.TextCursor()
		cursor.SetPosition(1)
		at := w.regex.CursorRect(cursor)
		move(w.regex.QPlainTextEdit, float64(at.Left()+2), float64(at.Top()+at.Height()/2))
		if got := w.hover; got != (hoverTarget{allMatches, 1}) {
			t.Errorf("over the group: %+v", got)
		}
		// Far past the end of the text is over no character.
		move(w.regex.QPlainTextEdit, float64(at.Left()+300), float64(at.Top()+at.Height()/2))
		if w.hover != noHover {
			t.Errorf("past the end: %+v", w.hover)
		}
		w.Close()
	})
}

// TestScreenshot renders the window to the path in REGHEAD_SCREENSHOT.
func TestScreenshot(t *testing.T) {
	path := os.Getenv("REGHEAD_SCREENSHOT")
	if path == "" {
		t.Skip("REGHEAD_SCREENSHOT is not set")
	}
	s := settings.Default()
	s.Mode = settings.ModeSubstitute
	s.Flags = "gi"
	s.Theme = os.Getenv("REGHEAD_THEME")
	w, _ := open(t, s)
	if !mainthread.Wait2(func() bool { return w.Grab().Save(path) }) {
		t.Error("the screenshot was not saved")
	}
	mainthread.Wait(func() { w.Close() })
}
