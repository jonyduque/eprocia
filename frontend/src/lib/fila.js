// Funções puras da lista da fila (ProcessList.svelte).

// Criado uma vez: toLocaleTimeString monta um formatador novo a cada chamada,
// e a lista inteira é redesenhada a cada queue:update.
const FORMATO_HORA = new Intl.DateTimeFormat('pt-BR', {
  hour: '2-digit',
  minute: '2-digit',
  second: '2-digit',
});

export function formatarHora(timestamp) {
  if (!timestamp) return '';
  const data = new Date(timestamp);
  if (Number.isNaN(data.getTime())) return '';
  return FORMATO_HORA.format(data);
}

export function filtrarItens(items, filtro) {
  const q = (filtro ?? '').toLowerCase();
  if (!q) return items;
  return items.filter(
    (item) =>
      item.name.toLowerCase().includes(q) ||
      (item.message && item.message.toLowerCase().includes(q)),
  );
}

// Para onde vai o foco depois de uma ação na linha `id`: a própria linha, se
// ela continua na lista; senão a seguinte que sobrou; senão a anterior; senão
// null (o chamador foca o título da lista).
export function alvoFoco(idsAntes, id, idsDepois) {
  if (id == null) return null;
  const restantes = new Set(idsDepois);
  if (restantes.has(id)) return id;
  const i = idsAntes.indexOf(id);
  if (i < 0) return idsDepois[0] ?? null;
  for (let j = i + 1; j < idsAntes.length; j++) {
    if (restantes.has(idsAntes[j])) return idsAntes[j];
  }
  for (let j = i - 1; j >= 0; j--) {
    if (restantes.has(idsAntes[j])) return idsAntes[j];
  }
  return null;
}

// Botões de linha têm data-acao; cada estado mostra um conjunto diferente.
// Prefere a mesma ação, depois Remover e Cancelar, depois a primeira que houver.
export function escolherAcao(disponiveis, preferida) {
  for (const acao of [preferida, 'remover', 'cancelar']) {
    if (acao && disponiveis.includes(acao)) return acao;
  }
  return disponiveis[0] ?? null;
}
