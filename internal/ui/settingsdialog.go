package ui

import (
	"github.com/nomad-software/qute/qtcore"
	"github.com/nomad-software/qute/qtgui"
	"github.com/nomad-software/qute/qtwidgets"
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
	*qtwidgets.QDialog
	tabs    *qtwidgets.QTabWidget
	heading *qtwidgets.QLabel
	theme   *qtwidgets.QComboBox
	buttons *qtwidgets.QDialogButtonBox
}

// newSettingsDialog returns the dialog, showing the window's settings.
func newSettingsDialog(w *Window) *settingsDialog {
	d := &settingsDialog{QDialog: qtwidgets.NewQDialog(w)}
	d.SetWindowTitle("Settings")
	d.SetMinimumSize(settingsLeastWidth, settingsLeastHeight)

	d.tabs = qtwidgets.NewQTabWidget(nil)
	d.tabs.AddTab(d.interfaceTab(w.config.Theme), "User interface")

	d.buttons = qtwidgets.NewQDialogButtonBoxWithStandardButton(qtwidgets.QDialogButtonBoxOk|qtwidgets.QDialogButtonBoxCancel, nil)
	d.buttons.OnAccepted(func() {
		w.setTheme(themes[d.theme.CurrentIndex()].name)
		d.Accept()
	})
	d.buttons.OnRejected(d.Reject)

	layout := qtwidgets.NewQVBoxLayout(d)
	layout.AddWidgetFull(d.tabs, 1, 0)
	layout.AddWidget(d.buttons)

	layout.Activate()
	hint := d.SizeHint()
	d.Resize(max(settingsWidth, hint.Width), max(settingsLeastHeight, hint.Height))
	return d
}

// interfaceTab makes the tab of how the program looks.
func (d *settingsDialog) interfaceTab(current string) *qtwidgets.QWidget {
	page := qtwidgets.NewQWidget(nil)
	// Some styles draw the pages of tabs lighter than the window, which in a
	// dark theme is a slab of grey.
	page.SetBackgroundRole(qtgui.QPaletteWindow)
	page.SetAutoFillBackground(true)

	d.heading = titleLabel("Appearance")
	d.theme = qtwidgets.NewQComboBox(nil)
	for i, t := range themes {
		d.theme.AddItem(t.label)
		if t.name == current {
			d.theme.SetCurrentIndex(i)
		}
	}
	form := qtwidgets.NewQFormLayout(nil)
	form.AddRowText("Theme", d.theme)

	layout := qtwidgets.NewQVBoxLayout(page)
	layout.SetContentsMargins(qtcore.QMargins{Left: settingsMargin, Top: settingsMargin, Right: settingsMargin, Bottom: settingsMargin})
	layout.AddWidget(d.heading)
	layout.AddLayout(form)
	layout.AddStretch()
	return page
}
