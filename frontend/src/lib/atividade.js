// Atividade por documento de um ZIP (queue:atividade / GetQueueActivity) e as
// opções de OCR e higienização (rótulo e descrição) usadas no painel, na barra
// recolhida e no reprocessamento.

export const OPCOES_OCR = [
  { valor: 'none', rotulo: 'Nenhum', descricao: 'Páginas só com imagem ficam sem texto.' },
  { valor: 'fast', rotulo: 'Rápido', descricao: 'Modelo leve: mais rápido, menos preciso.' },
  { valor: 'normal', rotulo: 'Normal', descricao: 'Equilíbrio entre velocidade e precisão.' },
  { valor: 'best', rotulo: 'Preciso', descricao: 'Modelo mais preciso, porém mais lento.' }
];

export const OPCOES_LIMPEZA = [
  { valor: 'none', rotulo: 'Nenhuma', descricao: 'Texto como extraído' },
  { valor: 'basic', rotulo: 'Básica', descricao: 'Remove espaços e linhas vazias' },
  { valor: 'standard', rotulo: 'Padrão', descricao: 'Remove cabeçalhos, rodapés e duplicatas; resume assinaturas' },
  { valor: 'aggressive', rotulo: 'Máxima', descricao: 'Padrão + ruído de OCR e parágrafos contínuos' }
];

// Posição do valor na lista (0 se desconhecido), para o indicador de nível.
export function nivelOpcao(opcoes, valor) {
  return Math.max(0, opcoes.findIndex((o) => o.valor === valor));
}

export function rotuloOpcao(opcoes, valor) {
  return opcoes.find((o) => o.valor === valor)?.rotulo ?? valor ?? '';
}

export function descricaoOpcao(opcoes, valor) {
  return opcoes.find((o) => o.valor === valor)?.descricao ?? '';
}

// Quem está trabalhando vem primeiro; concluídos por último. Dentro do mesmo
// estado vale a ordem do ZIP (cronológica).
const ORDEM_ESTADO = { ocr: 0, lendo: 1, erro: 2, aguardando: 3, concluido: 4 };

export function ordenarDocs(docs) {
  return (docs ?? [])
    .map((d, i) => ({ ...d, i }))
    .sort((a, b) => (ORDEM_ESTADO[a.estado] ?? 9) - (ORDEM_ESTADO[b.estado] ?? 9) || a.i - b.i);
}

// Motivos de leitura (AtividadeDoc.leitura) que deixam o documento fora do texto.
export const LEITURA_NAO_LIDO = ['sem_ocr', 'senha', 'ilegivel', 'prazo', 'ocr_falhou', 'sem_texto'];

export function naoLido(d) {
  return LEITURA_NAO_LIDO.includes(d?.leitura);
}

export function resumoAtividade(inst) {
  const r = { ativos: 0, emOcr: 0, aguardando: 0, concluidos: 0, erros: 0, paginasOcr: 0, naoLidos: 0, semOCR: 0 };
  for (const d of inst?.docs ?? []) {
    if (naoLido(d)) r.naoLidos++;
    if (d.leitura === 'sem_ocr') r.semOCR++;
    if (d.estado === 'lendo' || d.estado === 'ocr') r.ativos++;
    if (d.estado === 'ocr') r.emOcr++;
    if (d.estado === 'aguardando') r.aguardando++;
    if (d.estado === 'concluido') r.concluidos++;
    if (d.estado === 'erro' && !naoLido(d)) r.erros++;
    r.paginasOcr += d.ocr || 0;
  }
  return r;
}

// Aguardando montados no painel antes da linha "+ N aguardando"; o instantâneo
// chega a cada 200 ms e um ZIP grande tem centenas na espera.
export const LIMITE_AGUARDANDO = 20;

// Acima disto o flip mede linhas demais a cada instantâneo: as linhas trocam de
// lugar sem deslizar.
export const LIMITE_FLIP_DOCS = 60;

// Ordem de ordenarDocs e resumo de resumoAtividade num laço só, clonando apenas
// as linhas que vão ao DOM. A linha de expansão tem chave -1 e fica entre os
// aguardando e os concluídos.
export function prepararDocs(inst, { mostrarConcluidos = false, expandirAguardando = false } = {}) {
  const docs = inst?.docs ?? [];
  const resumo = { ativos: 0, emOcr: 0, aguardando: 0, concluidos: 0, erros: 0, paginasOcr: 0, naoLidos: 0, semOCR: 0 };
  const grupos = [[], [], [], [], [], []];
  for (let i = 0; i < docs.length; i++) {
    const d = docs[i];
    const nl = naoLido(d);
    if (nl) resumo.naoLidos++;
    if (d.leitura === 'sem_ocr') resumo.semOCR++;
    if (d.estado === 'lendo' || d.estado === 'ocr') resumo.ativos++;
    if (d.estado === 'ocr') resumo.emOcr++;
    if (d.estado === 'concluido') resumo.concluidos++;
    if (d.estado === 'erro' && !nl) resumo.erros++;
    resumo.paginasOcr += d.ocr || 0;
    const g = ORDEM_ESTADO[d.estado] ?? 5;
    if (d.estado === 'aguardando') {
      if (resumo.aguardando++ >= LIMITE_AGUARDANDO && !expandirAguardando) continue;
    } else if (d.estado === 'concluido' && !mostrarConcluidos) continue;
    grupos[g].push({ ...d, i });
  }
  const ocultos = Math.max(0, resumo.aguardando - grupos[3].length);
  const visiveis = [...grupos[0], ...grupos[1], ...grupos[2], ...grupos[3]];
  if (ocultos > 0) visiveis.push({ i: -1, mais: true, n: ocultos, expandido: false });
  else if (expandirAguardando && resumo.aguardando > LIMITE_AGUARDANDO) visiveis.push({ i: -1, mais: true, n: 0, expandido: true });
  visiveis.push(...grupos[4], ...grupos[5]);
  return { visiveis, resumo, aguardandoOcultos: ocultos };
}

// Vagas de um medidor: true para as ocupadas. Limites grandes (muitos
// núcleos) viram no máximo `max` segmentos proporcionais.
export function segmentos(ocupadas, limite, max = 16) {
  const total = Math.max(1, Math.min(limite || 1, max));
  const cheias = limite > max ? Math.round((ocupadas / limite) * total) : ocupadas;
  return Array.from({ length: total }, (_, i) => i < Math.min(cheias, total));
}
