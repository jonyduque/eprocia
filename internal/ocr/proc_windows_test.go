//go:build windows

package ocr

import (
	"os/exec"
	"testing"
)

// Sem CREATE_NO_WINDOW, cada página que vai ao OCR abre e fecha uma janela de
// console, porque o eprocIA é um app GUI.
func TestPrepareCmd_EscondeConsole(t *testing.T) {
	cmd := exec.Command("go", "version")
	prepareCmd(cmd)
	attr := cmd.SysProcAttr
	if attr == nil {
		t.Fatal("prepareCmd não definiu SysProcAttr")
	}
	if attr.CreationFlags&0x08000000 == 0 {
		t.Errorf("CreationFlags = %#x, falta CREATE_NO_WINDOW (0x08000000)", attr.CreationFlags)
	}
	if !attr.HideWindow {
		t.Error("HideWindow = false, quero true")
	}
}
