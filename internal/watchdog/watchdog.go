package watchdog

import (
	"errors"
	"fmt"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
)

// Watcher manages directory monitoring for ZIP files.
type Watcher struct {
	mu          sync.Mutex
	watcher     *fsnotify.Watcher
	currentPath string
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

// Start starts monitoring the specified path.
func (w *Watcher) Start(path string) error {
	w.mu.Lock()
	defer w.mu.Unlock()

	// If already watching the same path, do nothing
	if w.watcher != nil && w.currentPath == path {
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
	w.wg.Go(func() { w.watchLoop(watcherAtual, done) })

	// ZIPs baixados enquanto o app estava fechado não geram evento do fsnotify.
	pendentes, err := pendingZips(path, time.Now().Add(-janelaPendentes), w.falhas.Ignorar)
	if err != nil {
		log.Printf("Watchdog: falha ao procurar ZIPs pendentes em %s: %v", path, err)
	}
	if len(pendentes) > 0 {
		log.Printf("Watchdog: %d ZIP(s) pendente(s) encontrados em %s", len(pendentes), path)
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

	log.Printf("Watchdog started monitoring: %s", path)
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

	log.Printf("Watchdog stopped monitoring: %s", w.currentPath)
	w.watcher = nil
	w.currentPath = ""
}

// GetCurrentPath returns the currently monitored path.
func (w *Watcher) GetCurrentPath() string {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.currentPath
}

// watchLoop runs the background event listening.
// Recebe watcher e done por argumento: Start os reatribui sob w.mu, e um
// goroutine da pasta anterior não pode enxergar os da nova.
func (w *Watcher) watchLoop(watcher *fsnotify.Watcher, done chan struct{}) {
	for {
		select {
		case event, ok := <-watcher.Events:
			if !ok {
				return
			}

			// We monitor for file creation, write or rename (browser download completions)
			if event.Has(fsnotify.Create) || event.Has(fsnotify.Write) || event.Has(fsnotify.Rename) {
				if strings.ToLower(filepath.Ext(event.Name)) == ".zip" {
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
			log.Printf("Watchdog error: %v", err)

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
					w.onFileAdded(filePath)
					return
				}
			}
			lastSize = currentSize
		}
	}

	log.Printf("Watchdog: file %s could not be stabilized or opened", filePath)
}
