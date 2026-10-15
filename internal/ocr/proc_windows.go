//go:build windows

package ocr

import (
	"os/exec"
	"syscall"
)

// createNoWindow is the CREATE_NO_WINDOW process creation flag.
const createNoWindow = 0x08000000

// hideConsole keeps console tools (pdftoppm, tesseract) from flashing a console
// window for every OCR'd page: eprocIA is a GUI app, so Windows would otherwise
// allocate a new console for each child process.
func hideConsole(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{
		HideWindow:    true,
		CreationFlags: createNoWindow,
	}
}
