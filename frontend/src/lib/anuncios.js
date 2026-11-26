// Textos da região viva única da fila (role="status"). Só marcos: contadores
// que mudam a cada queue:atividade não são anunciados.
import { resumoAtividade } from './atividade.js';

function docs(n) {
  return `${n} documento${n === 1 ? '' : 's'}`;
}

function naoLidos(n) {
  return n === 1 ? '1 documento não lido' : `${n} documentos não lidos`;
}

// anterior: { [id]: { status, erros } } da chamada anterior (null na primeira,
// que só registra o estado). Devolve o estado novo e as mensagens dos marcos.
export function marcosFila(anterior, items, atividades) {
  const estado = {};
  const mensagens = [];
  for (const item of items ?? []) {
    const ativ = atividades?.[item.id];
    const erros = ativ && typeof ativ === 'object' ? resumoAtividade(ativ).erros : 0;
    estado[item.id] = { status: item.status, erros };
    const antes = anterior?.[item.id];
    if (!antes) continue;
    if (antes.status !== item.status && item.status === 'success') {
      let msg = `${item.name}: concluído, ${docs(item.docs_count || 0)}`;
      if (item.unread_docs > 0) msg += `, ${naoLidos(item.unread_docs)}`;
      mensagens.push(msg);
    } else if (antes.status !== item.status && item.status === 'error') {
      mensagens.push(`${item.name}: erro no processamento`);
    } else if (item.status === 'processing' && erros > 0 && erros !== antes.erros) {
      mensagens.push(`${item.name}: ${docs(erros)} com erro`);
    }
  }
  return { estado, mensagens };
}

// Com documentos não lidos, o aviso role="alert" já confirma a cópia.
export function anuncioCopia(nome, naoLidosN) {
  return naoLidosN > 0 ? '' : `TXT de ${nome} copiado`;
}

// antes undefined: primeira leitura, sem anúncio.
export function anuncioPausa(antes, depois, textoPausa) {
  if (antes === undefined || antes === depois) return '';
  return depois ? (textoPausa || 'Fila pausada') : 'Fila retomada';
}

export function anuncioFiltro(texto, n) {
  if (!texto) return '';
  if (n === 0) return 'Nenhum item corresponde ao filtro';
  return n === 1 ? '1 item corresponde ao filtro' : `${n} itens correspondem ao filtro`;
}
