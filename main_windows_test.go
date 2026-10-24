package main

import (
	"runtime"
	"sync"
	"sync/atomic"
	"syscall"
	"testing"
	"time"
)

var procGetCurrentThreadId = syscall.NewLazyDLL("kernel32.dll").NewProc("GetCurrentThreadId")

func threadAtual() uintptr {
	id, _, _ := procGetCurrentThreadId.Call()
	return id
}

// O laço do tray precisa ficar na mesma thread do SO do início ao fim, e
// nenhuma outra goroutine pode ocupar essa thread enquanto ele roda.
func TestExecutarTray_PrendeThreadDoSO(t *testing.T) {
	var rodando atomic.Bool
	var mu sync.Mutex
	vistas := map[uintptr]bool{}
	pare := make(chan struct{})
	var wg sync.WaitGroup
	for i := 0; i < 4*runtime.GOMAXPROCS(0); i++ {
		wg.Go(func() {
			for {
				select {
				case <-pare:
					return
				default:
				}
				// Só vale a leitura feita inteira enquanto o tray roda.
				antes := rodando.Load()
				id := threadAtual()
				if antes && rodando.Load() {
					mu.Lock()
					vistas[id] = true
					mu.Unlock()
				}
				runtime.Gosched()
			}
		})
	}

	fim := make(chan struct{})
	var inicial uintptr
	migrou := false
	go executarTray(func() {
		defer close(fim)
		inicial = threadAtual()
		rodando.Store(true)
		defer rodando.Store(false)
		for range 200 {
			time.Sleep(time.Millisecond)
			runtime.Gosched()
			if threadAtual() != inicial {
				migrou = true
			}
		}
	})
	<-fim
	close(pare)
	wg.Wait()

	if migrou {
		t.Fatal("a goroutine do tray mudou de thread do SO")
	}
	if vistas[inicial] {
		t.Fatal("outra goroutine rodou na thread do tray")
	}
}
