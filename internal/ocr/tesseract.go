package ocr

import (
	"embed"
	"fmt"
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
		
		// Check if pdftoppm.exe and tesseract.exe already exist with correct sizes to skip extraction
		pdftoppmPath := filepath.Join(tempDir, "poppler", "pdftoppm.exe")
		tesseractPath := filepath.Join(tempDir, "tesseract", "tesseract.exe")
		
		_, err1 := os.Stat(pdftoppmPath)
		_, err2 := os.Stat(tesseractPath)
		if err1 == nil && err2 == nil {
			binDir = tempDir
			return
		}

		err := os.MkdirAll(tempDir, 0755)
		if err != nil {
			initErr = fmt.Errorf("failed to create temp bin dir: %w", err)
			return
		}

		// Extract files recursively from the embed FS
		err = extractFS(EmbeddedFiles, "embed", tempDir)
		if err != nil {
			initErr = fmt.Errorf("failed to extract embedded binaries: %w", err)
			return
		}

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
			
			// If file exists and size matches, skip writing
			if info, err := os.Stat(destPath); err == nil && info.Size() == int64(len(data)) {
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
func RunOCR(pdfPath string, pageNum int, mode string) (string, error) {
	binDir, err := InitBinaries()
	if err != nil {
		return "", err
	}

	tessPath := filepath.Join(binDir, "tesseract", "tesseract.exe")
	pdfToPpmPath := filepath.Join(binDir, "poppler", "pdftoppm.exe")

	// Verify paths exist
	if _, err := os.Stat(tessPath); err != nil {
		return "", fmt.Errorf("embedded tesseract.exe not found at %s: %w", tessPath, err)
	}
	if _, err := os.Stat(pdfToPpmPath); err != nil {
		return "", fmt.Errorf("embedded pdftoppm.exe not found at %s: %w", pdfToPpmPath, err)
	}

	// Validate mode
	if mode != "fast" && mode != "normal" && mode != "best" {
		mode = "normal"
	}
	tessdataPath := filepath.Join(binDir, "tessdata", mode)

	// Create temp directory for this run's images and OCR output
	tempDir, err := os.MkdirTemp("", "eprocia_ocr_run")
	if err != nil {
		return "", fmt.Errorf("failed to create run temp dir: %w", err)
	}
	defer os.RemoveAll(tempDir)

	// Render PDF page to PNG using pdftoppm
	imgPrefix := filepath.Join(tempDir, "page")
	cmdPpm := exec.Command(pdfToPpmPath,
		"-f", fmt.Sprintf("%d", pageNum),
		"-l", fmt.Sprintf("%d", pageNum),
		"-png",
		"-r", "150",
		pdfPath,
		imgPrefix,
	)
	
	// Set DLL search path for Poppler on Windows by modifying PATH environment variable
	popplerDir := filepath.Join(binDir, "poppler")
	cmdPpm.Env = append(os.Environ(), "PATH="+popplerDir+";"+os.Getenv("PATH"))

	if err := cmdPpm.Run(); err != nil {
		return "", fmt.Errorf("embedded pdftoppm failed: %w", err)
	}

	// Find the rendered PNG file
	files, err := os.ReadDir(tempDir)
	if err != nil {
		return "", fmt.Errorf("failed to read run temp dir: %w", err)
	}

	var imgPath string
	for _, file := range files {
		if strings.HasPrefix(file.Name(), "page") && strings.HasSuffix(file.Name(), ".png") {
			imgPath = filepath.Join(tempDir, file.Name())
			break
		}
	}

	if imgPath == "" {
		return "", fmt.Errorf("failed to find extracted page image in run temp dir")
	}

	// Run tesseract OCR on the page image
	txtOutputPrefix := filepath.Join(tempDir, "output")
	
	// Exec tesseract
	cmdTess := exec.Command(tessPath,
		imgPath,
		txtOutputPrefix,
		"-l", "por",
	)
	
	// Set environment variables for Tesseract DLLs and traineddata path
	tesseractDir := filepath.Join(binDir, "tesseract")
	cmdTess.Env = append(os.Environ(), 
		"PATH="+tesseractDir+";"+os.Getenv("PATH"),
		"TESSDATA_PREFIX="+tessdataPath,
	)

	if err := cmdTess.Run(); err != nil {
		// Fallback to default (usually English) if Portuguese model fails
		cmdTessFallback := exec.Command(tessPath,
			imgPath,
			txtOutputPrefix,
		)
		cmdTessFallback.Env = append(os.Environ(), 
			"PATH="+tesseractDir+";"+os.Getenv("PATH"),
			"TESSDATA_PREFIX="+tessdataPath,
		)
		if err := cmdTessFallback.Run(); err != nil {
			return "", fmt.Errorf("embedded tesseract failed: %w", err)
		}
	}

	// Read result
	txtPath := txtOutputPrefix + ".txt"
	contentBytes, err := os.ReadFile(txtPath)
	if err != nil {
		return "", fmt.Errorf("failed to read OCR output: %w", err)
	}

	return string(contentBytes), nil
}
