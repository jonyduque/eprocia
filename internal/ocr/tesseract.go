package ocr

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// FindTesseract attempts to locate the tesseract executable.
func FindTesseract() (string, error) {
	// 1. Look in PATH
	if path, err := exec.LookPath("tesseract"); err == nil {
		return path, nil
	}

	// 2. Look in common Windows installation directories
	commonPaths := []string{
		`C:\Program Files\Tesseract-OCR\tesseract.exe`,
		`C:\Program Files (x86)\Tesseract-OCR\tesseract.exe`,
	}
	for _, p := range commonPaths {
		if _, err := os.Stat(p); err == nil {
			return p, nil
		}
	}

	return "", fmt.Errorf("tesseract executable not found")
}

// FindPdftoppm attempts to locate the pdftoppm executable.
func FindPdftoppm() (string, error) {
	// 1. Look in PATH
	if path, err := exec.LookPath("pdftoppm"); err == nil {
		return path, nil
	}

	// 2. Look in Scoop shim directory or home
	home, err := os.UserHomeDir()
	if err == nil {
		scoopPath := filepath.Join(home, "scoop", "shims", "pdftoppm.exe")
		if _, err := os.Stat(scoopPath); err == nil {
			return scoopPath, nil
		}
	}

	return "", fmt.Errorf("pdftoppm executable not found")
}

// RunOCR runs OCR on a specific page of a PDF file.
func RunOCR(pdfPath string, pageNum int) (string, error) {
	tessPath, err := FindTesseract()
	if err != nil {
		return "", err
	}

	pdfToPpmPath, err := FindPdftoppm()
	if err != nil {
		return "", err
	}

	// Create temp directory for temporary images and OCR output
	tempDir, err := os.MkdirTemp("", "eprocia_ocr")
	if err != nil {
		return "", fmt.Errorf("failed to create temp dir: %w", err)
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
	if err := cmdPpm.Run(); err != nil {
		return "", fmt.Errorf("pdftoppm failed: %w", err)
	}

	// pdftoppm appends page numbers automatically (e.g. "page-1.png" or "page-01.png")
	files, err := os.ReadDir(tempDir)
	if err != nil {
		return "", fmt.Errorf("failed to read temp dir: %w", err)
	}

	var imgPath string
	for _, file := range files {
		if strings.HasPrefix(file.Name(), "page") && strings.HasSuffix(file.Name(), ".png") {
			imgPath = filepath.Join(tempDir, file.Name())
			break
		}
	}

	if imgPath == "" {
		return "", fmt.Errorf("failed to find extracted page image in temp dir")
	}

	// Run tesseract OCR on the extracted page image
	txtOutputPrefix := filepath.Join(tempDir, "output")
	cmdTess := exec.Command(tessPath,
		imgPath,
		txtOutputPrefix,
		"-l", "por", // Portuguese language dataset
	)
	if err := cmdTess.Run(); err != nil {
		// Fallback to default (usually English) if Portuguese dataset is missing
		cmdTessFallback := exec.Command(tessPath,
			imgPath,
			txtOutputPrefix,
		)
		if err := cmdTessFallback.Run(); err != nil {
			return "", fmt.Errorf("tesseract failed: %w", err)
		}
	}

	// Read the output text file
	txtPath := txtOutputPrefix + ".txt"
	contentBytes, err := os.ReadFile(txtPath)
	if err != nil {
		return "", fmt.Errorf("failed to read OCR output file: %w", err)
	}

	return string(contentBytes), nil
}
