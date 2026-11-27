// Fictitious data for each captured UI state. Nothing here may resemble a real
// process: numbers use the 5000000-00.2026.4.00.xxxx pattern, paths are made up.

import { readFileSync } from 'node:fs';

const BASE_TS = 1767322800000; // 2026-01-02 03:00 UTC, fixed so captures are stable

const CONFIG = {
  monitored_path: 'C:\\Usuarios\\exemplo\\Downloads\\Processos',
  ocr_mode: 'normal',
  cleaning_level: 'standard',
  auto_start: true,
  watchdog_enabled: true,
};

// Versão do app (wails.json, a mesma do rodapé) e a nova, um minor acima (0.5.0 -> 0.6.0).
export const VERSAO_ATUAL = JSON.parse(readFileSync(new URL('../../wails.json', import.meta.url), 'utf8')).info.productVersion;
const VERSAO_NOVA = VERSAO_ATUAL.split('.').map((p, i) => (i === 1 ? Number(p) + 1 : i === 2 ? '0' : p)).join('.');
const atualizacao = (extra) => ({ ativo: true, fase: '', versao: VERSAO_NOVA, progresso: 0, mensagem: '', modo: 'trocar', motivo: '', atual: VERSAO_ATUAL, ...extra });

const CAMINHO_LONGO = 'C:\\Usuarios\\exemplo\\OneDrive - Empresa Exemplo\\Área de Trabalho\\Pasta com um nome bem longo\\Processos baixados';

export function processo(n) {
  return `5000000-00.2026.4.00.${String(n).padStart(4, '0')}`;
}

function item(n, status, extra = {}) {
  const nome = `${processo(n)}.zip`;
  return {
    id: `00000000-0000-4000-8000-${String(n).padStart(12, '0')}`,
    name: nome,
    path: `${CONFIG.monitored_path}\\${nome}`,
    status,
    progress: 0,
    message: '',
    output_path: '',
    docs_count: 0,
    pages_count: 0,
    estimated_tokens: 0,
    processed_docs: 0,
    total_docs: 0,
    processed_pages: 0,
    total_pages: 0,
    timestamp: BASE_TS + n * 60000,
    unread_docs: 0,
    ...extra,
  };
}

const sucesso = (n) => item(n, 'success', {
  progress: 100,
  message: 'Concluído',
  output_path: `${CONFIG.monitored_path}\\${processo(n)}.txt`,
  docs_count: 57, pages_count: 142, estimated_tokens: 98450, processed_docs: 57, total_docs: 57, processed_pages: 142, total_pages: 142,
});

const processando = (n, pct) => item(n, 'processing', {
  progress: pct,
  message: `Processando documento ${Math.round(57 * pct / 100)} de 57`,
  processed_docs: Math.round(57 * pct / 100), total_docs: 57,
  processed_pages: Math.round(142 * pct / 100), total_pages: 142,
});

// Atividade por documento (etl.InstantaneoAtividade) de um item em processamento.
function atividade(n) {
  const docs = [];
  const estados = ['ocr', 'ocr', 'lendo', 'erro', 'aguardando', 'aguardando', 'aguardando'];
  for (let i = 1; i <= 24; i++) {
    const estado = i <= 14 ? 'concluido' : (estados[i - 15] ?? 'aguardando');
    const paginas = 1 + ((i * 7) % 11);
    docs.push({
      nome: `Evento ${i} - DOC${(i % 3) + 1}.pdf`,
      evento: i,
      estado,
      paginas,
      feitas: estado === 'concluido' ? paginas : estado === 'ocr' || estado === 'lendo' ? Math.floor(paginas / 2) : 0,
      ocr: estado === 'ocr' ? 2 : 0,
      ...(estado === 'erro' ? { erro: 'falha ao abrir o PDF: xref inválida' } : {}),
    });
  }
  return { item_id: item(n, 'processing').id, docs, limite_docs: 7, limite_ocr: 7, ocr_ativos: 3 };
}

// Item concluído com documentos não lidos: o ZIP tem 3 documentos que o app
// não leu (sem OCR, com senha) ou leu por outro caminho (OCR).
function atividadeNaoLidos(n) {
  const doc = (i, nome, extra) => ({ nome, evento: i, estado: 'concluido', paginas: 4, feitas: 4, ocr: 0, leitura: 'lido', ...extra });
  const docs = [
    doc(1, 'Evento 1 - INIC1.pdf'),
    doc(2, 'Evento 2 - PET1.pdf'),
    doc(3, 'Evento 3 - LAUDO1.pdf', { ocr: 4, leitura: 'ocr' }),
    // erro: Leitura.Frase() de internal/etl/leitura.go. sem_ocr não tem
    // detalhe; o de senha é o texto que o pipeline manda para um PDF com senha
    // que a biblioteca recusa (pdf.ErrInvalidPassword embrulhado por abrirPDF).
    doc(4, 'Evento 4 - ANEXO1.pdf', { estado: 'erro', feitas: 3, paginas: 3, leitura: 'sem_ocr',
      erro: 'digitalizado; o OCR está desativado' }),
    doc(5, 'Evento 5 - ANEXO2.pdf', { estado: 'erro', feitas: 0, paginas: 0, leitura: 'senha',
      erro: 'protegido por senha', detalhe: 'falha ao abrir o PDF: encrypted PDF: invalid password' }),
    doc(6, 'Evento 6 - SENT1.pdf', { leitura: 'segundo_leitor' }),
  ];
  return { item_id: item(n, 'success').id, docs, limite_docs: 7, limite_ocr: 7, ocr_ativos: 0 };
}

// relatorioZip monta um models.RelatorioZip fictício (internal/models). As cinco
// categorias de página somam o total; ocr_sem_ganho e ocr_falhou_com_reserva são
// sub-contadores de lida_nativa.
function relatorioZip({ docs, pdf, html, paginas, ocr = null, atencao, porLeitura, totalMs, bytesTxt, tokens }) {
  const p = {
    lida_nativa: 0, lida_ocr: 0, vazia: 0, sem_ocr: 0, com_erro: 0,
    ocr_sem_ganho: 0, ocr_falhou_com_reserva: 0, de_nao_lidos: 0, de_omitidos: 0, duplicadas: 0,
    ...paginas,
  };
  p.total = p.lida_nativa + p.lida_ocr + p.vazia + p.sem_ocr + p.com_erro;
  const o = ocr ?? { processos: 0, blocos: 0, paginas_em_bloco: 0, refacoes: 0, prazos: 0,
    bloco_medio_ms: 0, bloco_mediana_ms: 0, bloco_p95_ms: 0, pagina_estimada_ms: 0 };
  const ocrParede = o.blocos > 0 ? Math.round(totalMs * 0.6) : 0;
  const naoLidos = Object.entries(porLeitura)
    .filter(([k]) => !['lido', 'segundo_leitor', 'ocr'].includes(k))
    .reduce((s, [, n]) => s + n, 0);
  return {
    documentos: { total: docs, lidos: docs - naoLidos, nao_lidos: naoLidos, omitidos: 0, duplicados_h7: 0, pdf, html, por_leitura: porLeitura },
    paginas: p,
    tempo: {
      total_ms: totalMs, abrir_ms: 40, pre_contagem_ms: 310, leitura_parede_ms: totalMs - 600, leitura_soma_ms: Math.round(totalMs * 2.4),
      ocr_trabalho_ms: o.blocos * o.bloco_medio_ms, ocr_parede_ms: ocrParede, formatacao_ms: 180, gravacao_ms: 20,
    },
    ocr: o,
    limpeza: { nivel: 'standard', bytes_antes: Math.round(bytesTxt * 1.22), bytes_depois: bytesTxt, bytes_duplicatas: 0,
      tokens_antes: Math.round(tokens * 1.22), tokens_depois: tokens },
    bytes_txt: bytesTxt,
    tokens,
    ...(atencao ? { atencao } : {}),
  };
}

const concluido = (n, rel) => item(n, 'success', {
  progress: 100,
  message: 'Concluído',
  output_path: `${CONFIG.monitored_path}\\${processo(n)}.txt`,
  docs_count: rel.documentos.total, pages_count: rel.paginas.total, estimated_tokens: rel.tokens,
  processed_docs: rel.documentos.total, total_docs: rel.documentos.total,
  processed_pages: rel.paginas.total, total_pages: rel.paginas.total,
  unread_docs: rel.documentos.nao_lidos,
  relatorio: rel,
});

// Fila do estado `relatorio`: um ZIP só com texto nativo, um com OCR e páginas
// em branco (nenhum aviso) e um com os três avisos e documentos para conferir.
const FILA_RELATORIO = [
  concluido(11, relatorioZip({
    docs: 57, pdf: 55, html: 2, porLeitura: { lido: 57 },
    paginas: { lida_nativa: 142 }, totalMs: 8400, bytesTxt: 393800, tokens: 98450,
  })),
  concluido(12, relatorioZip({
    docs: 31, pdf: 31, html: 0, porLeitura: { lido: 24, ocr: 7 },
    paginas: { lida_nativa: 60, lida_ocr: 24, vazia: 4, ocr_sem_ganho: 1 },
    ocr: { processos: 7, blocos: 7, paginas_em_bloco: 25, refacoes: 0, prazos: 0,
      bloco_medio_ms: 9800, bloco_mediana_ms: 9100, bloco_p95_ms: 16400, pagina_estimada_ms: 2744 },
    totalMs: 41200, bytesTxt: 251300, tokens: 62800,
  })),
  concluido(13, relatorioZip({
    docs: 18, pdf: 18, html: 0, porLeitura: { lido: 15, ocr: 2, sem_ocr: 1 },
    paginas: { lida_nativa: 48, lida_ocr: 6, vazia: 2, sem_ocr: 5, com_erro: 3, ocr_falhou_com_reserva: 2, de_nao_lidos: 3 },
    ocr: { processos: 3, blocos: 3, paginas_em_bloco: 11, refacoes: 1, prazos: 1,
      bloco_medio_ms: 12600, bloco_mediana_ms: 11800, bloco_p95_ms: 30000, pagina_estimada_ms: 3436 },
    atencao: [
      { nome: 'Evento 4 - ANEXO1.pdf', evento: 4, sem_ocr: [2, 3, 4, 5, 6], total_sem_ocr: 5 },
      { nome: 'Evento 9 - LAUDO2.pdf', evento: 9, com_erro: [1, 2, 3], total_com_erro: 3,
        ocr_falhou_com_reserva: [5, 9], total_ocr_falhou_com_reserva: 2 },
    ],
    totalMs: 52800, bytesTxt: 140200, tokens: 35050,
  })),
];

// Relatórios dos concluídos de `fila` e `nao-lidos`, com os números de sucesso():
// no app real todo item concluído traz o seu.
const REL_NATIVO = relatorioZip({
  docs: 57, pdf: 55, html: 2, porLeitura: { lido: 57 },
  paginas: { lida_nativa: 142 }, totalMs: 8400, bytesTxt: 393800, tokens: 98450,
});
// Os mesmos documentos de atividadeNaoLidos: um sem OCR e um com senha (as
// páginas deles ficam em de_nao_lidos); o sem OCR acende o "Ler com OCR".
const REL_NAO_LIDOS = relatorioZip({
  docs: 57, pdf: 55, html: 2, porLeitura: { lido: 52, segundo_leitor: 1, ocr: 2, sem_ocr: 1, senha: 1 },
  paginas: { lida_nativa: 131, lida_ocr: 8, de_nao_lidos: 3 },
  ocr: { processos: 2, blocos: 2, paginas_em_bloco: 8, refacoes: 0, prazos: 0,
    bloco_medio_ms: 8800, bloco_mediana_ms: 8800, bloco_p95_ms: 9900, pagina_estimada_ms: 2200 },
  totalMs: 21300, bytesTxt: 393800, tokens: 98450,
});

// listaAltoContraste: states captured with forced colors, from review.json
// (visual.alto_contraste). A missing list or key is an empty list.
export function listaAltoContraste(config) {
  const lista = config?.visual?.alto_contraste;
  return Array.isArray(lista) ? lista : [];
}

// Estado visual do bloco Atualizações: o painel de configurações aberto.
function estadoAtualizacao(extra, fila = []) {
  return { config: CONFIG, fila, pausado: false, LogAtivo: false, atualizacao: atualizacao(extra) };
}

export const ESTADOS = {
  vazio: { config: { ...CONFIG, monitored_path: '', watchdog_enabled: false }, fila: [], pausado: false },
  fila: {
    config: CONFIG,
    fila: [{ ...sucesso(1), relatorio: REL_NATIVO }, { ...sucesso(2), relatorio: REL_NATIVO }, item(3, 'pending'), item(4, 'pending'),
      item(5, 'cancelled', { message: 'Cancelado pelo usuário' })],
    pausado: false,
  },
  processando: {
    config: CONFIG,
    fila: [sucesso(1), processando(2, 42), item(3, 'pending'), item(4, 'pending')],
    pausado: false,
    atividades: { [item(2, 'processing').id]: atividade(2) },
  },
  erro: {
    config: CONFIG,
    fila: [
      item(1, 'error', { progress: 12, message: 'Falha', error: 'Arquivo ZIP corrompido ou incompleto: zip: not a valid zip file' }),
      sucesso(2),
      processando(3, 75),
    ],
    pausado: true,
  },
  'nao-lidos': {
    config: CONFIG,
    fila: [{ ...sucesso(1), unread_docs: 2, relatorio: REL_NAO_LIDOS }, { ...sucesso(2), relatorio: REL_NATIVO }],
    pausado: false,
    atividades: { [item(1, 'success').id]: atividadeNaoLidos(1) },
  },
  configuracoes: {
    config: { ...CONFIG, monitored_path: CAMINHO_LONGO, ocr_mode: 'best', cleaning_level: 'basic', forcar_ocr: false },
    fila: [],
    pausado: false,
    LogAtivo: false,
    // Bloco Atualizações (EstadoAtualizacao): download em curso, com a barra.
    atualizacao: atualizacao({ fase: 'baixando', progresso: 42 }),
  },
  // Bloco Atualizações (EstadoAtualizacao), um estado por situação. A fila de
  // fundo é a mesma do estado fila, salvo onde o estado pede outra.
  'atualizacao-em-dia': estadoAtualizacao({ fase: 'em_dia', versao: VERSAO_ATUAL }),
  'atualizacao-disponivel': estadoAtualizacao({ fase: 'disponivel' }),
  'atualizacao-instalador': estadoAtualizacao({ fase: 'disponivel', modo: 'instalador' }),
  'atualizacao-so-avisar': estadoAtualizacao({ fase: 'disponivel', modo: 'so_avisar', motivo: 'executável fora da pasta de instalação' }),
  'atualizacao-incompleta': estadoAtualizacao({ fase: 'incompleta' }),
  'atualizacao-baixando': estadoAtualizacao({ fase: 'baixando', progresso: 42 }),
  'atualizacao-instalada': estadoAtualizacao({ fase: 'instalada' }),
  'atualizacao-aguardando-fila': estadoAtualizacao({ fase: 'aguardando_fila' }, [sucesso(1), processando(2, 42), item(3, 'pending')]),
  'atualizacao-erro': estadoAtualizacao({ fase: 'erro', modo: '', versao: '', mensagem: 'Não foi possível consultar o GitHub agora' }),
  // Troca em curso: o Reprocessar do item concluído fica desativado.
  'atualizacao-reiniciando': estadoAtualizacao({ fase: 'reiniciando' }, [{ ...sucesso(1), relatorio: REL_NATIVO }, sucesso(2)]),
  relatorio: {
    config: CONFIG,
    fila: FILA_RELATORIO,
    pausado: false,
  },
  // Log ligado (EPROCIA_LOG): o bloco com "Abrir pasta de logs" aparece no fim do painel.
  'configuracoes-log': {
    config: { ...CONFIG, forcar_ocr: true, ocr_mode: 'normal' },
    fila: [],
    pausado: false,
    LogAtivo: true,
  },
  // OCR desligado: o interruptor Forçar OCR fica marcado e desabilitado, com a ajuda.
  'configuracoes-none': {
    config: { ...CONFIG, forcar_ocr: true, ocr_mode: 'none' },
    fila: [],
    pausado: false,
    LogAtivo: false,
  },
  // Categoria Processamento recolhida (eprocia.categorias, lib/categorias.js),
  // com o log ligado para o bloco Sistema mostrar o conteúdo todo.
  'configuracoes-recolhida': {
    config: { ...CONFIG, ocr_mode: 'normal' },
    fila: [],
    pausado: false,
    LogAtivo: true,
    categorias: { monitoramento: false, processamento: true, sistema: false },
    // Com Processamento recolhida o bloco Atualizações cabe a 768px: versão nova disponível.
    atualizacao: atualizacao({ fase: 'disponivel' }),
  },
};

// instalarStubs runs in the browser (page.addInitScript) before any app code.
// It replaces the Wails bindings (window.go.main.App) and runtime (window.runtime)
// with fakes that serve the given state. Unknown calls resolve to undefined.
export function instalarStubs({ estado, tema, versao }) {
  localStorage.setItem('theme', tema === 'claro' ? 'light' : 'dark');
  // Sem nada salvo, o app abre com o painel recolhido e as categorias fechadas
  // (lib/painel.js, lib/categorias.js). As capturas mostram o conteúdo de cada
  // estado: gravam o painel aberto (320 px) e as categorias expandidas, salvo a
  // que o estado pede recolhida. Só quando a chave falta: este script roda de
  // novo no page.reload(), e o que a captura gravou antes dele fica.
  if (localStorage.getItem('eprocia.sidebar') === null) {
    localStorage.setItem('eprocia.sidebar', JSON.stringify({ largura: 320, recolhido: false }));
  }
  if (localStorage.getItem('eprocia.categorias') === null) {
    const abertas = { monitoramento: false, processamento: false, sistema: false, atualizacoes: false };
    localStorage.setItem('eprocia.categorias', JSON.stringify({ ...abertas, ...estado.categorias }));
  }
  const dados = {
    GetConfig: estado.config,
    GetQueue: estado.fila,
    IsQueuePaused: estado.pausado,
    // Sem isto a chamada devolve undefined, o app toma a janela como oculta
    // e as capturas saem com spinner, pulso e realce parados.
    JanelaVisivel: true,
    GetQueueActivity: (id) => estado.atividades?.[id] ?? null,
    // Log desligado salvo quando o estado pede (EPROCIA_LOG só existe no app real).
    LogAtivo: estado.LogAtivo === true,
    // Sem a chave no estado, atualizador ligado e sem consulta (fase vazia).
    EstadoAtualizacao: estado.atualizacao ?? { ativo: true, fase: '', versao: '', progresso: 0, mensagem: '', modo: '', motivo: '', atual: versao ?? '' },
    GetVersao: versao ?? '',
    AbrirPastaLogs: undefined,
    ReprocessQueueItem: (_id, _ocr, _limpeza, _forcar) => undefined,
    SelectZipFiles: [],
    SelectDirectory: '',
    SelectZipFile: '',
  };
  const app = new Proxy({}, {
    get: (_, nome) => (...args) => Promise.resolve(structuredClone(
      typeof dados[nome] === 'function' ? dados[nome](...args) : dados[nome])),
  });
  window.go = { main: { App: app } };
  window.runtime = new Proxy({}, {
    get: (_, nome) => {
      if (nome === 'EventsOnMultiple') return () => () => {};
      return () => undefined;
    },
  });
  window.alert = () => {};
}
