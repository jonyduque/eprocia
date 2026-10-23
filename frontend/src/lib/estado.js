// Estado visual derivado de item.status + isPaused da fila. Queue.Pause só
// impede que o próximo item comece: o item em curso termina normalmente.

export function estadoItem(status, isPaused) {
  switch (status) {
    case 'processing':
      return 'processando';
    case 'pending':
      return isPaused ? 'pausado' : 'aguardando';
    case 'success':
      return 'sucesso';
    case 'error':
      return 'erro';
    case 'cancelled':
      return 'cancelado';
    default:
      return 'aguardando';
  }
}

export function contarAtivos(items) {
  return (items || []).filter((i) => estadoItem(i.status, false) === 'processando').length;
}

// Texto do chip na barra da lista; null quando a fila não está pausada.
export function avisoFila(items, isPaused) {
  if (!isPaused) return null;
  return contarAtivos(items) > 0 ? 'Fila pausada: termina o item atual' : 'Fila pausada';
}

export function rotuloFila(n) {
  return `${n} ${n === 1 ? 'item' : 'itens'} na fila`;
}
