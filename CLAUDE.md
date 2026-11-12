# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## What this is

**eprocIA** — Windows desktop ETL tool (Wails v2, Go backend + Svelte 5 frontend). Watches a folder for ZIP downloads from the Brazilian court system "Eproc", extracts PDF/HTML documents from inside the ZIP (no disk unpack), OCRs image-only PDF pages via embedded Tesseract/Poppler, sorts documents by their "Evento N" (event number) using natural sort, and writes a single structured XML-tagged `.txt` file next to the original ZIP for pasting into an LLM. Fully offline/local — no external API calls (see PRD.md RNF01).

Product spec: `PRD.md`. Visual/UX spec: `DESIGN.md`. Both in Portuguese.

## Commands

```bash
# Dev (hot reload for both Go and Svelte frontend)
wails dev

# Production build -> build/bin/ (Wails CLI must be built with x/tools > v0.30.0
# to work under Go 1.27; see "Harness de revisão" below)
wails build

# Release: exe + NSIS installer (makensis via `scoop install nsis`). The build
# rewrites wailsjs/, go.mod and package.json.md5 with CRLF-only noise: revert it.
# Publish with `gh release create vX.Y.Z --target main` (creates the tag remotely).
wails build -clean -nsis

# Per-user install, also `irm .../install.ps1 | iex` (no admin; elevated shells
# re-run it unelevated as the session user via a one-off scheduled task). Must
# stay Windows PowerShell 5.1 compatible and saved WITHOUT a BOM (a BOM breaks
# `irm | iex`); under -File the 5.1 re-reads itself as UTF-8, so never put
# characters whose UTF-8 bytes include 0x91-0x94 (arrows, Ó, Ô) in the source:
# 5.1 parses those as smart quotes. Read the version from FileMajorPart etc.:
# Wails writes the string table under language 0000, which FileVersionInfo ignores.
pwsh -File install.ps1 -DryRun   # -From exe, -Uninstall [-Purge], ...

# Go tests (whole repo)
go test ./...

# Single package / single test
go test ./internal/etl -run TestSortProcessFiles -v

# Frontend only (rarely needed standalone; wails dev drives it)
cd frontend && npm install
cd frontend && npm run dev
cd frontend && npm run build

go vet ./...

# Harness tests (hooks, statusline, workflows, visual stubs)
node --test .claude/hooks/hooks.test.mjs .claude/statusline.test.mjs .claude/workflows/workflows.test.mjs scripts/visual/estados.test.mjs

# Frontend logic tests (frontend/*.test.mjs, importing src/lib; CI runs the same glob)
node --test (Get-ChildItem frontend -Filter *.test.mjs).FullName   # pwsh
```

CI (`.github/workflows/ci.yml`, Windows runner, push/PR to `main`) runs the same gate minus the smoke test, plus `gofmt -l`. `security.yml` runs osv-scanner (all lockfiles, with `--config=./osv-scanner.toml`, whose exceptions each carry a reason and an expiry date) and govulncheck on every push and weekly. `release.yml` builds and attests the release from a `v*` tag as a draft; tags `v*` are immutable by ruleset, so rehearse with a pre-release tag only after bumping `productVersion`. The local checkout uses `core.autocrlf=true`, so `gofmt -l` on the working copy lists every file; check formatting with `git show HEAD:<file> | gofmt -d` or trust CI, which checks out with LF. `go.sum` and the OCR binaries in `internal/ocr/embed/` are versioned (a clean clone must build).

There is no repo-level lint config beyond `go vet`. Frontend tests are `frontend/*.test.mjs` (node --test, no browser): many read the `.svelte` sources and assert conventions with regexes, so a UI change can break them even when svelte-check passes; run that glob in every frontend gate. `go test ./...` includes `scripts/bench`, whose `TestFixtures_LegibleToOCR` runs the real OCR (a few seconds; skipped with `-short`).

## Architecture

**Entry point**: `main.go` embeds `frontend/dist` via `go:embed`, boots `wails.Run`, and runs `github.com/energye/systray` (Apache-2.0, pinned v1.0.3; replaced getlantern, which has no double-click) in a goroutine for the tray icon (`bandeja.go`: right click = menu Abrir/Ocultar/Sair, double left click = Abrir, which restores the previous state and never forces maximize; single left click does nothing so it cannot swallow the double click). Its menu and click callbacks run on the tray thread inside wndProc: dispatch actions in goroutines. Closing the window (X, Alt+F4, taskbar) hides it to tray through `OnBeforeClose` (`App.antesDeFechar`, not `HideWindowOnClose`, which bypasses it) with the fade/slide of `internal/janela` (skipped when Windows "animation effects" is off); the app only fully exits via tray "Sair", whose `App.Quit` sets `saindo` first so `OnBeforeClose` lets `runtime.Quit` through.

**`app.go`** — the `App` struct is bound to the frontend (all exported methods become callable JS via Wails' generated bindings in `frontend/src/wailsjs/go/main/App.js`). It's a thin layer: delegates file-drop/manual-select/config to `internal/config`, `internal/watchdog`, and the `internal/etl` queue/pipeline. Don't put business logic here — it belongs in `internal/etl`.

**Processing pipeline** (`internal/etl`):
- `watchdog.Watcher` (fsnotify) detects new `.zip` files in the monitored folder, waits for the file size to stabilize before firing the callback, then hands the path to `etl.Queue`.
- `etl.Queue` processes ZIPs **sequentially, one at a time** (`isWorking` flag + `startProcessing` self-chaining), oldest-pending-first. It tracks per-item status (`pending/processing/paused/cancelled/success/error`) and broadcasts state to the frontend via Wails events (`queue:update`, `queue:status`, throttled `queue:item`). `Queue.Reprocessar` puts a finished item back with its own OCR/cleaning override (the global config is untouched).
- `etl.Pipeline.ProcessZip` does the actual work for one ZIP:
  1. `zip_reader.ReadZipFiles` — lists PDF/HTML entries directly from the zip stream (skips dirs, `__MACOSX`, dotfiles).
  2. `sorter.SortProcessFiles` — sorts by `EventNum` (parsed from filename via `Evento (\d+)` regex, unmatched files sort last) then natural string compare for ties.
  3. Per-document processing runs **concurrently**, bounded by a semaphore sized `NumCPU()-1`: PDFs go through `pdf_parser.ParsePages` (native text extraction per page, page-level OCR fallback if native text is short AND the page has large embedded images, or the page is drawn as vectors, or its native text is unreadable), HTML goes through `html_parser.ExtractTextFromHTML` (tokenizer-based, strips `script`/`style`/`head`). OCR execution itself is bounded by a *second*, separate `NumCPU()-1` semaphore inside the pipeline (`ocrRunner`) — see review notes below on why this nesting matters.
  4. `formatter.GenerateConsolidatedOutput` builds the final `<contexto>/<evento>/<documento>/<pag>` XML-tagged text (format defined in PRD.md §6); `cleanPage` (formatter.go) applies the configured cleaning level per page: `none`, `basic`, `standard` (default; UI "Padrão") or `aggressive` (UI "Máxima"). The rules live in `higienizacao.go`; per-document state (`ctxDocumento`: repeated edge lines, letterhead already seen, signature markers) is built before the pages are cleaned. Duplicate pages/documents (H7) are decided by a pre-pass inside `GenerateConsolidatedOutput` (exact hash after the basic cleaning); omitted document types (`Config.OmitirTipos`, normalised on load; H10) are decided in the pipeline pre-population step, so an omitted document is never read or OCR'd. `<pag>` is omitted only for documents that had exactly 1 page in the source; empty pages emit no `<pag>`.
  5. Output is written as `<zipname>.txt` next to the original ZIP.
- Progress goes only through the `onProgress` callback (the queue turns it into `queue:item`; the pipeline emits no Wails events), tracked as pages-processed / total-pages-precomputed (PDF page counts are pre-scanned before the main loop). Workers read the counters without a lock, so updates can arrive out of order: the queue keeps only increases. Every document, failed ones included, ends through `encerrar`, which tops up its pre-counted pages.
- Per-document state (waiting/reading/OCR/done/error, pages, OCR processes in flight) lives in `etl.Atividade`, filled by `ProcessZipComAtividade` and sent as `queue:atividade` at most every 200 ms; `GetQueueActivity` returns the current snapshot. Document keys in the `eventos` map carry the file index because a ZIP may hold two entries with the same path.
- The reader of every PDF is decided once, in the pre-count, by `abrirPDF` (`internal/etl/abertura.go`): the library first and, when it refuses the file, the embedded Poppler (`ocr.InfoPDF` for the page count, then `parsePoppler` in `pdf_poppler.go` reads the text with `ocr.TextoPDF` and sends pages without text to OCR). A wrong password does not go to Poppler, and a document of an omitted type never reaches `abrirPDF`. An in-memory PDF that the library opened is released after the count and extracted and opened again by the worker; the Poppler and on-disk cases keep the source until the worker.
- Each document ends with a reading reason, the type `Leitura` (`internal/etl/leitura.go`): `lido`, `segundo_leitor`, `ocr`, or the unread ones `sem_ocr`, `senha`, `ilegivel`, `prazo`, `ocr_falhou` (`NaoLido()`, `Frase()`, `Acao()`). An unread document gets the key `nao_lido` (the phrase) in its map; the formatter writes `Documentos não lidos: N` in the header and `[documento não lido: <frase>; consulte o PDF original]` in the body (PRD.md §6). `QueueItem.UnreadDocs` (`unread_docs`) carries the count to the row, the end notification and the copy warning (`frontend/src/lib/naoLidos.js`). A scanned page with OCR off is written as `[página digitalizada: o OCR está desativado]`.
- What reaches the `.txt` or a document's `Erro`/`Detalhe` in the panel about a failure is a short fixed text, never a path, stderr or a dump of PDF objects. Every error string that leaves the pipeline goes through `textoErroSaida` (`abertura.go`): `erroSegundoLeitor` keeps its fixed phrase, I/O and process-start errors (`*fs.PathError`, `*os.LinkError`, `*os.SyscallError`, `*exec.Error`) become "falha de leitura ou gravação em disco", the rest goes through `resumirErroBiblioteca` (the library embeds dumps of PDF objects), and text that still looks like a path becomes "erro interno; detalhe no log". A page whose OCR failed with no native fallback gets a fixed marker, `[ERRO de OCR: tempo esgotado]` or `[ERRO de OCR: o OCR não conseguiu ler a página]`, and `ocr_erro` the same phrase. The full error goes only to the log. Keep it that way: the `.txt` is pasted into an LLM and the temp path holds the Windows user name. (A whole-ZIP failure still puts the raw error in `QueueItem.Error`, shown in the row and the failure notification; no `.txt` is written then.)

**OCR** (`internal/ocr/tesseract.go`): Poppler (`pdftoppm.exe`) and Tesseract binaries + `tessdata/{fast,normal,best}` are embedded via `go:embed embed` and self-extracted once per process to `%TEMP%/eprocIA_bin` (skipped if already present with matching size). `RunOCR` renders one PDF page to PNG via `pdftoppm`, then runs `tesseract.exe` with the Portuguese model, falling back to the default model on failure. `RunOCRPaginas` does several pages of one PDF with one `pdftoppm` per contiguous run and a single `tesseract` over a list file (texts split on `\f`, which tesseract puts *between* pages, not after the last); the pipeline uses it through `PDFParser.comOCREmLote`, splitting each document's OCR pages into at most `numWorkers` blocks of at most 20 pages (`maxPaginasBlocoOCR`), and redoes a failed block page by page. `internal/ocr/poppler.go` adds `InfoPDF` (`pdfinfo.exe`, page count) and `TextoPDF` (`pdftotext.exe -enc UTF-8`, one text per page), with `ErrSenha` for a password-protected file. Every external process runs under a deadline (`internal/ocr/prazo.go`: `rodarComPrazo`, `prazoDe`, `ErrPrazo`): render 30 s + 10 s per page, OCR 60 s + 60 s per page, `pdfinfo` 20 s, `pdftotext` 30 s + 2 s per page; a process that passes it is killed and the page or document is marked `prazo`. This is Windows-only (`.exe`, `explorer.exe` calls in `app.go`).

**Config** (`internal/config`): JSON file at `os.UserConfigDir()/eprocIA/config.json`. `Load`/`LoadFromFile` always backfill missing fields from `DefaultConfig()` rather than erroring — treat config loading as "merge with defaults", not strict parsing. `theme` is written only by `App.SetTheme` (SaveConfig keeps the stored one) and read in `main.go` for the native window background. `falhas.json` in the same folder (`watchdog.Falhas`) remembers failed ZIPs by path + mtime so the startup scan does not re-queue them.

**Frontend** (`frontend/src`): Svelte 5 in legacy syntax (`export let`, `$:`, `on:`) + Vite, no component framework/state library. `App.svelte` owns top-level state (queue items, per-item activity, config, theme) and subscribes to Wails events; `Settings.svelte` and `ProcessList.svelte` (with `AtividadeItem.svelte` and `ReprocessarItem.svelte` inside a row) are the children, communicating up through callback props. Pure logic lives in `src/lib/*.js`. `frontend/src/wailsjs/` is **generated** from the Go bindings — never hand-edit it; after changing an `App` method signature run `wails generate module` (seconds, no build).

## Conventions worth knowing

- **Push:** only the repository owner and the orchestrating main session may run `git push`, and the session only when the owner asks. Subagents, workflow agents (implementador, revisor-*, cetico) and background tasks never push, merge into `main` or create releases; they commit locally and report back.
- All Go log lines and most UI strings/errors are in Portuguese (pt-BR) — match that when adding logs/user-facing text.
- `models.ProcessFile.ZipFileRef` holds a live `*zip.File`; the underlying `zip.ReadCloser` is closed by the caller of `ReadZipFiles` (`defer rc.Close()` in `Pipeline.ProcessZip`) — don't read from a `ProcessFile` after that scope ends.
- The `eventos` map built in `Pipeline.ProcessZip` is `map[string]any` with nested nested nested `any` maps (event → documentos → paginas), pre-populated single-threaded before the concurrent document loop so each goroutine only writes into its own pre-existing leaf map — don't add new top-level map keys from inside a goroutine, only from the pre-population step.

## Harness de revisão

The repo carries a Claude Code harness for a general review and for fixing the findings the user approves. Spec: `docs/superpowers/specs/2026-10-15-harness-revisao-design.md`.

- `/revisar [backend|frontend|tudo] [lentes=EF,...]` runs the `eprocia-revisao` workflow: one read-only reviewer per lens (backend EF, ES, GM, TE, RN; frontend VI, AN, RS, AC, DF), visual captures with fictitious data, one skeptic per lens, and a synthesis that writes `docs/revisao/AAAA-MM-DD-relatorio.md` with stable ids (`B-EF-03`, `F-RS-01`). Nothing is fixed at this stage.
- `/corrigir <ids|todos>` runs the `eprocia-corrigir` workflow on branch `revisao/AAAA-MM-DD`: toolchain Step 0 and the Svelte 5 migration as prerequisites, then, per id, a fresh implementer (TDD), gates, a fresh reviewer, up to 3 attempts, and one commit `fix(<camada>): <título> [<id>]`. It never pushes or merges.
- Configuration lives in `.claude/review.json` (lenses, files, capture matrix, benchmark thresholds, fix order). Agents are in `.claude/agents/`, skills in `.claude/skills/` (`eprocia-build`, `eprocia-bench`, `eprocia-visual`, `eprocia-icone`), scripts in `scripts/{build,bench,visual,icone}`.
- Gates: `pwsh scripts/build/gate.ps1` (vet, `test -race`, `wails build`, 5 s smoke of the exe it started); benchmarks with `-count=10` compared by benchstat, accepted only with p < 0.05 and no other metric worse than 5%; visual before/after captures plus `svelte-check` and `npm run build`.
- Progress for the status line is in `docs/revisao/estado.json` (git-ignored).

Hooks (`.claude/settings.json`, logic in `.claude/hooks/*.mjs`):

- `proteger-gerados` blocks edits to `frontend/src/wailsjs/**`, `frontend/dist/**`, `internal/ocr/embed/**` and `build/bin/**`.
- `proteger-dados` blocks `git add`/`git commit` of `*.zip`, `*.pdf`, processing `.txt` outputs and anything under `EPROCIA_BENCH_DIR`, including broad `git add -A`/`.` when such files are not ignored. The synthetic fixtures in `scripts/bench/fixtures/` are allowed.
- `guarda-bench` warns when `go test -bench` runs with `-count` below 6.
- `go-check` reports `gofmt -l` and `go vet` for each edited `.go` file; `front-check` runs `svelte-check` for edited frontend files once it is installed.
- `ambiente` prints, at session start, the Go version, whether the Wails CLI works with Go 1.27, whether eprocia.exe is running, and whether `EPROCIA_BENCH_DIR` is set.

Known traps (fixed on 2026-10-14; keep them fixed):

- `github.com/dslipak/pdf`'s lexer loops forever on a stream it cannot read (missing or corrupt `/Contents`, corrupt font `/ToUnicode`), its `GetTextByRow` also loops on a `/Contents` array, and it panics on unsupported filters. `ExtractNativeText` reads text with `extrairTextoPosicional` (`internal/etl/pdf_texto.go`), which interprets an array stream by stream; `checkContentStream` and `fonteLegivel` read every stream fully before the lexer sees it and reject one the lexer would never finish (a dict, array or string left open at the end spins forever; `lexerTermina` in `pdf_lexer.go` mirrors the v0.0.2 lexer, so review it when bumping the library), `GetTextByRow` is only the fallback for a single stream with readable fonts, and `recover` guards the panics.
- `dslipak/pdf` refuses four classes of valid PDF that Eproc delivers: header `%PDF-2.0` (`not a PDF file: invalid header`), a PDF inside a PKCS#7 envelope (same error), an inconsistent object table (`loading {N 0}: found {M 0}`, a panic inside `NumPage()`) and AESV2 encryption (`unsupported PDF: encryption version V=4`). `abrirPDF` sends all four to Poppler; `internal/etl/pdf_classes_test.go` builds one synthetic PDF per class and runs the real binaries (skipped with `-short`).
- `pdftotext` writes `\f` after every page, the last one included (tesseract writes it only between pages): `TextoPDF` drops the empty tail instead of counting one more page.
- The `TestMain` of `internal/etl` (`abertura_test.go`) swaps `infoPDF`/`textoPDF` for fakes that fail; a test that needs the real second reader calls `trocarPoppler(t, ocr.InfoPDF, ocr.TextoPDF)`.
- A worktree under `.worktrees/` has no `frontend/dist`: run `npm ci` and `npm run build` in `frontend` before any Go gate, or `main.go` does not compile.
- `energye/systray` (as getlantern before it) on Windows needs ICO data; a PNG fails with `Unable to set icon` and leaves no way to reopen or quit a hidden window.
- `tesseract.exe` and `pdftoppm.exe` started from the GUI app need `CREATE_NO_WINDOW` (`internal/ocr/proc_windows.go`), or every OCR'd page flashes a console window.
- Pages whose text is drawn as vector outlines (iLovePDF) have no native text and no large images; `HasDrawnContent` sends them to OCR.
- The published Wails CLI (v2.12.0 with `x/tools v0.30.0`, v2.13.0 with `v0.44.0`) fails under Go 1.27 with `package "bytes" without types was imported`. Rebuild it with `pwsh scripts/build/recompilar-wails.ps1` (defaults to the go.mod version, v2.13.0). Drop the script once a published Wails CLI builds this repo under the go.mod Go version.
