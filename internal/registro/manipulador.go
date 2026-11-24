package registro

import (
	"bytes"
	"context"
	"fmt"
	"log/slog"
	"time"
)

// operacao é um passo de WithAttrs (attrs) ou WithGroup (grupo).
type operacao struct {
	grupo string
	attrs []slog.Attr
}

// manipulador é o slog.Handler do log em arquivo: formata a linha na
// goroutine de quem registra e a entrega à gravadora, sem bloquear.
type manipulador struct {
	g     *gravador
	nivel slog.Level
	ops   []operacao
}

func (g *gravador) manipulador(nivel slog.Level) *manipulador {
	return &manipulador{g: g, nivel: nivel}
}

func (h *manipulador) Enabled(_ context.Context, nivel slog.Level) bool {
	return nivel >= h.nivel && !h.g.encerrado.Load()
}

func (h *manipulador) Handle(_ context.Context, r slog.Record) error {
	if h.g.encerrado.Load() {
		return nil
	}
	h.g.enviar(formatar(h.g.saneador, h.ops, r), r.Level)
	return nil
}

func (h *manipulador) WithAttrs(attrs []slog.Attr) slog.Handler {
	if len(attrs) == 0 {
		return h
	}
	return h.com(operacao{attrs: attrs})
}

func (h *manipulador) WithGroup(nome string) slog.Handler {
	if nome == "" {
		return h
	}
	return h.com(operacao{grupo: nome})
}

func (h *manipulador) com(op operacao) *manipulador {
	ops := make([]operacao, len(h.ops), len(h.ops)+1)
	copy(ops, h.ops)
	return &manipulador{g: h.g, nivel: h.nivel, ops: append(ops, op)}
}

// formatar monta a linha JSON de r com um slog.JSONHandler próprio da
// chamada (nada compartilhado entre goroutines), refazendo as operações de
// With. Os valores de texto, erro e Stringer passam por Sanear, e a linha
// inteira passa de novo, para alcançar o que o json.Marshal serializa de
// struct, map e slice.
func formatar(s *Saneador, ops []operacao, r slog.Record) []byte {
	var buf bytes.Buffer
	var h slog.Handler = slog.NewJSONHandler(&buf, &slog.HandlerOptions{
		Level:       slog.LevelDebug,
		ReplaceAttr: substituidor(s),
	})
	for _, op := range ops {
		if op.grupo != "" {
			h = h.WithGroup(op.grupo)
		} else {
			h = h.WithAttrs(op.attrs)
		}
	}
	if err := h.Handle(context.Background(), r); err != nil {
		return nil
	}
	if s == nil {
		return buf.Bytes()
	}
	return []byte(s.Sanear(buf.String()))
}

func substituidor(s *Saneador) func([]string, slog.Attr) slog.Attr {
	sanear := func(t string) string {
		if s == nil {
			return t
		}
		return s.Sanear(t)
	}
	return func(grupos []string, a slog.Attr) slog.Attr {
		if len(grupos) == 0 {
			switch a.Key {
			case slog.TimeKey:
				if a.Value.Kind() == slog.KindTime {
					return slog.String("ts", a.Value.Time().Local().Format(time.RFC3339Nano))
				}
			case slog.LevelKey:
				return slog.Attr{Key: "nivel", Value: a.Value}
			}
		}
		switch a.Value.Kind() {
		case slog.KindString:
			a.Value = slog.StringValue(sanear(a.Value.String()))
		case slog.KindAny:
			switch v := a.Value.Any().(type) {
			case error:
				a.Value = slog.StringValue(sanear(textoSeguro(v.Error)))
			case fmt.Stringer:
				a.Value = slog.StringValue(sanear(textoSeguro(v.String)))
			}
		}
		return a
	}
}

// textoSeguro chama f e troca um pânico (Error ou String de ponteiro nil)
// por um texto fixo: registrar nunca derruba quem chama.
func textoSeguro(f func() string) (t string) {
	defer func() {
		if p := recover(); p != nil {
			t = fmt.Sprintf("!PANIC: %v", p)
		}
	}()
	return f()
}
