//go:build !ensaio

package atualizacao

// No build de produção o ensaio não existe: nenhuma variável de ambiente é
// lida e a consulta vai sempre a releases/latest.

func tagEnsaio() (string, bool) { return "", false }

func ensaioIgnoraSinal() bool { return false }
