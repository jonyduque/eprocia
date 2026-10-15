//go:build !windows

package ocr

import "os/exec"

// hideConsole is a no-op outside Windows, where child processes get no console window.
func hideConsole(cmd *exec.Cmd) {}
