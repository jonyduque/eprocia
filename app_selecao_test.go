package main

import (
	"reflect"
	"testing"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

func TestFiltrosSelecao(t *testing.T) {
	esperado := []runtime.FileFilter{
		{DisplayName: "ZIP do Eproc e PDF (*.zip;*.pdf)", Pattern: "*.zip;*.pdf"},
		{DisplayName: "Somente ZIP (*.zip)", Pattern: "*.zip"},
		{DisplayName: "Somente PDF (*.pdf)", Pattern: "*.pdf"},
	}
	if obtido := filtrosSelecao(); !reflect.DeepEqual(obtido, esperado) {
		t.Fatalf("filtros: %#v", obtido)
	}
}
