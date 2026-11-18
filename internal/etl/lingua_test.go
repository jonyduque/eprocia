package etl

import (
	"strings"
	"testing"
)

func TestContarLingua_Palavras(t *testing.T) {
	casos := []struct {
		entrada  string
		esperado contagemLingua
	}{
		{"k7Qz9X", contagemLingua{1, 1, 0}},
		{"aBc", contagemLingua{1, 1, 0}},
		{"xkcdt", contagemLingua{1, 1, 0}},
		{"QWRTPS", contagemLingua{1, 1, 0}},
		{"STJ", contagemLingua{1, 0, 0}},
		{"CNPJ", contagemLingua{1, 0, 0}},
		{"TJSP", contagemLingua{1, 0, 0}},
		{"petição", contagemLingua{1, 0, 0}},
		{"Requerente,", contagemLingua{1, 0, 0}},
		{"(artigo)", contagemLingua{1, 0, 0}},
		{"de", contagemLingua{1, 0, 1}},
		{"É", contagemLingua{1, 0, 1}},
		{"não", contagemLingua{1, 0, 1}},
		{"à", contagemLingua{1, 0, 1}},
		{"nao", contagemLingua{1, 0, 1}}, // acento perdido ainda é funcional
		{"NAO", contagemLingua{1, 0, 1}}, // caixa alta também
		{"NÃO", contagemLingua{1, 0, 1}},
		{"Aos", contagemLingua{1, 0, 1}},
		{"12/03/2026", contagemLingua{}},
		{"1.234,56", contagemLingua{}},
		{"123.456.789-00", contagemLingua{}},
		{"5000000-00.2026.8.26.0001", contagemLingua{}},
		{"a3f9c2e1b7d4", contagemLingua{}},
		{"ab", contagemLingua{}},
		{"--", contagemLingua{}},
		{"", contagemLingua{}},
		// Escolhas do implementador onde o brief não decide.
		{"a1b2c3d4-e5f6-7890-abcd-ef1234567890", contagemLingua{}}, // UUID: hexadecimal com hífens é neutro
		{"R$", contagemLingua{}},                                   // 2 caracteres, não funcional
		{"d'água", contagemLingua{1, 0, 0}},                        // apóstrofo no meio não separa nem pune
		{"guarda-chuva", contagemLingua{1, 0, 0}},                  // hífen no meio idem
		{"привет", contagemLingua{1, 0, 0}},                        // letra não latina fora da regra de vogais
	}
	for _, c := range casos {
		if got := contarLingua(c.entrada); got != c.esperado {
			t.Errorf("contarLingua(%q) = %+v, esperado %+v", c.entrada, got, c.esperado)
		}
	}
}

const paragrafoPt = `O requerente FULANO DE TAL ajuizou a presente ação em face de BELTRANO DE TAL, ` +
	`alegando que o contrato firmado entre as partes não foi cumprido na forma prevista, ` +
	`e que por isso a parte autora sofreu prejuízos de ordem material e moral. ` +
	`Citado, o réu apresentou contestação, na qual sustentou a ausência de prova do dano ` +
	`e requereu a improcedência dos pedidos. Houve réplica, e as partes foram intimadas para ` +
	`dizer se pretendiam produzir outras provas, tendo ambas pugnado pelo julgamento da lide ` +
	`no estado em que se encontra, nos termos do artigo 355 do Código de Processo Civil, ` +
	`processo 5000000-00.2026.8.26.0001, que tramita na comarca de origem.`

var palavrasAleatorias = strings.Fields(`k7Qz9X Xq4Lm2 bTrWq a9Zk3Q Pw8nB4 mX2vL9 tYh7Kd3 zQ5rT1 uJ6yN8 gF3dS0
cV9bN2 hK4jL7 wE1rT6 yU8iO3 pL5kJ9 qA2sD7 xZ6cV1 nM3bV8 fG7hJ4 dS1aQ5
rT9yU2 eW4qA6 oI8pL1 kJ3hG7 vC5xZ9 bN2mQ4 lK7jH1 sD6fG3 aS9dF5 zX1cV8
tR4eW7 yH2gF6 uI5oP3 iO8pA1 pQ3wE9 wS7xC2 eD4cV6 rF1vB8 tG5bN3 yH9nM7
uJ2mK4 iK6lO1 oL8pQ5 pA3sW9 aZ7xE2 sX4cR6 dC1vT8 fV5bY3 gB9nU7 hN2mI4
jM6kO1 kL8oP5 lP3aS9 qW7eR2 wE4tY6 eR1uI8 rT5oP3 tY9aS7 yU2dF4 uI6gH1
iO8jK5 oP3lZ9 pA7xC2 aS4vB6 sD1nM8 dF5qW3 fG9eR7 gH2tY4 hJ6uI1 jK8oP5
kL3aS9 lZ7dF2 zX4gH6 xC1jK8 cV5lQ3 vB9wE7 bN2rT4 nM6yU1 mQ8iO5 qW3pA9`)

func TestContarLingua_Textos(t *testing.T) {
	// Parágrafo em português.
	c := contarLingua(paragrafoPt)
	if c.palavras < 60 {
		t.Fatalf("parágrafo deveria ter 60 palavras ou mais, tem %d", c.palavras)
	}
	if c.implausiveis != 0 {
		t.Errorf("parágrafo com implausíveis = %d, esperado 0", c.implausiveis)
	}
	if c.funcionais*100 < 15*c.palavras {
		t.Errorf("funcionais = %d de %d, esperado 15%% ou mais", c.funcionais, c.palavras)
	}
	if c.isca() {
		t.Errorf("parágrafo em português não pode ser isca: %+v", c)
	}

	// 80 palavras aleatórias.
	if len(palavrasAleatorias) != 80 {
		t.Fatalf("lista fixa deveria ter 80 palavras, tem %d", len(palavrasAleatorias))
	}
	c = contarLingua(strings.Join(palavrasAleatorias, " "))
	if !c.isca() {
		t.Errorf("80 palavras aleatórias deveriam ser isca: %+v", c)
	}
	c = contarLingua(strings.Join(palavrasAleatorias[:29], " "))
	if c.isca() {
		t.Errorf("29 palavras não bastam para isca: %+v", c)
	}
	c = contarLingua(strings.Join(palavrasAleatorias[:30], " "))
	if !c.isca() {
		t.Errorf("30 palavras aleatórias deveriam ser isca: %+v", c)
	}

	// Rodapé com URLs e códigos entre frases (cerca de 30% de implausíveis).
	rodape := strings.Repeat("Para conferir o documento acesse o endereço do tribunal e informe o código ", 1) +
		"xK9Qz7 wR4tY2 hN8mP3 jL6vB1 cD5fG0 qS2zX8 bV7nM4 tR9kL3 yU5oP6 eW1aS9 pL4kJ7 mN2bV5 " +
		"A autenticidade pode ser verificada na forma da lei com a senha do processo e o número do protocolo da parte"
	c = contarLingua(rodape)
	if c.palavras < 30 || c.implausiveis*100 < 25*c.palavras || c.implausiveis*100 > 40*c.palavras {
		t.Fatalf("rodapé fora do perfil esperado (cerca de 30%% implausíveis): %+v", c)
	}
	if c.isca() {
		t.Errorf("rodapé com URLs e códigos não pode ser isca: %+v", c)
	}

	// Rótulos e códigos de rodapé, sem preposição nem artigo: só o limiar de
	// implausíveis decide. 12 de 42 (28,6%) não é isca; 17 de 42 (40,5%) é.
	rotulos := strings.Fields(`Protocolo Autenticação Agência Documento Emitido Página Código Comarca Foro Central
Cartório Distribuidor Volume Registro Selo Digital Sistema Consulta Recibo Número
Guia Valor Total Situação Parcela Tribunal Unidade Serviço Cadastro Versão`)
	codigos := strings.Fields(`k7Qz9X Xq4Lm2 bTrWq a9Zk3Q Pw8nB4 mX2vL9 tYh7Kd3 zQ5rT1 uJ6yN8 gF3dS0
cV9bN2 hK4jL7 wE1rT6 yU8iO3 pL5kJ9 qA2sD7 eR5tY9`)
	montar := func(nRot, nCod int) contagemLingua {
		return contarLingua(strings.Join(rotulos[:nRot], " ") + " " + strings.Join(codigos[:nCod], " "))
	}
	c = montar(30, 12)
	if c.palavras < 40 || c.funcionais*100 >= 5*c.palavras ||
		c.implausiveis*100 < 28*c.palavras || c.implausiveis*100 > 33*c.palavras {
		t.Fatalf("premissas do rodapé a 30%% falharam: %+v", c)
	}
	if c.isca() {
		t.Errorf("rodapé com cerca de 30%% de implausíveis não pode ser isca: %+v", c)
	}
	c = montar(25, 17)
	if c.palavras < 40 || c.funcionais*100 >= 5*c.palavras ||
		c.implausiveis*100 < 38*c.palavras || c.implausiveis*100 > 43*c.palavras {
		t.Fatalf("premissas do rodapé a 40%% falharam: %+v", c)
	}
	if !c.isca() {
		t.Errorf("rodapé com cerca de 40%% de implausíveis deveria ser isca: %+v", c)
	}

	// Comprovante.
	comprovante := "Comprovante de transferência Pix Valor R$ 1.234,56 Data 12/03/2026 Pagador FULANO DE TAL " +
		"Banco do Exemplo Agência 0001 Conta 12345-6 Recebedor BELTRANO DE TAL Chave Pix " +
		"a1b2c3d4-e5f6-7890-abcd-ef1234567890 ID da transação E12345678202603121234abcdEFGH567 " +
		"Autenticação 9F8E7D6C5B4A39281706F5E4D3C2B1A0"
	c = contarLingua(comprovante)
	if c.isca() {
		t.Errorf("comprovante não pode ser isca: %+v", c)
	}
	if c.funcionais*100 < 5*c.palavras {
		t.Errorf("comprovante com funcionais = %d de %d, esperado 5%% ou mais", c.funcionais, c.palavras)
	}

	// Vazio.
	if c := contarLingua(""); c != (contagemLingua{}) || c.isca() {
		t.Errorf("texto vazio: %+v, isca=%v", c, c.isca())
	}
}

func TestContarLingua_EntradaHostil(t *testing.T) {
	entradas := map[string]string{
		"utf8 inválido":    "\xff\xfe\xc3 abc\x80def \xe2\x82 ghi\xf0\x9f xk7Qz9 \xff",
		"palavra de 1 MiB": strings.Repeat("xB1", 350000),
		"só espaços":       strings.Repeat(" \t\r\n  ", 1000),
		"só pontuação":     strings.Repeat(".,;:!?-()[]{}\"'/\\", 1000),
		"controle":         "abc\x00def\x01 ghi\x1b[0m jkl\u0085mno",
	}
	for nome, e := range entradas {
		c := contarLingua(e)
		if c.implausiveis > c.palavras || c.funcionais > c.palavras || c.palavras < 0 {
			t.Errorf("%s: contagem incoerente %+v", nome, c)
		}
		_ = normalizarPalavra(e)
	}
	if c := contarLingua(entradas["palavra de 1 MiB"]); c.palavras != 1 || c.implausiveis != 1 {
		t.Errorf("palavra gigante: %+v, esperado 1 palavra implausível", c)
	}
	if c := contarLingua(entradas["só espaços"]); c != (contagemLingua{}) {
		t.Errorf("só espaços: %+v", c)
	}
	if c := contarLingua(entradas["só pontuação"]); c != (contagemLingua{}) {
		t.Errorf("só pontuação: %+v", c)
	}
}

func TestNormalizarPalavra(t *testing.T) {
	casos := map[string]string{
		"Petição": "peticao",
		"ÀS":      "as",
		"Órgão":   "orgao",
		"abc123":  "abc123",
		"Ç":       "c",
		"":        "",
	}
	for in, esp := range casos {
		if got := normalizarPalavra(in); got != esp {
			t.Errorf("normalizarPalavra(%q) = %q, esperado %q", in, got, esp)
		}
	}
}
