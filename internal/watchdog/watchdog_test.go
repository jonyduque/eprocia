package watchdog

import (
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

func TestWatcher_StartAndStop(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "watchdog_test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	var detectedFiles []string
	var mu sync.Mutex

	w := NewWatcher(func(path string) {
		mu.Lock()
		defer mu.Unlock()
		detectedFiles = append(detectedFiles, path)
	})

	err = w.Start(tmpDir)
	if err != nil {
		t.Fatalf("failed to start watcher: %v", err)
	}

	if w.GetCurrentPath() != tmpDir {
		t.Errorf("expected current path to be %s, got %s", tmpDir, w.GetCurrentPath())
	}

	w.Stop()

	if w.GetCurrentPath() != "" {
		t.Errorf("expected current path to be empty after stop, got %s", w.GetCurrentPath())
	}
}

func TestWatcher_DetectZipFile(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "watchdog_test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	detectedChan := make(chan string, 1)

	w := NewWatcher(func(path string) {
		detectedChan <- path
	})

	err = w.Start(tmpDir)
	if err != nil {
		t.Fatalf("failed to start watcher: %v", err)
	}
	defer w.Stop()

	// Write a zip file to the monitored directory
	zipPath := filepath.Join(tmpDir, "test_process.zip")
	// Write it in chunks to simulate writing/downloading
	file, err := os.Create(zipPath)
	if err != nil {
		t.Fatalf("failed to create dummy zip file: %v", err)
	}

	_, _ = file.Write([]byte("dummy zip content part 1"))
	_ = file.Sync()
	time.Sleep(100 * time.Millisecond)

	_, _ = file.Write([]byte("dummy zip content part 2"))
	_ = file.Close()

	// Wait for detection
	select {
	case detected := <-detectedChan:
		expected := filepath.Clean(zipPath)
		actual := filepath.Clean(detected)
		if expected != actual {
			t.Errorf("expected detected file path %s, got %s", expected, actual)
		}
	case <-time.After(3 * time.Second):
		t.Error("timed out waiting for zip file detection")
	}
}

func TestWatcher_IgnoreNonZipFile(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "watchdog_test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	detectedChan := make(chan string, 1)

	w := NewWatcher(func(path string) {
		detectedChan <- path
	})

	err = w.Start(tmpDir)
	if err != nil {
		t.Fatalf("failed to start watcher: %v", err)
	}
	defer w.Stop()

	// Write a txt file to the monitored directory
	txtPath := filepath.Join(tmpDir, "test_process.txt")
	err = os.WriteFile(txtPath, []byte("some txt content"), 0644)
	if err != nil {
		t.Fatalf("failed to create txt file: %v", err)
	}

	// Wait to ensure it is NOT detected
	select {
	case detected := <-detectedChan:
		t.Errorf("unexpectedly detected non-zip file: %s", detected)
	case <-time.After(1 * time.Second):
		// Success: no detection occurred
	}
}

// Um ZIP ainda em verificação na pasta antiga não pode disparar o callback
// depois de Stop + Start em outra pasta.
func TestWatcher_StopDescartaVerificacaoPendenteDaPastaAntiga(t *testing.T) {
	antiga := t.TempDir()
	nova := t.TempDir()

	disparos := make(chan string, 4)
	w := NewWatcher(func(path string) { disparos <- path })

	if err := w.Start(antiga); err != nil {
		t.Fatalf("Start(antiga): %v", err)
	}
	if err := os.WriteFile(filepath.Join(antiga, "processo.zip"), []byte("conteudo"), 0o644); err != nil {
		t.Fatal(err)
	}
	// Dá tempo ao fsnotify de entregar o evento e lançar handleNewFile, mas
	// não o bastante para o tamanho estabilizar (2 ticks de 200 ms).
	time.Sleep(100 * time.Millisecond)

	w.Stop()
	if err := w.Start(nova); err != nil {
		t.Fatalf("Start(nova): %v", err)
	}
	defer w.Stop()

	select {
	case p := <-disparos:
		t.Fatalf("callback disparado após Stop com arquivo da pasta antiga: %s", p)
	case <-time.After(1500 * time.Millisecond):
	}
}
