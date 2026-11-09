//go:build !windows

package janela

// Fora do Windows não há animação: só oculta.
type nativa struct{ ocultar func() }

func Nativa(ocultar func()) Janela { return nativa{ocultar: ocultar} }

func (nativa) Visivel() bool            { return true }
func (nativa) EfeitosAtivos() bool      { return false }
func (nativa) Preparar() (Estado, bool) { return Estado{}, false }
func (nativa) Aplicar(Estado, Quadro)   {}
func (n nativa) Ocultar()               { n.ocultar() }
func (nativa) Repor(Estado)             {}
