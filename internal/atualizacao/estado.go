package atualizacao

import "errors"

// Info é o resultado de Verificar. Modo: trocar, instalador, so_avisar,
// incompleta ou em_dia.
type Info struct {
	Versao   string `json:"versao"`
	NotasURL string `json:"notasURL"`
	Modo     string `json:"modo"`
	Motivo   string `json:"motivo"`
}

// Modos de Info que não são um Modo de instalação.
const (
	ModoIncompleta = "incompleta"
	ModoEmDia      = "em_dia"
)

// Estado é o que o frontend mostra da atualização. Em "instalada", Versao é
// a versão instalada que espera o reinício. Modo vazio em "erro" = a consulta
// falhou; preenchido = a falha veio depois de uma consulta boa. Atual é a
// versão em execução (não muda com Instalar: o processo continua o mesmo).
type Estado struct {
	Ativo     bool   `json:"ativo"`
	Fase      string `json:"fase"`
	Versao    string `json:"versao"`
	Progresso int    `json:"progresso"`
	Mensagem  string `json:"mensagem"`
	Modo      string `json:"modo"`
	Motivo    string `json:"motivo"`
	Atual     string `json:"atual"`
}

// Fases do Estado. FaseNada é o início (nada feito).
const (
	FaseNada              = ""
	FaseVerificando       = "verificando"
	FaseDisponivel        = "disponivel"
	FaseIncompleta        = "incompleta"
	FaseBaixando          = "baixando"
	FaseConferindo        = "conferindo"
	FaseInstalando        = "instalando"
	FaseInstalada         = "instalada"
	FaseAguardandoFila    = "aguardando_fila"
	FaseReiniciando       = "reiniciando"
	FaseAbrindoInstalador = "abrindo_instalador"
	FaseErro              = "erro"
	FaseEmDia             = "em_dia"

	// fasePreparando é interna e nunca emitida: a conferência da fila e a
	// preparação (monitor, fila, gravador) antes de reiniciar ou de abrir o
	// instalador. Estado() a mostra como reiniciando ou abrindo_instalador.
	fasePreparando = "preparando"
)

var (
	// ErrSemConsulta: Instalar sem um Verificar bom antes.
	ErrSemConsulta = errors.New("atualização sem consulta")
	// ErrNaoInstalavel: a consulta não permite instalar (ou não há o que reiniciar).
	ErrNaoInstalavel = errors.New("atualização não instalável")
	// ErrEmCurso: já há uma operação da atualização em curso.
	ErrEmCurso = errors.New("atualização em curso")
	// ErrDesligado: sem versão válida (build de desenvolvimento) ou encerrado.
	ErrDesligado = errors.New("atualização desligada")
	// ErrFaltaReiniciar: a versão nova já está instalada; falta reiniciar.
	ErrFaltaReiniciar = errors.New("a versão nova já está instalada; falta reiniciar")
)

// erroComFrase é devolvido por Verificar: o texto é só a frase da tela (o
// erro de origem carrega URL), e errors.Is continua achando a origem.
type erroComFrase struct {
	frase string
	causa error
}

func (e *erroComFrase) Error() string { return e.frase }
func (e *erroComFrase) Unwrap() error { return e.causa }
