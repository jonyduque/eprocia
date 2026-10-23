//go:build !windows

package autostart

// Aplicar não faz nada fora do Windows: o app só é distribuído para Windows.
func Aplicar(ativo bool) error { return nil }
