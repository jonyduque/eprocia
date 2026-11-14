package ocr

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// ErrSenha indica um PDF que o Poppler recusou por exigir senha.
var ErrSenha = errors.New("PDF protegido por senha")

// maxStderr limita quanto do stderr de um processo entra na mensagem de erro.
const maxStderr = 1024

var reNumPaginas = regexp.MustCompile(`(?m)^Pages:\s+(\d+)\s*$`)

// limitado guarda só os primeiros maxStderr bytes do que lhe escrevem.
type limitado struct{ buf bytes.Buffer }

func (l *limitado) Write(p []byte) (int, error) {
	if falta := maxStderr - l.buf.Len(); falta > 0 {
		l.buf.Write(p[:min(len(p), falta)])
	}
	return len(p), nil
}

func (l *limitado) String() string { return strings.TrimSpace(l.buf.String()) }

// popplerFerramentas devolve a pasta do Poppler extraída, sem tocar no
// tesseract nem no tessdata: InfoPDF e TextoPDF valem com o OCR desligado.
func popplerFerramentas() (string, error) {
	dir, err := initPoppler()
	if err != nil {
		log.Printf("[OCR] Falha ao inicializar o Poppler: %v", err)
		return "", err
	}
	return dir, nil
}

// rodarPoppler roda um executável do Poppler e devolve o stdout.
func rodarPoppler(ctx context.Context, popplerDir, exe string, prazo time.Duration, args ...string) (string, error) {
	caminho := filepath.Join(popplerDir, exe)
	if _, err := os.Stat(caminho); err != nil {
		return "", fmt.Errorf("%s embutido não encontrado em %s: %w", exe, caminho, err)
	}
	var saida bytes.Buffer
	var erroPadrao limitado
	err := rodarComPrazo(ctx, prazo, func(ctx context.Context) *exec.Cmd {
		cmd := exec.CommandContext(ctx, caminho, args...)
		cmd.Stdout = &saida
		cmd.Stderr = &erroPadrao
		prepareCmd(cmd)
		cmd.Env = append(os.Environ(), "PATH="+popplerDir+";"+os.Getenv("PATH"))
		return cmd
	})
	if err != nil {
		if strings.Contains(erroPadrao.String(), "Incorrect password") {
			return "", fmt.Errorf("falha no %s: %w", exe, ErrSenha)
		}
		if det := erroPadrao.String(); det != "" {
			return "", fmt.Errorf("falha no %s: %w: %s", exe, err, det)
		}
		return "", fmt.Errorf("falha no %s: %w", exe, err)
	}
	return saida.String(), nil
}

// InfoPDF devolve o número de páginas lido pelo pdfinfo.
func InfoPDF(ctx context.Context, pdfPath string) (paginas int, err error) {
	dir, err := popplerFerramentas()
	if err != nil {
		return 0, err
	}
	saida, err := rodarPoppler(ctx, dir, "pdfinfo.exe", prazoInfo, pdfPath)
	if err != nil {
		return 0, err
	}
	m := reNumPaginas.FindStringSubmatch(strings.ReplaceAll(saida, "\r", ""))
	if m == nil {
		return 0, fmt.Errorf("o pdfinfo não informou o número de páginas de %s", pdfPath)
	}
	n, err := strconv.Atoi(m[1])
	if err != nil {
		return 0, fmt.Errorf("número de páginas inválido no pdfinfo (%q): %w", m[1], err)
	}
	if n <= 0 {
		return 0, fmt.Errorf("o pdfinfo informou %d páginas em %s", n, pdfPath)
	}
	return n, nil
}

// TextoPDF devolve o texto nativo de cada página (índice 0 = página 1),
// sempre com len == paginas.
func TextoPDF(ctx context.Context, pdfPath string, paginas int) ([]string, error) {
	if paginas <= 0 {
		return nil, fmt.Errorf("número de páginas inválido: %d", paginas)
	}
	dir, err := popplerFerramentas()
	if err != nil {
		return nil, err
	}
	prazo := prazoDe(prazoTextoBase, prazoTextoPorPagina, paginas)
	saida, err := rodarPoppler(ctx, dir, "pdftotext.exe", prazo,
		"-enc", "UTF-8", "-f", "1", "-l", strconv.Itoa(paginas), pdfPath, "-")
	if err != nil {
		return nil, err
	}
	saida = strings.ReplaceAll(saida, "\r\n", "\n")
	// O pdftotext põe um \f depois de cada página, inclusive da última.
	textos := strings.Split(saida, "\f")
	if n := len(textos); n > 0 && textos[n-1] == "" {
		textos = textos[:n-1]
	}
	if len(textos) != paginas {
		return nil, fmt.Errorf("o pdftotext devolveu %d páginas de %s, esperava %d", len(textos), pdfPath, paginas)
	}
	return textos, nil
}
