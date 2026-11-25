package atualizacao

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"testing"
	"time"
)

type assetJSON struct {
	Name string `json:"name"`
	URL  string `json:"browser_download_url"`
	Size int64  `json:"size"`
}

type releaseJSON struct {
	TagName    string      `json:"tag_name"`
	Draft      bool        `json:"draft"`
	Prerelease bool        `json:"prerelease"`
	HTMLURL    string      `json:"html_url"`
	Assets     []assetJSON `json:"assets"`
}

// cenarioConsulta guarda o servidor de teste, o cliente e a release servida.
type cenarioConsulta struct {
	srv     *servidorContado
	cliente *Cliente
	regras  Regras
	rel     releaseJSON
	status  int
	corpo   []byte // quando não nil, substitui o JSON da release
	caminho string // caminho da última requisição
}

func novoCenario(t *testing.T) *cenarioConsulta {
	t.Helper()
	c := &cenarioConsulta{status: http.StatusOK}
	c.srv = novoServidorTLS(t, func(w http.ResponseWriter, r *http.Request) {
		c.caminho = r.URL.Path
		if c.status != http.StatusOK {
			w.WriteHeader(c.status)
			return
		}
		if c.corpo != nil {
			_, _ = w.Write(c.corpo)
			return
		}
		_ = json.NewEncoder(w).Encode(c.rel)
	})
	c.regras = regrasTeste(c.srv.URL)
	c.cliente = NovoCliente(c.regras, raizesDe(c.srv.Server), versaoTeste)
	return c
}

// assetDe monta um asset com a URL esperada para a tag.
func (c *cenarioConsulta) assetDe(tag, nome string, tamanho int64) assetJSON {
	return assetJSON{Name: nome, URL: c.regras.PrefixoDownload + tag + "/" + nome, Size: tamanho}
}

// releaseCompleta monta uma release com rótulo (versão dos nomes) e os três
// assets.
func (c *cenarioConsulta) releaseCompleta(tag, rotulo string) releaseJSON {
	return releaseJSON{
		TagName: tag,
		HTMLURL: "https://github.com/jonyduque/eprocia/releases/tag/" + tag,
		Assets: []assetJSON{
			c.assetDe(tag, "eprocia-"+rotulo+"-windows-amd64.exe", 1000),
			c.assetDe(tag, "eprocia-"+rotulo+"-windows-amd64-instalador.exe", 2000),
			c.assetDe(tag, "SHA256SUMS.txt", 300),
		},
	}
}

func TestVerificar_VersaoMaiorAceita(t *testing.T) {
	c := novoCenario(t)
	c.rel = c.releaseCompleta("v0.5.0", "0.5.0")
	rel, err := c.cliente.Verificar(context.Background(), versaoTeste)
	if err != nil {
		t.Fatalf("Verificar: %v", err)
	}
	if rel == nil {
		t.Fatal("Verificar devolveu nil")
	}
	if c.caminho != "/api/releases/latest" {
		t.Errorf("caminho = %q", c.caminho)
	}
	if rel.Tag != "v0.5.0" || rel.Versao != (Versao{0, 5, 0}) || rel.NotasURL != c.rel.HTMLURL {
		t.Errorf("cabeçalho = %+v", rel)
	}
	conferir := func(nome string, a Asset, j assetJSON) {
		if a.Nome != j.Name || a.URL != j.URL || a.Tamanho != j.Size {
			t.Errorf("%s = %+v, quer %+v", nome, a, j)
		}
	}
	conferir("Exe", rel.Exe, c.rel.Assets[0])
	conferir("Instalador", rel.Instalador, c.rel.Assets[1])
	conferir("Somas", rel.Somas, c.rel.Assets[2])
}

func TestVerificar_EmDia(t *testing.T) {
	for _, caso := range []struct{ tag, rotulo string }{
		{"v0.4.4", "0.4.4"},
		{"v0.4.3", "0.4.3"},
		{"0.5.0", "0.5.0"},
		{"v0.5.0-rc1", "0.5.0-rc1"},
		{"release-5", "5"},
	} {
		t.Run(caso.tag, func(t *testing.T) {
			c := novoCenario(t)
			c.rel = c.releaseCompleta(caso.tag, caso.rotulo)
			rel, err := c.cliente.Verificar(context.Background(), versaoTeste)
			if rel != nil || err != nil {
				t.Fatalf("Verificar = (%+v, %v), quer (nil, nil)", rel, err)
			}
		})
	}
}

func TestVerificar_DraftEPrereleaseIgnorados(t *testing.T) {
	for _, nome := range []string{"draft", "prerelease"} {
		t.Run(nome, func(t *testing.T) {
			c := novoCenario(t)
			c.rel = c.releaseCompleta("v0.5.0", "0.5.0")
			c.rel.Draft = nome == "draft"
			c.rel.Prerelease = nome == "prerelease"
			rel, err := c.cliente.Verificar(context.Background(), versaoTeste)
			if rel != nil || err != nil {
				t.Fatalf("Verificar = (%+v, %v), quer (nil, nil)", rel, err)
			}
		})
	}
}

func TestVerificar_Incompleta(t *testing.T) {
	const tag = "v0.5.0"
	casos := []struct {
		nome  string
		mexer func(c *cenarioConsulta, r *releaseJSON)
	}{
		{"sem exe", func(c *cenarioConsulta, r *releaseJSON) { r.Assets = r.Assets[1:] }},
		{"dois exe", func(c *cenarioConsulta, r *releaseJSON) {
			// Mesma versão e URL válida: só a contagem pode recusar.
			r.Assets = append(r.Assets, c.assetDe(tag, "eprocia-0.5.0-windows-amd64.exe", 10))
		}},
		{"dois instaladores", func(c *cenarioConsulta, r *releaseJSON) {
			r.Assets = append(r.Assets, c.assetDe(tag, "eprocia-0.5.0-windows-amd64-instalador.exe", 10))
		}},
		{"sem instalador", func(c *cenarioConsulta, r *releaseJSON) {
			r.Assets = append(r.Assets[:1], r.Assets[2])
		}},
		{"exe de outra versão", func(c *cenarioConsulta, r *releaseJSON) {
			r.Assets[0] = c.assetDe(tag, "eprocia-0.4.9-windows-amd64.exe", 10)
		}},
		{"sem somas", func(c *cenarioConsulta, r *releaseJSON) { r.Assets = r.Assets[:2] }},
		{"duas somas", func(c *cenarioConsulta, r *releaseJSON) {
			r.Assets = append(r.Assets, c.assetDe(tag, "SHA256SUMS.txt", 10))
		}},
		{"URL de outra tag", func(c *cenarioConsulta, r *releaseJSON) {
			r.Assets[0].URL = c.regras.PrefixoDownload + "v0.4.9/" + r.Assets[0].Name
		}},
		{"URL de outro nome", func(c *cenarioConsulta, r *releaseJSON) {
			r.Assets[1].URL = c.regras.PrefixoDownload + tag + "/outro.exe"
		}},
		{"URL de outro repositório", func(c *cenarioConsulta, r *releaseJSON) {
			r.Assets[2].URL = strings.Replace(r.Assets[2].URL, "/download/", "/outro/download/", 1)
		}},
	}
	for _, caso := range casos {
		t.Run(caso.nome, func(t *testing.T) {
			c := novoCenario(t)
			c.rel = c.releaseCompleta(tag, "0.5.0")
			caso.mexer(c, &c.rel)
			rel, err := c.cliente.Verificar(context.Background(), versaoTeste)
			if !errors.Is(err, ErrIncompleta) {
				t.Fatalf("erro = %v, quer ErrIncompleta", err)
			}
			quer := Release{Tag: tag, Versao: Versao{0, 5, 0}, NotasURL: c.rel.HTMLURL}
			if rel == nil || *rel != quer {
				t.Fatalf("release = %+v, quer %+v", rel, quer)
			}
		})
	}
}

func TestVerificar_RespostaAcimaDe1MiB(t *testing.T) {
	c := novoCenario(t)
	// JSON válido maior que o limite: o erro tem de vir do tamanho.
	corpo := []byte(`{"tag_name":"v0.5.0","html_url":"`)
	corpo = append(corpo, strings.Repeat("a", LimiteAPI+1-len(corpo)-2)...)
	corpo = append(corpo, `"}`...)
	if len(corpo) != LimiteAPI+1 {
		t.Fatalf("corpo com %d bytes", len(corpo))
	}
	c.corpo = corpo
	rel, err := c.cliente.Verificar(context.Background(), versaoTeste)
	if err == nil || errors.Is(err, ErrIncompleta) || errors.Is(err, ErrRecusada) || rel != nil {
		t.Fatalf("Verificar = (%+v, %v), quer erro comum", rel, err)
	}
}

func TestVerificar_JSONInvalidoEStatus(t *testing.T) {
	casos := []struct {
		nome     string
		status   int
		corpo    string
		recusada bool
	}{
		{"JSON inválido", http.StatusOK, `{"tag_name":`, false},
		{"500", http.StatusInternalServerError, "", false},
		{"404", http.StatusNotFound, "", false},
		{"403", http.StatusForbidden, "", true},
		{"429", http.StatusTooManyRequests, "", true},
	}
	for _, caso := range casos {
		t.Run(caso.nome, func(t *testing.T) {
			c := novoCenario(t)
			c.status = caso.status
			c.corpo = []byte(caso.corpo)
			rel, err := c.cliente.Verificar(context.Background(), versaoTeste)
			if err == nil || rel != nil {
				t.Fatalf("Verificar = (%+v, %v), quer erro", rel, err)
			}
			if errors.Is(err, ErrRecusada) != caso.recusada {
				t.Fatalf("erro = %v; ErrRecusada esperado: %v", err, caso.recusada)
			}
			if errors.Is(err, ErrIncompleta) {
				t.Fatalf("erro = %v, não deveria ser ErrIncompleta", err)
			}
		})
	}
}

func TestVerificar_Prazo(t *testing.T) {
	srv := novoServidorTLS(t, func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-r.Context().Done():
		case <-time.After(5 * time.Second):
		}
	})
	cl := NovoCliente(regrasTeste(srv.URL), raizesDe(srv.Server), versaoTeste)
	cl.prazoConsulta = 200 * time.Millisecond
	inicio := time.Now()
	rel, err := cl.Verificar(context.Background(), versaoTeste)
	if err == nil || rel != nil {
		t.Fatalf("Verificar = (%+v, %v), quer erro", rel, err)
	}
	if d := time.Since(inicio); d > 2*time.Second {
		t.Fatalf("Verificar levou %v", d)
	}
}
