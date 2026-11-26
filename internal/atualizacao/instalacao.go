package atualizacao

import (
	"os"
	"path/filepath"
	"strings"
)

// Modo diz como a atualização pode ser aplicada nesta instalação.
type Modo string

const (
	// ModoTrocar: instalação por usuário (install.ps1); o próprio app troca o exe.
	ModoTrocar Modo = "trocar"
	// ModoInstalador: instalação NSIS por máquina; a troca passa pelo instalador.
	ModoInstalador Modo = "instalador"
	// ModoSoAvisar: só avisa que há versão nova (Motivo diz por quê).
	ModoSoAvisar Modo = "so_avisar"
)

// Motivos de ModoSoAvisar.
const (
	MotivoForaDaPasta = "executável fora da pasta de instalação"
	MotivoSemEscrita  = "pasta de instalação sem permissão de escrita"
)

// Instalacao é o resultado de Detectar.
type Instalacao struct {
	Modo   Modo
	Pasta  string
	Motivo string
}

// Registro lê, só para consulta, as chaves de desinstalação do eprocIA.
// Erro conta como chave ou valor ausente.
type Registro interface {
	// InstallLocationUsuario é o InstallLocation gravado pelo install.ps1 em HKCU.
	InstallLocationUsuario() (string, error)
	// DisplayIconMaquina é o DisplayIcon gravado pelo instalador NSIS em HKLM.
	DisplayIconMaquina() (string, error)
}

// Detectar decide o modo de atualização do executável exe (caminho já
// resolvido): a pasta dele igual ao InstallLocation de HKCU dá ModoTrocar (ou
// ModoSoAvisar sem permissão de escrita); o exe igual ao DisplayIcon de HKLM
// dá ModoInstalador; o resto, ModoSoAvisar.
func Detectar(exe string, reg Registro, podeEscrever func(pasta string) bool) Instalacao {
	pasta := filepath.Dir(exe)
	if local, err := reg.InstallLocationUsuario(); err == nil && strings.TrimSpace(local) != "" && mesmoCaminho(local, pasta) {
		if !podeEscrever(pasta) {
			return Instalacao{Modo: ModoSoAvisar, Pasta: pasta, Motivo: MotivoSemEscrita}
		}
		return Instalacao{Modo: ModoTrocar, Pasta: pasta}
	}
	if icone, err := reg.DisplayIconMaquina(); err == nil {
		if alvo := limparIcone(icone); alvo != "" && mesmoCaminho(alvo, exe) {
			return Instalacao{Modo: ModoInstalador, Pasta: pasta}
		}
	}
	return Instalacao{Modo: ModoSoAvisar, Pasta: pasta, Motivo: MotivoForaDaPasta}
}

// mesmoCaminho diz se o caminho lido do registro (doRegistro) e o do exe
// (resolvido) são o mesmo, sem diferenciar caixa (acentos incluídos) nem a
// barra final. O install.ps1 grava o -Dir como digitado, então doRegistro é
// resolvido antes (relativo, nome 8.3, link simbólico) e, se ainda diferir,
// os dois são comparados como arquivo: EvalSymlinks não segue junções desde
// o Go 1.23, os.SameFile segue.
func mesmoCaminho(doRegistro, resolvido string) bool {
	a := resolverCaminho(strings.TrimSpace(doRegistro))
	b := filepath.Clean(strings.TrimSpace(resolvido))
	if strings.EqualFold(a, b) {
		return true
	}
	ia, errA := os.Stat(a)
	ib, errB := os.Stat(b)
	return errA == nil && errB == nil && os.SameFile(ia, ib)
}

// resolverCaminho torna p absoluto e sem links; em erro, devolve p limpo.
func resolverCaminho(p string) string {
	abs, err := filepath.Abs(p)
	if err != nil {
		return filepath.Clean(p)
	}
	if r, err := filepath.EvalSymlinks(abs); err == nil {
		return r
	}
	return abs
}

// limparIcone tira de um DisplayIcon o índice do ícone (",0") e as aspas.
func limparIcone(v string) string {
	v = strings.TrimSpace(v)
	if i := strings.LastIndexByte(v, ','); i >= 0 && soIndice(v[i+1:]) {
		v = strings.TrimSpace(v[:i])
	}
	return strings.TrimSpace(strings.Trim(v, `"`))
}

func soIndice(s string) bool {
	s = strings.TrimPrefix(strings.TrimSpace(s), "-")
	if s == "" {
		return false
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

// PodeEscrever cria e apaga um arquivo de nome aleatório em pasta.
func PodeEscrever(pasta string) bool {
	f, err := os.CreateTemp(pasta, ".eprocia-escrita-*")
	if err != nil {
		return false
	}
	nome := f.Name()
	errFechar := f.Close()
	errApagar := os.Remove(nome)
	return errFechar == nil && errApagar == nil
}
