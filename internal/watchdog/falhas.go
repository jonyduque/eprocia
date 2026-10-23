package watchdog

import (
	"encoding/json"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// Falhas lembra os ZIPs cujo processamento falhou, com o mtime da época. Um
// ZIP com erro não gera .txt, e sem este registro a varredura inicial o
// reenfileiraria a cada abertura do app durante toda a janelaPendentes. Um
// mtime diferente (novo download) volta a ser processado.
type Falhas struct {
	mu      sync.Mutex
	arquivo string
	itens   map[string]int64 // caminho normalizado -> mtime em UnixNano
}

// CarregarFalhas lê o registro de arquivo. Arquivo ausente ou corrompido vira
// registro vazio: no pior caso um ZIP com erro é tentado de novo.
func CarregarFalhas(arquivo string) *Falhas {
	f := &Falhas{arquivo: arquivo, itens: map[string]int64{}}
	dados, err := os.ReadFile(arquivo)
	if err != nil {
		if !os.IsNotExist(err) {
			log.Printf("Watchdog: falha ao ler o registro de ZIPs com erro: %v", err)
		}
		return f
	}
	if err := json.Unmarshal(dados, &f.itens); err != nil {
		log.Printf("Watchdog: registro de ZIPs com erro corrompido, ignorando: %v", err)
		f.itens = map[string]int64{}
	}
	return f
}

// Windows não diferencia maiúsculas em caminhos.
func chaveFalha(caminho string) string {
	return strings.ToLower(filepath.Clean(caminho))
}

// Registrar marca zip como falho no mtime atual dele.
func (f *Falhas) Registrar(zip string) {
	if f == nil {
		return
	}
	info, err := os.Stat(zip)
	if err != nil {
		return
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	f.itens[chaveFalha(zip)] = info.ModTime().UnixNano()
	f.gravarLocked()
}

// Esquecer tira zip do registro (por exemplo, depois de um sucesso).
func (f *Falhas) Esquecer(zip string) {
	if f == nil {
		return
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	if _, ok := f.itens[chaveFalha(zip)]; !ok {
		return
	}
	delete(f.itens, chaveFalha(zip))
	f.gravarLocked()
}

// Ignorar diz se zip, com este mtime, já falhou antes.
func (f *Falhas) Ignorar(zip string, mod time.Time) bool {
	if f == nil {
		return false
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	m, ok := f.itens[chaveFalha(zip)]
	return ok && m == mod.UnixNano()
}

// gravarLocked descarta entradas fora da janela da varredura, que nunca mais
// seriam consultadas, para o arquivo não crescer sem limite.
func (f *Falhas) gravarLocked() {
	limite := time.Now().Add(-janelaPendentes).UnixNano()
	for k, m := range f.itens {
		if m < limite {
			delete(f.itens, k)
		}
	}
	dados, err := json.Marshal(f.itens)
	if err == nil {
		err = os.MkdirAll(filepath.Dir(f.arquivo), 0o755)
	}
	if err == nil {
		err = os.WriteFile(f.arquivo, dados, 0o644)
	}
	if err != nil {
		log.Printf("Watchdog: falha ao gravar o registro de ZIPs com erro: %v", err)
	}
}
