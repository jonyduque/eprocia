package atualizacao

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"slices"
	"strings"
	"time"
)

// Regras dizem para onde o cliente pode ir: hosts aceitos (na URL inicial e
// em cada redirecionamento) e o prefixo exigido da URL inicial por tipo.
type Regras struct {
	Hosts                       []string
	PrefixoAPI, PrefixoDownload string
}

// RegrasGitHub são as regras do executável publicado: repositório fixo
// jonyduque/eprocia. Nenhuma parte vem de configuração, ambiente ou frontend.
func RegrasGitHub() Regras {
	return Regras{
		Hosts: []string{
			"api.github.com",
			"github.com",
			"objects.githubusercontent.com",
			"release-assets.githubusercontent.com",
		},
		PrefixoAPI:      "https://api.github.com/repos/jonyduque/eprocia/",
		PrefixoDownload: "https://github.com/jonyduque/eprocia/releases/download/",
	}
}

// ErrURLRecusada é devolvido quando a URL inicial ou um redirecionamento sai
// das regras.
var ErrURLRecusada = errors.New("URL fora das regras da atualização")

type tipoURL int

const (
	urlAPI tipoURL = iota
	urlDownload
)

const (
	maxRedirecionamentos = 10
	prazoConexao         = 15 * time.Second
)

// Cliente é o único cliente HTTP do eprocIA: só HTTPS, só os hosts das
// Regras, sem cookies nem credenciais.
type Cliente struct {
	regras    Regras
	http      *http.Client
	userAgent string

	// prazoConsulta é o prazo total de Verificar; zero usa PrazoConsulta
	// (os testes o reduzem).
	prazoConsulta time.Duration
}

// NovoCliente monta o cliente; raizes nil usa as raízes do sistema.
func NovoCliente(r Regras, raizes *x509.CertPool, versao Versao) *Cliente {
	c := &Cliente{regras: r, userAgent: "eprocIA/" + versao.String()}
	transporte := &http.Transport{
		Proxy:                 http.ProxyFromEnvironment,
		DialContext:           (&net.Dialer{Timeout: prazoConexao, KeepAlive: 30 * time.Second}).DialContext,
		TLSClientConfig:       &tls.Config{RootCAs: raizes, MinVersion: tls.VersionTLS12},
		ForceAttemptHTTP2:     true,
		TLSHandshakeTimeout:   prazoConexao,
		ResponseHeaderTimeout: prazoConexao,
		IdleConnTimeout:       90 * time.Second,
		MaxIdleConns:          4,
	}
	c.http = &http.Client{
		Transport: transporte,
		Jar:       nil,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= maxRedirecionamentos {
				return fmt.Errorf("%w: redirecionamentos demais", ErrURLRecusada)
			}
			if err := c.conferirHost(req.URL); err != nil {
				return err
			}
			// Nada de credencial ou cookie viaja num salto, mesmo que algo
			// os tenha posto.
			req.Header.Del("Authorization")
			req.Header.Del("Cookie")
			return nil
		},
	}
	return c
}

// conferirHost aplica esquema, credenciais na URL e lista de hosts.
func (c *Cliente) conferirHost(u *url.URL) error {
	if u.Scheme != "https" {
		return fmt.Errorf("%w: esquema não é https", ErrURLRecusada)
	}
	if u.User != nil {
		return fmt.Errorf("%w: URL com credenciais", ErrURLRecusada)
	}
	host := u.Hostname()
	if !slices.ContainsFunc(c.regras.Hosts, func(h string) bool { return strings.EqualFold(h, host) }) {
		return fmt.Errorf("%w: host fora da lista", ErrURLRecusada)
	}
	return nil
}

// conferirInicial aplica, além do host, o prefixo do tipo e recusa segmentos
// "." e ".." (que levariam para fora do prefixo).
func (c *Cliente) conferirInicial(bruta string, tipo tipoURL) (*url.URL, error) {
	u, err := url.Parse(bruta)
	if err != nil {
		return nil, fmt.Errorf("%w: URL inválida", ErrURLRecusada)
	}
	if err := c.conferirHost(u); err != nil {
		return nil, err
	}
	var prefixo string
	switch tipo {
	case urlAPI:
		prefixo = c.regras.PrefixoAPI
	case urlDownload:
		prefixo = c.regras.PrefixoDownload
	}
	if prefixo == "" || !strings.HasPrefix(bruta, prefixo) {
		return nil, fmt.Errorf("%w: fora do prefixo", ErrURLRecusada)
	}
	if strings.Contains(u.Path, `\`) {
		return nil, fmt.Errorf("%w: caminho com barra invertida", ErrURLRecusada)
	}
	for seg := range strings.SplitSeq(u.Path, "/") {
		if seg == "." || seg == ".." {
			return nil, fmt.Errorf("%w: caminho com . ou ..", ErrURLRecusada)
		}
	}
	return u, nil
}

// obter faz um GET dentro das regras. O chamador fecha o corpo.
func (c *Cliente) obter(ctx context.Context, url string, tipo tipoURL, accept string) (*http.Response, error) {
	u, err := c.conferirInicial(url, tipo)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("montar requisição: %w", err)
	}
	req.Header.Set("User-Agent", c.userAgent)
	if accept != "" {
		req.Header.Set("Accept", accept)
	}
	resp, err := c.http.Do(req)
	if err != nil {
		// Com erro do CheckRedirect, Do devolve também a resposta anterior
		// (corpo já fechado); quem chama não deve vê-la.
		return nil, err
	}
	return resp, nil
}
