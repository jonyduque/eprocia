// Command gerar writes the synthetic benchmark fixtures to scripts/bench/fixtures.
//
//	go run ./scripts/bench/gerar [-saida dir]
package main

import (
	"flag"
	"log"
	"os"
	"path/filepath"
	"sort"

	"eprocia/scripts/bench/pdfgen"
)

func main() {
	saida := flag.String("saida", filepath.Join("scripts", "bench", "fixtures"), "diretório de destino")
	flag.Parse()

	if err := os.MkdirAll(*saida, 0o755); err != nil {
		log.Fatalf("falha ao criar %s: %v", *saida, err)
	}
	files := pdfgen.Fixtures()
	names := make([]string, 0, len(files))
	for n := range files {
		names = append(names, n)
	}
	sort.Strings(names)
	for _, n := range names {
		p := filepath.Join(*saida, n)
		if err := os.WriteFile(p, files[n], 0o644); err != nil {
			log.Fatalf("falha ao gravar %s: %v", p, err)
		}
		log.Printf("%s (%d bytes)", p, len(files[n]))
	}
}
