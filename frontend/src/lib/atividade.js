// Atividade por documento de um ZIP (queue:atividade / GetQueueActivity) e as
// opções de OCR e higienização usadas no painel recolhido e no reprocessamento.

export const OPCOES_OCR = [
  { valor: 'none', rotulo: 'Desativado' },
  { valor: 'fast', rotulo: 'Rápido' },
  { valor: 'normal', rotulo: 'Normal' },
  { valor: 'best', rotulo: 'Preciso' }
];

export const OPCOES_LIMPEZA = [
  { valor: 'none', rotulo: 'Nenhuma' },
  { valor: 'basic', rotulo: 'Básica' },
  { valor: 'aggressive', rotulo: 'Agressiva' }
];

// Posição do valor na lista (0 se desconhecido), para o indicador de nível.
export function nivelOpcao(opcoes, valor) {
  return Math.max(0, opcoes.findIndex((o) => o.valor === valor));
}

export function proximaOpcao(opcoes, valor) {
  const i = opcoes.findIndex((o) => o.valor === valor);
  return opcoes[(i + 1) % opcoes.length];
}

export function rotuloOpcao(opcoes, valor) {
  return opcoes.find((o) => o.valor === valor)?.rotulo ?? valor ?? '';
}

// Quem está trabalhando vem primeiro; concluídos por último. Dentro do mesmo
// estado vale a ordem do ZIP (cronológica).
const ORDEM_ESTADO = { ocr: 0, lendo: 1, erro: 2, aguardando: 3, concluido: 4 };

export function ordenarDocs(docs) {
  return (docs ?? [])
    .map((d, i) => ({ ...d, i }))
    .sort((a, b) => (ORDEM_ESTADO[a.estado] ?? 9) - (ORDEM_ESTADO[b.estado] ?? 9) || a.i - b.i);
}

export function resumoAtividade(inst) {
  const r = { ativos: 0, emOcr: 0, aguardando: 0, concluidos: 0, erros: 0, paginasOcr: 0 };
  for (const d of inst?.docs ?? []) {
    if (d.estado === 'lendo' || d.estado === 'ocr') r.ativos++;
    if (d.estado === 'ocr') r.emOcr++;
    if (d.estado === 'aguardando') r.aguardando++;
    if (d.estado === 'concluido') r.concluidos++;
    if (d.estado === 'erro') r.erros++;
    r.paginasOcr += d.ocr || 0;
  }
  return r;
}

// Vagas de um medidor: true para as ocupadas. Limites grandes (muitos
// núcleos) viram no máximo `max` segmentos proporcionais.
export function segmentos(ocupadas, limite, max = 16) {
  const total = Math.max(1, Math.min(limite || 1, max));
  const cheias = limite > max ? Math.round((ocupadas / limite) * total) : ocupadas;
  return Array.from({ length: total }, (_, i) => i < Math.min(cheias, total));
}
