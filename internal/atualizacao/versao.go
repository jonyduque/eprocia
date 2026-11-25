package atualizacao

import (
	"cmp"
	"errors"
	"fmt"
	"strconv"
	"strings"
)

// Versao é uma versão X.Y.Z.
type Versao [3]int

// ParseVersao lê só "X.Y.Z" decimal: sem sufixo, sem "v", sem espaço.
func ParseVersao(s string) (Versao, error) {
	partes := strings.Split(s, ".")
	if len(partes) != 3 {
		return Versao{}, errors.New("versão deve ter o formato X.Y.Z")
	}
	var v Versao
	for i, p := range partes {
		if p == "" {
			return Versao{}, errors.New("parte da versão vazia")
		}
		for _, c := range p {
			if c < '0' || c > '9' {
				return Versao{}, errors.New("parte da versão não é decimal")
			}
		}
		n, err := strconv.Atoi(p)
		if err != nil {
			return Versao{}, fmt.Errorf("parte da versão inválida: %w", err)
		}
		v[i] = n
	}
	return v, nil
}

// ParseTag exige "v" minúsculo seguido de X.Y.Z.
func ParseTag(tag string) (Versao, error) {
	resto, ok := strings.CutPrefix(tag, "v")
	if !ok {
		return Versao{}, errors.New("tag deve começar com v")
	}
	return ParseVersao(resto)
}

// Comparar devolve -1, 0 ou 1 (comparação numérica parte a parte).
func (v Versao) Comparar(o Versao) int {
	for i := range v {
		if c := cmp.Compare(v[i], o[i]); c != 0 {
			return c
		}
	}
	return 0
}

// Valida é falso para 0.0.0.
func (v Versao) Valida() bool { return v != Versao{} }

func (v Versao) String() string { return fmt.Sprintf("%d.%d.%d", v[0], v[1], v[2]) }
