# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## What this is

**eprocIA** — Windows desktop ETL tool (Wails v2, Go backend + Svelte 4 frontend). Watches a folder for ZIP downloads from the Brazilian court system "Eproc", extracts PDF/HTML documents from inside the ZIP (no disk unpack), OCRs image-only PDF pages via embedded Tesseract/Poppler, sorts documents by their "Evento N" (event number) using natural sort, and writes a single structured XML-tagged `.txt` file next to the original ZIP for pasting into an LLM. Fully offline/local — no external API calls (see PRD.md RNF01).

Product spec: `PRD.md`. Visual/UX spec: `DESIGN.md`. Both in Portuguese.

## Commands

```bash
# Dev (hot reload for both Go and Svelte frontend)
wails dev

# Production build -> build/bin/
wails build

# Go tests (whole repo)
go test ./...

# Single package / single test
go test ./internal/etl -run TestSortProcessFiles -v

# Frontend only (rarely needed standalone; wails dev drives it)
cd frontend && npm install
cd frontend && npm run dev
cd frontend && npm run build

go vet ./...
```

There is no repo-level lint config beyond `go vet`; no frontend test suite.

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
  4. `formatter.GenerateConsolidatedOutput` builds the final `<contexto>/<evento>/<documento>/<pag>` XML-tagged text (format defined in PRD.md §6); `formatter.CleanText` applies the configured cleaning level (`none`/`basic`/`aggressive`) per page.
  5. Output is written as `<zipname>.txt` next to the original ZIP.
- Progress is reported both via a callback (`onProgress`) and Wails events (`process:start/progress/success/error`), tracked as pages-processed / total-pages-precomputed (PDF page counts are pre-scanned before the main loop to make the progress bar accurate).

**OCR** (`internal/ocr/tesseract.go`): Poppler (`pdftoppm.exe`) and Tesseract binaries + `tessdata/{fast,normal,best}` are embedded via `go:embed embed` and self-extracted once per process to `%TEMP%/eprocIA_bin` (skipped if already present with matching size). `RunOCR` renders one PDF page to PNG via `pdftoppm`, then runs `tesseract.exe` with the Portuguese model, falling back to the default model on failure. This is Windows-only (`.exe`, `explorer.exe` calls in `app.go`).

**Config** (`internal/config`): JSON file at `os.UserConfigDir()/eprocIA/config.json`. `Load`/`LoadFromFile` always backfill missing fields from `DefaultConfig()` rather than erroring — treat config loading as "merge with defaults", not strict parsing.

**Frontend** (`frontend/src`): plain Svelte 4 + Vite, no component framework/state library. `App.svelte` owns top-level state (queue items, config, theme) and subscribes to Wails events; `Settings.svelte` and `ProcessList.svelte` are the two child components, communicating up via `createEventDispatcher`. `frontend/src/wailsjs/` is **generated** by `wails dev`/`wails build` from the Go bindings — never hand-edit it, re-run `wails dev` if Go method signatures change and the frontend types look stale.

## Conventions worth knowing

- All Go log lines and most UI strings/errors are in Portuguese (pt-BR) — match that when adding logs/user-facing text.
- `models.ProcessFile.ZipFileRef` holds a live `*zip.File`; the underlying `zip.ReadCloser` is closed by the caller of `ReadZipFiles` (`defer rc.Close()` in `Pipeline.ProcessZip`) — don't read from a `ProcessFile` after that scope ends.
- The `eventos` map built in `Pipeline.ProcessZip` is `map[string]any` with nested nested nested `any` maps (event → documentos → paginas), pre-populated single-threaded before the concurrent document loop so each goroutine only writes into its own pre-existing leaf map — don't add new top-level map keys from inside a goroutine, only from the pre-population step.
