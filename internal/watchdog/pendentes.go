package watchdog

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// janelaPendentes limita a varredura inicial a downloads recentes, para não
// reprocessar o histórico inteiro de uma pasta de Downloads.
const janelaPendentes = 24 * time.Hour

// pendingZips lista os .zip de dir modificados depois de since que ainda não
// têm, ao lado, um <nome>.txt igual ou mais novo. Devolve do mais antigo ao
// mais novo. ignorar (opcional) descarta ZIPs que já falharam com o mesmo mtime.
func pendingZips(dir string, since time.Time, ignorar func(zip string, mod time.Time) bool) ([]string, error) {
	entradas, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}

	type pendente struct {
		caminho string
		mod     time.Time
	}
	var achados []pendente
	for _, e := range entradas {
		if e.IsDir() || !strings.EqualFold(filepath.Ext(e.Name()), ".zip") {
			continue
		}
		info, err := e.Info()
		if err != nil || !info.ModTime().After(since) {
			continue
		}
		caminho := filepath.Join(dir, e.Name())
		saida := strings.TrimSuffix(caminho, filepath.Ext(caminho)) + ".txt"
		if txt, err := os.Stat(saida); err == nil && !txt.ModTime().Before(info.ModTime()) {
			continue
		}
		if ignorar != nil && ignorar(caminho, info.ModTime()) {
			continue
		}
		achados = append(achados, pendente{caminho, info.ModTime()})
	}

	sort.SliceStable(achados, func(i, j int) bool { return achados[i].mod.Before(achados[j].mod) })
	caminhos := make([]string, len(achados))
	for i, a := range achados {
		caminhos[i] = a.caminho
	}
	return caminhos, nil
}
