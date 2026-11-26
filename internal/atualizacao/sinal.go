package atualizacao

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// Nomes, prefixos e flag do contrato entre versões: a versão nova conversa
// com a antiga só por eles, então não mudam sem pensar nas duas pontas.
const (
	NomeExe           = "eprocia.exe"
	NomeAnterior      = "eprocia.anterior.exe"
	NomeFalhou        = "eprocia.falhou.exe"
	PrefixoTemp       = "eprocia.exe.download-"
	PrefixoSinal      = "atualizacao-sinal-"
	FlagAposAtualizar = "--apos-atualizar="
	// FlagLimparAnterior pede ao processo novo que apague o
	// eprocia.anterior.exe depois de o antigo sair.
	FlagLimparAnterior = "--limpar-anterior"
	// PrazoSinal é quanto o processo antigo espera o sinal de vida do novo.
	PrazoSinal = 90 * time.Second
	// PrazoAntigoSair é quanto o processo novo espera o antigo sair.
	PrazoAntigoSair = 60 * time.Second
)

// CaminhoSinal é o arquivo em que o processo novo avisa ao antigo (pidAntigo)
// que abriu.
func CaminhoSinal(pastaDados string, pidAntigo int) string {
	return filepath.Join(pastaDados, PrefixoSinal+strconv.Itoa(pidAntigo))
}

// GravarSinal grava a versão do processo novo no sinal do antigo. Grava num
// temporário da mesma pasta e renomeia, para o antigo nunca ler meio arquivo;
// o temporário tem o prefixo do sinal, então a limpeza o apaga se sobrar.
func GravarSinal(pastaDados string, pidAntigo int, versao string) error {
	if err := os.MkdirAll(pastaDados, 0o755); err != nil {
		return err
	}
	destino := CaminhoSinal(pastaDados, pidAntigo)
	tmp, err := os.CreateTemp(pastaDados, filepath.Base(destino)+".tmp-*")
	if err != nil {
		return err
	}
	nomeTmp := tmp.Name()
	if _, err := tmp.WriteString(versao); err != nil {
		tmp.Close()
		os.Remove(nomeTmp)
		return err
	}
	if err := tmp.Close(); err != nil {
		os.Remove(nomeTmp)
		return err
	}
	if err := os.Rename(nomeTmp, destino); err != nil {
		os.Remove(nomeTmp)
		return err
	}
	return nil
}

// lerSinal lê a versão gravada no sinal e apaga o arquivo. ok é falso sem
// arquivo ou com arquivo vazio.
func lerSinal(caminho string) (versao string, ok bool) {
	dados, err := os.ReadFile(caminho)
	if err != nil {
		return "", false
	}
	_ = os.Remove(caminho)
	versao = strings.TrimSpace(string(dados))
	return versao, versao != ""
}
