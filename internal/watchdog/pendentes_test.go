package watchdog

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"
)

func criarComData(t *testing.T, caminho string, quando time.Time) {
	t.Helper()
	if err := os.WriteFile(caminho, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(caminho, quando, quando); err != nil {
		t.Fatal(err)
	}
}

func TestPendingZips_FiltraPorDataESaida(t *testing.T) {
	dir := t.TempDir()
	agora := time.Now()
	desde := agora.Add(-24 * time.Hour)

	criarComData(t, filepath.Join(dir, "novo.zip"), agora.Add(-1*time.Hour))
	criarComData(t, filepath.Join(dir, "MAIUSCULO.ZIP"), agora.Add(-2*time.Hour))
	criarComData(t, filepath.Join(dir, "antigo.zip"), agora.Add(-48*time.Hour))
	criarComData(t, filepath.Join(dir, "feito.zip"), agora.Add(-3*time.Hour))
	criarComData(t, filepath.Join(dir, "feito.txt"), agora.Add(-2*time.Hour))
	criarComData(t, filepath.Join(dir, "refeito.zip"), agora.Add(-1*time.Hour))
	criarComData(t, filepath.Join(dir, "refeito.txt"), agora.Add(-5*time.Hour))
	criarComData(t, filepath.Join(dir, "outro.pdf"), agora.Add(-1*time.Hour))
	if err := os.Mkdir(filepath.Join(dir, "pasta.zip"), 0o755); err != nil {
		t.Fatal(err)
	}

	got, err := pendingZips(dir, desde, nil)
	if err != nil {
		t.Fatal(err)
	}
	// Mais antigo primeiro, para a fila seguir a ordem de download.
	want := []string{
		filepath.Join(dir, "MAIUSCULO.ZIP"),
		filepath.Join(dir, "novo.zip"),
		filepath.Join(dir, "refeito.zip"),
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("pendingZips = %v, esperado %v", got, want)
	}
}

func TestWatcher_StartProcessaZipBaixadoAntes(t *testing.T) {
	dir := t.TempDir()
	zip := filepath.Join(dir, "baixado-com-app-fechado.zip")
	criarComData(t, zip, time.Now().Add(-10*time.Minute))

	detectados := make(chan string, 1)
	w := NewWatcher(func(p string) { detectados <- p })
	if err := w.Start(dir); err != nil {
		t.Fatal(err)
	}
	defer w.Stop()

	select {
	case p := <-detectados:
		if p != zip {
			t.Fatalf("detectado %q, esperado %q", p, zip)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("ZIP existente antes do Start não foi entregue ao callback")
	}
}
