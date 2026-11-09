package janela

import (
	"math"
	"slices"
	"sync"
	"testing"
	"time"
)

func TestSuavizar_SegueCurvaEaseOut(t *testing.T) {
	if v := suavizar(0); v != 0 {
		t.Fatalf("suavizar(0) = %v", v)
	}
	if v := suavizar(1); v != 1 {
		t.Fatalf("suavizar(1) = %v", v)
	}
	// cubic-bezier(.2,.8,.2,1) já passou de 80% do caminho na metade do tempo.
	if v := suavizar(0.5); v < 0.8 || v > 0.95 {
		t.Fatalf("suavizar(0.5) = %v, esperado entre 0.8 e 0.95", v)
	}
	ant := 0.0
	for i := 1; i <= 100; i++ {
		v := suavizar(float64(i) / 100)
		if v < ant-1e-9 {
			t.Fatalf("curva desce em %d%%: %v < %v", i, v, ant)
		}
		ant = v
	}
	if v := suavizar(-1); v != 0 {
		t.Fatalf("suavizar(-1) = %v", v)
	}
	if v := suavizar(2); v != 1 {
		t.Fatalf("suavizar(2) = %v", v)
	}
}

func TestQuadroOcultar_VaiDoOpacoAoTransparente(t *testing.T) {
	dur := 160 * time.Millisecond
	if q := QuadroOcultar(0, dur, 16, 12); q != (Quadro{Alfa: 255}) {
		t.Fatalf("início: %+v", q)
	}
	if q := QuadroOcultar(dur, dur, 16, 12); q != (Quadro{Alfa: 0, DX: 16, DY: 12}) {
		t.Fatalf("fim: %+v", q)
	}
	if q := QuadroOcultar(2*dur, dur, 16, 12); q != (Quadro{Alfa: 0, DX: 16, DY: 12}) {
		t.Fatalf("depois do fim: %+v", q)
	}
	if q := QuadroOcultar(-time.Millisecond, dur, 16, 12); q != (Quadro{Alfa: 255}) {
		t.Fatalf("antes do início: %+v", q)
	}
	if q := QuadroOcultar(0, 0, 16, 12); q != (Quadro{Alfa: 0, DX: 16, DY: 12}) {
		t.Fatalf("duração zero vai direto ao fim: %+v", q)
	}

	ant := QuadroOcultar(0, dur, 16, 12)
	for d := time.Millisecond; d <= dur; d += time.Millisecond {
		q := QuadroOcultar(d, dur, 16, 12)
		if q.Alfa > ant.Alfa || q.DX < ant.DX || q.DY < ant.DY {
			t.Fatalf("em %v o quadro voltou: %+v depois de %+v", d, q, ant)
		}
		ant = q
	}

	meio := QuadroOcultar(dur/2, dur, 16, 12)
	want := Quadro{
		Alfa: uint8(math.Round(255 * (1 - suavizar(0.5)))),
		DX:   int(math.Round(16 * suavizar(0.5))),
		DY:   int(math.Round(12 * suavizar(0.5))),
	}
	if meio != want {
		t.Fatalf("meio: %+v, esperado %+v", meio, want)
	}
}

func TestQuadroOcultar_SemDeslizeSoDesvanece(t *testing.T) {
	q := QuadroOcultar(80*time.Millisecond, 160*time.Millisecond, 0, 0)
	if q.DX != 0 || q.DY != 0 || q.Alfa == 0 || q.Alfa == 255 {
		t.Fatalf("quadro: %+v", q)
	}
}

// janelaFalsa registra as chamadas na ordem.
type janelaFalsa struct {
	mu       sync.Mutex
	visivel  bool
	efeitos  bool
	preparou bool
	recusa   bool
	chamadas []string
	alfas    []uint8
	aplicou  chan struct{}
}

func novaJanelaFalsa() *janelaFalsa {
	return &janelaFalsa{visivel: true, efeitos: true, aplicou: make(chan struct{}, 1000)}
}

func (j *janelaFalsa) reg(s string) {
	j.mu.Lock()
	j.chamadas = append(j.chamadas, s)
	j.mu.Unlock()
}

func (j *janelaFalsa) Visivel() bool       { return j.visivel }
func (j *janelaFalsa) EfeitosAtivos() bool { return j.efeitos }
func (j *janelaFalsa) Preparar() (Estado, bool) {
	j.reg("preparar")
	if j.recusa {
		return Estado{}, false
	}
	return Estado{X: 10, Y: 20, DX: 16, DY: 16}, true
}
func (j *janelaFalsa) Aplicar(_ Estado, q Quadro) {
	j.mu.Lock()
	if len(j.chamadas) == 0 || j.chamadas[len(j.chamadas)-1] != "aplicar" {
		j.chamadas = append(j.chamadas, "aplicar")
	}
	j.alfas = append(j.alfas, q.Alfa)
	j.mu.Unlock()
	j.aplicou <- struct{}{}
}
func (j *janelaFalsa) Ocultar()       { j.reg("ocultar") }
func (j *janelaFalsa) Repor(_ Estado) { j.reg("repor") }

func (j *janelaFalsa) lista() []string {
	j.mu.Lock()
	defer j.mu.Unlock()
	return slices.Clone(j.chamadas)
}

func animadorTeste(j Janela) *Animador {
	a := NovoAnimador(j)
	a.Duracao = 40 * time.Millisecond
	a.Passo = 2 * time.Millisecond
	return a
}

func TestAnimador_DesvaneceOcultaERepoe(t *testing.T) {
	j := novaJanelaFalsa()
	animadorTeste(j).Ocultar()

	if got, want := j.lista(), []string{"preparar", "aplicar", "ocultar", "repor"}; !slices.Equal(got, want) {
		t.Fatalf("chamadas %v, esperado %v", got, want)
	}
	if len(j.alfas) < 3 || j.alfas[len(j.alfas)-1] != 0 {
		t.Fatalf("alfas %v: esperava vários quadros terminando em 0", j.alfas)
	}
}

func TestAnimador_SemEfeitosOcultaDireto(t *testing.T) {
	j := novaJanelaFalsa()
	j.efeitos = false
	animadorTeste(j).Ocultar()
	if got := j.lista(); !slices.Equal(got, []string{"ocultar"}) {
		t.Fatalf("chamadas %v", got)
	}
}

func TestAnimador_JanelaOcultaNaoAnima(t *testing.T) {
	j := novaJanelaFalsa()
	j.visivel = false
	animadorTeste(j).Ocultar()
	if got := j.lista(); !slices.Equal(got, []string{"ocultar"}) {
		t.Fatalf("chamadas %v", got)
	}
}

func TestAnimador_PrepararRecusadoOcultaDireto(t *testing.T) {
	j := novaJanelaFalsa()
	j.recusa = true
	animadorTeste(j).Ocultar()
	if got := j.lista(); !slices.Equal(got, []string{"preparar", "ocultar"}) {
		t.Fatalf("chamadas %v", got)
	}
}

func TestAnimador_CancelarNoMeioRepoeSemOcultar(t *testing.T) {
	j := novaJanelaFalsa()
	a := animadorTeste(j)
	a.Duracao = time.Hour
	fim := make(chan struct{})
	go func() {
		a.Ocultar()
		close(fim)
	}()
	<-j.aplicou
	a.Cancelar()
	select {
	case <-fim:
	case <-time.After(5 * time.Second):
		t.Fatal("Ocultar não terminou depois de Cancelar")
	}
	if got, want := j.lista(), []string{"preparar", "aplicar", "repor"}; !slices.Equal(got, want) {
		t.Fatalf("chamadas %v, esperado %v", got, want)
	}
	// Cancelar de novo, sem animação, não trava.
	a.Cancelar()
}

func TestAnimador_OcultarDuasVezesAnimaUmaSo(t *testing.T) {
	j := novaJanelaFalsa()
	a := animadorTeste(j)
	a.Duracao = time.Hour
	fim := make(chan struct{})
	go func() {
		a.Ocultar()
		close(fim)
	}()
	<-j.aplicou
	a.Ocultar() // volta na hora: a primeira já vai ocultar
	a.Cancelar()
	<-fim
	n := 0
	for _, c := range j.lista() {
		if c == "preparar" {
			n++
		}
	}
	if n != 1 {
		t.Fatalf("preparou %d vezes: %v", n, j.lista())
	}
}

func TestAnimador_CancelarConcorrenteNaoEntraEmPanico(t *testing.T) {
	j := novaJanelaFalsa()
	a := animadorTeste(j)
	a.Duracao = time.Hour
	go a.Ocultar()
	<-j.aplicou
	var wg sync.WaitGroup
	for range 8 {
		wg.Go(a.Cancelar)
	}
	wg.Wait()
}
