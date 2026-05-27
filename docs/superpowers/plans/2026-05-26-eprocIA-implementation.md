# eprocIA Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build a local ETL tool (Wails v3 + Go) to consolidate judicial processes from ZIP files into structured XML-like TXT files for LLM consumption, including intelligent page-by-page OCR.

**Architecture:** Wails v3 (Frontend: React/Svelte, Backend: Go). Use `fsnotify` for directory watching, `gopdf` for PDF parsing, and embedded Tesseract for OCR.

**Tech Stack:** Go 1.21+, Wails v3, React/Svelte, fsnotify, gopdf, html2text, Tesseract OCR.

---

### Task 1: Project Initialization (Wails v3)

**Files:**
- Create: `wails.json`, `main.go`, `frontend/`
- Modify: `go.mod`

- [ ] **Step 1: Initialize Wails v3 project**
Run: `wails3 init -n eprocia -t svelte` (ou similar)
- [ ] **Step 2: Setup basic project structure**
Ensure `main.go` has the basic window and system tray setup.
- [ ] **Step 3: Commit**
`git add . && git commit -m "chore: initialize wails v3 project"`

---

### Task 2: Core Data Types and Configuration

**Files:**
- Create: `internal/config/config.go`
- Create: `internal/models/models.go`

- [ ] **Step 1: Define Configuration struct**
Include: `MonitoredPath`, `OcrEnabled`, `CleaningLevel`, `AutoStart`.
- [ ] **Step 2: Implement Save/Load Config logic**
Save to `%APPDATA%/eprocIA/config.json`.
- [ ] **Step 3: Commit**
`git add internal/ && git commit -m "feat: add config management"`

---

### Task 3: Watchdog Service

**Files:**
- Create: `internal/watchdog/watchdog.go`

- [ ] **Step 1: Implement fsnotify monitor**
Monitor the `MonitoredPath` for new `.zip` files.
- [ ] **Step 2: Implement Filter**
Only trigger for ZIPs containing at least one PDF/HTML/TXT.
- [ ] **Step 3: Integrate with Wails Events**
Emit `file:detected` event to frontend when a new ZIP starts processing.
- [ ] **Step 4: Commit**
`git add internal/watchdog/ && git commit -m "feat: implement watchdog service"`

---

### Task 4: Extraction Engine - Sorting & Indexing

**Files:**
- Create: `internal/etl/zip_reader.go`
- Create: `internal/etl/sorter.go`

- [ ] **Step 1: Implement ZIP listing**
Read ZIP structure in memory.
- [ ] **Step 2: Implement Hierarchical Sorter**
1. `Capa do processo.pdf` first.
2. Sort by Event (`$1`) then Document (`$2`) using the Regex: `Evento (\d+) - [A-Za-z_ ]+(\d+)\.(html|pdf)`.
- [ ] **Step 3: Commit**
`git add internal/etl/ && git commit -m "feat: implement zip indexing and sorting"`

---

### Task 5: PDF Parser with Intelligent Page Logic

**Files:**
- Create: `internal/etl/pdf_parser.go`

- [ ] **Step 1: Implement Page-by-Page iterator**
Use `gopdf` or similar to iterate pages.
- [ ] **Step 2: Implement Image Coverage Logic**
Calculate if images cover > 10% of page area.
- [ ] **Step 3: Implement Decision Logic**
Apply the criteria: 
- Blank page detection (no text, no images).
- OCR trigger if native extraction fails success criteria.
- [ ] **Step 4: Commit**
`git add internal/etl/ && git commit -m "feat: implement intelligent pdf parsing logic"`

---

### Task 6: Embedded OCR (Tesseract)

**Files:**
- Create: `internal/ocr/tesseract.go`
- Modify: `main.go` (for embedding)

- [ ] **Step 1: Setup //go:embed for Tesseract**
Include `tesseract.exe` and `por.traineddata`.
- [ ] **Step 2: Implement Temp Extraction**
Extract binaries to `%TEMP%/eprocIA/ocr/` on startup.
- [ ] **Step 3: Implement OCR Runner**
Run `tesseract` via `os/exec` on a per-page image basis.
- [ ] **Step 4: Commit**
`git add internal/ocr/ && git commit -m "feat: implement embedded ocr runner"`

---

### Task 7: Output Formatter (XML/Markdown)

**Files:**
- Create: `internal/etl/formatter.go`

- [ ] **Step 1: Define Templates**
Create `text/template` for Context/Summary and Event/Document/Page nodes.
- [ ] **Step 2: Implement Final Consolidation**
Concatenate all processed documents into the final `.txt`.
- [ ] **Step 3: Commit**
`git add internal/etl/ && git commit -m "feat: implement output formatter"`

---

### Task 8: Frontend - UI Implementation

**Files:**
- Modify: `frontend/src/App.svelte`
- Create: `frontend/src/components/ProcessList.svelte`
- Create: `frontend/src/components/Settings.svelte`

- [ ] **Step 1: Implement Compact Process List**
Follow `DESIGN.md` (high density, tray icon integration).
- [ ] **Step 2: Implement Drag & Drop (In/Out)**
- In: Handle file drop to start ETL.
- Out: Use Wails "Drag-to-Text" capability for processed items.
- [ ] **Step 3: Implement Settings Panel**
Toggle OCR, select path, cleaning levels.
- [ ] **Step 4: Commit**
`git add frontend/ && git commit -m "feat: implement compact ui and drag-and-drop"`

---

### Task 9: Final Integration and Testing

- [ ] **Step 1: End-to-End Test with Real Eproc ZIP**
Verify sorting, OCR of image-only pages, and XML structure.
- [ ] **Step 2: Verify System Tray and Background Operation**
Ensure it stays alive when minimized.
- [ ] **Step 3: Cleanup and Final Build**
Run `wails build` and verify the single EXE works.
- [ ] **Step 4: Commit**
`git commit -m "feat: final integration and build"`
