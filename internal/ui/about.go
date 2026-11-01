package ui

import (
	"runtime"
	"runtime/debug"

	qt "github.com/mappu/miqt/qt6"
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
	v := qt.QLibraryInfo_Version()
	defer runtime.KeepAlive(v)
	return "<h2>RegHead</h2><p>Version " + appVersion() + "</p>" +
		"<p>A regular expression tester: match, substitute and have your patterns explained.</p>" +
		"<p>Copyright © 2026 Gary Willoughby. Licensed under the GNU GPL version 3.</p>" +
		"<p>Uses Qt " + v.ToString() + " under the <a href=\"https://www.gnu.org/licenses/lgpl-3.0.html\">LGPLv3</a>, " +
		"dynamically linked so that it can be replaced (source at <a href=\"https://www.qt.io\">www.qt.io</a>), " +
		"through the <a href=\"https://github.com/mappu/miqt\">miqt</a> bindings under the MIT licence. " +
		"PCRE2 is Qt's, under its BSD licence. JavaScript is simulated on " +
		"<a href=\"https://github.com/dlclark/regexp2\">regexp2</a>, under the MIT licence.</p>"
}

// showAbout shows the about box until it is closed.
func showAbout(parent *qt.QWidget) {
	box := qt.NewQMessageBox(parent)
	defer box.DeleteLater()
	box.SetWindowTitle("About RegHead")
	box.SetTextFormat(qt.RichText)
	box.SetText(aboutText())
	pixmap := appIcon().PixmapWithExtent(96)
	box.SetIconPixmap(pixmap)
	runtime.KeepAlive(pixmap)
	box.Exec()
}

var appIconCache *qt.QIcon

// appIcon returns the icon of the program.
func appIcon() *qt.QIcon {
	if appIconCache == nil {
		pixmap := qt.NewQPixmap()
		defer pixmap.Delete()
		pixmap.LoadFromDataWithData(appicon.PNG)
		appIconCache = qt.NewQIcon2(pixmap)
	}
	return appIconCache
}
