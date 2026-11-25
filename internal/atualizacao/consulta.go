package atualizacao

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
	"time"

	"eprocia/internal/registro"
)

// Asset é um arquivo anexado à release.
type Asset struct {
	Nome, URL string
	Tamanho   int64
}

// Release é a release disponível. Com ErrIncompleta, só Tag, Versao e
// NotasURL vêm preenchidos.
type Release struct {
	Tag      string
	Versao   Versao
	NotasURL string

	Exe, Instalador, Somas Asset
}

var (
	// ErrIncompleta: a release é mais nova, mas falta ou sobra asset, ou um
	// asset não está onde deveria.
	ErrIncompleta = errors.New("release incompleta")
	// ErrRecusada: o GitHub respondeu 403 ou 429 (limite de consultas).
	ErrRecusada = errors.New("o GitHub recusou a consulta")
)

const (
	// PrazoConsulta é o prazo total de uma consulta, corpo incluído.
	PrazoConsulta = 15 * time.Second
	// LimiteAPI é o maior corpo aceito da API.
	LimiteAPI = 1 << 20
)

const (
	nomeSomas  = "SHA256SUMS.txt"
	acceptJSON = "application/vnd.github+json"
)

// rótulo é o que vem entre "eprocia-" e "-windows-amd64" no nome dos assets:
// X.Y.Z em produção, X.Y.Z-rcN no ensaio.
const (
	rotuloProducao = `\d+\.\d+\.\d+`
	rotuloEnsaio   = `\d+\.\d+\.\d+-rc\d+`
)

var (
	exeProducao        = regexp.MustCompile(`^eprocia-(` + rotuloProducao + `)-windows-amd64\.exe$`)
	instaladorProducao = regexp.MustCompile(`^eprocia-(` + rotuloProducao + `)-windows-amd64-instalador\.exe$`)
	exeEnsaio          = regexp.MustCompile(`^eprocia-(` + rotuloEnsaio + `)-windows-amd64\.exe$`)
	instaladorEnsaio   = regexp.MustCompile(`^eprocia-(` + rotuloEnsaio + `)-windows-amd64-instalador\.exe$`)
	tagRC              = regexp.MustCompile(`^v(\d+\.\d+\.\d+)-rc\d+$`)
)

type assetAPI struct {
	Name string `json:"name"`
	URL  string `json:"browser_download_url"`
	Size int64  `json:"size"`
}

type releaseAPI struct {
	TagName    string     `json:"tag_name"`
	Draft      bool       `json:"draft"`
	Prerelease bool       `json:"prerelease"`
	HTMLURL    string     `json:"html_url"`
	Assets     []assetAPI `json:"assets"`
}

// Verificar consulta a última release e devolve (nil, nil) quando não há
// versão mais nova que atual.
func (c *Cliente) Verificar(ctx context.Context, atual Versao) (*Release, error) {
	rel, err := c.verificar(ctx, atual)
	resultado := "em_dia"
	switch {
	case errors.Is(err, ErrIncompleta):
		resultado = "incompleta"
	case err != nil:
		resultado = "erro"
	case rel != nil:
		resultado = "disponivel"
	}
	registro.Logger(ctx).Info("consulta de atualização", "resultado", resultado)
	return rel, err
}

func (c *Cliente) verificar(ctx context.Context, atual Versao) (*Release, error) {
	prazo := c.prazoConsulta
	if prazo <= 0 {
		prazo = PrazoConsulta
	}
	ctx, cancelar := context.WithTimeout(ctx, prazo)
	defer cancelar()

	endereco := c.regras.PrefixoAPI + "releases/latest"
	tagRCEnsaio, ensaio := tagEnsaio()
	if ensaio {
		endereco = c.regras.PrefixoAPI + "releases/tags/" + tagRCEnsaio
	}

	api, err := c.lerRelease(ctx, endereco)
	if err != nil {
		return nil, err
	}
	if api.Draft {
		return nil, nil
	}

	var (
		versao              Versao
		rotulo              string
		reExe, reInstalador *regexp.Regexp
	)
	if ensaio {
		m := tagRC.FindStringSubmatch(api.TagName)
		if api.TagName != tagRCEnsaio || m == nil {
			return nil, nil
		}
		if versao, err = ParseVersao(m[1]); err != nil {
			return nil, nil
		}
		rotulo = strings.TrimPrefix(api.TagName, "v")
		reExe, reInstalador = exeEnsaio, instaladorEnsaio
	} else {
		if api.Prerelease {
			return nil, nil
		}
		if versao, err = ParseTag(api.TagName); err != nil {
			return nil, nil
		}
		rotulo = versao.String()
		reExe, reInstalador = exeProducao, instaladorProducao
	}
	if versao.Comparar(atual) <= 0 {
		return nil, nil
	}

	rel := &Release{Tag: api.TagName, Versao: versao, NotasURL: api.HTMLURL}
	exe, inst, somas, err := c.conferirAssets(api, rotulo, reExe, reInstalador)
	if err != nil {
		return rel, err
	}
	rel.Exe, rel.Instalador, rel.Somas = exe, inst, somas
	return rel, nil
}

// lerRelease faz o GET e decodifica o corpo, limitado a LimiteAPI.
func (c *Cliente) lerRelease(ctx context.Context, endereco string) (*releaseAPI, error) {
	resp, err := c.obter(ctx, endereco, urlAPI, acceptJSON)
	if err != nil {
		return nil, fmt.Errorf("consultar release: %w", err)
	}
	defer func() {
		_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, LimiteAPI))
		_ = resp.Body.Close()
	}()
	switch resp.StatusCode {
	case http.StatusOK:
	case http.StatusForbidden, http.StatusTooManyRequests:
		return nil, fmt.Errorf("%w (HTTP %d)", ErrRecusada, resp.StatusCode)
	default:
		return nil, fmt.Errorf("consultar release: HTTP %d", resp.StatusCode)
	}
	corpo, err := io.ReadAll(io.LimitReader(resp.Body, LimiteAPI+1))
	if err != nil {
		return nil, fmt.Errorf("ler resposta: %w", err)
	}
	if len(corpo) > LimiteAPI {
		return nil, errors.New("resposta da consulta acima do limite")
	}
	var api releaseAPI
	if err := json.Unmarshal(corpo, &api); err != nil {
		return nil, fmt.Errorf("resposta da consulta inválida: %w", err)
	}
	return &api, nil
}

// conferirAssets exige exatamente um exe, um instalador e um SHA256SUMS.txt,
// com o rótulo da tag no nome e a URL exata do prefixo de download.
func (c *Cliente) conferirAssets(api *releaseAPI, rotulo string, reExe, reInstalador *regexp.Regexp) (exe, inst, somas Asset, err error) {
	var exes, insts, somass []Asset
	for _, a := range api.Assets {
		asset := Asset{Nome: a.Name, URL: a.URL, Tamanho: a.Size}
		var alvo *[]Asset
		var rotuloNome string
		if m := reExe.FindStringSubmatch(a.Name); m != nil {
			alvo, rotuloNome = &exes, m[1]
		} else if m := reInstalador.FindStringSubmatch(a.Name); m != nil {
			alvo, rotuloNome = &insts, m[1]
		} else if a.Name == nomeSomas {
			alvo, rotuloNome = &somass, rotulo
		} else {
			continue
		}
		*alvo = append(*alvo, asset)
		if rotuloNome != rotulo {
			return Asset{}, Asset{}, Asset{}, fmt.Errorf("%w: versão do asset diferente da tag", ErrIncompleta)
		}
		if a.URL != c.regras.PrefixoDownload+api.TagName+"/"+a.Name {
			return Asset{}, Asset{}, Asset{}, fmt.Errorf("%w: endereço de asset fora do esperado", ErrIncompleta)
		}
	}
	if len(exes) != 1 || len(insts) != 1 || len(somass) != 1 {
		return Asset{}, Asset{}, Asset{}, fmt.Errorf("%w: assets faltando ou repetidos", ErrIncompleta)
	}
	return exes[0], insts[0], somass[0], nil
}
