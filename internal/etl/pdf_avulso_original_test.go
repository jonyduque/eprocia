package etl

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"eprocia/internal/ocr"

	"github.com/dslipak/pdf"
	"golang.org/x/sys/windows"
)

// Testes da Task 4: o original do PDF avulso nunca é alterado, apagado nem
// usado como temporário, e o arquivo trocado depois da conferência de
// LerEntrada termina com a frase fixa. Todo conteúdo é fictício.

// semTocarOriginal falha se o caminho recebido por um leitor externo (Poppler
// ou OCR) é o do PDF do usuário.
func semTocarOriginal(t *testing.T, mu *sync.Mutex, vistos *[]string, original string) func(string) {
	return func(c string) {
		mu.Lock()
		defer mu.Unlock()
		*vistos = append(*vistos, c)
		if mesmoCaminho(c, original) {
			t.Errorf("leitor externo recebeu o caminho do original: %s", c)
		}
	}
}

func TestProcessarEntrada_OriginalPreservado(t *testing.T) {
	casos := []struct {
		nome     string
		conteudo func(t *testing.T) []byte
		ocrMode  string
		real     bool
		preparar func(t *testing.T, original string) // ganchos
		cancelar bool
	}{
		{nome: "sucesso", conteudo: func(*testing.T) []byte {
			return pdfComTexto("Sentenca ficticia de FULANO DE TAL", "Segunda pagina ficticia")
		}, ocrMode: "none"},
		{nome: "erro", conteudo: func(*testing.T) []byte { return pdfTruncado() }, ocrMode: "none"},
		{nome: "cancelamento", conteudo: func(*testing.T) []byte {
			return pdfComTexto("Pagina um ficticia de FULANO DE TAL", "Pagina dois ficticia", "Pagina tres ficticia")
		}, ocrMode: "none", cancelar: true},
		{nome: "panic", conteudo: func(*testing.T) []byte { return pdfComTexto("Sentenca ficticia de FULANO DE TAL") }, ocrMode: "none",
			preparar: func(t *testing.T, _ string) {
				antigo := contarPaginasPDF
				t.Cleanup(func() { contarPaginasPDF = antigo })
				contarPaginasPDF = func(io.ReaderAt, int64) (*pdf.Reader, int, error) {
					panic("pdf corrompido ficticio")
				}
			}},
		{nome: "prazo", conteudo: func(t *testing.T) []byte { return buildPDF(objetosDigitalizado(t)) }, ocrMode: "fast",
			preparar: func(t *testing.T, original string) {
				var mu sync.Mutex
				var vistos []string
				ver := semTocarOriginal(t, &mu, &vistos, original)
				antigoPagina, antigoPaginas := executarOCR, executarOCRPaginas
				t.Cleanup(func() { executarOCR, executarOCRPaginas = antigoPagina, antigoPaginas })
				executarOCR = func(_ context.Context, c string, _ int, _ string) (string, error) {
					ver(c)
					return "", fmt.Errorf("falha no tesseract embutido: %w", ocr.ErrPrazo)
				}
				executarOCRPaginas = func(_ context.Context, c string, _ []int, _ string, _ func(int)) ([]string, error) {
					ver(c)
					return nil, fmt.Errorf("falha no tesseract embutido: %w", ocr.ErrPrazo)
				}
				t.Cleanup(func() {
					if len(vistos) == 0 {
						t.Error("o OCR falso não foi chamado")
					}
				})
			}},
		{nome: "poppler", conteudo: func(*testing.T) []byte { return pdfEmEnvelope(buildPDF(objetosTexto())) }, ocrMode: "none", real: true,
			preparar: func(t *testing.T, original string) {
				var mu sync.Mutex
				var vistos []string
				ver := semTocarOriginal(t, &mu, &vistos, original)
				trocarPoppler(t, func(ctx context.Context, c string) (int, error) {
					ver(c)
					return ocr.InfoPDF(ctx, c)
				}, func(ctx context.Context, c string, n int) ([]string, error) {
					ver(c)
					return ocr.TextoPDF(ctx, c, n)
				})
				t.Cleanup(func() {
					if len(vistos) == 0 {
						t.Error("o Poppler não foi chamado")
					}
				})
			}},
		{nome: "acima_do_teto", conteudo: func(*testing.T) []byte { return pdfAcimaDoTeto("TEXTO-ACIMA-DO-TETO-FICTICIO") }, ocrMode: "none"},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			var tmp string
			if c.real {
				if testing.Short() {
					t.Skip("usa o Poppler real")
				}
				// Não isolarTemp: o pacote ocr guarda a pasta da primeira extração.
				tmp = tempClasses(t)
			} else {
				tmp = isolarTemp(t)
			}
			fixarRelogio(t)
			dir := t.TempDir()
			original := gravarArquivo(t, filepath.Join(dir, nomeAvulso), c.conteudo(t))
			antes := sha(t, original)
			if c.preparar != nil {
				c.preparar(t, original)
			}
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			var progresso func(float64, string, int, int, int, int)
			if c.cancelar {
				var uma sync.Once
				progresso = func(float64, string, int, int, int, int) { uma.Do(cancel) }
			}
			_, _, _, _, err := NewPipeline().ProcessarEntrada(ctx, Entrada{Kind: "pdf", Caminhos: []string{original}}, cfgAvulso(c.ocrMode), progresso, nil)
			if c.cancelar && err == nil {
				t.Error("cancelamento sem erro")
			}
			if !c.cancelar && err != nil {
				t.Errorf("ProcessarEntrada: %v", err)
			}
			if _, errStat := os.Stat(original); errStat != nil {
				t.Fatalf("original sumiu: %v", errStat)
			}
			if sha(t, original) != antes {
				t.Error("original alterado")
			}
			semTemporarios(t, tmp)
		})
	}
}

// trocaDoArquivo é o que acontece com o PDF depois da conferência.
type trocaDoArquivo struct {
	nome  string
	fazer func(t *testing.T, caminho string, original []byte)
	frase string
}

const (
	fraseMudou = "[ERRO ao abrir PDF: o arquivo mudou durante a leitura]"
	fraseDisco = "[ERRO ao abrir PDF: falha de leitura ou gravação em disco]"
)

func trocasDoArquivo(acimaDoTeto bool) []trocaDoArquivo {
	maior := func(t *testing.T, c string, original []byte) {
		gravarArquivo(t, c, append(append([]byte{}, original...), []byte("\n% enchimento ficticio maior\n")...))
	}
	menor := func(t *testing.T, c string, original []byte) {
		gravarArquivo(t, c, original[:len(original)/2])
	}
	zero := func(t *testing.T, c string, _ []byte) { gravarArquivo(t, c, nil) }
	// Acima do teto, o apagado some antes da própria extração na pré-contagem
	// (a troca acontece quando o outro PDF é contado): a extração falha, a
	// fonte não fica guardada, e o worker, que tenta de novo, também não o
	// acha. Termina com a frase de disco, como o apagado abaixo do teto.
	apagado := func(t *testing.T, c string, _ []byte) {
		if err := os.Remove(c); err != nil {
			t.Errorf("apagar: %v", err)
		}
	}
	return []trocaDoArquivo{{"maior", maior, fraseMudou}, {"menor", menor, fraseMudou}, {"zero_bytes", zero, fraseMudou},
		{"apagado", apagado, fraseDisco}}
}

func TestProcessarEntrada_ArquivoTrocadoNoMeio(t *testing.T) {
	const textoOutro = "CONTESTACAO-FICTICIA-DE-FULANO-DE-TAL"
	for _, teto := range []bool{false, true} {
		for _, tr := range trocasDoArquivo(teto) {
			nomeCaso := tr.nome
			if teto {
				nomeCaso = "acima_do_teto/" + tr.nome
			}
			t.Run(nomeCaso, func(t *testing.T) {
				tmp := isolarTemp(t)
				fixarRelogio(t)
				dir := t.TempDir()
				nomeAlvo := cnjAvulso + " - Evento 1 - INIC1.pdf"
				nomeOutro := cnjAvulso + " - Evento 2 - CONT1.pdf"
				var conteudo []byte
				if teto {
					conteudo = pdfAcimaDoTeto("PETICAO-FICTICIA-ACIMA-DO-TETO")
					// O alvo vai depois do outro: o temporário em disco é
					// gravado na pré-contagem, então a troca acontece quando o
					// outro é contado.
					nomeAlvo, nomeOutro = cnjAvulso+" - Evento 2 - INIC1.pdf", cnjAvulso+" - Evento 1 - CONT1.pdf"
				} else {
					conteudo = pdfComTexto("Peticao inicial ficticia de FULANO DE TAL")
				}
				alvo := gravarArquivo(t, filepath.Join(dir, nomeAlvo), conteudo)
				outro := gravarArquivo(t, filepath.Join(dir, nomeOutro), pdfComTexto(textoOutro))
				if teto {
					antigo := contarPaginasPDF
					t.Cleanup(func() { contarPaginasPDF = antigo })
					var uma sync.Once
					contarPaginasPDF = func(r io.ReaderAt, n int64) (*pdf.Reader, int, error) {
						uma.Do(func() { tr.fazer(t, alvo, conteudo) })
						return antigo(r, n)
					}
				} else {
					t.Cleanup(func() { aoFimDaPreContagem = nil })
					aoFimDaPreContagem = func(*memoriaPDF) { tr.fazer(t, alvo, conteudo) }
				}
				atv := NovaAtividade("x")
				txt := lerTexto(t, processarAvulso(t, Entrada{Kind: "pdf", Caminhos: []string{alvo, outro}}, cfgAvulso("none"), atv))
				if d := docPorNome(t, atv, nomeAlvo); d.Leitura != string(LeituraIlegivel) {
					t.Errorf("leitura = %q, esperava ilegivel", d.Leitura)
				}
				if !strings.Contains(txt, tr.frase) {
					t.Errorf("sem %q:\n%s", tr.frase, txt)
				}
				if !strings.Contains(txt, textoOutro) {
					t.Errorf("o outro PDF não foi lido:\n%s", txt)
				}
				if strings.Contains(strings.ToLower(txt), strings.ToLower(dir)) || strings.Contains(txt, filepath.ToSlash(dir)) {
					t.Errorf("TXT com a pasta do teste:\n%s", txt)
				}
				semTemporarios(t, tmp)
			})
		}
	}
}

func TestProcessarEntrada_ArquivoTravadoNoLote(t *testing.T) {
	tmp := isolarTemp(t)
	fixarRelogio(t)
	dir := t.TempDir()
	const textoOutro = "CONTESTACAO-FICTICIA-DE-FULANO-DE-TAL"
	nomeAlvo := cnjAvulso + " - Evento 1 - INIC1.pdf"
	alvo := gravarArquivo(t, filepath.Join(dir, nomeAlvo), pdfComTexto("Peticao inicial ficticia de FULANO DE TAL"))
	outro := gravarArquivo(t, filepath.Join(dir, cnjAvulso+" - Evento 2 - CONT1.pdf"), pdfComTexto(textoOutro))
	var h windows.Handle
	t.Cleanup(func() {
		aoFimDaPreContagem = nil
		if h != 0 {
			windows.CloseHandle(h)
		}
	})
	aoFimDaPreContagem = func(*memoriaPDF) {
		p, err := windows.UTF16PtrFromString(alvo)
		if err != nil {
			t.Errorf("caminho: %v", err)
			return
		}
		h, err = windows.CreateFile(p, windows.GENERIC_READ, 0 /* dwShareMode */, nil,
			windows.OPEN_EXISTING, windows.FILE_ATTRIBUTE_NORMAL, 0)
		if err != nil {
			t.Errorf("travar: %v", err)
		}
	}
	atv := NovaAtividade("x")
	txt := lerTexto(t, processarAvulso(t, Entrada{Kind: "pdf", Caminhos: []string{alvo, outro}}, cfgAvulso("none"), atv))
	if d := docPorNome(t, atv, nomeAlvo); d.Leitura != string(LeituraIlegivel) {
		t.Errorf("leitura = %q, esperava ilegivel", d.Leitura)
	}
	if !strings.Contains(txt, fraseDisco) {
		t.Errorf("sem %q:\n%s", fraseDisco, txt)
	}
	if !strings.Contains(txt, textoOutro) {
		t.Errorf("o outro PDF não foi lido:\n%s", txt)
	}
	semTemporarios(t, tmp)
}
