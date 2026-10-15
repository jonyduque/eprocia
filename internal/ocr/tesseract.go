package ocr

import (
	"bytes"
	"context"
	"embed"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
)

//go:embed embed
var EmbeddedFiles embed.FS

var (
	binDir   string
	initOnce sync.Once
	initErr  error
)

// InitBinaries extracts the poppler, tesseract, and tessdata files to a temp directory once.
func InitBinaries() (string, error) {
	initOnce.Do(func() {
		tempDir := filepath.Join(os.TempDir(), "eprocIA_bin")
		log.Printf("[OCR] Inicializando diretório temporário para binários: %s", tempDir)
		
		log.Printf("[OCR] Iniciando verificação e extração do embed.FS para %s...", tempDir)
		err := os.MkdirAll(tempDir, 0755)
		if err != nil {
			initErr = fmt.Errorf("failed to create temp bin dir: %w", err)
			return
		}

		// Extract files recursively from the embed FS (skips files with matching sizes)
		err = extractFS(EmbeddedFiles, "embed", tempDir)
		if err != nil {
			log.Printf("[OCR] Falha ao extrair arquivos embutidos: %v", err)
			initErr = fmt.Errorf("failed to extract embedded binaries: %w", err)
			return
		}

		log.Printf("[OCR] Verificação/extração de binários concluída com sucesso no diretório: %s", tempDir)
		binDir = tempDir
	})

	return binDir, initErr
}

// extractFS extracts files recursively from an embed.FS.
func extractFS(fs embed.FS, srcDir, destDir string) error {
	entries, err := fs.ReadDir(srcDir)
	if err != nil {
		return err
	}

	for _, entry := range entries {
		srcPath := srcDir + "/" + entry.Name()
		destPath := filepath.Join(destDir, entry.Name())

		if entry.IsDir() {
			err = os.MkdirAll(destPath, 0755)
			if err != nil {
				return err
			}
			err = extractFS(fs, srcPath, destPath)
			if err != nil {
				return err
			}
		} else {
			data, err := fs.ReadFile(srcPath)
			if err != nil {
				return err
			}
			
			// Skip writing only if the existing file's content is byte-identical
			// (a size-only check would keep a stale binary after an upgrade that
			// ships a same-size replacement).
			if existing, err := os.ReadFile(destPath); err == nil && bytes.Equal(existing, data) {
				continue
			}

			err = os.WriteFile(destPath, data, 0755)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

// RunOCR runs OCR on a specific page of a PDF file using the specified mode (fast, normal, best).
func RunOCR(ctx context.Context, pdfPath string, pageNum int, mode string) (string, string, error) {
	log.Printf("[OCR] Iniciando RunOCR para o PDF %s, página %d, modo %s", pdfPath, pageNum, mode)
	binDir, err := InitBinaries()
	if err != nil {
		log.Printf("[OCR] Falha ao inicializar binários: %v", err)
		return "", "", err
	}

	tessPath := filepath.Join(binDir, "tesseract", "tesseract.exe")
	pdfToPpmPath := filepath.Join(binDir, "poppler", "pdftoppm.exe")

	log.Printf("[OCR] Verificando se os caminhos dos executáveis existem...")
	// Verify paths exist
	if _, err := os.Stat(tessPath); err != nil {
		return "", "", fmt.Errorf("embedded tesseract.exe not found at %s: %w", tessPath, err)
	}
	if _, err := os.Stat(pdfToPpmPath); err != nil {
		return "", "", fmt.Errorf("embedded pdftoppm.exe not found at %s: %w", pdfToPpmPath, err)
	}

	// Validate mode
	if mode != "fast" && mode != "normal" && mode != "best" {
		mode = "normal"
	}
	tessdataPath := filepath.Join(binDir, "tessdata", mode)
	log.Printf("[OCR] Usando tessdata no modo %s em: %s", mode, tessdataPath)

	// Create temp directory for this run's images and OCR output
	tempDir, err := os.MkdirTemp("", "eprocia_ocr_run")
	if err != nil {
		return "", "", fmt.Errorf("failed to create run temp dir: %w", err)
	}
	defer func() {
		log.Printf("[OCR] Removendo pasta temporária de processamento: %s", tempDir)
		os.RemoveAll(tempDir)
	}()

	// Render PDF page to PNG using pdftoppm
	imgPrefix := filepath.Join(tempDir, "page")
	log.Printf("[OCR] Executando pdftoppm para extrair a página %d do PDF %s...", pageNum, pdfPath)
	cmdPpm := exec.CommandContext(ctx, pdfToPpmPath,
		"-f", fmt.Sprintf("%d", pageNum),
		"-l", fmt.Sprintf("%d", pageNum),
		"-png",
		"-r", "150",
		pdfPath,
		imgPrefix,
	)
	
	// Set DLL search path for Poppler on Windows by modifying PATH environment variable
	popplerDir := filepath.Join(binDir, "poppler")
	prepareCmd(cmdPpm)
	cmdPpm.Env = append(os.Environ(), "PATH="+popplerDir+";"+os.Getenv("PATH"))

	log.Printf("[OCR] Rodando pdftoppm...")
	if err := cmdPpm.Run(); err != nil {
		log.Printf("[OCR] pdftoppm falhou: %v", err)
		return "", "", fmt.Errorf("embedded pdftoppm failed: %w", err)
	}
	log.Printf("[OCR] pdftoppm terminou com sucesso.")

	// Find the rendered PNG file
	files, err := os.ReadDir(tempDir)
	if err != nil {
		return "", "", fmt.Errorf("failed to read run temp dir: %w", err)
	}

	var imgPath string
	for _, file := range files {
		if strings.HasPrefix(file.Name(), "page") && strings.HasSuffix(file.Name(), ".png") {
			imgPath = filepath.Join(tempDir, file.Name())
			break
		}
	}

	if imgPath == "" {
		return "", "", fmt.Errorf("failed to find extracted page image in run temp dir")
	}
	log.Printf("[OCR] Imagem temporária da página encontrada em: %s", imgPath)

	// Run tesseract OCR on the page image
	txtOutputPrefix := filepath.Join(tempDir, "output")
	
	// Exec tesseract
	log.Printf("[OCR] Executando tesseract no idioma 'por'...")
	cmdTess := exec.CommandContext(ctx, tessPath,
		imgPath,
		txtOutputPrefix,
		"-l", "por",
	)
	
	// Set environment variables for Tesseract DLLs and traineddata path
	tesseractDir := filepath.Join(binDir, "tesseract")
	prepareCmd(cmdTess)
	cmdTess.Env = append(os.Environ(), 
		"PATH="+tesseractDir+";"+os.Getenv("PATH"),
		"TESSDATA_PREFIX="+tessdataPath,
		"OMP_NUM_THREADS=1",
		"OMP_THREAD_LIMIT=1",
	)

	if err := cmdTess.Run(); err != nil {
		log.Printf("[OCR] Tesseract em português falhou: %v. Tentando inglês como fallback...", err)
		// Fallback to default (usually English) if Portuguese model fails
		cmdTessFallback := exec.CommandContext(ctx, tessPath,
			imgPath,
			txtOutputPrefix,
		)
		prepareCmd(cmdTessFallback)
		cmdTessFallback.Env = append(os.Environ(), 
			"PATH="+tesseractDir+";"+os.Getenv("PATH"),
			"TESSDATA_PREFIX="+tessdataPath,
			"OMP_NUM_THREADS=1",
			"OMP_THREAD_LIMIT=1",
		)
		if err := cmdTessFallback.Run(); err != nil {
			log.Printf("[OCR] Tesseract fallback (inglês) falhou: %v", err)
			return "", "", fmt.Errorf("embedded tesseract failed: %w", err)
		}
	}
	log.Printf("[OCR] Tesseract OCR finalizado com sucesso.")

	// Read result
	txtPath := txtOutputPrefix + ".txt"
	contentBytes, err := os.ReadFile(txtPath)
	if err != nil {
		return "", "", fmt.Errorf("failed to read OCR output: %w", err)
	}

	log.Printf("[OCR] OCR concluído. Caracteres extraídos: %d", len(contentBytes))
	return string(contentBytes), imgPath, nil
}
