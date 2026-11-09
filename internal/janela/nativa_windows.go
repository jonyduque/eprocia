package janela

import (
	"os"
	"sync"
	"unsafe"

	"golang.org/x/sys/windows"
)

var (
	user32                         = windows.NewLazySystemDLL("user32.dll")
	procFindWindowExW              = user32.NewProc("FindWindowExW")
	procGetWindowLongPtrW          = user32.NewProc("GetWindowLongPtrW")
	procSetWindowLongPtrW          = user32.NewProc("SetWindowLongPtrW")
	procSetLayeredWindowAttributes = user32.NewProc("SetLayeredWindowAttributes")
	procSetWindowPos               = user32.NewProc("SetWindowPos")
	procIsIconic                   = user32.NewProc("IsIconic")
	procIsZoomed                   = user32.NewProc("IsZoomed")
	procSystemParametersInfoW      = user32.NewProc("SystemParametersInfoW")
	procGetDpiForWindow            = user32.NewProc("GetDpiForWindow")
	procGetWindowRect              = user32.NewProc("GetWindowRect")
)

const (
	wsExLayered            = 0x00080000
	lwaAlpha               = 0x2
	swpNoSize              = 0x0001
	swpNoZOrder            = 0x0004
	swpNoActivate          = 0x0010
	swpAsyncWindowPos      = 0x4000
	spiGetClientAreaAnimat = 0x1042
	// classeWails é a classe da janela principal do Wails v2 no Windows.
	classeWails = "wailsWindow"
)

// gwlExStyle é -20; variável para a conversão a uintptr estender o sinal.
var gwlExStyle = -20

type nativa struct {
	ocultar func()

	once sync.Once
	hwnd windows.HWND
}

// Nativa devolve a janela principal deste processo. ocultar é o
// runtime.WindowHide do Wails, para o estado interno dele seguir certo.
func Nativa(ocultar func()) Janela {
	return &nativa{ocultar: ocultar}
}

// janelaPrincipal acha a janela do Wails deste processo (pode haver outra
// cópia do app rodando de outra pasta).
func (n *nativa) janelaPrincipal() windows.HWND {
	n.once.Do(func() {
		classe, _ := windows.UTF16PtrFromString(classeWails)
		pid := uint32(os.Getpid())
		var h uintptr
		for {
			h, _, _ = procFindWindowExW.Call(0, h, uintptr(unsafe.Pointer(classe)), 0)
			if h == 0 {
				return
			}
			var dono uint32
			if _, err := windows.GetWindowThreadProcessId(windows.HWND(h), &dono); err == nil && dono == pid {
				n.hwnd = windows.HWND(h)
				return
			}
		}
	})
	return n.hwnd
}

func (n *nativa) Visivel() bool {
	h := n.janelaPrincipal()
	return h == 0 || windows.IsWindowVisible(h)
}

func (n *nativa) EfeitosAtivos() bool {
	var ativo int32
	r, _, _ := procSystemParametersInfoW.Call(spiGetClientAreaAnimat, 0, uintptr(unsafe.Pointer(&ativo)), 0)
	return r != 0 && ativo != 0
}

func (n *nativa) Preparar() (Estado, bool) {
	h := n.janelaPrincipal()
	if h == 0 {
		return Estado{}, false
	}
	if r, _, _ := procIsIconic.Call(uintptr(h)); r != 0 {
		return Estado{}, false
	}
	var ret windows.Rect
	if r, _, _ := procGetWindowRect.Call(uintptr(h), uintptr(unsafe.Pointer(&ret))); r == 0 {
		return Estado{}, false
	}
	e := Estado{X: int(ret.Left), Y: int(ret.Top)}
	if r, _, _ := procIsZoomed.Call(uintptr(h)); r == 0 {
		d := DeslizePadrao * dpi(h) / 96
		e.DX, e.DY = d, d
	}
	ex, _, _ := procGetWindowLongPtrW.Call(uintptr(h), uintptr(gwlExStyle))
	e.ExStyle = ex
	if ex&wsExLayered == 0 {
		procSetWindowLongPtrW.Call(uintptr(h), uintptr(gwlExStyle), ex|wsExLayered)
	}
	// Uma janela que acabou de virar camada some até receber o alfa.
	if r, _, _ := procSetLayeredWindowAttributes.Call(uintptr(h), 0, 255, lwaAlpha); r == 0 {
		procSetWindowLongPtrW.Call(uintptr(h), uintptr(gwlExStyle), ex)
		return Estado{}, false
	}
	return e, true
}

func dpi(h windows.HWND) int {
	if procGetDpiForWindow.Find() != nil {
		return 96
	}
	r, _, _ := procGetDpiForWindow.Call(uintptr(h))
	if r == 0 {
		return 96
	}
	return int(r)
}

func (n *nativa) Aplicar(e Estado, q Quadro) {
	h := n.janelaPrincipal()
	procSetLayeredWindowAttributes.Call(uintptr(h), 0, uintptr(q.Alfa), lwaAlpha)
	if e.DX != 0 || e.DY != 0 {
		mover(h, e.X+q.DX, e.Y+q.DY)
	}
}

func (n *nativa) Ocultar() { n.ocultar() }

func (n *nativa) Repor(e Estado) {
	h := n.janelaPrincipal()
	if h == 0 {
		return
	}
	if e.DX != 0 || e.DY != 0 {
		mover(h, e.X, e.Y)
	}
	procSetLayeredWindowAttributes.Call(uintptr(h), 0, 255, lwaAlpha)
	if e.ExStyle&wsExLayered == 0 {
		procSetWindowLongPtrW.Call(uintptr(h), uintptr(gwlExStyle), e.ExStyle)
	}
}

// mover não espera a thread da janela (SWP_ASYNCWINDOWPOS): a animação roda
// fora dela e não pode travar se a thread principal estiver ocupada.
func mover(h windows.HWND, x, y int) {
	procSetWindowPos.Call(uintptr(h), 0, uintptr(x), uintptr(y), 0, 0,
		swpNoSize|swpNoZOrder|swpNoActivate|swpAsyncWindowPos)
}
