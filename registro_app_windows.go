package main

import (
	"fmt"

	"golang.org/x/sys/windows"
)

// versaoWindows é a versão e o build reais (RtlGetVersion não sofre o
// mascaramento de compatibilidade de GetVersionEx).
func versaoWindows() string {
	v := windows.RtlGetVersion()
	if v == nil {
		return ""
	}
	return fmt.Sprintf("%d.%d.%d", v.MajorVersion, v.MinorVersion, v.BuildNumber)
}
