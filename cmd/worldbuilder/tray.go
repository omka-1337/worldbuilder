//go:build !darwin || cgo

package main

import (
	"bytes"
	_ "embed"
	"encoding/binary"
	"log"
	"os"
	"path/filepath"
	"runtime"

	"fyne.io/systray"
)

//go:embed icon.png
var iconPNG []byte

// trayAvailable reports whether a tray icon can be shown. On Linux it needs
// a D-Bus session (absent over SSH or in a bare console), and the tray
// library cannot shut down cleanly without one.
func trayAvailable() bool {
	if runtime.GOOS != "linux" && runtime.GOOS != "freebsd" && runtime.GOOS != "openbsd" && runtime.GOOS != "netbsd" {
		return true
	}
	if os.Getenv("DBUS_SESSION_BUS_ADDRESS") != "" {
		return true
	}
	if dir := os.Getenv("XDG_RUNTIME_DIR"); dir != "" {
		if _, err := os.Stat(filepath.Join(dir, "bus")); err == nil {
			return true
		}
	}
	return false
}

// runTray shows the tray icon and blocks until quitTray is called. Its menu
// opens the app or the library folder, or stops the server through quit.
func runTray(url, library string, quit func()) {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("tray stopped: %v", r)
		}
	}()
	systray.Run(func() {
		if runtime.GOOS == "windows" {
			systray.SetIcon(icoFromPNG(iconPNG))
		} else {
			systray.SetIcon(iconPNG)
		}
		systray.SetTitle("Worldbuilder")
		systray.SetTooltip("Worldbuilder — " + url)
		open := systray.AddMenuItem("Відкрити Worldbuilder", url)
		folder := systray.AddMenuItem("Відкрити папку світів", library)
		systray.AddSeparator()
		stop := systray.AddMenuItem("Зупинити Worldbuilder", "")
		systray.SetOnTapped(func() { openBrowser(url) })
		go func() {
			for {
				select {
				case <-open.ClickedCh:
					openBrowser(url)
				case <-folder.ClickedCh:
					openBrowser(library) // the system opener shows a folder in the file manager
				case <-stop.ClickedCh:
					quit()
					return
				}
			}
		}()
	}, nil)
}

func quitTray() { systray.Quit() }

// icoFromPNG wraps a PNG in an ICO container, which Windows wants for tray
// icons; ICO files may hold PNG data as is.
func icoFromPNG(png []byte) []byte {
	var b bytes.Buffer
	le := binary.LittleEndian
	_ = binary.Write(&b, le, [3]uint16{0, 1, 1}) // reserved, type icon, one image
	// 0 for width and height means 256 pixels.
	_ = binary.Write(&b, le, struct {
		W, H, Colors, Reserved uint8
		Planes, BPP            uint16
		Size, Offset           uint32
	}{0, 0, 0, 0, 1, 32, uint32(len(png)), 6 + 16})
	b.Write(png)
	return b.Bytes()
}
