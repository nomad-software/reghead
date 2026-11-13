package ui

import (
	"runtime/debug"

	"github.com/nomad-software/qute/qtcore"
	"github.com/nomad-software/qute/qtgui"
	"github.com/nomad-software/qute/qtwidgets"

	"github.com/nomad-software/reghead/internal/appicon"
)

// appVersion returns the version Go recorded when the program was built.
func appVersion() string {
	if info, ok := debug.ReadBuildInfo(); ok && info.Main.Version != "" && info.Main.Version != "(devel)" {
		return info.Main.Version
	}
	return "dev"
}

// aboutText returns what the about box says.
func aboutText() string {
	return "<h2>RegHead</h2><p>Version " + appVersion() + "</p>" +
		"<p>A regular expression tester: match, substitute and have your patterns explained.</p>" +
		"<p>Copyright © 2026 Gary Willoughby. Licensed under the GNU GPL version 3.</p>" +
		"<p>Uses Qt " + qtcore.QLibraryInfoVersion().ToString() + " under the <a href=\"https://www.gnu.org/licenses/lgpl-3.0.html\">LGPLv3</a>, " +
		"dynamically linked so that it can be replaced (source at <a href=\"https://www.qt.io\">www.qt.io</a>), " +
		"through the <a href=\"https://github.com/nomad-software/qute\">qute</a> bindings under the GNU GPL version 3. " +
		"PCRE2 is Qt's, under its BSD licence. JavaScript is simulated on " +
		"<a href=\"https://github.com/dlclark/regexp2\">regexp2</a>, under the MIT licence.</p>"
}

// showAbout shows the about box until it is closed.
func showAbout(parent qtwidgets.QWidgetLike) {
	box := qtwidgets.NewQMessageBox(parent)
	defer box.DeleteLater()
	box.SetWindowTitle("About RegHead")
	box.SetTextFormat(qtcore.RichText)
	box.SetText(aboutText())
	box.SetIconPixmap(appIcon().PixmapExtent(96))
	box.Exec()
}

var appIconCache *qtgui.QIcon

// appIcon returns the icon of the program.
func appIcon() *qtgui.QIcon {
	if appIconCache == nil {
		pixmap := qtgui.NewQPixmap()
		pixmap.LoadFromData(appicon.PNG)
		appIconCache = qtgui.NewQIconWithPixmap(pixmap)
	}
	return appIconCache
}
