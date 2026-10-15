package main

import _ "embed"

// DefaultIcon is the system tray icon. On Windows, getlantern/systray only
// accepts ICO data (a PNG fails with "Unable to set icon" and leaves the tray
// empty, so a window hidden on close could no longer be reopened or quit).
//
//go:embed build/windows/icon.ico
var DefaultIcon []byte
