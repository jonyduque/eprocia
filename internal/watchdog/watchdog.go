package watchdog

import (
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
)

// Watcher manages directory monitoring for ZIP files (and PDFs, when asked).
type Watcher struct {
	mu          sync.Mutex
	watcher     *fsnotify.Watcher
	currentPath string
	extensoes   []string
	onFileAdded func(string)
	falhas      *Falhas
	done        chan struct{}
	wg          sync.WaitGroup
}

// NewWatcher creates a new Watcher instance.
func NewWatcher(onFileAdded func(string)) *Watcher {
	return &Watcher{
		onFileAdded: onFileAdded,
	}
}

// SetFalhas define o registro de ZIPs com erro que a varredura inicial pula.
func (w *Watcher) SetFalhas(f *Falhas) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.falhas = f
}

// Start starts monitoring the specified path for the given extensions
// (Extensoes; nil or empty means [".zip"]). It is a no-op only when the same
// path is already watched with the same set of extensions.
func (w *Watcher) Start(path string, extensoes []string) error {
	extensoes = normalizarExtensoes(extensoes)
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.watcher != nil && w.currentPath == path && slices.Equal(w.extensoes, extensoes) {
		return nil
	}

	// Stop existing watcher if any
	w.stopLocked()

	if path == "" {
		return nil
	}

	// Verify the path exists and is a directory
	info, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("failed to access path: %w", err)
	}
	if !info.IsDir() {
		return fmt.Errorf("path is not a directory: %s", path)
	}

	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return fmt.Errorf("failed to create watcher: %w", err)
	}

	w.watcher = watcher
	w.currentPath = path
	w.extensoes = extensoes
	w.done = make(chan struct{})

	err = w.watcher.Add(path)
	if err != nil {
		w.watcher.Close()
		w.watcher = nil
		w.currentPath = ""
		return fmt.Errorf("failed to add path to watcher: %w", err)
	}

	// Cópias locais: o closure não pode ler w.watcher/w.done depois que um
	// novo Start os reatribuir.
	watcherAtual, done := w.watcher, w.done
	w.wg.Go(func() { w.watchLoop(watcherAtual, done, extensoes) })

	// Arquivos baixados enquanto o app estava fechado não geram evento do fsnotify.
	pendentes, err := pendingZips(path, time.Now().Add(-janelaPendentes), extensoes, w.falhas.Ignorar)
	if err != nil {
		slog.Warn("watchdog: falha ao procurar arquivos pendentes", "pasta", path, "erro", err)
	}
	if len(pendentes) > 0 {
		slog.Info("watchdog: arquivos pendentes encontrados", "pasta", path, "quantidade", len(pendentes))
		w.wg.Go(func() {
			// Em série, para a fila receber na ordem de download.
			for _, p := range pendentes {
				select {
				case <-done:
					return
				default:
				}
				w.handleNewFile(p, done)
			}
		})
	}

	slog.Info("watchdog: monitoramento iniciado", "pasta", path)
	return nil
}

// Stop stops the current watcher.
func (w *Watcher) Stop() {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.stopLocked()
}

func (w *Watcher) stopLocked() {
	if w.watcher == nil {
		return
	}

	close(w.done)
	w.watcher.Close()
	w.wg.Wait()

	slog.Info("watchdog: monitoramento encerrado", "pasta", w.currentPath)
	w.watcher = nil
	w.currentPath = ""
	w.extensoes = nil
}

// GetCurrentPath returns the currently monitored path.
func (w *Watcher) GetCurrentPath() string {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.currentPath
}

// watchLoop runs the background event listening.
// Recebe watcher, done e extensoes por argumento: Start os reatribui sob
// w.mu, e um goroutine da pasta anterior não pode enxergar os da nova.
func (w *Watcher) watchLoop(watcher *fsnotify.Watcher, done chan struct{}, extensoes []string) {
	for {
		select {
		case event, ok := <-watcher.Events:
			if !ok {
				return
			}

			// We monitor for file creation, write or rename (browser download completions)
			if event.Has(fsnotify.Create) || event.Has(fsnotify.Write) || event.Has(fsnotify.Rename) {
				if aceito(event.Name, extensoes) {
					// Launch a goroutine to wait for the file to be completely written/unlocked
					// wg.Go aqui é seguro: watchLoop ainda conta no wg, então
					// o contador não está em zero enquanto stopLocked espera.
					w.wg.Go(func() { w.handleNewFile(event.Name, done) })
				}
			}

		case err, ok := <-watcher.Errors:
			if !ok {
				return
			}
			slog.Warn("watchdog: erro do monitor de pasta", "erro", err)

		case <-done:
			return
		}
	}
}

// handleNewFile waits for the file to be completely written, then triggers the callback.
func (w *Watcher) handleNewFile(filePath string, done <-chan struct{}) {
	// Wait to ensure file is completely written and unlocked.
	// We check if the file size is stable and if it can be opened.
	var lastSize int64 = -1
	retries := 15
	ticker := time.NewTicker(200 * time.Millisecond)
	defer ticker.Stop()

	for range retries {
		select {
		case <-done:
			return
		case <-ticker.C:
			info, err := os.Stat(filePath)
			if err != nil {
				if errors.Is(err, fs.ErrNotExist) {
					// File might have been renamed or deleted
					return
				}
				continue
			}

			currentSize := info.Size()
			mod := info.ModTime()
			// If size is stable (greater than 0, and hasn't changed since last check)
			if currentSize > 0 && currentSize == lastSize {
				// Verify if we can open it for reading
				file, err := os.Open(filePath)
				if err == nil {
					file.Close()
					select {
					case <-done:
						return
					default:
					}
					// Evento atrasado (Write do navegador ou do antivírus)
					// de um PDF já convertido geraria outro <nome>_N.txt.
					if strings.EqualFold(filepath.Ext(filePath), ".pdf") && jaConvertido(filePath, mod) {
						slog.Debug("watchdog: PDF já convertido, ignorado", "zip", filepath.Base(filePath))
						return
					}
					w.onFileAdded(filePath)
					return
				}
			}
			lastSize = currentSize
		}
	}

	slog.Warn("watchdog: arquivo não estabilizou ou não abriu", "zip", filepath.Base(filePath))
}
