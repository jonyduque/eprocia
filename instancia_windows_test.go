package main

import (
	"fmt"
	"testing"
	"time"
)

// Enquanto a primeira trava estiver aberta, a segunda com o mesmo nome falha;
// depois de liberada, volta a ser possível.
func TestTravarInstancia_SegundaFalhaAteLiberar(t *testing.T) {
	nome := fmt.Sprintf(`Local\eprocIA-teste-%d`, time.Now().UnixNano())
	liberar, ok, err := travarInstancia(nome)
	if err != nil || !ok {
		t.Fatalf("primeira trava: ok=%v err=%v", ok, err)
	}
	if _, ok2, err := travarInstancia(nome); err != nil || ok2 {
		t.Fatalf("segunda trava deveria falhar: ok=%v err=%v", ok2, err)
	}
	liberar()
	liberar2, ok3, err := travarInstancia(nome)
	if err != nil || !ok3 {
		t.Fatalf("trava depois de liberar: ok=%v err=%v", ok3, err)
	}
	liberar2()
}
