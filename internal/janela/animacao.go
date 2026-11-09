// Package janela anima a janela principal ao ir para a bandeja: desvanece o
// alfa da janela em camadas e a desliza um pouco para o canto inferior
// direito, depois a oculta e repõe alfa, posição e estilo.
package janela

import (
	"math"
	"sync"
	"time"
)

// Padrões da animação de ocultar (DESIGN.md, seção 7).
const (
	DuracaoPadrao = 160 * time.Millisecond
	PassoPadrao   = 10 * time.Millisecond
	// DeslizePadrao é o deslocamento em cada eixo a 96 dpi.
	DeslizePadrao = 16
)

// Quadro é o estado da janela num instante da animação.
type Quadro struct {
	Alfa   uint8
	DX, DY int
}

// QuadroOcultar devolve o quadro em decorrido: o alfa vai de 255 a 0 e o
// deslocamento de (0,0) a (dx,dy), na curva --ease-out do app.
func QuadroOcultar(decorrido, duracao time.Duration, dx, dy int) Quadro {
	p := 1.0
	if duracao > 0 {
		p = suavizar(float64(decorrido) / float64(duracao))
	}
	return Quadro{
		Alfa: uint8(math.Round(255 * (1 - p))),
		DX:   int(math.Round(float64(dx) * p)),
		DY:   int(math.Round(float64(dy) * p)),
	}
}

// suavizar resolve cubic-bezier(.2,.8,.2,1), o --ease-out de style.css,
// para t em [0,1].
func suavizar(t float64) float64 {
	if t <= 0 {
		return 0
	}
	if t >= 1 {
		return 1
	}
	const x1, y1, x2, y2 = 0.2, 0.8, 0.2, 1.0
	bez := func(s, a, b float64) float64 {
		u := 1 - s
		return 3*u*u*s*a + 3*u*s*s*b + s*s*s
	}
	// x(s) é monotônica em [0,1]: bisseção basta e não diverge.
	lo, hi := 0.0, 1.0
	for range 40 {
		m := (lo + hi) / 2
		if bez(m, x1, x2) < t {
			lo = m
		} else {
			hi = m
		}
	}
	return bez((lo+hi)/2, y1, y2)
}

// Estado guarda o que Preparar mudou na janela, para Repor desfazer.
type Estado struct {
	X, Y    int // posição original, em pixels físicos
	DX, DY  int // deslize total; zero quando maximizada
	ExStyle uintptr
}

// Janela é o que a animação precisa da janela nativa.
type Janela interface {
	Visivel() bool
	// EfeitosAtivos lê a opção "Efeitos de animação" do Windows.
	EfeitosAtivos() bool
	// Preparar guarda posição e estilo e liga a transparência; ok=false
	// pula a animação (janela minimizada, não encontrada).
	Preparar() (e Estado, ok bool)
	Aplicar(e Estado, q Quadro)
	Ocultar()
	// Repor devolve alfa, posição e estilo originais.
	Repor(e Estado)
}

// Animador serializa a animação de ocultar: um segundo Ocultar durante a
// rampa volta na hora e Cancelar (vindo de Show ou Quit) interrompe a rampa
// e repõe a janela antes de devolver.
type Animador struct {
	j       Janela
	Duracao time.Duration
	Passo   time.Duration

	mu       sync.Mutex
	fim      chan struct{} // nil sem animação; fechado ao terminar
	cancelar chan struct{}
}

func NovoAnimador(j Janela) *Animador {
	return &Animador{j: j, Duracao: DuracaoPadrao, Passo: PassoPadrao}
}

// Ocultar anima e oculta a janela; bloqueia até o fim da rampa (~Duracao).
func (a *Animador) Ocultar() {
	a.mu.Lock()
	if a.fim != nil {
		a.mu.Unlock()
		return
	}
	if !a.j.Visivel() || !a.j.EfeitosAtivos() {
		a.mu.Unlock()
		a.j.Ocultar()
		return
	}
	e, ok := a.j.Preparar()
	if !ok {
		a.mu.Unlock()
		a.j.Ocultar()
		return
	}
	fim, cancelar := make(chan struct{}), make(chan struct{})
	a.fim, a.cancelar = fim, cancelar
	a.mu.Unlock()

	defer func() {
		a.mu.Lock()
		a.fim, a.cancelar = nil, nil
		a.mu.Unlock()
		close(fim)
	}()

	inicio := time.Now()
	for {
		d := time.Since(inicio)
		a.j.Aplicar(e, QuadroOcultar(d, a.Duracao, e.DX, e.DY))
		if d >= a.Duracao {
			break
		}
		select {
		case <-cancelar:
			a.j.Repor(e)
			return
		case <-time.After(a.Passo):
		}
	}
	select {
	case <-cancelar:
		a.j.Repor(e)
		return
	default:
	}
	a.j.Ocultar()
	a.j.Repor(e)
}

// Cancelar interrompe a animação em curso e espera a janela ser reposta.
// Sem animação, volta na hora.
func (a *Animador) Cancelar() {
	a.mu.Lock()
	fim, cancelar := a.fim, a.cancelar
	if fim == nil {
		a.mu.Unlock()
		return
	}
	select {
	case <-cancelar:
	default:
		close(cancelar)
	}
	a.mu.Unlock()
	<-fim
}
