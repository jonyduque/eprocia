package historico

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"eprocia/internal/models"
)

// pasta é a pasta fictícia dos itens de teste (não precisa existir).
const pasta = `C:\Users\fulano\Downloads`

const cnj = "5000000-00.2026.8.26.0001"

func em(nome string) string { return pasta + `\` + nome }

// js devolve s como literal JSON (com as barras escapadas).
func js(s string) string {
	b, _ := json.Marshal(s)
	return string(b)
}

// tresItens são os três itens de TestCarregar_TresItens, como Carregar deve
// devolvê-los.
func tresItens() []Item {
	return []Item{
		{
			Name: cnj + ".zip", Path: em(cnj + ".zip"), Status: "success",
			Message: "Concluído", OutputPath: em(cnj + ".txt"),
			DocsCount: 12, PagesCount: 80, EstimatedTokens: 40000, UnreadDocs: 1,
			Timestamp: 1795000000000, FinishedAt: 1795000090000,
			OcrMode: "normal", CleaningLevel: "standard", Kind: "zip",
		},
		{
			Name: "peticao.pdf", Path: em("peticao.pdf"), Status: "error",
			Message: "Falhou", OutputPath: em("peticao_2.txt"),
			DocsCount: 1, PagesCount: 3, Error: "falha de leitura",
			Timestamp: 1795000100000, FinishedAt: 1795000110000,
			OcrMode: "fast", CleaningLevel: "basic", Kind: "pdf",
		},
		{
			Name: cnj + " (3 PDFs)", Path: em(cnj + " Evento 1.pdf"), Status: "success",
			OutputPath: em(cnj + ".txt"), DocsCount: 3, PagesCount: 9,
			Timestamp: 1795000200000, FinishedAt: 1795000290000,
			OcrMode: "best", CleaningLevel: "aggressive", Kind: "pdf",
			Paths: []string{em(cnj + " Evento 1.pdf"), em(cnj + " Evento 2.pdf"), em(cnj + " Evento 3.pdf")},
		},
	}
}

// jsonTresItens é o JSON escrito à mão com os três itens.
func jsonTresItens() string {
	return `{
  "versao": 1,
  "itens": [
    {"name": ` + js(cnj+".zip") + `, "path": ` + js(em(cnj+".zip")) + `, "status": "success",
     "message": "Concluído", "output_path": ` + js(em(cnj+".txt")) + `,
     "docs_count": 12, "pages_count": 80, "estimated_tokens": 40000, "unread_docs": 1,
     "error": "", "timestamp": 1795000000000, "finished_at": 1795000090000,
     "ocr_mode": "normal", "cleaning_level": "standard", "kind": "zip"},
    {"name": "peticao.pdf", "path": ` + js(em("peticao.pdf")) + `, "status": "error",
     "message": "Falhou", "output_path": ` + js(em("peticao_2.txt")) + `,
     "docs_count": 1, "pages_count": 3, "estimated_tokens": 0, "unread_docs": 0,
     "error": "falha de leitura", "timestamp": 1795000100000, "finished_at": 1795000110000,
     "ocr_mode": "fast", "cleaning_level": "basic", "kind": "pdf"},
    {"name": ` + js(cnj+" (3 PDFs)") + `, "path": ` + js(em(cnj+" Evento 1.pdf")) + `, "status": "success",
     "message": "", "output_path": ` + js(em(cnj+".txt")) + `,
     "docs_count": 3, "pages_count": 9, "estimated_tokens": 0, "unread_docs": 0,
     "error": "", "timestamp": 1795000200000, "finished_at": 1795000290000,
     "ocr_mode": "best", "cleaning_level": "aggressive", "kind": "pdf",
     "paths": [` + js(em(cnj+" Evento 1.pdf")) + `, ` + js(em(cnj+" Evento 2.pdf")) + `, ` + js(em(cnj+" Evento 3.pdf")) + `]}
  ]
}`
}

func escreverArquivo(t *testing.T, caminho string, dados []byte) {
	t.Helper()
	if err := os.WriteFile(caminho, dados, 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestCarregar_TresItens(t *testing.T) {
	arq := filepath.Join(t.TempDir(), "historico.json")
	escreverArquivo(t, arq, []byte(jsonTresItens()))
	got, err := Carregar(arq)
	if err != nil {
		t.Fatalf("erro: %v", err)
	}
	if want := tresItens(); !reflect.DeepEqual(got, want) {
		t.Fatalf("itens:\n got %+v\nwant %+v", got, want)
	}
}

func TestCarregar_Ausente(t *testing.T) {
	got, err := Carregar(filepath.Join(t.TempDir(), "historico.json"))
	if err != nil || len(got) != 0 {
		t.Fatalf("got %v, %v; want vazio, nil", got, err)
	}
}

func TestCarregar_Corrompido(t *testing.T) {
	casos := map[string]string{
		"truncado":      `{"versao":1,"itens":[`,
		"vazio":         ``,
		"sem versao":    `{"itens":[]}`,
		"versao zero":   `{"versao":0,"itens":[]}`,
		"so espacos":    "  \r\n",
		"lista de nada": `[]`,
	}
	for nome, conteudo := range casos {
		t.Run(nome, func(t *testing.T) {
			dir := t.TempDir()
			arq := filepath.Join(dir, "historico.json")
			escreverArquivo(t, arq, []byte(conteudo))
			got, err := Carregar(arq)
			if err != nil || len(got) != 0 {
				t.Fatalf("got %v, %v; want vazio, nil", got, err)
			}
			if _, err := os.Stat(arq); !errors.Is(err, os.ErrNotExist) {
				t.Fatalf("historico.json ainda existe: %v", err)
			}
			achados, _ := filepath.Glob(filepath.Join(dir, "historico.json.corrompido-????????-??????"))
			if len(achados) != 1 {
				t.Fatalf("arquivos .corrompido-*: %v; want 1", achados)
			}
			todos, _ := os.ReadDir(dir)
			if len(todos) != 1 {
				t.Fatalf("arquivos na pasta: %d; want 1", len(todos))
			}
		})
	}
}

func TestCarregar_ComBOM(t *testing.T) {
	arq := filepath.Join(t.TempDir(), "historico.json")
	j := `{"versao":1,"itens":[{"name":"a.zip","path":` + js(em("a.zip")) + `,"status":"success","kind":"zip"}]}`
	escreverArquivo(t, arq, append([]byte{0xEF, 0xBB, 0xBF}, j...))
	got, err := Carregar(arq)
	if err != nil || len(got) != 1 || got[0].Path != em("a.zip") {
		t.Fatalf("got %+v, %v; want o item", got, err)
	}
}

func TestCarregar_VersaoNova(t *testing.T) {
	arq := filepath.Join(t.TempDir(), "historico.json")
	dados := []byte(`{"versao":2,"itens":[{"name":"a.zip","path":` + js(em("a.zip")) + `,"status":"success","kind":"zip","novo":true}]}`)
	escreverArquivo(t, arq, dados)
	got, err := Carregar(arq)
	if len(got) != 0 || !errors.Is(err, ErrVersaoNova) {
		t.Fatalf("got %v, %v; want vazio, ErrVersaoNova", got, err)
	}
	depois, _ := os.ReadFile(arq)
	if !bytes.Equal(depois, dados) {
		t.Fatalf("o arquivo mudou")
	}
}

// itensNumerados gera n itens ZIP válidos, o mais novo primeiro.
func itensNumerados(n int) []Item {
	itens := make([]Item, n)
	for i := range itens {
		nome := fmt.Sprintf("p%03d.zip", i)
		itens[i] = Item{Name: nome, Path: em(nome), Status: "success", Kind: "zip", Timestamp: int64(n - i)}
	}
	return itens
}

func TestCarregar_Teto(t *testing.T) {
	var b strings.Builder
	b.WriteString(`{"versao":1,"itens":[`)
	for i := range 501 {
		if i > 0 {
			b.WriteString(",")
		}
		nome := fmt.Sprintf("p%03d.zip", i)
		b.WriteString(`{"name":"` + nome + `","path":` + js(em(nome)) + `,"status":"success","kind":"zip"}`)
	}
	b.WriteString(`]}`)
	arq := filepath.Join(t.TempDir(), "historico.json")
	escreverArquivo(t, arq, []byte(b.String()))
	got, err := Carregar(arq)
	if err != nil || len(got) != 500 {
		t.Fatalf("len %d, %v; want 500, nil", len(got), err)
	}
	if got[0].Name != "p000.zip" || got[499].Name != "p499.zip" {
		t.Fatalf("primeiro %q, último %q", got[0].Name, got[499].Name)
	}
}

func TestPodar(t *testing.T) {
	itens := itensNumerados(3)
	if got := Podar(itens, 2); !reflect.DeepEqual(got, itens[:2]) {
		t.Fatalf("teto 2: %+v", got)
	}
	if got := Podar(itens, 10); !reflect.DeepEqual(got, itens) {
		t.Fatalf("teto 10: %+v", got)
	}
}

func TestCarregar_Validacao(t *testing.T) {
	p1, p2, p3 := em(cnj+" Evento 1.pdf"), em(cnj+" Evento 2.pdf"), em(cnj+" Evento 3.pdf")
	itens := []string{
		// descartados
		`{"name":"processo.zip","path":"processo.zip","status":"success","kind":"zip"}`,
		`{"name":"x.exe","path":` + js(em("x.exe")) + `,"status":"success"}`,
		`{"name":"t.zip","path":` + js(em("t.zip")) + `,"status":"turbo","kind":"zip"}`,
		`{"name":"o.zip","path":` + js(em("o.zip")) + `,"status":"success","kind":"zip","output_path":` + js(`C:\Users\fulano\Documents\o.txt`) + `}`,
		`{"name":"lote","path":` + js(p1) + `,"status":"success","kind":"pdf","output_path":` + js(em(cnj+" Evento 1.txt")) + `,"paths":[` + js(p1) + `,` + js(p2) + `,` + js(p3) + `]}`,
		`{"name":"lote2","path":` + js(p1) + `,"status":"success","kind":"pdf","paths":[` + js(p1) + `,"Evento 2.pdf",` + js(p3) + `]}`,
		// extra (fora dos oito do brief): kind desconhecido
		`{"name":"k.zip","path":` + js(em("k.zip")) + `,"status":"success","kind":"rar"}`,
		// mantidos
		`{"name":"z.zip","path":` + js(em("z.zip")) + `,"status":"success","output_path":` + js(em("z.txt")) + `}`,
		`{"name":"d.PDF","path":` + js(em("d.PDF")) + `,"status":"interrupted","output_path":""}`,
	}
	arq := filepath.Join(t.TempDir(), "historico.json")
	escreverArquivo(t, arq, []byte(`{"versao":1,"itens":[`+strings.Join(itens, ",")+`]}`))
	got, err := Carregar(arq)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 {
		t.Fatalf("ficaram %d itens: %+v; want 2", len(got), got)
	}
	if got[0].Name != "z.zip" || got[0].Kind != "zip" {
		t.Fatalf("item 0: %+v", got[0])
	}
	if got[1].Name != "d.PDF" || got[1].Kind != "pdf" {
		t.Fatalf("item 1: %+v", got[1])
	}
}

func TestCarregar_KindPreenchidoNaoTroca(t *testing.T) {
	// Um kind preenchido nunca é trocado pelo derivado do Path.
	arq := filepath.Join(t.TempDir(), "historico.json")
	escreverArquivo(t, arq, []byte(`{"versao":1,"itens":[{"name":"a","path":`+js(em("a.zip"))+`,"status":"error","kind":"pdf"}]}`))
	got, err := Carregar(arq)
	if err != nil || len(got) != 1 || got[0].Kind != "pdf" {
		t.Fatalf("got %+v, %v; want kind pdf", got, err)
	}
}

func TestCarregar_SaidaAnterior(t *testing.T) {
	// saida_anterior passa pela mesma regra do output_path (SaidaDoItem), mas
	// um valor inválido só some: o item fica.
	pdf := em("peticao.pdf")
	itens := []string{
		`{"name":"a","path":` + js(pdf) + `,"status":"interrupted","kind":"pdf","saida_anterior":` + js(em("peticao_2.txt")) + `}`,
		`{"name":"b","path":` + js(pdf) + `,"status":"interrupted","kind":"pdf","saida_anterior":` + js(`C:\Users\fulano\Documents\outro.txt`) + `}`,
		`{"name":"c","path":` + js(pdf) + `,"status":"interrupted","kind":"pdf","saida_anterior":"relativo.txt"}`,
	}
	arq := filepath.Join(t.TempDir(), "historico.json")
	escreverArquivo(t, arq, []byte(`{"versao":1,"itens":[`+strings.Join(itens, ",")+`]}`))
	got, err := Carregar(arq)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 3 {
		t.Fatalf("ficaram %d itens: %+v; want 3", len(got), got)
	}
	if got[0].SaidaAnterior != em("peticao_2.txt") {
		t.Errorf("válida: SaidaAnterior = %q", got[0].SaidaAnterior)
	}
	for _, it := range got[1:] {
		if it.SaidaAnterior != "" {
			t.Errorf("%s: SaidaAnterior inválida mantida: %q", it.Name, it.SaidaAnterior)
		}
	}
}

func TestItem_SaidaAnteriorOmitidaVazia(t *testing.T) {
	b, err := json.Marshal(Item{Name: "a", Path: em("a.zip"), Status: "success", Kind: "zip"})
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(b, []byte("saida_anterior")) {
		t.Errorf("saida_anterior vazia gravada: %s", b)
	}
	b, _ = json.Marshal(Item{SaidaAnterior: em("a.txt")})
	if !bytes.Contains(b, []byte(`"saida_anterior":`)) {
		t.Errorf("saida_anterior não gravada: %s", b)
	}
}

// relatorioCompleto é um models.RelatorioZip com todos os campos de contagem
// preenchidos e sem Atencao, como o paraHistorico o grava.
func relatorioCompleto() *models.RelatorioZip {
	return &models.RelatorioZip{
		Documentos: models.DocsRelatorio{
			Total: 42, Lidos: 38, NaoLidos: 2, Omitidos: 2, DuplicadosH7: 1, PDF: 40, HTML: 2,
			TiposOmitidos: []string{"PROCURACAO"},
			PorLeitura:    map[string]int{"sem_ocr": 1, "senha": 1},
		},
		Paginas: models.PaginasRelatorio{
			Total: 310, LidaNativa: 250, LidaOCR: 50, Vazia: 4, SemOCR: 3, ComErro: 3,
			OCRSemGanho: 2, OCRFalhouComReserva: 1, DeNaoLidos: 12, DeOmitidos: 6, Duplicadas: 5,
			PorMotivo: map[string]int{"padrao": 40, "imagem_descoberta": 8, "isca": 2},
		},
		Tempo: models.TempoRelatorio{
			TotalMs: 95000, AbrirMs: 120, PreContagemMs: 800, LeituraParedeMs: 80000,
			LeituraSomaMs: 300000, OCRTrabalhoMs: 240000, OCRParedeMs: 70000,
			FormatacaoMs: 900, GravacaoMs: 40,
		},
		OCR: models.OCRRelatorio{
			Processos: 12, Blocos: 4, PaginasEmBloco: 40, Refacoes: 1, Prazos: 1,
			BlocoMedioMs: 18000, BlocoMedianaMs: 17000, BlocoP95Ms: 30000, PaginaEstimadaMs: 4800,
		},
		Limpeza: models.LimpezaRelatorio{
			Nivel: "standard", BytesAntes: 900000, BytesDepois: 700000, BytesDuplicatas: 20000,
			TokensAntes: 225000, TokensDepois: 175000,
		},
		BytesTxt: 700000,
		Tokens:   175000,
	}
}

func TestGravador_RelatorioIdaEVolta(t *testing.T) {
	arq := filepath.Join(t.TempDir(), "historico.json")
	itens := tresItens()
	itens[0].Relatorio = relatorioCompleto()
	g := novoGravadorTeste(t, arq, Opcoes{})
	if err := g.GravarAgora(itens); err != nil {
		t.Fatal(err)
	}
	got, err := Carregar(arq)
	if err != nil {
		t.Fatal(err)
	}
	want := tresItens()
	want[0].Relatorio = relatorioCompleto()
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("ida e volta:\n got %+v\nwant %+v", got[0].Relatorio, want[0].Relatorio)
	}
	dados, _ := os.ReadFile(arq)
	if n := strings.Count(string(dados), `"relatorio":`); n != 1 {
		t.Errorf(`"relatorio": aparece %d vezes; want 1 (omitempty nos itens sem relatório)`, n)
	}
}

func TestHistorico_TamanhoPorItem(t *testing.T) {
	itens := make([]Item, Teto)
	for i := range itens {
		nome := fmt.Sprintf("%s-%03d.zip", cnj, i)
		itens[i] = Item{
			Name: nome, Path: em(nome), Status: "success", Message: "Concluído",
			OutputPath: em(strings.TrimSuffix(nome, ".zip") + ".txt"),
			DocsCount:  42, PagesCount: 310, EstimatedTokens: 175000, UnreadDocs: 2,
			Timestamp: 1795000000000, FinishedAt: 1795000095000,
			OcrMode: "normal", CleaningLevel: "standard", Kind: "zip",
			Relatorio: relatorioCompleto(),
		}
	}
	arq := filepath.Join(t.TempDir(), "historico.json")
	g := novoGravadorTeste(t, arq, Opcoes{})
	if err := g.GravarAgora(itens); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(arq)
	if err != nil {
		t.Fatal(err)
	}
	const limite = 1_500_000
	if info.Size() > limite {
		t.Fatalf("%d itens com relatório: %d bytes; máximo %d", Teto, info.Size(), limite)
	}
	if !bytes.Contains(lerArquivo(t, arq), []byte(`"relatorio":`)) {
		t.Fatal("o relatório não foi gravado")
	}
	t.Logf("%d itens com relatório: %d bytes (%d por item)", Teto, info.Size(), info.Size()/Teto)
}

func lerArquivo(t *testing.T, arq string) []byte {
	t.Helper()
	dados, err := os.ReadFile(arq)
	if err != nil {
		t.Fatal(err)
	}
	return dados
}
