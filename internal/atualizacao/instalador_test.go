//go:build windows

package atualizacao

import (
	"crypto/sha256"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"unsafe"

	"golang.org/x/sys/windows"
)

const nomeInstaladorTeste = "eprocia-0.5.0-windows-amd64-instalador.exe"

var versaoInstaladorTeste = Versao{0, 5, 0}

// instaladorBaixado grava um exe carimbado 0.5.0 com o nome do asset do
// instalador, numa subpasta nova de t.TempDir(), e monta o Baixado certo.
func instaladorBaixado(t *testing.T) Baixado {
	t.Helper()
	pasta, err := NovaPastaInstalador(filepath.Join(t.TempDir(), "atualizacao"))
	if err != nil {
		t.Fatalf("NovaPastaInstalador: %v", err)
	}
	caminho := filepath.Join(pasta, nomeInstaladorTeste)
	copiarAuxiliar(t, caminho, versaoInstaladorTeste)
	dados, err := os.ReadFile(caminho)
	if err != nil {
		t.Fatal(err)
	}
	return Baixado{
		Caminho: caminho,
		Asset:   Asset{Nome: nomeInstaladorTeste, Tamanho: int64(len(dados))},
		Versao:  versaoInstaladorTeste,
		SHA256:  sha256.Sum256(dados),
	}
}

func escritaFalha(t *testing.T, caminho string) bool {
	t.Helper()
	f, err := os.OpenFile(caminho, os.O_WRONLY, 0)
	if err != nil {
		return true
	}
	_ = f.Close()
	return false
}

func TestReabrirConferido_TravaEscrita(t *testing.T) {
	b := instaladorBaixado(t)
	tr, err := ReabrirConferido(b)
	if err != nil {
		t.Fatalf("ReabrirConferido: %v", err)
	}
	t.Cleanup(tr.Fechar)
	if !escritaFalha(t, b.Caminho) {
		t.Fatal("escrita no instalador travado deveria falhar")
	}
	if err := os.Remove(b.Caminho); err == nil {
		t.Fatal("remover o instalador travado deveria falhar")
	}
	if v, err := VersaoPE(b.Caminho); err != nil || v != versaoInstaladorTeste {
		t.Fatalf("VersaoPE com o arquivo travado = %v, %v", v, err)
	}
	if err := tr.Abrir(func(string) error { return nil }); err != nil {
		t.Fatalf("Abrir: %v", err)
	}
	if escritaFalha(t, b.Caminho) {
		t.Fatal("depois de Abrir a escrita deveria voltar a funcionar")
	}
}

func TestReabrirConferido_SHAErrado(t *testing.T) {
	b := instaladorBaixado(t)
	b.SHA256[0] ^= 0xff
	tr, err := ReabrirConferido(b)
	if !errors.Is(err, ErrNaoConferiu) {
		t.Fatalf("erro = %v, quer ErrNaoConferiu", err)
	}
	if tr != nil {
		chamado := false
		_ = tr.Abrir(func(string) error { chamado = true; return nil })
		if chamado {
			t.Fatal("shell não deveria ser chamado")
		}
		t.Fatal("ReabrirConferido devolveu Travado com erro")
	}
	if escritaFalha(t, b.Caminho) {
		t.Fatal("com a conferência recusada o handle deveria estar fechado")
	}
}

func TestAbrir_PassaCaminhoEFechaHandle(t *testing.T) {
	b := instaladorBaixado(t)
	tr, err := ReabrirConferido(b)
	if err != nil {
		t.Fatalf("ReabrirConferido: %v", err)
	}
	t.Cleanup(tr.Fechar)
	var recebido string
	travadoDurante := false
	err = tr.Abrir(func(caminho string) error {
		recebido = caminho
		travadoDurante = escritaFalha(t, caminho)
		return nil
	})
	if err != nil {
		t.Fatalf("Abrir: %v", err)
	}
	if recebido != b.Caminho {
		t.Fatalf("shell recebeu %q, quer %q", recebido, b.Caminho)
	}
	if !travadoDurante {
		t.Fatal("o handle deveria continuar aberto durante o shell")
	}
	if escritaFalha(t, b.Caminho) {
		t.Fatal("o handle deveria fechar depois do shell")
	}
}

func TestAbrir_Cancelado(t *testing.T) {
	b := instaladorBaixado(t)
	tr, err := ReabrirConferido(b)
	if err != nil {
		t.Fatalf("ReabrirConferido: %v", err)
	}
	t.Cleanup(tr.Fechar)
	err = tr.Abrir(func(string) error { return ErrCancelado })
	if !errors.Is(err, ErrCancelado) {
		t.Fatalf("erro = %v, quer ErrCancelado", err)
	}
	if escritaFalha(t, b.Caminho) {
		t.Fatal("o handle deveria fechar mesmo com erro do shell")
	}
}

func TestErroShell(t *testing.T) {
	if err := erroShell(windows.ERROR_CANCELLED); !errors.Is(err, ErrCancelado) {
		t.Fatalf("ERROR_CANCELLED -> %v, quer ErrCancelado", err)
	}
	err := erroShell(windows.ERROR_FILE_NOT_FOUND)
	if err == nil || errors.Is(err, ErrCancelado) {
		t.Fatalf("ERROR_FILE_NOT_FOUND -> %v", err)
	}
	if !errors.Is(err, windows.ERROR_FILE_NOT_FOUND) {
		t.Fatalf("o errno deveria seguir no erro: %v", err)
	}
}

func TestShellExecuteInfo_Tamanho(t *testing.T) {
	if unsafe.Sizeof(uintptr(0)) != 8 {
		t.Skip("tamanho medido só em 64 bits")
	}
	if n := unsafe.Sizeof(shellExecuteInfoW{}); n != 112 {
		t.Fatalf("sizeof(shellExecuteInfoW) = %d, quer 112", n)
	}
}

func TestNovaPastaInstalador_UmaPorDownload(t *testing.T) {
	base := filepath.Join(t.TempDir(), "eprocIA", "atualizacao")
	a, err := NovaPastaInstalador(base)
	if err != nil {
		t.Fatalf("primeira: %v", err)
	}
	b, err := NovaPastaInstalador(base)
	if err != nil {
		t.Fatalf("segunda: %v", err)
	}
	if a == b {
		t.Fatalf("duas chamadas devolveram a mesma pasta %q", a)
	}
	for _, p := range []string{a, b} {
		if filepath.Dir(p) != base || !strings.HasPrefix(p, base+string(filepath.Separator)) {
			t.Fatalf("%q fora de %q", p, base)
		}
		if st, err := os.Stat(p); err != nil || !st.IsDir() {
			t.Fatalf("%q não é pasta: %v", p, err)
		}
	}
}
