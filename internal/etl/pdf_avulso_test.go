package etl

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/sha256"
	"errors"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"eprocia/internal/models"
	"eprocia/internal/ocr"
)

// Testes do PDF avulso no pipeline (Task 3). Todo conteúdo é fictício.

const cnjAvulso = "5000000-00.2026.8.26.0001"

const nomeAvulso = cnjAvulso + " - Evento 12 - SENT1.pdf"

// fixarRelogio fixa a "Data de Consolidação" em 26/11/2026 10:00:00.
func fixarRelogio(t *testing.T) {
	t.Helper()
	antigo := agoraFormatter
	t.Cleanup(func() { agoraFormatter = antigo })
	agoraFormatter = func() time.Time { return time.Date(2026, 11, 26, 10, 0, 0, 0, time.Local) }
}

func cfgAvulso(ocrMode string) *models.Config {
	return &models.Config{CleaningLevel: "standard", OcrMode: ocrMode, OmitirTipos: []string{}}
}

func gravarArquivo(t *testing.T, caminho string, conteudo []byte) string {
	t.Helper()
	if err := os.WriteFile(caminho, conteudo, 0o644); err != nil {
		t.Fatalf("gravar %s: %v", caminho, err)
	}
	return caminho
}

func sha(t *testing.T, caminho string) [32]byte {
	t.Helper()
	b, err := os.ReadFile(caminho)
	if err != nil {
		t.Fatalf("ler %s: %v", caminho, err)
	}
	return sha256.Sum256(b)
}

func lerTexto(t *testing.T, caminho string) string {
	t.Helper()
	b, err := os.ReadFile(caminho)
	if err != nil {
		t.Fatalf("ler %s: %v", caminho, err)
	}
	return string(b)
}

func processarAvulso(t *testing.T, e Entrada, cfg *models.Config, atv *Atividade) string {
	t.Helper()
	out, _, _, _, err := NewPipeline().ProcessarEntrada(context.Background(), e, cfg, nil, atv)
	if err != nil {
		t.Fatalf("ProcessarEntrada: %v", err)
	}
	return out
}

func mesmoCaminho(a, b string) bool {
	return strings.EqualFold(filepath.Clean(a), filepath.Clean(b))
}

type pdfNomeado struct {
	nome     string
	conteudo []byte
}

// avulsoEZip processa os PDFs soltos (numa pasta, na ordem dada) e um ZIP com
// os mesmos PDFs na raiz, e devolve os dois TXT.
func avulsoEZip(t *testing.T, pdfs []pdfNomeado, cfg *models.Config) (string, string) {
	t.Helper()
	dir := t.TempDir()
	caminhos := make([]string, len(pdfs))
	entradas := make([]entradaZip, len(pdfs))
	for i, p := range pdfs {
		caminhos[i] = gravarArquivo(t, filepath.Join(dir, p.nome), p.conteudo)
		entradas[i] = entradaZip{p.nome, p.conteudo}
	}
	outAvulso := processarAvulso(t, Entrada{Kind: "pdf", Caminhos: caminhos}, cfg, nil)
	zipPath := gravarZipComo(t, "processo.zip", entradas)
	outZip, _, _, _, err := NewPipeline().ProcessZip(context.Background(), zipPath, cfg, nil)
	if err != nil {
		t.Fatalf("ProcessZip: %v", err)
	}
	return lerTexto(t, outAvulso), lerTexto(t, outZip)
}

func lerFixture(t *testing.T, nome string) []byte {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("..", "..", "scripts", "bench", "fixtures", nome))
	if err != nil {
		t.Fatalf("ler fixture %s: %v", nome, err)
	}
	return b
}

func TestFormatter_RelogioFixo(t *testing.T) {
	fixarRelogio(t)
	mapa := func() map[string]any {
		return map[string]any{"12": map[string]any{"index": 12, "documentos": map[string]any{
			"0:" + nomeAvulso: map[string]any{"index": 1, "nome": nomeAvulso, "paginas": map[string]any{
				"1": map[string]any{"index": 1, "ocr": false, "content": "Sentenca ficticia de FULANO DE TAL"},
			}},
		}}}
	}
	a := GerarSaida(mapa(), "standard", 0).Texto
	b := GerarSaida(mapa(), "standard", 0).Texto
	if a != b {
		t.Error("dois GerarSaida do mesmo mapa deram textos diferentes")
	}
	if !strings.Contains(a, "Data de Consolidação: 26/11/2026 10:00:00\n") {
		t.Errorf("sem a data fixa:\n%s", a)
	}
}

func TestProcessarEntrada_EquivaleAoZip(t *testing.T) {
	casos := []struct {
		nome     string
		conteudo func(t *testing.T) []byte
		ocrMode  string
		real     bool // usa Poppler ou OCR reais
	}{
		{"nativo", func(t *testing.T) []byte { return lerFixture(t, "nativo.pdf") }, "none", false},
		{"vetorial", func(t *testing.T) []byte { return lerFixture(t, "vetorial.pdf") }, "fast", true},
		{"escaneado", func(t *testing.T) []byte { return lerFixture(t, "escaneado.pdf") }, "fast", true},
		{"pdf20", func(*testing.T) []byte { return pdfVersao20(objetosTexto()) }, "none", true},
		{"envelope", func(*testing.T) []byte { return pdfEmEnvelope(buildPDF(objetosTexto())) }, "none", true},
		{"truncado", func(*testing.T) []byte { return pdfTruncado() }, "none", true},
		{"aesv2", func(*testing.T) []byte { return pdfAESV2(objetosTexto(), "") }, "none", true},
		{"senha", func(*testing.T) []byte { return pdfAESV2(objetosTexto(), "segredo") }, "none", true},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			fixarRelogio(t)
			if c.real {
				if testing.Short() {
					t.Skip("usa Poppler ou OCR reais")
				}
				// Não isolarTemp: o cache do pacote ocr guarda a pasta da
				// primeira extração (ver tempClasses).
				tempClasses(t)
				trocarPoppler(t, ocr.InfoPDF, ocr.TextoPDF)
			} else {
				isolarTemp(t)
			}
			avulso, doZip := avulsoEZip(t, []pdfNomeado{{nomeAvulso, c.conteudo(t)}}, cfgAvulso(c.ocrMode))
			if avulso != doZip {
				t.Errorf("TXT do avulso difere do ZIP\n--- avulso:\n%s\n--- zip:\n%s", avulso, doZip)
			}
		})
	}
}

func TestProcessarEntrada_VariosPDFsEquivaleAoZip(t *testing.T) {
	if testing.Short() {
		t.Skip("usa o Poppler real")
	}
	fixarRelogio(t)
	tempClasses(t)
	trocarPoppler(t, ocr.InfoPDF, ocr.TextoPDF)
	pdfs := []pdfNomeado{
		{nomeAvulso, pdfComTexto("Sentenca ficticia de FULANO DE TAL julgando procedente o pedido")},
		{"anexo.pdf", pdfComTexto("Anexo ficticio sem numero de evento para FULANO DE TAL")},
		{cnjAvulso + " - Evento 3 - INIC1.pdf", pdfComTexto("Peticao inicial ficticia de FULANO DE TAL")},
		{cnjAvulso + " - Evento 7 - LAUDO1.pdf", pdfAESV2(objetosTexto(), "segredo")},
	}
	avulso, doZip := avulsoEZip(t, pdfs, cfgAvulso("none"))
	if avulso != doZip {
		t.Errorf("TXT do lote difere do ZIP\n--- avulso:\n%s\n--- zip:\n%s", avulso, doZip)
	}
	if !strings.Contains(avulso, "[documento não lido: protegido por senha; consulte o PDF original]") {
		t.Errorf("sem o marcador de senha:\n%s", avulso)
	}
	if !strings.Contains(avulso, "Documentos não lidos: 1\n") {
		t.Errorf("cabeçalho sem 1 não lido:\n%s", avulso)
	}
}

func TestProcessarEntrada_OmissaoNaoValeParaAvulso(t *testing.T) {
	isolarTemp(t)
	fixarRelogio(t)
	const texto = "PROCURACAO-FICTICIA-DE-FULANO-DE-TAL"
	// Sem o prefixo CNJ: tipoDocumento só tira "Evento N - " do começo do
	// nome, e com o CNJ na frente nem o ZIP omitiria.
	nome := "Evento 1 - PROC1.pdf"
	conteudo := pdfComTexto(texto + " outorga poderes ao advogado ficticio")
	cfg := cfgAvulso("none")
	cfg.OmitirTipos = []string{"PROC"}

	var eventos map[string]any
	antigo := gerarSaida
	t.Cleanup(func() { gerarSaida = antigo })
	gerarSaida = func(ev map[string]any, nivel string, semOCR int) SaidaConsolidada {
		eventos = ev
		return antigo(ev, nivel, semOCR)
	}

	dir := t.TempDir()
	pdfPath := gravarArquivo(t, filepath.Join(dir, nome), conteudo)
	txt := lerTexto(t, processarAvulso(t, Entrada{Kind: "pdf", Caminhos: []string{pdfPath}}, cfg, nil))
	if !strings.Contains(txt, texto) {
		t.Errorf("avulso de tipo omitido sem o texto:\n%s", txt)
	}
	for _, ev := range eventos {
		for _, d := range ev.(map[string]any)["documentos"].(map[string]any) {
			if _, ok := d.(map[string]any)["omitido"]; ok {
				t.Error("avulso com a chave omitido")
			}
		}
	}

	zipPath := gravarZipComo(t, "processo.zip", []entradaZip{{nome, conteudo}})
	out, _, _, _, err := NewPipeline().ProcessZip(context.Background(), zipPath, cfg, nil)
	if err != nil {
		t.Fatalf("ProcessZip: %v", err)
	}
	doZip := lerTexto(t, out)
	if strings.Contains(doZip, texto) || !strings.Contains(doZip, "[documento omitido: PROC") {
		t.Errorf("no ZIP o PROC deveria continuar omitido:\n%s", doZip)
	}
}

func docPorNome(t *testing.T, atv *Atividade, nome string) AtividadeDoc {
	t.Helper()
	for _, d := range atv.Instantaneo().Docs {
		if d.Nome == nome {
			return d
		}
	}
	t.Fatalf("documento %s fora da atividade", nome)
	return AtividadeDoc{}
}

func TestProcessarEntrada_MarcadorDoAvulso(t *testing.T) {
	t.Run("apagado depois de LerEntrada", func(t *testing.T) {
		tmp := isolarTemp(t)
		fixarRelogio(t)
		dir := t.TempDir()
		a := gravarArquivo(t, filepath.Join(dir, cnjAvulso+" - Evento 1 - INIC1.pdf"), pdfComTexto("Peticao inicial ficticia de FULANO DE TAL"))
		nomeB := cnjAvulso + " - Evento 2 - CONT1.pdf"
		b := gravarArquivo(t, filepath.Join(dir, nomeB), pdfComTexto("Contestacao ficticia de FULANO DE TAL"))
		c := gravarArquivo(t, filepath.Join(dir, cnjAvulso+" - Evento 3 - SENT1.pdf"), pdfComTexto("Sentenca ficticia de FULANO DE TAL"))
		t.Cleanup(func() { aoFimDaPreContagem = nil })
		aoFimDaPreContagem = func(*memoriaPDF) {
			if err := os.Remove(b); err != nil {
				t.Errorf("apagar: %v", err)
			}
		}
		atv := NovaAtividade("x")
		txt := lerTexto(t, processarAvulso(t, Entrada{Kind: "pdf", Caminhos: []string{a, b, c}}, cfgAvulso("none"), atv))
		if d := docPorNome(t, atv, nomeB); d.Leitura != string(LeituraIlegivel) {
			t.Errorf("leitura = %q, esperava ilegivel", d.Leitura)
		}
		if !strings.Contains(txt, "[ERRO ao abrir PDF: falha de leitura ou gravação em disco]") {
			t.Errorf("sem o marcador do avulso:\n%s", txt)
		}
		if strings.Contains(strings.ToLower(txt), strings.ToLower(dir)) || strings.Contains(txt, filepath.ToSlash(dir)) {
			t.Errorf("TXT com a pasta do teste:\n%s", txt)
		}
		semTemporarios(t, tmp)
	})
	t.Run("zero bytes num lote", func(t *testing.T) {
		isolarTemp(t)
		fixarRelogio(t)
		dir := t.TempDir()
		a := gravarArquivo(t, filepath.Join(dir, cnjAvulso+" - Evento 1 - INIC1.pdf"), pdfComTexto("Peticao inicial ficticia de FULANO DE TAL"))
		nomeZ := cnjAvulso + " - Evento 2 - VAZIO1.pdf"
		z := gravarArquivo(t, filepath.Join(dir, nomeZ), nil)
		atv := NovaAtividade("x")
		txt := lerTexto(t, processarAvulso(t, Entrada{Kind: "pdf", Caminhos: []string{a, z}}, cfgAvulso("none"), atv))
		if d := docPorNome(t, atv, nomeZ); d.Leitura != string(LeituraIlegivel) {
			t.Errorf("leitura = %q, esperava ilegivel", d.Leitura)
		}
		if !strings.Contains(txt, "[ERRO ao analisar PDF: ") {
			t.Errorf("sem o marcador de análise:\n%s", txt)
		}
	})
	t.Run("ZIP com CRC errado", func(t *testing.T) {
		isolarTemp(t)
		fixarRelogio(t)
		const marca = "CONTEUDO-CORROMPIDO-FICTICIO"
		buf := new(bytes.Buffer)
		zw := zip.NewWriter(buf)
		for _, e := range []entradaZip{
			{"Evento 1 - INIC1.pdf", pdfComTexto("Peticao inicial ficticia de FULANO DE TAL")},
			{"Evento 2 - CONT1.pdf", pdfComTexto(marca + " contestacao")},
		} {
			w, err := zw.CreateHeader(&zip.FileHeader{Name: e.nome, Method: zip.Store})
			if err != nil {
				t.Fatal(err)
			}
			w.Write(e.conteudo)
		}
		if err := zw.Close(); err != nil {
			t.Fatal(err)
		}
		dados := buf.Bytes()
		i := bytes.Index(dados, []byte(marca))
		if i < 0 {
			t.Fatal("marca fora do ZIP")
		}
		dados[i] = 'X'
		zipPath := gravarArquivo(t, filepath.Join(t.TempDir(), "processo.zip"), dados)
		out, _, _, _, err := NewPipeline().ProcessZip(context.Background(), zipPath, cfgAvulso("none"), nil)
		if err != nil {
			t.Fatalf("ProcessZip: %v", err)
		}
		if txt := lerTexto(t, out); !strings.Contains(txt, "[ERRO ao extrair PDF do ZIP: ") {
			t.Errorf("sem o marcador do ZIP:\n%s", txt)
		}
	})
}

// Um PDF que não passou em LerEntrada (Tamanho 0) e cuja leitura falha ou
// traz bytes no worker termina com a frase de disco, nunca com a de arquivo
// trocado nem com um caminho.
func TestExtrairPDF_AvulsoNaoConferido(t *testing.T) {
	isolarTemp(t)
	dir := t.TempDir()
	pasta := filepath.Join(dir, "pasta.pdf")
	if err := os.Mkdir(pasta, 0o755); err != nil {
		t.Fatal(err)
	}
	cheio := gravarArquivo(t, filepath.Join(dir, "reapareceu.pdf"), pdfComTexto("Texto ficticio"))
	mem := &memoriaPDF{bufs: make(chan []byte, 2)}
	mem.disponivel.Store(maxTotalPDFEmMemoria)
	for _, c := range []string{pasta, cheio} {
		f := ParseNome(filepath.Base(c))
		f.Avulso, f.Abrir = true, abrirArquivo(c)
		fonte, err := extrairPDF(f, mem)
		if err == nil {
			fonte.liberar()
			t.Errorf("%s: extrairPDF deveria falhar", filepath.Base(c))
			continue
		}
		if txt := textoErroSaida(err); txt != "falha de leitura ou gravação em disco" {
			t.Errorf("%s: texto = %q", filepath.Base(c), txt)
		}
	}
}

// Uma entrada de ZIP (Avulso=false) que traz um byte além do tamanho
// declarado não é truncada calada: falha com a frase do ZIP, não com a de
// arquivo trocado, e devolve a reserva de memória.
func TestExtrairPDF_EntradaDoZipMaiorQueODeclarado(t *testing.T) {
	dados := pdfComTexto("Texto ficticio de FULANO DE TAL")
	mem := &memoriaPDF{bufs: make(chan []byte, 2)}
	mem.disponivel.Store(maxTotalPDFEmMemoria)
	f := ParseNome("Evento 1 - SENT1.pdf")
	f.Tamanho = int64(len(dados))
	f.Abrir = func() (io.ReadCloser, error) {
		return io.NopCloser(bytes.NewReader(append(slices.Clone(dados), 'x'))), nil
	}
	fonte, err := extrairPDF(f, mem)
	if err == nil {
		fonte.liberar()
		t.Fatal("extrairPDF deveria falhar com a entrada maior que o declarado")
	}
	if got := err.Error(); got != "entrada do ZIP maior que o tamanho declarado" {
		t.Errorf("erro = %q", got)
	}
	if got := mem.disponivel.Load(); got != maxTotalPDFEmMemoria {
		t.Errorf("memória disponível = %d, esperava %d (reserva devolvida)", got, maxTotalPDFEmMemoria)
	}
}

func TestProcessarEntrada_NomeDaSaida(t *testing.T) {
	t.Run("um PDF com o TXT ocupado", func(t *testing.T) {
		isolarTemp(t)
		fixarRelogio(t)
		dir := t.TempDir()
		pdfPath := gravarArquivo(t, filepath.Join(dir, "processo.pdf"), pdfComTexto("Sentenca ficticia de FULANO DE TAL"))
		alheio := gravarArquivo(t, filepath.Join(dir, "processo.txt"), []byte("TXT ALHEIO"))
		antes := sha(t, alheio)
		out := processarAvulso(t, Entrada{Kind: "pdf", Caminhos: []string{pdfPath}}, cfgAvulso("none"), nil)
		if !mesmoCaminho(out, filepath.Join(dir, "processo_2.txt")) {
			t.Errorf("saída = %s, esperava processo_2.txt", out)
		}
		if sha(t, alheio) != antes {
			t.Error("processo.txt alterado")
		}
	})
	t.Run("lote de pastas diferentes", func(t *testing.T) {
		isolarTemp(t)
		fixarRelogio(t)
		dir1, dir2 := t.TempDir(), t.TempDir()
		p1 := gravarArquivo(t, filepath.Join(dir1, cnjAvulso+" - Evento 12 - SENT1.pdf"), pdfComTexto("Sentenca ficticia"))
		p2 := gravarArquivo(t, filepath.Join(dir2, cnjAvulso+" - Evento 3 - INIC1.pdf"), pdfComTexto("Peticao inicial ficticia"))
		out := processarAvulso(t, Entrada{Kind: "pdf", Caminhos: []string{p1, p2}}, cfgAvulso("none"), nil)
		if !mesmoCaminho(out, filepath.Join(dir2, cnjAvulso+".txt")) {
			t.Errorf("saída = %s, esperava %s.txt em dir2", out, cnjAvulso)
		}
	})
}

func TestProcessarEntrada_LoteNaoTomaOTxtDoZip(t *testing.T) {
	isolarTemp(t)
	fixarRelogio(t)
	dir := t.TempDir()
	gravarArquivo(t, filepath.Join(dir, cnjAvulso+".zip"), []byte("ZIP FICTICIO"))
	txtZip := gravarArquivo(t, filepath.Join(dir, cnjAvulso+".txt"), []byte("TXT DO ZIP"))
	antes := sha(t, txtZip)
	var caminhos []string
	for _, n := range []string{"Evento 1 - INIC1", "Evento 2 - CONT1", "Evento 3 - SENT1"} {
		caminhos = append(caminhos, gravarArquivo(t, filepath.Join(dir, cnjAvulso+" - "+n+".pdf"),
			pdfComTexto("Peca ficticia "+n+" de FULANO DE TAL")))
	}
	out := processarAvulso(t, Entrada{Kind: "pdf", Caminhos: caminhos}, cfgAvulso("none"), nil)
	if !mesmoCaminho(out, filepath.Join(dir, cnjAvulso+"_2.txt")) {
		t.Errorf("saída = %s, esperava %s_2.txt", out, cnjAvulso)
	}
	if sha(t, txtZip) != antes {
		t.Error("o TXT do ZIP foi alterado")
	}
}

func TestProcessarEntrada_SaidaAnterior(t *testing.T) {
	// pastaComPDF monta processo.pdf e um processo.txt alheio.
	pastaComPDF := func(t *testing.T) (dir, pdfPath string, alheio [32]byte) {
		isolarTemp(t)
		fixarRelogio(t)
		dir = t.TempDir()
		pdfPath = gravarArquivo(t, filepath.Join(dir, "processo.pdf"), pdfComTexto("Sentenca ficticia de FULANO DE TAL"))
		return dir, pdfPath, sha(t, gravarArquivo(t, filepath.Join(dir, "processo.txt"), []byte("TXT ALHEIO")))
	}
	t.Run("regrava a anterior", func(t *testing.T) {
		dir, pdfPath, alheio := pastaComPDF(t)
		anterior := gravarArquivo(t, filepath.Join(dir, "processo_2.txt"), []byte("ANTIGO"))
		out := processarAvulso(t, Entrada{Kind: "pdf", Caminhos: []string{pdfPath}, SaidaAnterior: anterior}, cfgAvulso("none"), nil)
		if !mesmoCaminho(out, anterior) {
			t.Errorf("saída = %s, esperava %s", out, anterior)
		}
		if txt := lerTexto(t, anterior); !strings.Contains(txt, "Sentenca ficticia") {
			t.Errorf("processo_2.txt não regravado: %q", txt)
		}
		if _, err := os.Stat(filepath.Join(dir, "processo_3.txt")); err == nil {
			t.Error("processo_3.txt criado")
		}
		if sha(t, filepath.Join(dir, "processo.txt")) != alheio {
			t.Error("processo.txt alterado")
		}
	})
	t.Run("anterior apagada", func(t *testing.T) {
		dir, pdfPath, _ := pastaComPDF(t)
		anterior := filepath.Join(dir, "processo_2.txt")
		out := processarAvulso(t, Entrada{Kind: "pdf", Caminhos: []string{pdfPath}, SaidaAnterior: anterior}, cfgAvulso("none"), nil)
		if !mesmoCaminho(out, anterior) {
			t.Errorf("saída = %s, esperava processo_2.txt", out)
		}
	})
	for _, c := range []struct {
		nome     string
		anterior func(t *testing.T, dir string) string
	}{
		{"outra pasta", func(t *testing.T, _ string) string {
			return gravarArquivo(t, filepath.Join(t.TempDir(), "processo_2.txt"), []byte("OUTRA PASTA"))
		}},
		{"outro nome", func(t *testing.T, dir string) string {
			return gravarArquivo(t, filepath.Join(dir, "outro.txt"), []byte("OUTRO NOME"))
		}},
		{"unidade sintética", func(*testing.T, string) string { return `D:\fulano\processo_2.txt` }},
	} {
		t.Run(c.nome, func(t *testing.T) {
			dir, pdfPath, _ := pastaComPDF(t)
			anterior := c.anterior(t, dir)
			var antes [32]byte
			existe := false
			if _, err := os.Stat(anterior); err == nil {
				existe, antes = true, sha(t, anterior)
			}
			out := processarAvulso(t, Entrada{Kind: "pdf", Caminhos: []string{pdfPath}, SaidaAnterior: anterior}, cfgAvulso("none"), nil)
			if !mesmoCaminho(out, filepath.Join(dir, "processo_2.txt")) {
				t.Errorf("saída = %s, esperava nome livre processo_2.txt", out)
			}
			if existe && sha(t, anterior) != antes {
				t.Error("a saída anterior alheia foi alterada")
			}
		})
	}
}

// Guarda de regressão: o ZIP continua gravando por cima do próprio TXT.
func TestProcessarEntrada_ZipSobrescreveOProprioTxt(t *testing.T) {
	isolarTemp(t)
	fixarRelogio(t)
	zipPath := gravarZipComo(t, "processo.zip", []entradaZip{{nomeAvulso, pdfComTexto("Sentenca ficticia de FULANO DE TAL")}})
	dir := filepath.Dir(zipPath)
	antigo := gravarArquivo(t, filepath.Join(dir, "processo.txt"), []byte("ANTIGO"))
	out, _, _, _, err := NewPipeline().ProcessZip(context.Background(), zipPath, cfgAvulso("none"), nil)
	if err != nil {
		t.Fatalf("ProcessZip: %v", err)
	}
	if !mesmoCaminho(out, antigo) {
		t.Errorf("saída = %s, esperava %s", out, antigo)
	}
	if txt := lerTexto(t, antigo); !strings.Contains(txt, "Sentenca ficticia") {
		t.Errorf("processo.txt não regravado: %q", txt)
	}
	if _, err := os.Stat(filepath.Join(dir, "processo_2.txt")); err == nil {
		t.Error("processo_2.txt criado para ZIP")
	}
}

func TestProcessarEntrada_KindIncompativel(t *testing.T) {
	isolarTemp(t)
	dir := t.TempDir()
	pdfPath := gravarArquivo(t, filepath.Join(dir, "x.pdf"), pdfComTexto("Texto ficticio"))
	zipDir := filepath.Dir(gravarZipComo(t, "y.zip", []entradaZip{{"Evento 1 - INIC1.pdf", pdfComTexto("Texto ficticio")}}))
	zipPath := filepath.Join(zipDir, "y.zip")
	for _, c := range []struct {
		e   Entrada
		dir string
	}{
		{Entrada{Kind: "zip", Caminhos: []string{pdfPath}}, dir},
		{Entrada{Kind: "pdf", Caminhos: []string{zipPath}}, zipDir},
	} {
		_, _, _, _, err := NewPipeline().ProcessarEntrada(context.Background(), c.e, cfgAvulso("none"), nil, nil)
		if !errors.Is(err, ErrTipoNaoSuportado) {
			t.Errorf("Kind %s: err = %v, esperava ErrTipoNaoSuportado", c.e.Kind, err)
		}
		if ee, ok := errors.AsType[ErroEtapa](err); !ok || ee.Etapa != EtapaAbrirZip {
			t.Errorf("Kind %s: etapa errada: %v", c.e.Kind, err)
		}
		if txts, _ := filepath.Glob(filepath.Join(c.dir, "*.txt")); len(txts) != 0 {
			t.Errorf("Kind %s: TXT gravado: %v", c.e.Kind, txts)
		}
	}
}
