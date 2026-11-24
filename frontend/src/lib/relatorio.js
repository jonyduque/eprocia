// Relatório de processamento de um ZIP (QueueItem.relatorio, models.RelatorioZip):
// linha-resumo, avisos, decisão do botão "Ler com OCR" e formatação do painel.
// Item antigo ou sem sucesso não tem relatório: toda função aceita undefined.
import {
  textoPaginasSemOCR,
  textoPaginasComErro,
  textoOCRFalhouComReserva,
  textoDocsSemOCR,
  textoTotalPaginas,
  textoPaginasForaDoResumo,
  textoListaPaginas,
  PARTE_TEXTO_PDF,
  PARTE_OCR,
  PARTE_EM_BRANCO
} from './textos.js';

const num = (v) => (Number.isFinite(v) ? v : 0);

export function resumoPaginas(rel) {
  const p = rel?.paginas;
  const total = num(p?.total);
  if (total <= 0) return null;
  const partes = [
    [num(p.lida_nativa), PARTE_TEXTO_PDF],
    [num(p.lida_ocr), PARTE_OCR],
    [num(p.vazia), PARTE_EM_BRANCO]
  ]
    .filter(([n]) => n > 0)
    .map(([n, t]) => `${n} ${t}`);
  const cabeca = textoTotalPaginas(total);
  const base = partes.length > 0 ? `${cabeca}: ${partes.join(', ')}` : cabeca;
  // O contador da linha (pages_count) soma estas; o total do resumo, não.
  const fora = num(p.de_nao_lidos) + num(p.de_omitidos);
  return fora > 0 ? `${base}; ${textoPaginasForaDoResumo(fora)}` : base;
}

// Só o que pede atenção: página em branco é normal e nunca vira aviso.
const AVISOS = [
  ['sem_ocr', textoPaginasSemOCR],
  ['com_erro', textoPaginasComErro],
  ['ocr_falhou_com_reserva', textoOCRFalhouComReserva]
];

// O documento digitalizado não lido por falta de OCR vem primeiro: as páginas
// dele contam em de_nao_lidos e não gerariam aviso nenhum.
export function avisosPaginas(rel) {
  const p = rel?.paginas;
  if (!p) return [];
  const docs = num(rel?.documentos?.por_leitura?.sem_ocr);
  const deDocs = docs > 0 ? [{ tipo: 'docs_sem_ocr', texto: textoDocsSemOCR(docs) }] : [];
  return deDocs.concat(AVISOS.filter(([k]) => num(p[k]) > 0).map(([tipo, f]) => ({ tipo, texto: f(num(p[tipo])) })));
}

// Página candidata que ficou sem OCR, ou documento inteiro não lido por isso
// (as páginas dele contam em de_nao_lidos, não em sem_ocr).
export function mostrarLerComOCR(rel) {
  return num(rel?.paginas?.sem_ocr) > 0 || num(rel?.documentos?.por_leitura?.sem_ocr) > 0;
}

export function reducaoLimpeza(rel) {
  const l = rel?.limpeza;
  const antes = num(l?.bytes_antes);
  if (!l || l.nivel === 'none' || antes <= 0) return null;
  const pct = Math.round(((antes - num(l.bytes_depois)) / antes) * 100);
  return Math.min(100, Math.max(0, pct));
}

const decimal = (v) => v.toFixed(1).replace('.', ',');

export function formatarDuracao(ms) {
  const v = Math.max(0, num(ms));
  if (v === 0) return '0 s';
  if (v < 60_000) return `${decimal(v / 1000)} s`;
  const s = Math.round(v / 1000);
  return `${Math.floor(s / 60)} min ${s % 60} s`;
}

export function formatarBytes(b) {
  const v = Math.max(0, num(b));
  if (v < 1024) return `${v} B`;
  if (v < 1024 * 1024) return `${decimal(v / 1024)} KB`;
  return `${decimal(v / (1024 * 1024))} MB`;
}

const CASOS = [
  ['sem_ocr', 'total_sem_ocr'],
  ['com_erro', 'total_com_erro'],
  ['ocr_falhou_com_reserva', 'total_ocr_falhou_com_reserva']
];

// Documentos com páginas para conferir: nome, evento e, por caso, os números
// das páginas (a lista vem limitada; o total diz quantas ficaram de fora).
export function linhasAtencao(rel) {
  const lista = Array.isArray(rel?.atencao) ? rel.atencao : [];
  return lista.map((d) => ({
    nome: d.nome ?? '',
    evento: num(d.evento),
    casos: CASOS.filter(([k]) => Array.isArray(d[k]) && d[k].length > 0).map(([tipo, kt]) => ({
      tipo,
      paginas: textoListaPaginas(d[tipo], num(d[kt]))
    }))
  }));
}
