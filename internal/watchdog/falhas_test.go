package watchdog

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestFalhas_PersisteEIgnoraSoComMesmoMtime(t *testing.T) {
	dir := t.TempDir()
	arquivo := filepath.Join(dir, "estado", "falhas.json")
	zip := filepath.Join(dir, "proc.zip")
	criarComData(t, zip, time.Now().Add(-time.Hour))
	// O NTFS guarda o mtime em passos de 100 ns; compara com o valor lido.
	info, err := os.Stat(zip)
	if err != nil {
		t.Fatal(err)
	}
	quando := info.ModTime()

	f := CarregarFalhas(arquivo)
	f.Registrar(zip)

	// Outra instância (próxima abertura do app) lê do disco.
	g := CarregarFalhas(arquivo)
	if !g.Ignorar(zip, quando) {
		t.Fatal("falha registrada não sobreviveu à releitura")
	}
	if !g.Ignorar(strings.ToUpper(zip), quando) {
		t.Fatal("caminho no Windows não diferencia maiúsculas")
	}
	if g.Ignorar(zip, quando.Add(time.Minute)) {
		t.Fatal("ZIP baixado de novo (mtime diferente) foi ignorado")
	}

	g.Esquecer(zip)
	if CarregarFalhas(arquivo).Ignorar(zip, quando) {
		t.Fatal("Esquecer não removeu a falha do disco")
	}
}

func TestFalhas_NilEArquivoCorrompidoNaoIgnoram(t *testing.T) {
	var nada *Falhas
	if nada.Ignorar("x.zip", time.Now()) {
		t.Fatal("registro nil ignorou um ZIP")
	}
	nada.Registrar("x.zip")
	nada.Esquecer("x.zip")

	arquivo := filepath.Join(t.TempDir(), "falhas.json")
	if err := os.WriteFile(arquivo, []byte("{corrompido"), 0o644); err != nil {
		t.Fatal(err)
	}
	if CarregarFalhas(arquivo).Ignorar("x.zip", time.Now()) {
		t.Fatal("arquivo corrompido ignorou um ZIP")
	}
}

func TestPendingZips_PulaZipQueJaFalhou(t *testing.T) {
	dir := t.TempDir()
	agora := time.Now()
	falhou := filepath.Join(dir, "falhou.zip")
	novo := filepath.Join(dir, "novo.zip")
	criarComData(t, falhou, agora.Add(-2*time.Hour))
	criarComData(t, novo, agora.Add(-1*time.Hour))

	f := CarregarFalhas(filepath.Join(t.TempDir(), "falhas.json"))
	f.Registrar(falhou)

	got, err := pendingZips(dir, agora.Add(-janelaPendentes), f.Ignorar)
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{novo}; !reflect.DeepEqual(got, want) {
		t.Fatalf("pendingZips = %v, esperado %v", got, want)
	}

	// Re-download: o navegador grava um arquivo novo, com outro mtime.
	criarComData(t, falhou, agora.Add(-time.Minute))
	got, _ = pendingZips(dir, agora.Add(-janelaPendentes), f.Ignorar)
	if want := []string{novo, falhou}; !reflect.DeepEqual(got, want) {
		t.Fatalf("depois do re-download: %v, esperado %v", got, want)
	}
}

func TestWatcher_StartNaoReenfileiraZipQueFalhou(t *testing.T) {
	dir := t.TempDir()
	zip := filepath.Join(dir, "falhou.zip")
	criarComData(t, zip, time.Now().Add(-10*time.Minute))
	f := CarregarFalhas(filepath.Join(t.TempDir(), "falhas.json"))
	f.Registrar(zip)

	detectados := make(chan string, 1)
	w := NewWatcher(func(p string) { detectados <- p })
	w.SetFalhas(f)
	if err := w.Start(dir); err != nil {
		t.Fatal(err)
	}
	defer w.Stop()

	select {
	case p := <-detectados:
		t.Fatalf("ZIP que já falhou foi reenfileirado: %s", p)
	case <-time.After(3 * time.Second):
	}
}
