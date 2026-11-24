// Textos novos de interface do relatório de processamento, do Forçar OCR e do
// log (plano 2026-11-24). Ficam juntos para o dono revisar e trocar depois de
// ver as capturas, sem caçar literais nos componentes.

function plural(n, um, varios) {
  return n === 1 ? um : varios;
}

// --- Linha-resumo e avisos do item concluído ---------------------------------

export function textoPaginasSemOCR(n) {
  return plural(n, '1 página tem imagem e o OCR estava desligado', `${n} páginas têm imagem e o OCR estava desligado`);
}

export function textoPaginasComErro(n) {
  return plural(n, '1 página não pôde ser lida', `${n} páginas não puderam ser lidas`);
}

export function textoOCRFalhouComReserva(n) {
  return plural(
    n,
    '1 página ficou com o texto do PDF porque o OCR falhou',
    `${n} páginas ficaram com o texto do PDF porque o OCR falhou`
  );
}

export function textoTotalPaginas(n) {
  return plural(n, '1 página', `${n} páginas`);
}

export const PARTE_TEXTO_PDF = 'do texto do PDF';
export const PARTE_OCR = 'por OCR';
export const PARTE_EM_BRANCO = 'em branco';

export const EXPLICACAO_OCR =
  'Página em branco ou só com texto não vai ao OCR: isso é normal. O aviso aparece só quando a página tem imagem ou desenho e o texto do PDF não bastou.';

export const ROTULO_LER_COM_OCR = 'Ler com OCR';
export const DICA_LER_COM_OCR = 'Processar este ZIP de novo com o OCR ligado, só nas páginas que precisam';
export const DICA_ABRIR_RELATORIO = 'Ver os detalhes do processamento';
export const DICA_FECHAR_RELATORIO = 'Ocultar os detalhes do processamento';

// --- Painel de detalhes (RelatorioItem) ---------------------------------------

export const ROTULO_PAINEL_RELATORIO = 'Detalhes do processamento';
export const ROTULO_TEXTO_PDF = 'Texto do PDF';
export const ROTULO_LIDA_OCR = 'Lida por OCR';
export const ROTULO_EM_BRANCO = 'Em branco';
export const ROTULO_SEM_OCR = 'Precisava de OCR e ficou sem';
export const ROTULO_COM_ERRO = 'Com erro';
export const ROTULO_OCR_FALHOU_COM_RESERVA = 'OCR falhou, ficou o texto do PDF';
export const ROTULO_TEMPO_TOTAL = 'Tempo total';
export const ROTULO_TEMPO_OCR = 'Tempo de trabalho de OCR';
export const NOTA_TEMPO_OCR = 'Soma dos processos de OCR em paralelo; pode passar do tempo total.';
export const ROTULO_ESTIMATIVA_PAGINA = 'Estimativa por página';
export const ROTULO_LIMPEZA = 'Limpeza';
export const ROTULO_PAGINAS_REPETIDAS = 'Páginas repetidas';
export const ROTULO_TIPOS_OMITIDOS = 'Tipos omitidos';
export const TITULO_PAGINAS = 'Páginas';
export const TITULO_ATENCAO = 'Documentos para conferir no PDF original';

export function textoReducao(pct) {
  return `${pct}% menor`;
}

// "pág. 3", "págs. 3, 7, 9", e "e mais N" quando a lista veio cortada (o
// relatório traz no máximo os 50 primeiros números de cada caso).
export function textoListaPaginas(lista, total) {
  const nums = Array.isArray(lista) ? lista : [];
  const n = Math.max(total || 0, nums.length);
  const base = `${n === 1 ? 'pág.' : 'págs.'} ${nums.join(', ')}`;
  const resto = n - nums.length;
  return resto > 0 ? `${base} e mais ${resto}` : base;
}

// --- Task 16: Forçar OCR e log --------------------------------------------------

export const ROTULO_FORCAR_OCR_CONFIG = 'Forçar OCR em todas as páginas de PDF';
export const AJUDA_FORCAR_OCR_CONFIG =
  'Cada página passa pelo OCR, inclusive as que já têm texto: o processamento fica bem mais lento e, numa página com texto bom, o resultado pode sair um pouco pior. Páginas repetidas também passam pelo OCR.';
export const ROTULO_FORCAR_OCR_REPROCESSAR = 'Forçar OCR em todas as páginas';
export const AJUDA_FORCAR_OCR_REPROCESSAR = 'Mais lento; páginas repetidas também passam pelo OCR.';
export const ROTULO_ABRIR_LOGS = 'Abrir pasta de logs';
export const TEXTO_LOG_ATIVO = 'Log e telemetria ativos. Estes arquivos ficam só neste computador.';
