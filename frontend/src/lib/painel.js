// Largura e estado recolhido do painel lateral, salvos entre sessões.
import { limitarPainel } from './divisor.js';

export const CHAVE_PAINEL = 'eprocia.sidebar';
export const PAINEL_PADRAO = Object.freeze({ largura: 320, recolhido: false });

export function lerPainel(storage, larguraJanela) {
  let salvo = null;
  try {
    salvo = JSON.parse(storage?.getItem(CHAVE_PAINEL) ?? 'null');
  } catch {
    salvo = null;
  }
  const largura = Number.isFinite(salvo?.largura) ? salvo.largura : PAINEL_PADRAO.largura;
  return {
    largura: limitarPainel(largura, larguraJanela),
    recolhido: typeof salvo?.recolhido === 'boolean' ? salvo.recolhido : PAINEL_PADRAO.recolhido,
  };
}

export function gravarPainel(storage, { largura, recolhido }) {
  if (!storage) return false;
  try {
    storage.setItem(CHAVE_PAINEL, JSON.stringify({ largura, recolhido }));
    return true;
  } catch {
    return false;
  }
}
