package atualizacao

import (
	"context"
	"crypto/x509"
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"sync/atomic"
	"testing"
)

// servidorContado é um servidor de teste que conta as requisições recebidas.
type servidorContado struct {
	*httptest.Server
	n atomic.Int32
}

func novoServidorTLS(t *testing.T, h http.HandlerFunc) *servidorContado {
	t.Helper()
	s := &servidorContado{}
	s.Server = httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		s.n.Add(1)
		h(w, r)
	}))
	t.Cleanup(s.Close)
	return s
}

func novoServidorHTTP(t *testing.T) *servidorContado {
	t.Helper()
	s := &servidorContado{}
	s.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		s.n.Add(1)
	}))
	t.Cleanup(s.Close)
	return s
}

func raizesDe(srvs ...*httptest.Server) *x509.CertPool {
	p := x509.NewCertPool()
	for _, s := range srvs {
		if c := s.Certificate(); c != nil {
			p.AddCert(c)
		}
	}
	return p
}

func regrasTeste(base string) Regras {
	return Regras{
		Hosts:           []string{"127.0.0.1", "::1"},
		PrefixoAPI:      base + "/api/",
		PrefixoDownload: base + "/download/",
	}
}

var versaoTeste = Versao{0, 4, 4}

func fechar(r *http.Response) {
	if r != nil {
		_, _ = io.Copy(io.Discard, r.Body)
		_ = r.Body.Close()
	}
}

func TestCliente_RecusaHTTPInicial(t *testing.T) {
	srv := novoServidorHTTP(t)
	c := NovoCliente(regrasTeste(srv.URL), nil, versaoTeste)
	r, err := c.obter(context.Background(), srv.URL+"/api/x", urlAPI, "application/json")
	fechar(r)
	if !errors.Is(err, ErrURLRecusada) {
		t.Fatalf("erro = %v, quer ErrURLRecusada", err)
	}
	if n := srv.n.Load(); n != 0 {
		t.Fatalf("servidor recebeu %d requisições", n)
	}
}

func TestCliente_RecusaForaDoPrefixo(t *testing.T) {
	srv := novoServidorTLS(t, func(http.ResponseWriter, *http.Request) {})
	c := NovoCliente(regrasTeste(srv.URL), raizesDe(srv.Server), versaoTeste)
	casos := []struct {
		nome string
		url  string
		tipo tipoURL
	}{
		{"download fora", srv.URL + "/outro/x", urlDownload},
		{"download com prefixo da API", srv.URL + "/api/x", urlDownload},
		{"download com ponto-ponto", srv.URL + "/download/../outro/x", urlDownload},
		{"download com barra invertida codificada", srv.URL + "/download/..%5C..%5Cx", urlDownload},
		{"download com barra invertida", srv.URL + `/download/..\..\x`, urlDownload},
		{"api fora", srv.URL + "/outro/x", urlAPI},
		{"api com prefixo do download", srv.URL + "/download/x", urlAPI},
	}
	for _, cs := range casos {
		t.Run(cs.nome, func(t *testing.T) {
			r, err := c.obter(context.Background(), cs.url, cs.tipo, "*/*")
			fechar(r)
			if !errors.Is(err, ErrURLRecusada) {
				t.Fatalf("erro = %v, quer ErrURLRecusada", err)
			}
		})
	}
	if n := srv.n.Load(); n != 0 {
		t.Fatalf("servidor recebeu %d requisições", n)
	}
	// Controle: dentro do prefixo chega.
	r, err := c.obter(context.Background(), srv.URL+"/download/x", urlDownload, "*/*")
	fechar(r)
	if err != nil {
		t.Fatalf("URL dentro do prefixo recusada: %v", err)
	}
}

func TestCliente_ErroDeMontagemNaoERecusa(t *testing.T) {
	srv := novoServidorTLS(t, func(http.ResponseWriter, *http.Request) {})
	c := NovoCliente(regrasTeste(srv.URL), raizesDe(srv.Server), versaoTeste)
	var semCtx context.Context // nil: NewRequestWithContext falha
	r, err := c.obter(semCtx, srv.URL+"/api/x", urlAPI, "*/*")
	fechar(r)
	if err == nil || errors.Is(err, ErrURLRecusada) {
		t.Fatalf("erro = %v, quer erro que não seja ErrURLRecusada", err)
	}
}

func TestCliente_RecusaHostForaDaLista(t *testing.T) {
	srv := novoServidorTLS(t, func(http.ResponseWriter, *http.Request) {})
	base := strings.Replace(srv.URL, "127.0.0.1", "localhost", 1)
	c := NovoCliente(regrasTeste(base), raizesDe(srv.Server), versaoTeste)
	r, err := c.obter(context.Background(), base+"/api/x", urlAPI, "*/*")
	fechar(r)
	if !errors.Is(err, ErrURLRecusada) {
		t.Fatalf("erro = %v, quer ErrURLRecusada", err)
	}
	if n := srv.n.Load(); n != 0 {
		t.Fatalf("servidor recebeu %d requisições", n)
	}
}

func testarRedirecionamentoRecusado(t *testing.T, destino *servidorContado, alvo string) {
	t.Helper()
	origem := novoServidorTLS(t, func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, alvo, http.StatusFound)
	})
	c := NovoCliente(regrasTeste(origem.URL), raizesDe(origem.Server, destino.Server), versaoTeste)
	r, err := c.obter(context.Background(), origem.URL+"/download/x", urlDownload, "*/*")
	if r != nil {
		fechar(r)
		t.Error("resposta devolvida junto com o erro")
	}
	if !errors.Is(err, ErrURLRecusada) {
		t.Fatalf("erro = %v, quer ErrURLRecusada", err)
	}
	if origem.n.Load() != 1 {
		t.Fatalf("origem recebeu %d requisições, quer 1", origem.n.Load())
	}
	if n := destino.n.Load(); n != 0 {
		t.Fatalf("destino recebeu %d requisições", n)
	}
}

func TestCliente_RedirecionamentoParaHostForaRecusado(t *testing.T) {
	destino := novoServidorTLS(t, func(http.ResponseWriter, *http.Request) {})
	alvo := strings.Replace(destino.URL, "127.0.0.1", "localhost", 1) + "/download/x"
	testarRedirecionamentoRecusado(t, destino, alvo)
}

func TestCliente_RedirecionamentoParaHTTPRecusado(t *testing.T) {
	destino := novoServidorHTTP(t)
	testarRedirecionamentoRecusado(t, destino, destino.URL+"/download/x")
}

func TestCliente_RedirecionamentoParaOutroHostDaListaAceito(t *testing.T) {
	ln, err := net.Listen("tcp", "[::1]:0")
	if err != nil {
		t.Skipf("sem IPv6 de loopback nesta máquina: %v", err)
	}
	destino := &servidorContado{}
	destino.Server = httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		destino.n.Add(1)
		_, _ = io.WriteString(w, "segundo")
	}))
	_ = destino.Listener.Close()
	destino.Listener = ln
	destino.StartTLS()
	t.Cleanup(destino.Close)

	origem := novoServidorTLS(t, func(w http.ResponseWriter, r *http.Request) {
		// Fora do prefixo de propósito: o prefixo vale só para a URL inicial.
		http.Redirect(w, r, destino.URL+"/objeto/1", http.StatusFound)
	})
	c := NovoCliente(regrasTeste(origem.URL), raizesDe(origem.Server, destino.Server), versaoTeste)
	r, err := c.obter(context.Background(), origem.URL+"/download/x", urlDownload, "*/*")
	if err != nil {
		t.Fatalf("redirecionamento recusado: %v", err)
	}
	corpo, _ := io.ReadAll(r.Body)
	fechar(r)
	if string(corpo) != "segundo" || destino.n.Load() != 1 {
		t.Fatalf("corpo = %q, destino recebeu %d", corpo, destino.n.Load())
	}
}

func TestCliente_RedirecionamentoSemFimRecusado(t *testing.T) {
	var srv *servidorContado
	srv = novoServidorTLS(t, func(w http.ResponseWriter, r *http.Request) {
		// Para de redirecionar depois de 30, para que um cliente sem limite
		// falhe aqui em vez de girar para sempre.
		if srv.n.Load() <= 30 {
			http.Redirect(w, r, srv.URL+"/laco", http.StatusFound)
		}
	})
	c := NovoCliente(regrasTeste(srv.URL), raizesDe(srv.Server), versaoTeste)
	r, err := c.obter(context.Background(), srv.URL+"/download/x", urlDownload, "*/*")
	if r != nil {
		fechar(r)
		t.Error("resposta devolvida junto com o erro")
	}
	if !errors.Is(err, ErrURLRecusada) {
		t.Fatalf("erro = %v, quer ErrURLRecusada", err)
	}
	if n := srv.n.Load(); n > 11 {
		t.Fatalf("servidor recebeu %d requisições, quer no máximo 11", n)
	}
}

func TestCliente_Cabecalhos(t *testing.T) {
	type cab struct{ ua, accept, auth, cookie string }
	var vistos []cab
	srv := novoServidorTLS(t, func(w http.ResponseWriter, r *http.Request) {
		vistos = append(vistos, cab{r.UserAgent(), r.Header.Get("Accept"), r.Header.Get("Authorization"), r.Header.Get("Cookie")})
		http.SetCookie(w, &http.Cookie{Name: "sessao", Value: "abc", Path: "/"})
		if r.URL.Path == "/api/primeira" {
			http.Redirect(w, r, "/api/segunda", http.StatusFound)
		}
	})
	c := NovoCliente(regrasTeste(srv.URL), raizesDe(srv.Server), versaoTeste)
	for _, u := range []string{"/api/primeira", "/api/terceira"} {
		r, err := c.obter(context.Background(), srv.URL+u, urlAPI, "application/vnd.github+json")
		fechar(r)
		if err != nil {
			t.Fatalf("%s: %v", u, err)
		}
	}
	if len(vistos) != 3 {
		t.Fatalf("servidor viu %d requisições, quer 3", len(vistos))
	}
	for i, v := range vistos {
		if v.ua != "eprocIA/0.4.4" || v.accept != "application/vnd.github+json" || v.auth != "" || v.cookie != "" {
			t.Errorf("requisição %d: %+v", i, v)
		}
	}
}

func TestRegrasGitHub_Valores(t *testing.T) {
	r := RegrasGitHub()
	hosts := []string{"api.github.com", "github.com", "objects.githubusercontent.com", "release-assets.githubusercontent.com"}
	if !slices.Equal(slices.Sorted(slices.Values(r.Hosts)), hosts) {
		t.Errorf("Hosts = %v", r.Hosts)
	}
	if r.PrefixoAPI != "https://api.github.com/repos/jonyduque/eprocia/" {
		t.Errorf("PrefixoAPI = %q", r.PrefixoAPI)
	}
	if r.PrefixoDownload != "https://github.com/jonyduque/eprocia/releases/download/" {
		t.Errorf("PrefixoDownload = %q", r.PrefixoDownload)
	}
}
