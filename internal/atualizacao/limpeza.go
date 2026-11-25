package atualizacao

import (
	"errors"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
)

// PastaAtualizacao é a pasta de trabalho da atualização em %LOCALAPPDATA%.
func PastaAtualizacao(localAppData string) string {
	return filepath.Join(localAppData, "eprocIA", "atualizacao")
}

// Limpar apaga as sobras de uma troca: eprocia.falhou.exe e os temporários de
// download na instalação, os sinais na pasta de dados e o conteúdo da pasta
// de atualização. Mantém eprocia.anterior.exe (é o "Voltar"). Erros são
// ignorados, com um aviso no log por arquivo que ficou.
func Limpar(pastaInstalacao, pastaDados, pastaAtualizacao string) {
	apagarSe(pastaInstalacao, func(nome string) bool {
		return strings.EqualFold(nome, NomeFalhou) || temPrefixo(nome, PrefixoTemp)
	})
	apagarSe(pastaDados, func(nome string) bool { return temPrefixo(nome, PrefixoSinal) })
	// A pasta de atualização perde todo o conteúdo: se ela for uma junção ou
	// um link, apagar o conteúdo seria apagar o alvo.
	if info, err := os.Lstat(pastaAtualizacao); err == nil && info.Mode()&(fs.ModeSymlink|fs.ModeIrregular) != 0 {
		slog.Default().Warn("limpeza da atualização: pasta de atualização é link ou junção; não limpa")
		return
	}
	apagarSe(pastaAtualizacao, func(string) bool { return true })
}

// temPrefixo compara sem caixa: o sistema de arquivos do Windows também.
func temPrefixo(nome, prefixo string) bool {
	return len(nome) >= len(prefixo) && strings.EqualFold(nome[:len(prefixo)], prefixo)
}

func apagarSe(pasta string, apagar func(nome string) bool) {
	if pasta == "" {
		return
	}
	entradas, err := os.ReadDir(pasta)
	if err != nil {
		if !errors.Is(err, fs.ErrNotExist) {
			slog.Default().Warn("limpeza da atualização: pasta não lida", "erro", err)
		}
		return
	}
	for _, e := range entradas {
		if !apagar(e.Name()) {
			continue
		}
		if err := os.RemoveAll(filepath.Join(pasta, e.Name())); err != nil {
			slog.Default().Warn("limpeza da atualização: arquivo ficou", "arquivo", e.Name(), "erro", err)
		}
	}
}
