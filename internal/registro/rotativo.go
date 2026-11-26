package registro

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// ArquivoRotativo é um arquivo de log que gira por tamanho: o ativo é
// <base><ext>; ao passar do limite ele vira <base>.1<ext>, os anteriores se
// deslocam e o de número maior que arquivados é apagado. Não é seguro para
// uso concorrente: só a goroutine gravadora o usa.
type ArquivoRotativo struct {
	dir, base, ext string
	limite         int64
	arquivados     int

	f   *os.File
	tam int64

	// renomear é os.Rename; os testes trocam por uma função que falha.
	renomear func(de, para string) error
	// aoFalharGiro, se definido, é chamado na primeira falha de giro de uma
	// sequência (uma falha seguida de outras não avisa de novo).
	aoFalharGiro func(error)
	falhouGiro   bool
}

// AbrirRotativo abre (ou cria) <dir>\<base><ext> para acréscimo.
func AbrirRotativo(dir, base, ext string, limite int64, arquivados int) (*ArquivoRotativo, error) {
	a := &ArquivoRotativo{dir: dir, base: base, ext: ext, limite: limite, arquivados: arquivados, renomear: os.Rename}
	if err := a.abrir(); err != nil {
		return nil, err
	}
	return a, nil
}

func (a *ArquivoRotativo) nome(n int) string {
	if n == 0 {
		return filepath.Join(a.dir, a.base+a.ext)
	}
	return filepath.Join(a.dir, fmt.Sprintf("%s.%d%s", a.base, n, a.ext))
}

func (a *ArquivoRotativo) abrir() error {
	f, err := os.OpenFile(a.nome(0), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		return err
	}
	info, err := f.Stat()
	if err != nil {
		f.Close()
		return err
	}
	a.f, a.tam = f, info.Size()
	return nil
}

// Write grava p no arquivo ativo. Se p faria o arquivo passar do limite, gira
// antes; um giro que falha não impede a escrita, que segue no arquivo atual,
// e a próxima escrita além do limite tenta de novo.
func (a *ArquivoRotativo) Write(p []byte) (int, error) {
	if a.f == nil {
		if err := a.abrir(); err != nil {
			return 0, err
		}
	}
	if a.tam > 0 && a.tam+int64(len(p)) > a.limite {
		if err := a.girar(); err != nil {
			if !a.falhouGiro && a.aoFalharGiro != nil {
				a.aoFalharGiro(err)
			}
			a.falhouGiro = true
		} else {
			a.falhouGiro = false
		}
		if a.f == nil {
			if err := a.abrir(); err != nil {
				return 0, err
			}
		}
	}
	n, err := a.f.Write(p)
	a.tam += int64(n)
	return n, err
}

// girar fecha o ativo (o Windows não renomeia arquivo aberto), desloca os
// arquivados e reabre um ativo vazio. Em falha de renomear, reabre o ativo
// atual para acréscimo.
func (a *ArquivoRotativo) girar() error {
	if err := a.f.Close(); err != nil {
		a.f = nil
		return err
	}
	a.f = nil
	var errGiro error
	if a.arquivados <= 0 {
		errGiro = os.Remove(a.nome(0))
	} else {
		if err := os.Remove(a.nome(a.arquivados)); err != nil && !errors.Is(err, os.ErrNotExist) {
			errGiro = err
		}
		for i := a.arquivados - 1; i >= 1 && errGiro == nil; i-- {
			if _, err := os.Stat(a.nome(i)); errors.Is(err, os.ErrNotExist) {
				continue
			}
			errGiro = a.renomear(a.nome(i), a.nome(i+1))
		}
		if errGiro == nil {
			errGiro = a.renomear(a.nome(0), a.nome(1))
		}
	}
	if err := a.abrir(); err != nil {
		return errors.Join(errGiro, err)
	}
	return errGiro
}

// Close fecha o arquivo ativo.
func (a *ArquivoRotativo) Close() error {
	if a.f == nil {
		return nil
	}
	err := a.f.Close()
	a.f = nil
	return err
}

// ApagarAntigos apaga, em dir, os arquivos eprocia*.log modificados há mais
// de idade. Uma pasta que não existe não é erro; os demais arquivos ficam.
func ApagarAntigos(dir string, idade time.Duration, agora time.Time) error {
	ents, err := os.ReadDir(dir)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return err
	}
	var errs []error
	for _, e := range ents {
		nome := strings.ToLower(e.Name())
		if !e.Type().IsRegular() || !strings.HasPrefix(nome, "eprocia") || !strings.HasSuffix(nome, ".log") {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		if agora.Sub(info.ModTime()) > idade {
			if err := os.Remove(filepath.Join(dir, e.Name())); err != nil && !errors.Is(err, os.ErrNotExist) {
				errs = append(errs, err)
			}
		}
	}
	return errors.Join(errs...)
}
