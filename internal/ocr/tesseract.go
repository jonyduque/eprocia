package ocr

import (
	"bytes"
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
	"slices"
	"strconv"
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
		return "", fmt.Errorf("falha ao criar a pasta temporária dos binários: %w", err)
	}
	if err := extrairBase(tempDir); err != nil {
		log.Printf("[OCR] Falha ao extrair arquivos embutidos: %v", err)
		return "", fmt.Errorf("falha ao extrair os binários embutidos: %w", err)
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

// ferramentas reúne os caminhos dos executáveis e do tessdata de um modo.
type ferramentas struct {
	tess, pdftoppm, tessDir, popplerDir, tessdata string
}

func prepararFerramentas(mode string) (ferramentas, error) {
	binDir, err := InitBinaries()
	if err != nil {
		log.Printf("[OCR] Falha ao inicializar binários: %v", err)
		return ferramentas{}, err
	}
	f := ferramentas{
		tessDir:    filepath.Join(binDir, "tesseract"),
		popplerDir: filepath.Join(binDir, "poppler"),
	}
	f.tess = filepath.Join(f.tessDir, "tesseract.exe")
	f.pdftoppm = filepath.Join(f.popplerDir, "pdftoppm.exe")

	if _, err := os.Stat(f.tess); err != nil {
		return ferramentas{}, fmt.Errorf("tesseract.exe embutido não encontrado em %s: %w", f.tess, err)
	}
	if _, err := os.Stat(f.pdftoppm); err != nil {
		return ferramentas{}, fmt.Errorf("pdftoppm.exe embutido não encontrado em %s: %w", f.pdftoppm, err)
	}

	if mode != "fast" && mode != "normal" && mode != "best" {
		mode = "normal"
	}
	f.tessdata, err = tessdataDoModo(binDir, mode)
	if err != nil {
		log.Printf("[OCR] Falha ao extrair tessdata do modo %s: %v", mode, err)
		return ferramentas{}, err
	}
	log.Printf("[OCR] Usando tessdata no modo %s em: %s", mode, f.tessdata)
	return f, nil
}

// renderizar grava em dir as páginas primeira..ultima como p-<n>.png.
func (f ferramentas) renderizar(ctx context.Context, pdfPath, dir string, primeira, ultima int) error {
	log.Printf("[OCR] Executando pdftoppm para as páginas %d a %d do PDF %s...", primeira, ultima, pdfPath)
	cmd := exec.CommandContext(ctx, f.pdftoppm,
		"-f", strconv.Itoa(primeira),
		"-l", strconv.Itoa(ultima),
		"-png",
		"-r", "150",
		pdfPath,
		filepath.Join(dir, "p"),
	)
	prepareCmd(cmd)
	// As DLLs do Poppler são achadas pelo PATH.
	cmd.Env = append(os.Environ(), "PATH="+f.popplerDir+";"+os.Getenv("PATH"))
	if err := cmd.Run(); err != nil {
		log.Printf("[OCR] pdftoppm falhou: %v", err)
		return fmt.Errorf("falha no pdftoppm embutido: %w", err)
	}
	return nil
}

// separadorPaginas acumula a saída do tesseract e chama aoConcluir(i) quando
// chega o \f que separa a imagem i da seguinte. O tesseract escreve o separador
// antes do texto da próxima imagem, então a última só se conclui quando o
// processo termina.
type separadorPaginas struct {
	buf        []byte
	lidos      int
	concluidas int
	aoConcluir func(i int)
}

func (s *separadorPaginas) Write(p []byte) (int, error) {
	s.buf = append(s.buf, p...)
	s.separar()
	return len(p), nil
}

// ReadFrom lê direto para buf; sem ele o exec copiaria a saída por um buffer
// intermediário de 32 KB a cada processo.
func (s *separadorPaginas) ReadFrom(r io.Reader) (int64, error) {
	var total int64
	for {
		if cap(s.buf)-len(s.buf) < 512 {
			s.buf = slices.Grow(s.buf, max(4096, cap(s.buf)))
		}
		n, err := r.Read(s.buf[len(s.buf):cap(s.buf)])
		s.buf = s.buf[:len(s.buf)+n]
		total += int64(n)
		s.separar()
		if err == io.EOF {
			return total, nil
		}
		if err != nil {
			return total, err
		}
	}
}

func (s *separadorPaginas) separar() {
	for {
		i := bytes.IndexByte(s.buf[s.lidos:], '\f')
		if i < 0 {
			break
		}
		s.lidos += i + 1
		if s.aoConcluir != nil {
			s.aoConcluir(s.concluidas)
		}
		s.concluidas++
	}
	s.lidos = len(s.buf)
}

// String devolve a saída com \n: no Windows o tesseract escreve a saída padrão
// em modo texto (\r\n), ao contrário do arquivo que gravava antes. O \r sai
// no próprio buf, para não copiar a saída duas vezes.
func (s *separadorPaginas) String() string {
	out := s.buf[:0]
	for i, b := range s.buf {
		if b == '\r' && i+1 < len(s.buf) && s.buf[i+1] == '\n' {
			continue
		}
		out = append(out, b)
	}
	s.buf, s.lidos = out, len(out)
	return string(out)
}

// reconhecer roda o tesseract em dir sobre entrada (uma imagem ou um arquivo
// com uma imagem por linha) e devolve o texto que ele escreve na saída padrão.
// Os caminhos são relativos a dir, para que um %TEMP% com acentos não dependa
// de como o tesseract decodifica o arquivo de lista. aoConcluir(i) é chamado
// uma vez por imagem, assim que o texto dela termina de chegar.
func (f ferramentas) reconhecer(ctx context.Context, dir, entrada string, aoConcluir func(i int)) (string, error) {
	env := append(os.Environ(),
		"PATH="+f.tessDir+";"+os.Getenv("PATH"),
		"TESSDATA_PREFIX="+f.tessdata,
		"OMP_NUM_THREADS=1",
		"OMP_THREAD_LIMIT=1",
	)
	// O fallback refaz as imagens que a primeira tentativa já avisou.
	avisadas := 0
	avisar := func(i int) {
		if i >= avisadas {
			avisadas = i + 1
			if aoConcluir != nil {
				aoConcluir(i)
			}
		}
	}
	rodar := func(args ...string) (string, error) {
		saida := &separadorPaginas{aoConcluir: avisar}
		cmd := exec.CommandContext(ctx, f.tess, append([]string{entrada, "stdout"}, args...)...)
		cmd.Dir = dir
		cmd.Stdout = saida
		prepareCmd(cmd)
		cmd.Env = env
		if err := cmd.Run(); err != nil {
			return "", err
		}
		avisar(saida.concluidas)
		return saida.String(), nil
	}

	log.Printf("[OCR] Executando tesseract no idioma 'por'...")
	texto, err := rodar("-l", "por")
	if err != nil {
		log.Printf("[OCR] Tesseract em português falhou: %v. Tentando inglês como fallback...", err)
		texto, err = rodar()
		if err != nil {
			log.Printf("[OCR] Tesseract fallback (inglês) falhou: %v", err)
			return "", fmt.Errorf("falha no tesseract embutido: %w", err)
		}
	}
	return texto, nil
}

// RunOCR faz o OCR de uma página do PDF no modo pedido (fast, normal ou best).
func RunOCR(ctx context.Context, pdfPath string, pageNum int, mode string) (string, error) {
	textos, err := ocrPaginas(ctx, pdfPath, []int{pageNum}, mode, nil)
	if err != nil {
		return "", err
	}
	return textos[0], nil
}

// RunOCRPaginas faz o OCR de várias páginas distintas do mesmo PDF e devolve os
// textos na ordem de paginas. Abre um pdftoppm por trecho contíguo e um único
// tesseract, que carrega o modelo uma vez para todas as páginas. aoConcluir,
// se não for nil, recebe cada número de página assim que o texto dela sai do
// tesseract, antes do fim do bloco.
func RunOCRPaginas(ctx context.Context, pdfPath string, paginas []int, mode string, aoConcluir func(pagina int)) ([]string, error) {
	return ocrPaginas(ctx, pdfPath, paginas, mode, aoConcluir)
}

func ocrPaginas(ctx context.Context, pdfPath string, paginas []int, mode string, aoConcluir func(pagina int)) ([]string, error) {
	if len(paginas) == 0 {
		return nil, nil
	}
	log.Printf("[OCR] Iniciando OCR do PDF %s, páginas %v, modo %s", pdfPath, paginas, mode)
	f, err := prepararFerramentas(mode)
	if err != nil {
		return nil, err
	}

	tempDir, err := os.MkdirTemp("", "eprocia_ocr_run")
	if err != nil {
		return nil, fmt.Errorf("falha ao criar a pasta temporária do OCR: %w", err)
	}
	defer func() {
		log.Printf("[OCR] Removendo pasta temporária de processamento: %s", tempDir)
		os.RemoveAll(tempDir)
	}()

	ordenadas := slices.Clone(paginas)
	slices.Sort(ordenadas)
	if len(slices.Compact(ordenadas)) != len(paginas) {
		return nil, fmt.Errorf("páginas repetidas no pedido de OCR: %v", paginas)
	}
	for i := 0; i < len(ordenadas); {
		j := i
		for j+1 < len(ordenadas) && ordenadas[j+1] == ordenadas[j]+1 {
			j++
		}
		if err := f.renderizar(ctx, pdfPath, tempDir, ordenadas[i], ordenadas[j]); err != nil {
			return nil, err
		}
		i = j + 1
	}

	// O pdftoppm completa o número com zeros conforme o total de páginas do
	// PDF (p-1.png, p-01.png...), então o nome é lido de volta do diretório.
	entradas, err := os.ReadDir(tempDir)
	if err != nil {
		return nil, fmt.Errorf("falha ao ler a pasta temporária do OCR: %w", err)
	}
	porPagina := make(map[int]string, len(entradas))
	for _, e := range entradas {
		num, ok := strings.CutPrefix(e.Name(), "p-")
		if num, ok2 := strings.CutSuffix(num, ".png"); ok && ok2 {
			if n, err := strconv.Atoi(num); err == nil {
				porPagina[n] = e.Name()
			}
		}
	}
	nomes := make([]string, len(paginas))
	for i, p := range paginas {
		nome, ok := porPagina[p]
		if !ok {
			return nil, fmt.Errorf("imagem da página %d não encontrada na pasta temporária do OCR", p)
		}
		nomes[i] = nome
	}

	entrada := nomes[0]
	if len(nomes) > 1 {
		entrada = "lista.txt"
		if err := os.WriteFile(filepath.Join(tempDir, entrada), []byte(strings.Join(nomes, "\n")+"\n"), 0644); err != nil {
			return nil, fmt.Errorf("falha ao gravar a lista de imagens do OCR: %w", err)
		}
	}
	var avisar func(i int)
	if aoConcluir != nil {
		avisar = func(i int) {
			if i < len(paginas) {
				aoConcluir(paginas[i])
			}
		}
	}
	texto, err := f.reconhecer(ctx, tempDir, entrada, avisar)
	if err != nil {
		return nil, err
	}
	log.Printf("[OCR] OCR concluído. Caracteres extraídos: %d", len(texto))
	if len(nomes) == 1 {
		return []string{texto}, nil
	}

	// O tesseract separa o texto de cada imagem da lista com \f, sem
	// separador depois da última, como na saída de uma imagem só.
	partes := strings.Split(texto, "\f")
	if len(partes) != len(nomes) {
		return nil, fmt.Errorf("o tesseract devolveu %d páginas para %d imagens", len(partes), len(nomes))
	}
	return partes, nil
}
