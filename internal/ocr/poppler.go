package ocr

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
)

// ErrSenha indica um PDF que o Poppler recusou por exigir senha.
var ErrSenha = errors.New("PDF protegido por senha")

// maxStderr limita quanto do stderr de um processo entra na mensagem de erro.
const maxStderr = 1024

var reNumPaginas = regexp.MustCompile(`(?m)^Pages:[ \t]+(\d+)[ \t]*$`)

// maxSaidaPoppler é o teto do stdout de um processo do Poppler: muito acima de
// um documento real e abaixo de um uso de memória que derrube o app.
var maxSaidaPoppler = 64 << 20

// errSaidaGrande indica que o stdout de um processo do Poppler passou do teto.
var errSaidaGrande = errors.New("saída do Poppler acima do limite")

// limitado guarda só os primeiros max bytes do que lhe escrevem e marca se
// descartou algo. Descarta em vez de devolver erro: com erro no Write o exec
// para de ler e o filho trava no pipe até o prazo.
type limitado struct {
	buf    bytes.Buffer
	max    int
	cortou bool
}

func (l *limitado) Write(p []byte) (int, error) {
	falta := max(l.max-l.buf.Len(), 0)
	if falta < len(p) {
		l.cortou = true
	}
	l.buf.Write(p[:min(len(p), falta)])
	return len(p), nil
}

// blocoLimitado é o menor pedaço que ReadFrom reserva de uma vez no buffer.
const blocoLimitado = 512

// tamDescarte é o bloco de leitura do descarte depois do limite: com 512 bytes,
// 80 MiB acima do teto custavam ~160 mil leituras.
const tamDescarte = 32 << 10

// descartes guarda os blocos de descarte entre processos: o conteúdo é jogado
// fora, e o pool evita alocar 32 KiB a cada stderr que passa do limite.
var descartes = sync.Pool{New: func() any { return new([tamDescarte]byte) }}

// ReadFrom lê de r até o EOF direto no espaço livre do buffer, sem passar do
// limite, e descarta o resto em blocos de tamDescarte (só pegos do pool quando
// o descarte começa), continuando a ler para o filho não travar com o pipe
// cheio. Sem ele, o io.Copy do exec (cmd.Stdout e cmd.Stderr que não são
// *os.File) aloca 32 KiB por processo.
func (l *limitado) ReadFrom(r io.Reader) (int64, error) {
	var total int64
	var lixo []byte
	defer func() {
		if lixo != nil {
			descartes.Put((*[tamDescarte]byte)(lixo))
		}
	}()
	for {
		var b []byte
		if falta := l.max - l.buf.Len(); falta > 0 {
			l.buf.Grow(min(falta, blocoLimitado))
			b = l.buf.AvailableBuffer()
			b = b[:min(cap(b), falta)]
		} else {
			if lixo == nil {
				lixo = descartes.Get().(*[tamDescarte]byte)[:]
			}
			b = lixo
		}
		n, err := r.Read(b)
		total += int64(n)
		if n > 0 {
			if lixo == nil {
				l.buf.Write(b[:n])
			} else {
				l.cortou = true
			}
		}
		if err == io.EOF {
			return total, nil
		}
		if err != nil {
			return total, err
		}
	}
}

// String serve só ao stderr.
func (l *limitado) String() string { return strings.TrimSpace(l.buf.String()) }

// popplerFerramentas devolve a pasta do Poppler extraída, sem tocar no
// tesseract nem no tessdata: InfoPDF e TextoPDF valem com o OCR desligado.
func popplerFerramentas() (string, error) {
	return initPoppler()
}

// rodarPoppler roda um executável do Poppler e devolve o stdout.
func rodarPoppler(ctx context.Context, popplerDir, exe string, prazo time.Duration, args ...string) (string, error) {
	caminho := filepath.Join(popplerDir, exe)
	if _, err := os.Stat(caminho); err != nil {
		return "", fmt.Errorf("%s embutido não encontrado em %s: %w", exe, caminho, err)
	}
	saida := limitado{max: maxSaidaPoppler}
	// O stderr fica com rodarComPrazo, que o devolve no *ErroProcesso.
	err := rodarComPrazo(ctx, prazo, func(ctx context.Context) *exec.Cmd {
		cmd := exec.CommandContext(ctx, caminho, args...)
		cmd.Stdout = &saida
		prepareCmd(cmd)
		cmd.Env = append(os.Environ(), "PATH="+popplerDir+";"+os.Getenv("PATH"))
		return cmd
	})
	if err != nil {
		if ep, ok := errors.AsType[*ErroProcesso](err); ok && strings.Contains(ep.Stderr, "Incorrect password") {
			// Sem o stderr na mensagem; o *ErroProcesso segue na cadeia.
			return "", fmt.Errorf("falha no %s: %w: %w", exe, ErrSenha, err)
		}
		return "", fmt.Errorf("falha no %s: %w", exe, err)
	}
	if saida.cortou {
		return "", fmt.Errorf("falha no %s: %w (%d bytes)", exe, errSaidaGrande, maxSaidaPoppler)
	}
	// Sem TrimSpace: tiraria o começo da página 1 e os \f finais.
	return saida.buf.String(), nil
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
