//go:build darwin && !cgo

package main

// Cross-compiled macOS builds have no cgo, which the macOS tray needs, so
// they run without a tray icon.

func trayAvailable() bool                      { return false }
func runTray(url, library string, quit func()) {}
func quitTray()                                {}
