package ocr

import (
	"context"
	"embed"
	"fmt"
	"io"
	"io/fs"
	"log"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strings"
	"sync"

	"eprocia/internal/ocr/versao"
)

//go:generate go run ./versao/gerar

//go:embed embed
var EmbeddedFiles embed.FS

// arquivoVersao guarda, em cada diretório extraído, o versaoEmbutidos que o
// gravou. Com o carimbo igual basta comparar tamanhos, sem ler conteúdo.
const arquivoVersao = ".versao"

var (
	binDir       string
	tessdataDirs = map[string]string{}
	initMu       sync.Mutex
)

// InitBinaries extracts poppler and tesseract to a temp directory; the
// tessdata of each mode is extracted on demand by tessdataDoModo.
// Success is cached for the process; a failure is not, so the next OCR call
// retries instead of failing until the app is restarted.
func InitBinaries() (string, error) {
	initMu.Lock()
	defer initMu.Unlock()
	if binDir != "" {
		return binDir, nil
	}

	tempDir := filepath.Join(os.TempDir(), "eprocIA_bin")
	log.Printf("[OCR] Iniciando verificação e extração do embed.FS para %s...", tempDir)
	if err := os.MkdirAll(tempDir, 0755); err != nil {
		log.Printf("[OCR] Falha ao criar diretório de binários: %v", err)
		return "", fmt.Errorf("failed to create temp bin dir: %w", err)
	}
	if err := extrairBase(tempDir); err != nil {
		log.Printf("[OCR] Falha ao extrair arquivos embutidos: %v", err)
		return "", fmt.Errorf("failed to extract embedded binaries: %w", err)
	}

	log.Printf("[OCR] Verificação/extração de binários concluída com sucesso no diretório: %s", tempDir)
	binDir = tempDir
	return binDir, nil
}

// tessdataDoModo extrai, uma vez por processo, o tessdata do modo pedido.
func tessdataDoModo(base, modo string) (string, error) {
	initMu.Lock()
	defer initMu.Unlock()
	if dir, ok := tessdataDirs[modo]; ok {
		return dir, nil
	}
	dir, err := extrairTessdata(base, modo)
	if err != nil {
		return "", err
	}
	tessdataDirs[modo] = dir
	return dir, nil
}

// CalcularVersaoEmbutidos recalcula o carimbo de embed/; o teste o compara com
// a constante gerada para pegar um versao_embed.go esquecido.
func CalcularVersaoEmbutidos() (string, error) {
	return versao.Calcular(EmbeddedFiles, "embed")
}

func extrairBase(dest string) error {
	for _, sub := range []string{"poppler", "tesseract"} {
		fontes, err := arquivosDe("embed/" + sub)
		if err != nil {
			return err
		}
		if err := sincronizar(filepath.Join(dest, sub), fontes); err != nil {
			return err
		}
	}
	return nil
}

// extrairTessdata grava em dest/tessdata/<modo> o por.traineddata do modo e o
// eng.traineddata compartilhado, que o fallback do tesseract procura no mesmo
// TESSDATA_PREFIX.
func extrairTessdata(dest, modo string) (string, error) {
	fontes, err := arquivosDe("embed/tessdata/" + modo)
	if err != nil {
		return "", err
	}
	fontes = append(fontes, "embed/tessdata/eng.traineddata")
	dir := filepath.Join(dest, "tessdata", modo)
	if err := sincronizar(dir, fontes); err != nil {
		return "", err
	}
	return dir, nil
}

func arquivosDe(dir string) ([]string, error) {
	entradas, err := fs.ReadDir(EmbeddedFiles, dir)
	if err != nil {
		return nil, err
	}
	var fontes []string
	for _, e := range entradas {
		if !e.IsDir() {
			fontes = append(fontes, path.Join(dir, e.Name()))
		}
	}
	return fontes, nil
}

// sincronizar copia as fontes embutidas para dir. Com o carimbo da versão atual
// só regrava arquivo de tamanho diferente (gravação interrompida, por exemplo);
// sem ele, regrava tudo e grava o carimbo por último.
func sincronizar(dir string, fontes []string) error {
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	carimbo := filepath.Join(dir, arquivoVersao)
	atual := false
	if b, err := os.ReadFile(carimbo); err == nil && string(b) == versaoEmbutidos {
		atual = true
	}

	var buf []byte
	for _, src := range fontes {
		info, err := fs.Stat(EmbeddedFiles, src)
		if err != nil {
			return err
		}
		dst := filepath.Join(dir, path.Base(src))
		if atual {
			if st, err := os.Stat(dst); err == nil && st.Size() == info.Size() {
				continue
			}
		}
		if buf == nil {
			buf = make([]byte, 256<<10)
		}
		if err := copiarEmbutido(src, dst, buf); err != nil {
			return fmt.Errorf("falha ao extrair %s: %w", src, err)
		}
	}
	if atual {
		return nil
	}
	return os.WriteFile(carimbo, []byte(versaoEmbutidos), 0644)
}

// copiarEmbutido grava src em um temporário ao lado de dst e renomeia, para que
// uma cópia interrompida nunca fique com o nome final.
func copiarEmbutido(src, dst string, buf []byte) error {
	in, err := EmbeddedFiles.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	tmp, err := os.CreateTemp(filepath.Dir(dst), ".extraindo-*")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	// Os wrappers escondem ReadFrom/WriteTo para que io.CopyBuffer use buf em
	// vez de alocar 32 KB por arquivo.
	_, err = io.CopyBuffer(struct{ io.Writer }{tmp}, struct{ io.Reader }{in}, buf)
	if cerr := tmp.Close(); err == nil {
		err = cerr
	}
	if err == nil {
		err = os.Chmod(tmpName, 0755)
	}
	if err == nil {
		err = os.Rename(tmpName, dst)
	}
	if err != nil {
		os.Remove(tmpName)
	}
	return err
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
	tessdataPath, err := tessdataDoModo(binDir, mode)
	if err != nil {
		log.Printf("[OCR] Falha ao extrair tessdata do modo %s: %v", mode, err)
		return "", "", err
	}
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
