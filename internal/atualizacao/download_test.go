package atualizacao

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

const (
	tagDownload  = "v0.5.0"
	nomeDownload = "eprocia.exe.download-teste"
)

// cenarioDownload serve um exe e um SHA256SUMS.txt pelo prefixo de download.
type cenarioDownload struct {
	srv     *servidorContado
	cliente *Cliente
	rel     *Release
	exe     []byte
	somas   []byte
	pasta   string

	// servirExe, quando não nil, substitui a resposta do exe.
	servirExe func(w http.ResponseWriter, r *http.Request)
	// parar é fechado no fim do teste (solta handlers parados).
	parar chan struct{}
}

func exeCarimbado(t *testing.T, v Versao) []byte {
	t.Helper()
	caminho := filepath.Join(t.TempDir(), "carimbado.exe")
	copiarAuxiliar(t, caminho, v)
	dados, err := os.ReadFile(caminho)
	if err != nil {
		t.Fatal(err)
	}
	return dados
}

func somasPara(conteudo []byte, nome string) []byte {
	s := sha256.Sum256(conteudo)
	return []byte(hex.EncodeToString(s[:]) + " *" + nome + "\n")
}

// novoCenarioDownload monta o cenário com o exe dado e somas certas.
func novoCenarioDownload(t *testing.T, exe []byte) *cenarioDownload {
	t.Helper()
	c := &cenarioDownload{exe: exe, pasta: t.TempDir(), parar: make(chan struct{})}
	c.srv = novoServidorTLS(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/download/" + tagDownload + "/" + nomeExe:
			if c.servirExe != nil {
				c.servirExe(w, r)
				return
			}
			w.Header().Set("Content-Length", strconv.Itoa(len(c.exe)))
			_, _ = w.Write(c.exe)
		case "/download/" + tagDownload + "/" + nomeSomas:
			_, _ = w.Write(c.somas)
		default:
			http.NotFound(w, r)
		}
	})
	// Registrado depois do Close do servidor: roda antes dele.
	t.Cleanup(func() { close(c.parar) })
	regras := regrasTeste(c.srv.URL)
	c.cliente = NovoCliente(regras, raizesDe(c.srv.Server), versaoTeste)
	asset := func(nome string, tam int64) Asset {
		return Asset{Nome: nome, URL: regras.PrefixoDownload + tagDownload + "/" + nome, Tamanho: tam}
	}
	c.somas = append(somasPara([]byte("outro"), nomeInst), somasPara(exe, nomeExe)...)
	c.rel = &Release{
		Tag:        tagDownload,
		Versao:     Versao{0, 5, 0},
		Exe:        asset(nomeExe, int64(len(exe))),
		Instalador: asset(nomeInst, 1000),
		Somas:      asset(nomeSomas, int64(len(c.somas))),
	}
	return c
}

func (c *cenarioDownload) baixar(ctx context.Context, progresso func(feito, total int64)) (Baixado, error) {
	return c.cliente.Baixar(ctx, c.rel, c.rel.Exe, c.pasta, nomeDownload, progresso)
}

func pastaVazia(t *testing.T, pasta string) {
	t.Helper()
	entradas, err := os.ReadDir(pasta)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entradas {
		t.Errorf("sobrou %s na pasta", e.Name())
	}
}

func TestBaixar_Confere(t *testing.T) {
	c := novoCenarioDownload(t, exeCarimbado(t, Versao{0, 5, 0}))
	b, err := c.baixar(context.Background(), nil)
	if err != nil {
		t.Fatalf("Baixar: %v", err)
	}
	quer := Baixado{
		Caminho: filepath.Join(c.pasta, nomeDownload),
		Asset:   c.rel.Exe,
		Versao:  Versao{0, 5, 0},
		SHA256:  sha256.Sum256(c.exe),
	}
	if b != quer {
		t.Fatalf("Baixado = %+v, quer %+v", b, quer)
	}
	gravado, err := os.ReadFile(b.Caminho)
	if err != nil || !bytes.Equal(gravado, c.exe) {
		t.Fatalf("arquivo gravado difere do servido (err %v)", err)
	}
	// Fechado: no Windows, renomear um arquivo aberto sem FILE_SHARE_DELETE falha.
	if err := os.Rename(b.Caminho, filepath.Join(c.pasta, "eprocia.exe")); err != nil {
		t.Fatalf("arquivo baixado não renomeável (ficou aberto?): %v", err)
	}
}

func TestBaixar_SHAErradoApaga(t *testing.T) {
	c := novoCenarioDownload(t, exeCarimbado(t, Versao{0, 5, 0}))
	c.somas = somasPara([]byte("outro conteúdo"), nomeExe)
	c.rel.Somas.Tamanho = int64(len(c.somas))
	_, err := c.baixar(context.Background(), nil)
	if !errors.Is(err, ErrNaoConferiu) {
		t.Fatalf("erro = %v, quer ErrNaoConferiu", err)
	}
	pastaVazia(t, c.pasta)
}

func TestBaixar_RecalculoPegaAlteracao(t *testing.T) {
	c := novoCenarioDownload(t, exeCarimbado(t, Versao{0, 5, 0}))
	chamado := false
	c.cliente.depoisDeCopiar = func(caminho string) {
		chamado = true
		f, err := os.OpenFile(caminho, os.O_RDWR, 0)
		if err != nil {
			t.Errorf("abrir para alterar: %v", err)
			return
		}
		defer f.Close()
		ultimo := []byte{c.exe[len(c.exe)-1] ^ 0xff}
		if _, err := f.WriteAt(ultimo, int64(len(c.exe)-1)); err != nil {
			t.Errorf("alterar: %v", err)
		}
	}
	_, err := c.baixar(context.Background(), nil)
	if !chamado {
		t.Fatal("depoisDeCopiar não foi chamado")
	}
	if !errors.Is(err, ErrNaoConferiu) {
		t.Fatalf("erro = %v, quer ErrNaoConferiu", err)
	}
	pastaVazia(t, c.pasta)
}

func TestBaixar_ContentLengthAcimaDoTeto(t *testing.T) {
	c := novoCenarioDownload(t, exeCarimbado(t, Versao{0, 5, 0}))
	c.servirExe = func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Length", strconv.Itoa(LimiteAsset+1))
		w.WriteHeader(http.StatusOK)
		w.(http.Flusher).Flush()
	}
	gravou := false
	_, err := c.baixar(context.Background(), func(feito, total int64) {
		if feito > 0 {
			gravou = true
		}
	})
	if err == nil || errors.Is(err, ErrNaoConferiu) || !strings.Contains(err.Error(), "teto") {
		t.Fatalf("erro = %v, quer recusa pelo teto", err)
	}
	if gravou {
		t.Fatal("gravou bytes antes de recusar o Content-Length")
	}
	pastaVazia(t, c.pasta)
}

func TestBaixar_AssetAcimaDoTetoSemRequisicao(t *testing.T) {
	c := novoCenarioDownload(t, exeCarimbado(t, Versao{0, 5, 0}))
	c.cliente.limiteAsset = int64(len(c.exe) - 1)
	_, err := c.baixar(context.Background(), nil)
	if err == nil || !strings.Contains(err.Error(), "teto") {
		t.Fatalf("erro = %v, quer recusa pelo teto", err)
	}
	if n := c.srv.n.Load(); n != 0 {
		t.Fatalf("servidor recebeu %d requisições", n)
	}
	pastaVazia(t, c.pasta)
}

func TestBaixar_CorpoMaiorQueOAnunciado(t *testing.T) {
	c := novoCenarioDownload(t, exeCarimbado(t, Versao{0, 5, 0}))
	// Sem Content-Length (chunked): só a contagem pega o excesso.
	c.servirExe = func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.(http.Flusher).Flush()
		_, _ = w.Write(c.exe)
		_, _ = w.Write([]byte("bytes a mais"))
	}
	_, err := c.baixar(context.Background(), nil)
	if err == nil {
		t.Fatal("aceitou corpo maior que o tamanho do asset")
	}
	pastaVazia(t, c.pasta)
}

func TestBaixar_TamanhoDiferenteDoAsset(t *testing.T) {
	t.Run("Content-Length diferente", func(t *testing.T) {
		c := novoCenarioDownload(t, exeCarimbado(t, Versao{0, 5, 0}))
		c.rel.Exe.Tamanho += 10
		_, err := c.cliente.Baixar(context.Background(), c.rel, c.rel.Exe, c.pasta, nomeDownload, nil)
		if err == nil {
			t.Fatal("aceitou Content-Length diferente do tamanho do asset")
		}
		pastaVazia(t, c.pasta)
	})
	t.Run("corpo menor sem Content-Length", func(t *testing.T) {
		c := novoCenarioDownload(t, exeCarimbado(t, Versao{0, 5, 0}))
		c.servirExe = func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			w.(http.Flusher).Flush()
			_, _ = w.Write(c.exe[:len(c.exe)-10])
		}
		_, err := c.baixar(context.Background(), nil)
		if err == nil {
			t.Fatal("aceitou corpo menor que o tamanho do asset")
		}
		pastaVazia(t, c.pasta)
	})
}

func TestBaixar_CanceladoApaga(t *testing.T) {
	c := novoCenarioDownload(t, exeCarimbado(t, Versao{0, 5, 0}))
	ctx, cancelar := context.WithCancel(context.Background())
	defer cancelar()
	chamadas := 0
	_, err := c.baixar(ctx, func(feito, total int64) {
		chamadas++
		cancelar()
	})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("erro = %v, quer context.Canceled", err)
	}
	if chamadas != 1 {
		t.Fatalf("progresso chamado %d vezes depois de cancelar, quer 1", chamadas)
	}
	pastaVazia(t, c.pasta)
}

func TestBaixar_ParadoAbortaEApaga(t *testing.T) {
	c := novoCenarioDownload(t, exeCarimbado(t, Versao{0, 5, 0}))
	c.cliente.prazoSemDados = 300 * time.Millisecond
	c.servirExe = func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Length", strconv.Itoa(len(c.exe)))
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(c.exe[:len(c.exe)/2])
		w.(http.Flusher).Flush()
		select {
		case <-r.Context().Done():
		case <-c.parar:
		}
	}
	inicio := time.Now()
	_, err := c.baixar(context.Background(), nil)
	if !errors.Is(err, ErrParado) {
		t.Fatalf("erro = %v, quer ErrParado", err)
	}
	if d := time.Since(inicio); d > 10*time.Second {
		t.Fatalf("demorou %v para abortar", d)
	}
	pastaVazia(t, c.pasta)
}

func TestBaixar_NaoPEOuVersaoErrada(t *testing.T) {
	t.Run("sem MZ", func(t *testing.T) {
		corpo := bytes.Repeat([]byte("eprocIA "), 1000)
		c := novoCenarioDownload(t, corpo)
		_, err := c.baixar(context.Background(), nil)
		if !errors.Is(err, ErrNaoConferiu) {
			t.Fatalf("erro = %v, quer ErrNaoConferiu", err)
		}
		pastaVazia(t, c.pasta)
	})
	t.Run("MZ sem recurso de versão", func(t *testing.T) {
		corpo := append([]byte("MZ"), bytes.Repeat([]byte{0}, 4096)...)
		c := novoCenarioDownload(t, corpo)
		_, err := c.baixar(context.Background(), nil)
		if !errors.Is(err, ErrNaoConferiu) {
			t.Fatalf("erro = %v, quer ErrNaoConferiu", err)
		}
		pastaVazia(t, c.pasta)
	})
	t.Run("versão 0.4.9 em release v0.5.0", func(t *testing.T) {
		c := novoCenarioDownload(t, exeCarimbado(t, Versao{0, 4, 9}))
		_, err := c.baixar(context.Background(), nil)
		if !errors.Is(err, ErrNaoConferiu) {
			t.Fatalf("erro = %v, quer ErrNaoConferiu", err)
		}
		pastaVazia(t, c.pasta)
	})
}

func TestBaixar_ProgressoCrescente(t *testing.T) {
	c := novoCenarioDownload(t, exeCarimbado(t, Versao{0, 5, 0}))
	var feitos []int64
	_, err := c.baixar(context.Background(), func(feito, total int64) {
		if total != int64(len(c.exe)) {
			t.Errorf("total = %d, quer %d", total, len(c.exe))
		}
		feitos = append(feitos, feito)
	})
	if err != nil {
		t.Fatalf("Baixar: %v", err)
	}
	if len(feitos) == 0 {
		t.Fatal("progresso nunca chamado")
	}
	for i := 1; i < len(feitos); i++ {
		if feitos[i] < feitos[i-1] {
			t.Fatalf("progresso decresceu: %d depois de %d", feitos[i], feitos[i-1])
		}
	}
	if ultimo := feitos[len(feitos)-1]; ultimo != int64(len(c.exe)) {
		t.Fatalf("último progresso = %d, quer %d", ultimo, len(c.exe))
	}
}

func TestBaixar_NomeJaExistente(t *testing.T) {
	c := novoCenarioDownload(t, exeCarimbado(t, Versao{0, 5, 0}))
	caminho := filepath.Join(c.pasta, nomeDownload)
	if err := os.WriteFile(caminho, []byte("antigo"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := c.baixar(context.Background(), nil)
	if !errors.Is(err, fs.ErrExist) {
		t.Fatalf("erro = %v, quer fs.ErrExist", err)
	}
	if dados, _ := os.ReadFile(caminho); string(dados) != "antigo" {
		t.Fatalf("arquivo existente foi sobrescrito: %q", dados)
	}
}

func TestBaixar_URLDiferenteDaEsperada(t *testing.T) {
	casos := []struct {
		nome   string
		mexer  func(c *cenarioDownload)
		eAsset bool
	}{
		{"asset de outra tag", func(c *cenarioDownload) {
			c.rel.Exe.URL = strings.Replace(c.rel.Exe.URL, tagDownload, "v0.4.9", 1)
		}, true},
		{"asset com outro nome na URL", func(c *cenarioDownload) {
			c.rel.Exe.URL = strings.Replace(c.rel.Exe.URL, nomeExe, nomeInst, 1)
		}, true},
		{"asset fora do prefixo", func(c *cenarioDownload) {
			c.rel.Exe.URL = strings.Replace(c.rel.Exe.URL, "/download/", "/outro/", 1)
		}, true},
		{"somas de outra tag", func(c *cenarioDownload) {
			c.rel.Somas.URL = strings.Replace(c.rel.Somas.URL, tagDownload, "v0.4.9", 1)
		}, false},
		{"somas com outro nome", func(c *cenarioDownload) {
			c.rel.Somas.Nome = "OUTRO.txt"
			c.rel.Somas.URL = strings.Replace(c.rel.Somas.URL, nomeSomas, "OUTRO.txt", 1)
		}, false},
	}
	for _, cs := range casos {
		t.Run(cs.nome, func(t *testing.T) {
			c := novoCenarioDownload(t, exeCarimbado(t, Versao{0, 5, 0}))
			cs.mexer(c)
			_, err := c.cliente.Baixar(context.Background(), c.rel, c.rel.Exe, c.pasta, nomeDownload, nil)
			if !errors.Is(err, ErrURLRecusada) {
				t.Fatalf("erro = %v, quer ErrURLRecusada", err)
			}
			if n := c.srv.n.Load(); n != 0 {
				t.Fatalf("servidor recebeu %d requisições", n)
			}
			pastaVazia(t, c.pasta)
		})
	}
}

func TestBaixar_SomasAcimaDoTeto(t *testing.T) {
	c := novoCenarioDownload(t, exeCarimbado(t, Versao{0, 5, 0}))
	c.somas = append(bytes.Repeat([]byte("#"), LimiteSomas), c.somas...)
	_, err := c.baixar(context.Background(), nil)
	if err == nil || !strings.Contains(err.Error(), "teto") {
		t.Fatalf("erro = %v, quer recusa pelo teto", err)
	}
	pastaVazia(t, c.pasta)
}
