package ui

import (
	"runtime"

	qt "github.com/mappu/miqt/qt6"
)

const (
	// settingsMargin is the space between the edge of a tab and what is on it.
	settingsMargin = 20
	// settingsWidth is the width the dialog opens at, and settingsLeastWidth and
	// settingsLeastHeight the smallest it can be made.
	settingsWidth       = 600
	settingsLeastWidth  = 560
	settingsLeastHeight = 300
)

// settingsDialog is the program's settings, which take effect when accepted.
type settingsDialog struct {
	*qt.QDialog
	tabs    *qt.QTabWidget
	heading *qt.QLabel
	theme   *qt.QComboBox
	buttons *qt.QDialogButtonBox
}

// newSettingsDialog returns the dialog, showing the window's settings.
func newSettingsDialog(w *Window) *settingsDialog {
	d := &settingsDialog{QDialog: qt.NewQDialog(w.QWidget)}
	d.SetWindowTitle("Settings")
	d.SetMinimumSize2(settingsLeastWidth, settingsLeastHeight)

	d.tabs = qt.NewQTabWidget2()
	d.tabs.AddTab(d.interfaceTab(w.config.Theme), "User interface")

	d.buttons = qt.NewQDialogButtonBox4(qt.QDialogButtonBox__Ok | qt.QDialogButtonBox__Cancel)
	d.buttons.OnAccepted(func() {
		w.setTheme(themes[d.theme.CurrentIndex()].name)
		d.Accept()
	})
	d.buttons.OnRejected(d.Reject)

	layout := qt.NewQVBoxLayout(d.QWidget)
	layout.AddWidget2(d.tabs.QWidget, 1)
	layout.AddWidget(d.buttons.QWidget)

	layout.Activate()
	hint := d.SizeHint()
	d.Resize(max(settingsWidth, hint.Width()), max(settingsLeastHeight, hint.Height()))
	runtime.KeepAlive(hint)
	return d
}

// interfaceTab makes the tab of how the program looks.
func (d *settingsDialog) interfaceTab(current string) *qt.QWidget {
	page := qt.NewQWidget2()
	// Some styles draw the pages of tabs lighter than the window, which in a
	// dark theme is a slab of grey.
	page.SetBackgroundRole(qt.QPalette__Window)
	page.SetAutoFillBackground(true)

	d.heading = titleLabel("Appearance")
	d.theme = qt.NewQComboBox2()
	for i, t := range themes {
		d.theme.AddItem(t.label)
		if t.name == current {
			d.theme.SetCurrentIndex(i)
		}
	}
	form := qt.NewQFormLayout2()
	form.AddRow3("Theme", d.theme.QWidget)

	layout := qt.NewQVBoxLayout(page)
	layout.SetContentsMargins(settingsMargin, settingsMargin, settingsMargin, settingsMargin)
	layout.AddWidget(d.heading.QWidget)
	layout.AddLayout(form.QLayout)
	layout.AddStretch()
	return page
}
