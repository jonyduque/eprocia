# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## What this is

**eprocIA** — Windows desktop ETL tool (Wails v2, Go backend + Svelte 4 frontend). Watches a folder for ZIP downloads from the Brazilian court system "Eproc", extracts PDF/HTML documents from inside the ZIP (no disk unpack), OCRs image-only PDF pages via embedded Tesseract/Poppler, sorts documents by their "Evento N" (event number) using natural sort, and writes a single structured XML-tagged `.txt` file next to the original ZIP for pasting into an LLM. Fully offline/local — no external API calls (see PRD.md RNF01).

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

# Harness tests (hooks, statusline, workflows, visual stubs) and frontend lib tests
node --test .claude/hooks/hooks.test.mjs .claude/statusline.test.mjs .claude/workflows/workflows.test.mjs scripts/visual/estados.test.mjs frontend/src/lib/atividade.test.mjs
```

CI (`.github/workflows/ci.yml`, Windows runner, push/PR to `main`) runs the same gate minus the smoke test, plus `gofmt -l`. `security.yml` runs osv-scanner (all lockfiles, with `--config=./osv-scanner.toml`, whose exceptions each carry a reason and an expiry date) and govulncheck on every push and weekly. `release.yml` builds and attests the release from a `v*` tag as a draft; tags `v*` are immutable by ruleset, so rehearse with a pre-release tag only after bumping `productVersion`. The local checkout uses `core.autocrlf=true`, so `gofmt -l` on the working copy lists every file; check formatting with `git show HEAD:<file> | gofmt -d` or trust CI, which checks out with LF. `go.sum` and the OCR binaries in `internal/ocr/embed/` are versioned (a clean clone must build).

There is no repo-level lint config beyond `go vet`; no frontend test suite. `go test ./...` includes `scripts/bench`, whose `TestFixtures_LegibleToOCR` runs the real OCR (a few seconds; skipped with `-short`).

## Architecture

**Entry point**: `main.go` embeds `frontend/dist` via `go:embed`, boots `wails.Run`, and runs `github.com/getlantern/systray` in a goroutine for the tray icon (Abrir/Ocultar/Sair). Closing the window hides it to tray (`HideWindowOnClose`); the app only fully exits via tray "Sair".

**`app.go`** — the `App` struct is bound to the frontend (all exported methods become callable JS via Wails' generated bindings in `frontend/src/wailsjs/go/main/App.js`). It's a thin layer: delegates file-drop/manual-select/config to `internal/config`, `internal/watchdog`, and the `internal/etl` queue/pipeline. Don't put business logic here — it belongs in `internal/etl`.

**Processing pipeline** (`internal/etl`):
- `watchdog.Watcher` (fsnotify) detects new `.zip` files in the monitored folder, waits for the file size to stabilize before firing the callback, then hands the path to `etl.Queue`.
- `etl.Queue` processes ZIPs **sequentially, one at a time** (`isWorking` flag + `startProcessing` self-chaining), oldest-pending-first. It tracks per-item status (`pending/processing/paused/cancelled/success/error`) and broadcasts state to the frontend via Wails events (`queue:update`, `queue:status`, throttled `queue:item`). `Queue.Reprocessar` puts a finished item back with its own OCR/cleaning override (the global config is untouched).
- `etl.Pipeline.ProcessZip` does the actual work for one ZIP:
  1. `zip_reader.ReadZipFiles` — lists PDF/HTML entries directly from the zip stream (skips dirs, `__MACOSX`, dotfiles).
  2. `sorter.SortProcessFiles` — sorts by `EventNum` (parsed from filename via `Evento (\d+)` regex, unmatched files sort last) then natural string compare for ties.
  3. Per-document processing runs **concurrently**, bounded by a semaphore sized `NumCPU()-1`: PDFs go through `pdf_parser.ParsePages` (native text extraction per page, page-level OCR fallback if native text is short AND the page has large embedded images), HTML goes through `html_parser.ExtractTextFromHTML` (tokenizer-based, strips `script`/`style`/`head`). OCR execution itself is bounded by a *second*, separate `NumCPU()-1` semaphore inside the pipeline (`ocrRunner`) — see review notes below on why this nesting matters.
  4. `formatter.GenerateConsolidatedOutput` builds the final `<contexto>/<evento>/<documento>/<pag>` XML-tagged text (format defined in PRD.md §6); `cleanPage` (formatter.go) applies the configured cleaning level (`none`/`basic`/`aggressive`) per page.
  5. Output is written as `<zipname>.txt` next to the original ZIP.
- Progress goes only through the `onProgress` callback (the queue turns it into `queue:item`; the pipeline emits no Wails events), tracked as pages-processed / total-pages-precomputed (PDF page counts are pre-scanned before the main loop). Workers read the counters without a lock, so updates can arrive out of order: the queue keeps only increases. Every document, failed ones included, ends through `encerrar`, which tops up its pre-counted pages.
- Per-document state (waiting/reading/OCR/done/error, pages, OCR processes in flight) lives in `etl.Atividade`, filled by `ProcessZipComAtividade` and sent as `queue:atividade` at most every 200 ms; `GetQueueActivity` returns the current snapshot. Document keys in the `eventos` map carry the file index because a ZIP may hold two entries with the same path.

**OCR** (`internal/ocr/tesseract.go`): Poppler (`pdftoppm.exe`) and Tesseract binaries + `tessdata/{fast,normal,best}` are embedded via `go:embed embed` and self-extracted once per process to `%TEMP%/eprocIA_bin` (skipped if already present with matching size). `RunOCR` renders one PDF page to PNG via `pdftoppm`, then runs `tesseract.exe` with the Portuguese model, falling back to the default model on failure. `RunOCRPaginas` does several pages of one PDF with one `pdftoppm` per contiguous run and a single `tesseract` over a list file (texts split on `\f`, which tesseract puts *between* pages, not after the last); the pipeline uses it through `PDFParser.comOCREmLote`, splitting each document's OCR pages into at most `numWorkers` blocks, and redoes a failed block page by page. This is Windows-only (`.exe`, `explorer.exe` calls in `app.go`).

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

- `github.com/dslipak/pdf` loops forever when a page's `/Contents` is an array, missing or corrupt, and panics on unsupported filters. `checkContentStream` and the `recover` in `internal/etl/pdf_parser.go` guard against it.
- `getlantern/systray` on Windows needs ICO data; a PNG fails with `Unable to set icon` and leaves no way to reopen or quit a hidden window.
- `tesseract.exe` and `pdftoppm.exe` started from the GUI app need `CREATE_NO_WINDOW` (`internal/ocr/proc_windows.go`), or every OCR'd page flashes a console window.
- Pages whose text is drawn as vector outlines (iLovePDF) have no native text and no large images; `HasDrawnContent` sends them to OCR.
- The published Wails CLI (v2.12.0 with `x/tools v0.30.0`, v2.13.0 with `v0.44.0`) fails under Go 1.27 with `package "bytes" without types was imported`. Rebuild it with `pwsh scripts/build/recompilar-wails.ps1` (defaults to the go.mod version, v2.13.0). Drop the script once a published Wails CLI builds this repo under the go.mod Go version.
