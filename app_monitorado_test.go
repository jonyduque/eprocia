package main

import (
	"reflect"
	"testing"
)

// filaFalsa registra a ordem das chamadas do callback do watcher.
type filaFalsa struct {
	reprocessa bool
	chamadas   []string
	adicionado [][]string
	origens    []string
}

func (f *filaFalsa) ReprocessarMonitorado(p string) bool {
	f.chamadas = append(f.chamadas, "reprocessar:"+p)
	return f.reprocessa
}

func (f *filaFalsa) AddFilesComOrigem(paths []string, origem string) int {
	f.chamadas = append(f.chamadas, "add")
	f.adicionado = append(f.adicionado, paths)
	f.origens = append(f.origens, origem)
	return len(paths)
}

func TestEncaminharMonitorado(t *testing.T) {
	const caminho = `C:\Users\fulano\Downloads\5000000-00.2026.8.26.0001.pdf`

	t.Run("reprocessado não adiciona", func(t *testing.T) {
		f := &filaFalsa{reprocessa: true}
		encaminharMonitorado(f, caminho)
		if want := []string{"reprocessar:" + caminho}; !reflect.DeepEqual(f.chamadas, want) {
			t.Fatalf("chamadas = %v, esperado %v", f.chamadas, want)
		}
	})

	t.Run("sem item adiciona depois de consultar", func(t *testing.T) {
		f := &filaFalsa{}
		encaminharMonitorado(f, caminho)
		if want := []string{"reprocessar:" + caminho, "add"}; !reflect.DeepEqual(f.chamadas, want) {
			t.Fatalf("chamadas = %v, esperado %v", f.chamadas, want)
		}
		if !reflect.DeepEqual(f.adicionado, [][]string{{caminho}}) || !reflect.DeepEqual(f.origens, []string{"monitorada"}) {
			t.Fatalf("AddFilesComOrigem(%v, %v)", f.adicionado, f.origens)
		}
	})
}
