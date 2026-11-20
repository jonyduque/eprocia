// Junta os eventos da fila (queue:update com a lista inteira, queue:item com
// um item) e aplica o resultado no máximo uma vez por quadro, para que rajadas
// de progresso não refaçam a lista a cada evento. Pausada (janela na bandeja),
// só guarda o último estado e o aplica num quadro ao retomar.

export function criarFilaPorQuadro(
  ler,
  aplicar,
  agendar = (fn) => requestAnimationFrame(fn),
  cancelarAgendado = (id) => cancelAnimationFrame(id)
) {
  let lista = null;
  let itens = new Map();
  let agendado = null;
  let pausada = false;

  function descarregar() {
    agendado = null;
    const base = lista ?? ler();
    const trocas = itens;
    lista = null;
    itens = new Map();
    aplicar(trocas.size ? base.map((i) => trocas.get(i.id) ?? i) : base);
  }

  function agendarSePreciso() {
    if (!pausada && agendado === null) agendado = agendar(descarregar);
  }

  return {
    lista(nova) {
      lista = nova || [];
      itens = new Map();
      agendarSePreciso();
    },
    item(novo) {
      if (!novo) return;
      itens.set(novo.id, novo);
      agendarSePreciso();
    },
    pausar() {
      pausada = true;
    },
    retomar() {
      pausada = false;
      if (lista !== null || itens.size) agendarSePreciso();
    },
    cancelar() {
      if (agendado !== null) cancelarAgendado(agendado);
      agendado = null;
      lista = null;
      itens = new Map();
    },
  };
}
