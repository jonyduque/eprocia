package main

import "testing"

// A trava é por executável: o mesmo caminho (sem diferença de maiúsculas,
// como no Windows) dá o mesmo nome; outra pasta dá outro nome.
func TestNomeTravaInstancia(t *testing.T) {
	a := nomeTravaInstancia(`C:\Users\x\AppData\Local\eprocIA\eprocia.exe`)
	b := nomeTravaInstancia(`c:\users\X\appdata\local\EPROCIA\EPROCIA.EXE`)
	c := nomeTravaInstancia(`C:\Projetos\eprocia\build\bin\eprocia.exe`)
	if a != b {
		t.Errorf("mesmo executável com outra caixa deu nomes diferentes: %q, %q", a, b)
	}
	if a == c {
		t.Errorf("executáveis em pastas diferentes deram o mesmo nome: %q", a)
	}
	const prefixo = `Local\eprocIA-instancia-`
	if len(a) <= len(prefixo) || a[:len(prefixo)] != prefixo {
		t.Errorf("nome %q sem o prefixo %q", a, prefixo)
	}
}
