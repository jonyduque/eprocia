package watchdog

import (
	"fmt"
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
	done        chan struct{}
	wg          sync.WaitGroup
}

// NewWatcher creates a new Watcher instance.
func NewWatcher(onFileAdded func(string)) *Watcher {
	return &Watcher{
		onFileAdded: onFileAdded,
	}
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

	w.wg.Add(1)
	go w.watchLoop()

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
func (w *Watcher) watchLoop() {
	defer w.wg.Done()

	for {
		select {
		case event, ok := <-w.watcher.Events:
			if !ok {
				return
			}

			// We monitor for file creation, write or rename (browser download completions)
			if event.Has(fsnotify.Create) || event.Has(fsnotify.Write) || event.Has(fsnotify.Rename) {
				if strings.ToLower(filepath.Ext(event.Name)) == ".zip" {
					// Launch a goroutine to wait for the file to be completely written/unlocked
					go w.handleNewFile(event.Name)
				}
			}

		case err, ok := <-w.watcher.Errors:
			if !ok {
				return
			}
			log.Printf("Watchdog error: %v", err)

		case <-w.done:
			return
		}
	}
}

// handleNewFile waits for the file to be completely written, then triggers the callback.
func (w *Watcher) handleNewFile(filePath string) {
	// Wait to ensure file is completely written and unlocked.
	// We check if the file size is stable and if it can be opened.
	var lastSize int64 = -1
	retries := 15
	ticker := time.NewTicker(200 * time.Millisecond)
	defer ticker.Stop()

	for i := 0; i < retries; i++ {
		select {
		case <-w.done:
			return
		case <-ticker.C:
			info, err := os.Stat(filePath)
			if err != nil {
				if os.IsNotExist(err) {
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
					w.onFileAdded(filePath)
					return
				}
			}
			lastSize = currentSize
		}
	}

	log.Printf("Watchdog: file %s could not be stabilized or opened", filePath)
}
