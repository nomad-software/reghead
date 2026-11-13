package ui

import (
	"github.com/nomad-software/qute/qtgui"
	"github.com/nomad-software/qute/qtwidgets"

	"github.com/nomad-software/reghead/internal/engine"
	"github.com/nomad-software/reghead/internal/settings"
)

// modes are the modes, in the order of their menu.
var modes = []string{settings.ModeMatch, settings.ModeSubstitute}

// radioGroup adds actions to a menu of which one is checked at a time.
func radioGroup(menu *qtwidgets.QMenu, labels []string, checked int, chosen func(i int)) []*qtgui.QAction {
	group := qtgui.NewQActionGroup(menu)
	actions := make([]*qtgui.QAction, len(labels))
	for i, label := range labels {
		a := group.AddActionText(label)
		a.SetCheckable(true)
		a.SetChecked(i == checked)
		a.OnTriggered(func(bool) { chosen(i) })
		menu.AddAction(a)
		actions[i] = a
	}
	return actions
}

// shortcut gives an action its keys.
func shortcut(a *qtgui.QAction, keys string) {
	a.SetShortcut(qtgui.NewQKeySequenceWithKey(keys))
}

// makeMenus makes the menu bar.
func (w *Window) makeMenus() {
	bar := w.MenuBar()

	file := bar.AddMenuTitle("&File")
	preferences := file.AddActionText("&Settings...")
	shortcut(preferences, "Ctrl+,")
	preferences.OnTriggered(func(bool) { w.showSettings() })
	file.AddSeparator()
	w.quitAction = file.AddActionText("&Quit")
	shortcut(w.quitAction, "Ctrl+Q")
	w.quitAction.OnTriggered(func(bool) { w.Close() })

	mode := bar.AddMenuTitle("&Mode")
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
	w.engineActions = radioGroup(bar.AddMenuTitle("&Engine"), names, current, func(i int) {
		w.setEngine(engines[i])
		w.regexChanged()
	})

	w.examplesMenu = bar.AddMenuTitle("E&xamples")
	w.exampleActions = make([]*qtgui.QAction, len(examples))
	for _, candidate := range engines {
		// Styles can leave the arrow of a submenu right against its text, so the title
		// keeps some room for it.
		submenu := w.examplesMenu.AddMenuTitle(candidate.Name() + submenuArrowRoom)
		for i, e := range examples {
			if e.engine == candidate.Name() {
				action := submenu.AddActionText(e.name)
				action.OnTriggered(func(bool) { w.showExample(e) })
				w.exampleActions[i] = action
			}
		}
	}

	help := bar.AddMenuTitle("&Help")
	about := help.AddActionText("&About")
	shortcut(about, "Ctrl+I")
	about.OnTriggered(func(bool) { showAbout(w) })
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
