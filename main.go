// RegHead is a desktop regular expression tester.
package main

import (
	"fmt"
	"os"
	"os/signal"
	"runtime"
	"syscall"

	qt "github.com/mappu/miqt/qt6"
	"github.com/mappu/miqt/qt6/mainthread"

	"github.com/nomad-software/reghead/internal/appicon"
	"github.com/nomad-software/reghead/internal/desktop"
	"github.com/nomad-software/reghead/internal/settings"
	"github.com/nomad-software/reghead/internal/ui"
)

func main() {
	path, err := settings.Path()
	if err == nil {
		err = settings.Prepare(path)
	}
	if err != nil {
		path = "settings.json"
		fmt.Fprintln(os.Stderr, "reghead: keeping settings in the current directory:", err)
	}

	qt.NewQApplication(os.Args)
	qt.QCoreApplication_SetApplicationName("reghead")
	// Under Wayland this is the name a window goes by, which a desktop matches to the
	// desktop entry of that name for its icon and its place in the task bar.
	qt.QGuiApplication_SetDesktopFileName(desktop.Name)

	// On Linux the desktop is given an entry for the program, which starts the copy
	// that is running now, so that it can be started from the desktop's menus and kept
	// in a dock.
	if runtime.GOOS == "linux" {
		if err := desktop.Install(appicon.PNG); err != nil {
			fmt.Fprintln(os.Stderr, "reghead: the desktop entry could not be written:", err)
		}
	}

	window := ui.NewWindow(path)
	window.Start()

	// Being told to stop, as by Ctrl+C in a terminal or at logout, closes the window as
	// usual, which saves the settings.
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM, syscall.SIGHUP)
	go func() {
		<-stop
		mainthread.Start(func() {
			window.Close()
			qt.QCoreApplication_Quit()
		})
	}()

	qt.QApplication_Exec()
}
