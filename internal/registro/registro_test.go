package registro

import (
	"context"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
	"testing"
)

func TestIniciar_ConcorrenteNaoVazaGravador(t *testing.T) {
	t.Cleanup(ResetarParaTeste)
	dir := t.TempDir()
	for range 20 {
		var wg sync.WaitGroup
		for range 8 {
			wg.Go(func() {
				if _, err := Iniciar(Opcoes{Valor: "1", PastaDados: dir, Saneador: saneadorTeste(), Saida: &bufSeguro{}}); err != nil {
					t.Error(err)
				}
			})
		}
		wg.Wait()
	}
	ResetarParaTeste()
	if n := gravadoresVivos.Load(); n != 0 {
		t.Errorf("gravadores vivos depois de ResetarParaTeste = %d, quer 0", n)
	}
}

func TestIniciar_Desligado(t *testing.T) {
	t.Cleanup(ResetarParaTeste)
	dir := t.TempDir()
	ativo, err := Iniciar(Opcoes{Valor: "", PastaDados: dir, Saneador: saneadorTeste()})
	if ativo || err != nil {
		t.Fatalf("Iniciar = (%v, %v), quer (false, nil)", ativo, err)
	}
	if Ativo() {
		t.Error("Ativo() deveria ser false")
	}
	if Manipulador() != nil {
		t.Error("Manipulador() deveria ser nil")
	}
	ents, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(ents) != 0 {
		t.Errorf("a pasta deveria continuar vazia, tem %d entradas", len(ents))
	}
}

func TestIniciar_Valores(t *testing.T) {
	ctx := context.Background()
	casos := []struct {
		valor          string
		detalhado      bool
		naoReconhecido bool
	}{
		{"1", false, false},
		{"detalhado", true, false},
		{"talvez", false, true},
	}
	for _, c := range casos {
		t.Run(c.valor, func(t *testing.T) {
			t.Cleanup(ResetarParaTeste)
			dir := t.TempDir()
			ativo, err := Iniciar(Opcoes{Valor: c.valor, PastaDados: dir, Saneador: saneadorTeste()})
			if !ativo || err != nil {
				t.Fatalf("Iniciar = (%v, %v), quer (true, nil)", ativo, err)
			}
			if !Ativo() {
				t.Error("Ativo() deveria ser true")
			}
			h := Manipulador()
			if h == nil {
				t.Fatal("Manipulador() nil com o registro ativo")
			}
			if got := h.Enabled(ctx, slog.LevelDebug); got != c.detalhado {
				t.Errorf("Enabled(Debug) = %v, quer %v", got, c.detalhado)
			}
			if !h.Enabled(ctx, slog.LevelInfo) {
				t.Error("Enabled(Info) deveria ser true")
			}
			if Detalhado() != c.detalhado {
				t.Errorf("Detalhado() = %v", Detalhado())
			}
			if ValorNaoReconhecido() != c.naoReconhecido {
				t.Errorf("ValorNaoReconhecido() = %v", ValorNaoReconhecido())
			}
			logs := filepath.Join(dir, "logs")
			if Pasta() != logs {
				t.Errorf("Pasta() = %q, quer %q", Pasta(), logs)
			}
			slog.New(h).Info("teste de início")
			Encerrar()
			b, err := os.ReadFile(filepath.Join(logs, "eprocia.log"))
			if err != nil {
				t.Fatal(err)
			}
			if len(linhasJSON(t, string(b))) != 1 {
				t.Errorf("eprocia.log = %q, quer 1 linha", b)
			}
			if Ativo() {
				t.Error("Ativo() deveria ser false depois de Encerrar")
			}
			Encerrar()
		})
	}
}

func TestIniciar_PastaSomenteLeitura(t *testing.T) {
	t.Cleanup(ResetarParaTeste)
	arq := filepath.Join(t.TempDir(), "arquivo")
	if err := os.WriteFile(arq, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	ativo, err := Iniciar(Opcoes{Valor: "1", PastaDados: arq, Saneador: saneadorTeste()})
	if ativo || err == nil {
		t.Fatalf("Iniciar = (%v, %v), quer (false, erro)", ativo, err)
	}
	if Ativo() {
		t.Error("Ativo() deveria ser false")
	}
	if Manipulador() != nil {
		t.Error("Manipulador() deveria ser nil")
	}
}

func TestLogger_Contexto(t *testing.T) {
	if Logger(context.Background()) != slog.Default() {
		t.Error("sem logger no contexto, Logger deveria devolver slog.Default()")
	}
	l := slog.New(slog.DiscardHandler)
	if Logger(ComLogger(context.Background(), l)) != l {
		t.Error("Logger deveria devolver o logger de ComLogger")
	}
}
