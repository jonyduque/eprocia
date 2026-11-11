package main

import "github.com/energye/systray"

// bandeja é a parte do systray que o app usa; os testes trocam por uma falsa.
type bandeja interface {
	SetIcon(icone []byte)
	SetTitle(titulo string)
	SetTooltip(dica string)
	AddMenuItem(titulo, dica string, clique func())
	SetOnDClick(fn func())
	Quit()
}

type acoesBandeja struct {
	abrir, ocultar, sair func()
}

// prepararBandeja liga o duplo clique esquerdo a Abrir, que restaura o
// estado anterior da janela (não força maximizar) e a traz para a frente.
// Roda antes de systray.Run, na thread do tray. O clique simples fica sem
// ação: abrir o menu nele engoliria o segundo clique do duplo clique; o
// menu abre no botão direito.
func prepararBandeja(b bandeja, ac acoesBandeja) {
	b.SetOnDClick(func() { go ac.abrir() })
}

// montarMenu roda no onReady. Os cliques chegam na thread do tray, dentro
// do wndProc: as ações vão em goroutines para não travá-lo (Ocultar anima
// por ~160 ms).
func montarMenu(b bandeja, ac acoesBandeja) {
	b.SetIcon(DefaultIcon)
	b.SetTitle("eprocIA")
	b.SetTooltip("eprocIA - ETL Judicial")
	b.AddMenuItem("Abrir", "Abrir o eprocIA", func() { go ac.abrir() })
	b.AddMenuItem("Ocultar", "Ocultar a janela", func() { go ac.ocultar() })
	b.AddMenuItem("Sair", "Sair do aplicativo", func() {
		go func() {
			b.Quit()
			ac.sair()
		}()
	})
}

// bandejaSystray liga a interface ao github.com/energye/systray.
type bandejaSystray struct{}

func (bandejaSystray) SetIcon(icone []byte)   { systray.SetIcon(icone) }
func (bandejaSystray) SetTitle(titulo string) { systray.SetTitle(titulo) }
func (bandejaSystray) SetTooltip(dica string) { systray.SetTooltip(dica) }
func (bandejaSystray) Quit()                  { systray.Quit() }
func (bandejaSystray) AddMenuItem(titulo, dica string, clique func()) {
	systray.AddMenuItem(titulo, dica).Click(clique)
}
func (bandejaSystray) SetOnDClick(fn func()) {
	systray.SetOnDClick(func(systray.IMenu) { fn() })
}

func acoesDoApp(app *App) acoesBandeja {
	return acoesBandeja{abrir: app.Show, ocultar: app.Hide, sair: app.Quit}
}
