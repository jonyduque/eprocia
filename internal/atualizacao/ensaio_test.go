package atualizacao

import (
	"fmt"
	"os"
	"regexp"
	"strings"
	"testing"
)

var palavraEnsaio = regexp.MustCompile(`(?i)\bensaio\b`)

// problemasReleaseYML lista o que levaria a tag ensaio ao build publicado:
// -tags em qualquer linha, GOFLAGS, ou a palavra "ensaio" fora de comentário
// (os comentários do release.yml já chamam a rc de "ensaio"). Sem nenhuma
// linha com "wails build" também é problema.
func problemasReleaseYML(texto string) []string {
	var problemas []string
	achou := 0
	n := 0
	for linha := range strings.Lines(texto) {
		n++
		codigo := strings.TrimSpace(linha)
		if strings.HasPrefix(codigo, "#") {
			continue
		}
		if strings.Contains(codigo, "wails build") {
			achou++
		}
		// Em qualquer linha: uma continuação (\) leva o -tags a outra linha
		// que não a do "wails build". O gatilho "tags: ['v*']" não tem hífen.
		if strings.Contains(codigo, "-tags") {
			problemas = append(problemas, fmt.Sprintf("linha %d: -tags", n))
		}
		if strings.Contains(codigo, "GOFLAGS") {
			problemas = append(problemas, fmt.Sprintf("linha %d: GOFLAGS", n))
		}
		if palavraEnsaio.MatchString(codigo) {
			problemas = append(problemas, fmt.Sprintf("linha %d: palavra ensaio", n))
		}
	}
	if achou == 0 {
		problemas = append(problemas, "nenhuma linha com wails build")
	}
	return problemas
}

// TestEnsaio_ReleaseSemTags garante que o build publicado não leva a tag
// ensaio.
func TestEnsaio_ReleaseSemTags(t *testing.T) {
	dados, err := os.ReadFile("../../.github/workflows/release.yml")
	if err != nil {
		t.Fatalf("ler release.yml: %v", err)
	}
	if p := problemasReleaseYML(string(dados)); len(p) > 0 {
		t.Fatalf("release.yml: %v", p)
	}
}

func TestEnsaio_GuardaDoReleaseYML(t *testing.T) {
	// O gatilho "tags: ['v*']" não é -tags.
	const bom = "on:\n  push:\n    tags: ['v*']\n# rc serve de ensaio\n      - name: wails build\n        run: wails build -clean -s -nsis -trimpath\n"
	if p := problemasReleaseYML(bom); len(p) > 0 {
		t.Fatalf("texto bom recusado: %v", p)
	}
	for nome, texto := range map[string]string{
		"sem wails build":      "run: go build\n",
		"-tags":                "run: wails build -tags ensaio\n",
		"-tags na continuação": "run: |\n  wails build -clean \\\n    -tags x\n",
		"GOFLAGS":              "env:\n  GOFLAGS: -tags=x\nrun: wails build\n",
		"palavra ensaio":       "run: wails build\nenv:\n  MODO: Ensaio\n",
	} {
		if p := problemasReleaseYML(texto); len(p) == 0 {
			t.Errorf("%s: aceito", nome)
		}
	}
}
