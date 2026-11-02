package ui

import (
	qt "github.com/mappu/miqt/qt6"

	"github.com/nomad-software/reghead/internal/engine"
	"github.com/nomad-software/reghead/internal/settings"
)

// modes are the modes, in the order of their menu.
var modes = []string{settings.ModeMatch, settings.ModeSubstitute}

// radioGroup adds actions to a menu of which one is checked at a time.
func radioGroup(menu *qt.QMenu, labels []string, checked int, chosen func(i int)) []*qt.QAction {
	group := qt.NewQActionGroup(menu.QObject)
	actions := make([]*qt.QAction, len(labels))
	for i, label := range labels {
		a := group.AddActionWithText(label)
		a.SetCheckable(true)
		a.SetChecked(i == checked)
		a.OnTriggered(func() { chosen(i) })
		menu.QWidget.AddAction(a)
		actions[i] = a
	}
	return actions
}

// shortcut gives an action its keys.
func shortcut(a *qt.QAction, keys string) {
	seq := qt.NewQKeySequence2(keys)
	a.SetShortcut(seq)
	seq.Delete()
}

// makeMenus makes the menu bar.
func (w *Window) makeMenus() {
	bar := w.MenuBar()

	file := bar.AddMenuWithTitle("&File")
	preferences := file.AddActionWithText("&Settings...")
	shortcut(preferences, "Ctrl+,")
	preferences.OnTriggered(w.showSettings)
	file.AddSeparator()
	w.quitAction = file.AddActionWithText("&Quit")
	shortcut(w.quitAction, "Ctrl+Q")
	w.quitAction.OnTriggered(func() { w.Close() })

	mode := bar.AddMenuWithTitle("&Mode")
	checked := 0
	if w.config.Mode == settings.ModeSubstitute {
		checked = 1
	}
	w.modeActions = radioGroup(mode, []string{"&Match", "&Substitution"}, checked, func(i int) { w.setMode(modes[i]) })
	shortcut(w.modeActions[0], "Ctrl+1")
	shortcut(w.modeActions[1], "Ctrl+2")

	engines := engine.All()
	names := make([]string, len(engines))
	current := 0
	for i, e := range engines {
		names[i] = e.Name()
		if e == w.engine {
			current = i
		}
	}
	w.engineActions = radioGroup(bar.AddMenuWithTitle("&Engine"), names, current, func(i int) {
		w.setEngine(engines[i])
		w.regexChanged()
	})

	w.examplesMenu = bar.AddMenuWithTitle("E&xamples")
	w.exampleActions = make([]*qt.QAction, len(examples))
	for _, candidate := range engines {
		// Styles can leave the arrow of a submenu right against its text, so the title
		// keeps some room for it.
		submenu := w.examplesMenu.AddMenuWithTitle(candidate.Name() + submenuArrowRoom)
		for i, e := range examples {
			if e.engine == candidate.Name() {
				action := submenu.AddActionWithText(e.name)
				action.OnTriggered(func() { w.showExample(e) })
				w.exampleActions[i] = action
			}
		}
	}

	help := bar.AddMenuWithTitle("&Help")
	about := help.AddActionWithText("&About")
	shortcut(about, "Ctrl+I")
	about.OnTriggered(func() { showAbout(w.QWidget) })
}

// regexTitleText returns the title of the regex field, naming the engine in brackets.
func regexTitleText(e engine.Engine) string {
	return "Regular Expression (" + e.Name() + ")"
}

// setEngine changes the engine, and all that depends on it: the flags it offers,
// keeping those it shares with the last, how they show in the regex field, and the
// syntax of the replacement.
func (w *Window) setEngine(e engine.Engine) {
	w.engine = e
	w.regexTitle.SetText(regexTitleText(e))
	w.replacement.SetPlaceholderText(e.ReplacementHint())
	w.config.Flags = w.offeredFlags(w.config.Flags)
	w.fillFlagsMenu()
}

// showExample replaces what is entered with an example.
func (w *Window) showExample(e example) {
	for i, candidate := range engine.All() {
		if candidate.Name() == e.engine {
			w.engineActions[i].SetChecked(true)
			w.setEngine(candidate)
		}
	}
	w.config.Flags = e.flags
	w.fillFlagsMenu()
	for i, mode := range modes {
		if mode == e.mode() {
			w.modeActions[i].SetChecked(true)
		}
	}
	w.replacement.SetText(e.replacement)
	w.text.SetPlainText(e.text)
	w.regex.SetText(e.pattern)
	w.setMode(e.mode())
}

// showSettings shows the settings dialog until it is answered.
func (w *Window) showSettings() {
	d := newSettingsDialog(w)
	defer d.DeleteLater()
	d.Exec()
}

// setTheme changes the theme and saves it.
func (w *Window) setTheme(theme string) {
	if theme == w.config.Theme {
		return
	}
	w.config.Theme = theme
	applyTheme(theme)
	w.saveSettings()
}

// setMode switches between matching and substitution, keeping the size of the
// panes for when substitution is shown again.
func (w *Window) setMode(mode string) {
	if mode == w.config.Mode {
		return
	}
	w.rememberTextHeight()
	w.config.Mode = mode
	w.substitution.SetVisible(mode == settings.ModeSubstitute)
	w.restoreTextHeight()
	w.refresh()
}
