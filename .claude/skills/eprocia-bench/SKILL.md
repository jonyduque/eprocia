---
name: eprocia-bench
description: Mede desempenho do pipeline do eprocIA (tempo, memória, processos de OCR, pausa de GC) e compara antes/depois com benchstat. Use para evidenciar ou aceitar achados de eficiência (EF) e checar regressão em correções de estabilidade (ES).
---

# Benchmark do eprocIA

## Peças

- `scripts/bench/pdfgen/` — gerador determinístico de fixtures fictícias.
- `scripts/bench/fixtures/` — fixtures versionadas: `nativo.pdf` (3 páginas de texto), `escaneado.pdf` (imagem, vai ao OCR), `vetorial.pdf` (texto desenhado como vetor, vai ao OCR), `contents-array.pdf` (`/Contents [ ref ]`, não pode travar), `documento.html` e `misto.zip` (50 documentos, 25 eventos).
- `scripts/bench/bench_test.go`:
  - `BenchmarkSintetico/ocr=<modo>` — pipeline completo em `misto.zip`;
  - `BenchmarkParsePDF/<fixture>` — custo Go de cada caminho de extração, sem OCR;
  - `BenchmarkReal/zipNN/ocr=<modo>` — cada `*.zip` de `EPROCIA_BENCH_DIR` (pulado se a variável não existir). Os nomes são `zip01`, `zip02`… para que nomes reais de processos nunca apareçam na saída.
- Métricas extras: `pico-mem-B` (pico amostrado de `/memory/classes/total:bytes`), `procs/op` (processos `pdftoppm`/`tesseract` por ZIP, via `ocr.SpawnedProcesses`) e `gc-pausa-ns/op`.
- O modo de OCR vem de `EPROCIA_BENCH_OCR` (`none` por padrão, ou `fast`, `normal`, `best`). `none` mede o custo do Go; `fast` mede o pipeline com OCR.

## Rodar uma comparação

Feche o eprocIA antes (ele disputa CPU). Grave os arquivos fora do repositório, por exemplo no diretório temporário da sessão.

```bash
# linha de base, no commit anterior à mudança
go test ./scripts/bench -run '^$' -bench . -count=10 -benchmem > "$TMP/antes.txt"
EPROCIA_BENCH_OCR=fast go test ./scripts/bench -run '^$' -bench Sintetico -count=10 -benchmem >> "$TMP/antes.txt"

# depois da mudança: mesmos comandos, saída em depois.txt
go run golang.org/x/perf/cmd/benchstat@latest "$TMP/antes.txt" "$TMP/depois.txt"
```

Depois do Passo 0 do toolchain (skill `eprocia-build`), o benchstat fica declarado no `go.mod` e o comando passa a ser `go tool benchstat antes.txt depois.txt`.

Corpus real: `EPROCIA_BENCH_DIR=D:\\caminho go test ./scripts/bench -run '^$' -bench Real -count=10 -benchmem`. Nada desse diretório entra no repositório; o hook `proteger-dados` bloqueia.

## Critério de aceite

- **EF:** a métrica-alvo melhora com `p < 0,05` no benchstat, e nenhuma outra métrica (`sec/op`, `B/op`, `allocs/op`, `pico-mem-B`, `procs/op`, `gc-pausa-ns/op`) piora mais que 5%.
- **ES:** nenhuma métrica piora mais que 5%.
- `-count` menor que 6 não serve como evidência (o hook `guarda-bench` avisa). Use 10.
- Cole na seção do achado a tabela do benchstat (só nomes `zipNN` e de fixtures).

## Fixtures

Mudou `pdfgen`? Regere e confira o teste:

```bash
go run ./scripts/bench/gerar
go test ./scripts/bench -run TestFixtures -v
```

`TestFixtures_UpToDate` falha se os arquivos versionados não baterem byte a byte com o gerador; `TestFixtures_ExtractionPaths` garante que cada fixture exercita o caminho a que se propõe; `TestFixtures_LegibleToOCR` (pulado com `-short`) garante que o OCR real lê o texto das fixtures de imagem e vetor.

## Validação desta skill

`go test ./scripts/bench -run '^$' -bench . -benchtime=1x -count=1` deve terminar com `PASS` e mostrar as métricas extras na linha de `BenchmarkSintetico`.
