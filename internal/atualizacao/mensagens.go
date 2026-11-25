package atualizacao

import "fmt"

// Frases da atualização mostradas ao usuário (o frontend as exibe como vêm).
const (
	MsgSemRede      = "Não foi possível consultar o GitHub agora"
	MsgRecusada     = "O GitHub recusou a consulta agora; tente mais tarde"
	MsgNaoConferiu  = "O download não conferiu; nada foi alterado"
	MsgInterrompido = "O download foi interrompido; nada foi alterado"
	MsgRetomeFila   = "Retome ou limpe a fila para atualizar"
)

// MsgAbrindo é mostrada enquanto a versão nova arranca.
func MsgAbrindo(v string) string { return fmt.Sprintf("Abrindo a versão %s...", v) }

// MsgNaoAbriu: a versão nova não deu sinal e a troca foi revertida.
func MsgNaoAbriu(nova, atual string) string {
	return fmt.Sprintf("A versão %s não abriu; o eprocIA continua na versão %s", nova, atual)
}

// MsgInstalacaoCancelada: o usuário recusou o pedido de administrador.
func MsgInstalacaoCancelada(atual string) string {
	return fmt.Sprintf("A instalação foi cancelada; o eprocIA continua na versão %s", atual)
}

// MsgReversaoIncompleta: a troca falhou e nem a reversão restaurou os nomes.
const MsgReversaoIncompleta = "A atualização falhou e a versão anterior não pôde ser restaurada; reinstale o eprocIA"

// MsgNaoAplicou: a troca falhou antes de o processo novo arrancar; nada mudou.
func MsgNaoAplicou(atual string) string {
	return fmt.Sprintf("A atualização não pôde ser aplicada; o eprocIA continua na versão %s", atual)
}
