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
# Publish with `gh release create vX.Y.Z --target main` (creates the tag remotely;
# git push is denied in settings, the user pushes commits with `! git push`).
wails build -clean -nsis

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
```

There is no repo-level lint config beyond `go vet`; no frontend test suite. `go test ./...` includes `scripts/bench`, whose `TestFixtures_LegibleToOCR` runs the real OCR (a few seconds; skipped with `-short`).

## Architecture

**Entry point**: `main.go` embeds `frontend/dist` via `go:embed`, boots `wails.Run`, and runs `github.com/getlantern/systray` in a goroutine for the tray icon (Abrir/Ocultar/Sair). Closing the window hides it to tray (`HideWindowOnClose`); the app only fully exits via tray "Sair".

**`app.go`** — the `App` struct is bound to the frontend (all exported methods become callable JS via Wails' generated bindings in `frontend/src/wailsjs/go/main/App.js`). It's a thin layer: delegates file-drop/manual-select/config to `internal/config`, `internal/watchdog`, and the `internal/etl` queue/pipeline. Don't put business logic here — it belongs in `internal/etl`.

**Processing pipeline** (`internal/etl`):
- `watchdog.Watcher` (fsnotify) detects new `.zip` files in the monitored folder, waits for the file size to stabilize before firing the callback, then hands the path to `etl.Queue`.
- `etl.Queue` processes ZIPs **sequentially, one at a time** (`isWorking` flag + `startProcessing` self-chaining), oldest-pending-first. It tracks per-item status (`pending/processing/paused/cancelled/success/error`) and broadcasts state to the frontend via Wails events (`queue:update`, `queue:status`).
- `etl.Pipeline.ProcessZip` does the actual work for one ZIP:
  1. `zip_reader.ReadZipFiles` — lists PDF/HTML entries directly from the zip stream (skips dirs, `__MACOSX`, dotfiles).
  2. `sorter.SortProcessFiles` — sorts by `EventNum` (parsed from filename via `Evento (\d+)` regex, unmatched files sort last) then natural string compare for ties.
  3. Per-document processing runs **concurrently**, bounded by a semaphore sized `NumCPU()-1`: PDFs go through `pdf_parser.ParsePages` (native text extraction per page, page-level OCR fallback if native text is short AND the page has large embedded images), HTML goes through `html_parser.ExtractTextFromHTML` (tokenizer-based, strips `script`/`style`/`head`). OCR execution itself is bounded by a *second*, separate `NumCPU()-1` semaphore inside the pipeline (`ocrRunner`) — see review notes below on why this nesting matters.
  4. `formatter.GenerateConsolidatedOutput` builds the final `<contexto>/<evento>/<documento>/<pag>` XML-tagged text (format defined in PRD.md §6); `cleanPage` (formatter.go) applies the configured cleaning level (`none`/`basic`/`aggressive`) per page.
  5. Output is written as `<zipname>.txt` next to the original ZIP.
- Progress is reported both via a callback (`onProgress`) and Wails events (`process:start/progress/success/error`), tracked as pages-processed / total-pages-precomputed (PDF page counts are pre-scanned before the main loop to make the progress bar accurate).

**OCR** (`internal/ocr/tesseract.go`): Poppler (`pdftoppm.exe`) and Tesseract binaries + `tessdata/{fast,normal,best}` are embedded via `go:embed embed` and self-extracted once per process to `%TEMP%/eprocIA_bin` (skipped if already present with matching size). `RunOCR` renders one PDF page to PNG via `pdftoppm`, then runs `tesseract.exe` with the Portuguese model, falling back to the default model on failure. This is Windows-only (`.exe`, `explorer.exe` calls in `app.go`).

**Config** (`internal/config`): JSON file at `os.UserConfigDir()/eprocIA/config.json`. `Load`/`LoadFromFile` always backfill missing fields from `DefaultConfig()` rather than erroring — treat config loading as "merge with defaults", not strict parsing. `theme` is written only by `App.SetTheme` (SaveConfig keeps the stored one) and read in `main.go` for the native window background. `falhas.json` in the same folder (`watchdog.Falhas`) remembers failed ZIPs by path + mtime so the startup scan does not re-queue them.

**Frontend** (`frontend/src`): plain Svelte 4 + Vite, no component framework/state library. `App.svelte` owns top-level state (queue items, config, theme) and subscribes to Wails events; `Settings.svelte` and `ProcessList.svelte` are the two child components, communicating up via `createEventDispatcher`. `frontend/src/wailsjs/` is **generated** by `wails dev`/`wails build` from the Go bindings — never hand-edit it, re-run `wails dev` if Go method signatures change and the frontend types look stale.

## Conventions worth knowing

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
- The published Wails CLI v2.12.0 is built with `x/tools v0.30.0` and fails under Go 1.27 with `package "bytes" without types was imported`. Rebuild it with `pwsh scripts/build/recompilar-wails.ps1` (or fall back to `GOTOOLCHAIN=go1.25.5 wails build`).
