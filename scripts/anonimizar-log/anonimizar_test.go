package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// rodar anonimiza entrada e devolve as linhas de saída e as sobras.
func rodar(t *testing.T, entrada string) ([]string, []int) {
	t.Helper()
	var saida bytes.Buffer
	sobras, err := anonimizar(strings.NewReader(entrada), &saida)
	if err != nil {
		t.Fatalf("anonimizar: %v", err)
	}
	return strings.Split(strings.TrimSuffix(saida.String(), "\n"), "\n"), sobras
}

// campo lê o valor de texto de chave numa linha JSON.
func campo(t *testing.T, linha, chave string) string {
	t.Helper()
	var m map[string]any
	if err := json.Unmarshal([]byte(linha), &m); err != nil {
		t.Fatalf("linha de saída não é JSON: %v\n%s", err, linha)
	}
	v, _ := m[chave].(string)
	return v
}

func TestAnonimizar_RotulosEstaveis(t *testing.T) {
	dados, err := os.ReadFile("testdata/entrada.jsonl")
	if err != nil {
		t.Fatal(err)
	}
	linhas, sobras := rodar(t, string(dados))
	if len(linhas) != 7 {
		t.Fatalf("esperava 7 linhas, veio %d", len(linhas))
	}
	if len(sobras) != 0 {
		t.Errorf("sobras = %v, esperava nenhuma", sobras)
	}
	saida := strings.Join(linhas, "\n")
	for _, nome := range []string{"5000000-00.2026.4.00.0001", "5000000-00.2026.4.00.0002", "FULANO DE TAL", "PROCURAÇÃO"} {
		if strings.Contains(saida, nome) {
			t.Errorf("a saída ainda contém %q", nome)
		}
	}
	for _, i := range []int{0, 1, 4} {
		if got := campo(t, linhas[i], "zip"); got != "zip-1" {
			t.Errorf("linha %d: zip = %q, esperava zip-1", i+1, got)
		}
	}
	if got := campo(t, linhas[5], "zip"); got != "zip-2" {
		t.Errorf("linha 6: zip = %q, esperava zip-2", got)
	}
	if got := campo(t, linhas[2], "doc"); got != "doc-1" {
		t.Errorf("linha 3: doc = %q, esperava doc-1", got)
	}
	if got := campo(t, linhas[3], "doc"); got != "doc-2" {
		t.Errorf("linha 4: doc = %q, esperava doc-2", got)
	}
	if got := campo(t, linhas[6], "doc"); got != "doc-1" {
		t.Errorf("linha 7: doc = %q, esperava doc-1", got)
	}
	if !strings.Contains(linhas[4], `"atencao":[{"nome":"doc-2","evento":2,"sem_ocr":[1]}]`) {
		t.Errorf("nome em relatorio.atencao não virou doc-2: %s", linhas[4])
	}
	// O resto da linha fica como estava, na mesma ordem.
	esperada := `{"ts":"2026-11-24T10:00:00-03:00","nivel":"INFO","msg":"ZIP adicionado à fila","item_id":1,"zip":"zip-1","origem":"pasta"}`
	if linhas[0] != esperada {
		t.Errorf("linha 1:\n got %s\nwant %s", linhas[0], esperada)
	}
}

func TestAnonimizar_NomeDentroDoErro(t *testing.T) {
	entrada := `{"msg":"zip iniciado","zip":"5000000-00.2026.4.00.0003.zip"}` + "\n" +
		`{"msg":"falha ao abrir 5000000-00.2026.4.00.0003.zip","doc":"Evento 3 - SENTENÇA.pdf","erro":"abrir C:\\Usuarios\\exemplo\\5000000-00.2026.4.00.0003.zip: Evento 3 - SENTENÇA.pdf corrompido"}` + "\n"
	linhas, _ := rodar(t, entrada)
	if got := campo(t, linhas[1], "erro"); got != `abrir C:\Usuarios\exemplo\zip-1: doc-1 corrompido` {
		t.Errorf("erro = %q", got)
	}
	if got := campo(t, linhas[1], "msg"); got != "falha ao abrir zip-1" {
		t.Errorf("msg = %q", got)
	}
}

func TestAnonimizar_SobraDeCaminho(t *testing.T) {
	entrada := `{"msg":"a","erro":"abrir C:\\Users\\fulano\\x"}` + "\n" +
		`{"msg":"b","erro":"abrir C:\\Users\\<usuario>\\x"}` + "\n" +
		`{"msg":"c","caminho":"/Users/fulano/x"}` + "\n" +
		`{"msg":"d","caminho":"%USERPROFILE%\\x"}` + "\n"
	_, sobras := rodar(t, entrada)
	if !slices.Equal(sobras, []int{1, 3}) {
		t.Errorf("sobras = %v, esperava [1 3]", sobras)
	}
}

func TestAnonimizar_LinhaInvalidaSegue(t *testing.T) {
	entrada := `{"msg":"ok","zip":"5000000-00.2026.4.00.0004.zip"}` + "\n" +
		`panic: C:\Users\fulano\x em 5000000-00.2026.4.00.0004.zip` + "\n" +
		`texto solto sem caminho` + "\n"
	linhas, sobras := rodar(t, entrada)
	if len(linhas) != 3 {
		t.Fatalf("esperava 3 linhas, veio %d", len(linhas))
	}
	if linhas[1] != `panic: C:\Users\fulano\x em zip-1` {
		t.Errorf("linha 2 = %q", linhas[1])
	}
	if linhas[2] != "texto solto sem caminho" {
		t.Errorf("linha 3 = %q", linhas[2])
	}
	if !slices.Equal(sobras, []int{2}) {
		t.Errorf("sobras = %v, esperava [2]", sobras)
	}
}

func TestAnonimizar_RadicalDoZip(t *testing.T) {
	entrada := `{"msg":"item na vez","zip":"5000000-00.2026.4.00.0006.zip"}` + "\n" +
		`{"msg":"falha","etapa":"gravar","erro":"open C:\\Usuarios\\exemplo\\5000000-00.2026.4.00.0006.txt: acesso negado","pasta":"C:\\Usuarios\\exemplo\\5000000-00.2026.4.00.0006"}` + "\n"
	linhas, _ := rodar(t, entrada)
	if got := campo(t, linhas[1], "erro"); got != `open C:\Usuarios\exemplo\zip-1.txt: acesso negado` {
		t.Errorf("erro = %q", got)
	}
	if got := campo(t, linhas[1], "pasta"); got != `C:\Usuarios\exemplo\zip-1` {
		t.Errorf("pasta = %q", got)
	}
}

func TestAnonimizar_TxtDePDF(t *testing.T) {
	entrada := `{"msg":"item na vez","zip":"5000000-00.2026.8.26.0001 - Evento 12 - SENT1.pdf"}` + "\n" +
		`{"msg":"falha","etapa":"gravar","erro":"open C:\\Users\\<usuario>\\Downloads\\5000000-00.2026.8.26.0001 - Evento 12 - SENT1.txt: acesso negado","pasta":"C:\\Users\\<usuario>\\5000000-00.2026.8.26.0001 - Evento 12 - SENT1"}` + "\n"
	linhas, sobras := rodar(t, entrada)
	todasValidas(t, linhas)
	if len(sobras) != 0 {
		t.Errorf("sobras = %v", sobras)
	}
	if got := campo(t, linhas[0], "zip"); got != "zip-1" {
		t.Errorf("zip = %q", got)
	}
	if got := campo(t, linhas[1], "erro"); got != `open C:\Users\<usuario>\Downloads\zip-1.txt: acesso negado` {
		t.Errorf("erro = %q", got)
	}
	if got := campo(t, linhas[1], "pasta"); got != `C:\Users\<usuario>\zip-1` {
		t.Errorf("pasta = %q", got)
	}
}

func TestAnonimizar_PDFSemDiferencaDeCaixa(t *testing.T) {
	entrada := `{"msg":"x","zip":"processo-exemplo.PDF"}` + "\n" +
		`{"msg":"y","erro":"open C:\\Users\\<usuario>\\processo-exemplo.txt: acesso negado"}` + "\n"
	linhas, _ := rodar(t, entrada)
	if got := campo(t, linhas[1], "erro"); got != `open C:\Users\<usuario>\zip-1.txt: acesso negado` {
		t.Errorf("erro = %q", got)
	}
}

func TestAnonimizar_NomeDeItemComContagem(t *testing.T) {
	entrada := `{"msg":"item na vez","zip":"5000000-00.2026.8.26.0001 (3 PDFs)"}` + "\n" +
		`{"msg":"concluído 5000000-00.2026.8.26.0001 (3 PDFs)","saida":"C:\\Users\\<usuario>\\5000000-00.2026.8.26.0001.txt"}` + "\n" +
		`{"msg":"falha","pasta":"C:\\Users\\<usuario>\\5000000-00.2026.8.26.0001"}` + "\n"
	linhas, _ := rodar(t, entrada)
	todasValidas(t, linhas)
	saida := strings.Join(linhas, "\n")
	if strings.Contains(saida, "5000000-00.2026.8.26.0001") || strings.Contains(saida, "PDFs") {
		t.Errorf("sobrou o nome do item: %s", saida)
	}
	if got := campo(t, linhas[0], "zip"); got != "zip-1" {
		t.Errorf("zip = %q", got)
	}
	if got := campo(t, linhas[1], "msg"); got != "concluído zip-1" {
		t.Errorf("msg = %q", got)
	}
	if got := campo(t, linhas[1], "saida"); got != `C:\Users\<usuario>\zip-1.txt` {
		t.Errorf("saida = %q", got)
	}
	if got := campo(t, linhas[2], "pasta"); got != `C:\Users\<usuario>\zip-1` {
		t.Errorf("pasta = %q", got)
	}
}

func TestAnonimizar_TxtComSufixoN(t *testing.T) {
	for _, nome := range []string{"processo-exemplo.zip", "processo-exemplo.pdf"} {
		entrada := `{"msg":"x","zip":"` + nome + `"}` + "\n" +
			`{"msg":"y","a":"C:\\Users\\<usuario>\\processo-exemplo_2.txt","b":"C:\\Users\\<usuario>\\processo-exemplo_999.txt"}` + "\n"
		linhas, _ := rodar(t, entrada)
		if got := campo(t, linhas[1], "a"); got != `C:\Users\<usuario>\zip-1_2.txt` {
			t.Errorf("%s: a = %q", nome, got)
		}
		if got := campo(t, linhas[1], "b"); got != `C:\Users\<usuario>\zip-1_999.txt` {
			t.Errorf("%s: b = %q", nome, got)
		}
	}
	// Os limites do sufixo valem para o radical curto, que só é trocado como
	// nome de TXT (o radical de 4 ou mais caracteres é trocado também
	// sozinho, onde aparece delimitado, e por isso nunca sobra).
	entrada := `{"msg":"x","zip":"abc.pdf"}` + "\n" +
		`{"msg":"y","a":"abc_2.txt","b":"abc_999.txt","c":"abc_1.txt","d":"abc_1000.txt","e":"abc_02.txt"}` + "\n"
	linhas, _ := rodar(t, entrada)
	want := `{"msg":"y","a":"zip-1_2.txt","b":"zip-1_999.txt","c":"abc_1.txt","d":"abc_1000.txt","e":"abc_02.txt"}`
	if linhas[1] != want {
		t.Errorf("linha 2:\n got %s\nwant %s", linhas[1], want)
	}
}

// todasValidas falha se alguma linha não for JSON válido.
func todasValidas(t *testing.T, linhas []string) {
	t.Helper()
	for i, l := range linhas {
		if !json.Valid([]byte(l)) {
			t.Errorf("linha %d não é JSON válido: %s", i+1, l)
		}
	}
}

func TestAnonimizar_RadicalCurtoNaoTrocaNumeros(t *testing.T) {
	entrada := `{"msg":"item na vez","item_id":1,"zip":"1.zip","bytes":1500,"paginas":[1,21]}` + "\n" +
		`{"msg":"falha","item_id":1,"erro":"open C:\\Usuarios\\exemplo\\1.txt: acesso negado"}` + "\n"
	linhas, _ := rodar(t, entrada)
	todasValidas(t, linhas)
	want0 := `{"msg":"item na vez","item_id":1,"zip":"zip-1","bytes":1500,"paginas":[1,21]}`
	if linhas[0] != want0 {
		t.Errorf("linha 1:\n got %s\nwant %s", linhas[0], want0)
	}
	if got := campo(t, linhas[1], "erro"); got != `open C:\Usuarios\exemplo\zip-1.txt: acesso negado` {
		t.Errorf("erro = %q", got)
	}
}

func TestAnonimizar_RadicalNaoTrocaChave(t *testing.T) {
	entrada := `{"msg":"x","zip":"pasta.zip"}` + "\n" +
		`{"msg":"y","pasta":"C:\\Usuarios\\exemplo\\pasta","etapa":"pasta ok","outra":"pastas"}` + "\n"
	linhas, _ := rodar(t, entrada)
	todasValidas(t, linhas)
	want := `{"msg":"y","pasta":"C:\\Usuarios\\exemplo\\zip-1","etapa":"zip-1 ok","outra":"pastas"}`
	if linhas[1] != want {
		t.Errorf("linha 2:\n got %s\nwant %s", linhas[1], want)
	}
}

func TestAnonimizar_RadicalComMaisDelimitadores(t *testing.T) {
	entrada := `{"msg":"x","zip":"5000000-00.2026.4.00.0010.zip"}` + "\n" +
		`{"msg":"y","pasta":"C:\\Usuarios\\exemplo\\5000000-00.2026.4.00.0010_anexos (5000000-00.2026.4.00.0010)","lista":"[5000000-00.2026.4.00.0010],5000000-00.2026.4.00.0010-b"}` + "\n"
	linhas, _ := rodar(t, entrada)
	todasValidas(t, linhas)
	if strings.Contains(linhas[1], "5000000-00.2026.4.00.0010") {
		t.Errorf("o CNJ sobrou: %s", linhas[1])
	}
	if got := campo(t, linhas[1], "pasta"); got != `C:\Usuarios\exemplo\zip-1_anexos (zip-1)` {
		t.Errorf("pasta = %q", got)
	}
	if got := campo(t, linhas[1], "lista"); got != `[zip-1],zip-1-b` {
		t.Errorf("lista = %q", got)
	}
}

func TestAnonimizar_JSONQueNaoEObjeto(t *testing.T) {
	entrada := `"texto"` + "\n" + "3\n" + "[]\n" + `["a",{"zip":"x.zip"}]` + "\n" +
		`{"msg":"ok","zip":"5000000-00.2026.4.00.0007.zip"}` + "\n"
	linhas, _ := rodar(t, entrada)
	if len(linhas) != 5 || linhas[0] != `"texto"` || linhas[2] != "[]" {
		t.Errorf("linhas = %q", linhas)
	}
	if got := campo(t, linhas[4], "zip"); got != "zip-1" {
		t.Errorf("zip = %q, esperava zip-1 (só objetos registram nomes)", got)
	}
}

func TestTemSobra_GrafiaEscapada(t *testing.T) {
	escapado := `C:\\Users\\` + `\` + `u003cusuario` + `\` + `u003e\\x`
	if temSobra(escapado) {
		t.Errorf("a grafia escapada de <usuario> não é sobra: %s", escapado)
	}
}

// gravar cria o arquivo com o conteúdo dado.
func gravar(t *testing.T, caminho, conteudo string) {
	t.Helper()
	if err := os.WriteFile(caminho, []byte(conteudo), 0o600); err != nil {
		t.Fatal(err)
	}
}

func TestExecutar_GravaAoLadoESobras(t *testing.T) {
	dir := t.TempDir()
	entrada := filepath.Join(dir, "eprocia.1.log")
	gravar(t, entrada, `{"msg":"a","zip":"5000000-00.2026.4.00.0005.zip","erro":"C:/Users/fulano/x"}`+"\n")
	res, err := executar([]string{entrada})
	if err != nil {
		t.Fatal(err)
	}
	if want := filepath.Join(dir, "eprocia.1.anon.log"); res[0].saida != want {
		t.Errorf("saida = %q, esperava %q", res[0].saida, want)
	}
	got, _ := os.ReadFile(res[0].saida)
	if want := `{"msg":"a","zip":"zip-1","erro":"C:/Users/fulano/x"}` + "\n"; string(got) != want {
		t.Errorf("conteúdo = %q", got)
	}
	if !slices.Equal(res[0].sobras, []int{1}) {
		t.Errorf("sobras = %v", res[0].sobras)
	}
}

func TestExecutar_VariosArquivosMesmosRotulos(t *testing.T) {
	dir := t.TempDir()
	a := filepath.Join(dir, "eprocia.log")
	b := filepath.Join(dir, "eprocia.1.log")
	gravar(t, a, `{"msg":"a","zip":"5000000-00.2026.4.00.0008.zip","doc":"Evento 1 - INICIAL.pdf"}`+"\n")
	gravar(t, b, `{"msg":"b","zip":"5000000-00.2026.4.00.0009.zip"}`+"\n"+
		`{"msg":"c","zip":"5000000-00.2026.4.00.0008.zip","doc":"Evento 1 - INICIAL.pdf"}`+"\n")
	res, err := executar([]string{a, b})
	if err != nil {
		t.Fatal(err)
	}
	if len(res) != 2 {
		t.Fatalf("esperava 2 resultados, veio %d", len(res))
	}
	ga, _ := os.ReadFile(filepath.Join(dir, "eprocia.anon.log"))
	gb, _ := os.ReadFile(filepath.Join(dir, "eprocia.1.anon.log"))
	if want := `{"msg":"a","zip":"zip-1","doc":"doc-1"}` + "\n"; string(ga) != want {
		t.Errorf("eprocia.anon.log = %q", ga)
	}
	if want := `{"msg":"b","zip":"zip-2"}` + "\n" + `{"msg":"c","zip":"zip-1","doc":"doc-1"}` + "\n"; string(gb) != want {
		t.Errorf("eprocia.1.anon.log = %q", gb)
	}
}
