package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"regexp"
	"slices"
	"strings"
)

// maxLinha limita o tamanho de uma linha do log (a do relatório de um ZIP
// grande passa de 64 KiB, o padrão do bufio.Scanner).
const maxLinha = 64 << 20

// usuarioEmCaminho casa \Users\ ou /Users/ (com separador repetido, como em
// JSON escapado) em qualquer grafia de maiúsculas.
var usuarioEmCaminho = regexp.MustCompile(`(?i)[\\/]+users[\\/]+`)

// rotulos guarda o rótulo de cada nome, na ordem da primeira aparição.
type rotulos struct {
	rotulo map[string]string
	nZip   int
	nDoc   int
}

func (r *rotulos) zip(nome string) {
	if nome == "" || r.rotulo[nome] != "" {
		return
	}
	r.nZip++
	r.rotulo[nome] = fmt.Sprintf("zip-%d", r.nZip)
}

func (r *rotulos) doc(nome string) {
	if nome == "" || r.rotulo[nome] != "" {
		return
	}
	r.nDoc++
	r.rotulo[nome] = fmt.Sprintf("doc-%d", r.nDoc)
}

// anonimizar copia o log de r para w trocando cada nome de ZIP (atributo
// zip) por zip-N e cada nome de documento (atributo doc e nome dentro de
// relatorio.atencao) por doc-N, com N na ordem da primeira aparição no
// arquivo. O nome é trocado onde aparecer, em qualquer campo ou numa linha
// que não é JSON; o resto da linha fica como está. sobras traz os números
// (a partir de 1) das linhas de saída com \Users\ ou /Users/ seguido de
// algo que não seja <usuario>.
func anonimizar(r io.Reader, w io.Writer) (sobras []int, err error) {
	var linhas []string
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 0, 64<<10), maxLinha)
	for sc.Scan() {
		linhas = append(linhas, sc.Text())
	}
	if err := sc.Err(); err != nil {
		return nil, fmt.Errorf("ler o log: %w", err)
	}

	rot := &rotulos{rotulo: map[string]string{}}
	for _, l := range linhas {
		coletar(l, rot)
	}
	troca := substituidor(rot.rotulo)

	bw := bufio.NewWriter(w)
	for i, l := range linhas {
		l = troca.Replace(l)
		if temSobra(l) {
			sobras = append(sobras, i+1)
		}
		bw.WriteString(l)
		bw.WriteByte('\n')
	}
	if err := bw.Flush(); err != nil {
		return sobras, fmt.Errorf("gravar a saída: %w", err)
	}
	return sobras, nil
}

// coletar percorre a linha JSON na ordem do texto e registra os nomes de ZIP
// e de documento. Uma linha que não é JSON não registra nada.
func coletar(linha string, rot *rotulos) {
	if !json.Valid([]byte(linha)) {
		return
	}
	dec := json.NewDecoder(strings.NewReader(linha))
	// caminho tem uma entrada por objeto ou lista aberta: a chave atual do
	// objeto ou "[]" para lista.
	var caminho []string
	esperaChave := false
	for {
		tok, err := dec.Token()
		if err != nil {
			return
		}
		switch v := tok.(type) {
		case json.Delim:
			switch v {
			case '{':
				caminho = append(caminho, "")
				esperaChave = true
			case '[':
				caminho = append(caminho, "[]")
				esperaChave = false
			case '}', ']':
				caminho = caminho[:len(caminho)-1]
				esperaChave = len(caminho) > 0 && caminho[len(caminho)-1] != "[]"
			}
			continue
		case string:
			if esperaChave {
				caminho[len(caminho)-1] = v
				esperaChave = false
				continue
			}
			registrar(caminho, v, rot)
		}
		// Depois de um valor dentro de objeto vem outra chave.
		esperaChave = len(caminho) > 0 && caminho[len(caminho)-1] != "[]"
	}
}

// registrar decide se o valor de texto no caminho dado é nome de ZIP ou de
// documento.
func registrar(caminho []string, valor string, rot *rotulos) {
	chave := caminho[len(caminho)-1]
	switch {
	case chave == "zip":
		rot.zip(valor)
	case chave == "doc":
		rot.doc(valor)
	case chave == "nome" && dentroDeAtencao(caminho):
		rot.doc(valor)
	}
}

// dentroDeAtencao diz se o caminho termina em relatorio.atencao[].nome.
func dentroDeAtencao(c []string) bool {
	n := len(c)
	return n >= 4 && c[n-4] == "relatorio" && c[n-3] == "atencao" && c[n-2] == "[]"
}

// substituidor troca cada nome pelo seu rótulo, em texto puro e nas grafias
// escapadas de JSON (com e sem o escape de HTML do json.Marshal). Os nomes
// mais longos vêm primeiro: numa mesma posição, o Replacer usa o primeiro
// par que casa, e um nome contido em outro não pode cortar o maior.
func substituidor(rotulo map[string]string) *strings.Replacer {
	type par struct{ de, para string }
	var pares []par
	for nome, r := range rotulo {
		vistos := map[string]bool{}
		for _, forma := range []string{nome, escaparJSON(nome, false), escaparJSON(nome, true)} {
			if !vistos[forma] {
				vistos[forma] = true
				pares = append(pares, par{forma, r})
			}
		}
	}
	slices.SortFunc(pares, func(a, b par) int {
		if d := len(b.de) - len(a.de); d != 0 {
			return d
		}
		return strings.Compare(a.de, b.de)
	})
	args := make([]string, 0, 2*len(pares))
	for _, p := range pares {
		args = append(args, p.de, p.para)
	}
	return strings.NewReplacer(args...)
}

// escaparJSON devolve s como apareceria dentro de uma string JSON, sem as
// aspas.
func escaparJSON(s string, html bool) string {
	var b bytes.Buffer
	enc := json.NewEncoder(&b)
	enc.SetEscapeHTML(html)
	if err := enc.Encode(s); err != nil {
		return s
	}
	t := strings.TrimSuffix(b.String(), "\n")
	return t[1 : len(t)-1]
}

// temSobra diz se a linha tem \Users\ ou /Users/ seguido de algo que não
// seja o <usuario> do saneamento (também na grafia <usuario>).
func temSobra(linha string) bool {
	for _, m := range usuarioEmCaminho.FindAllStringIndex(linha, -1) {
		resto := linha[m[1]:]
		if resto == "" || strings.HasPrefix(resto, "<usuario>") || strings.HasPrefix(resto, `<usuario>`) {
			continue
		}
		return true
	}
	return false
}
