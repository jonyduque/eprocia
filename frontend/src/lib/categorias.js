// Categorias recolhidas do painel de Configurações, salvas entre sessões.
// Sem storage (ou com o valor salvo ilegível), todas ficam expandidas.

export const CHAVE_CATEGORIAS = 'eprocia.categorias';
export const CATEGORIAS = Object.freeze(['monitoramento', 'processamento', 'sistema']);

// { monitoramento, processamento, sistema }: true = recolhida.
export function lerCategorias(storage) {
  let salvo = null;
  try {
    salvo = JSON.parse(storage?.getItem(CHAVE_CATEGORIAS) ?? 'null');
  } catch {
    salvo = null;
  }
  const valido = salvo && typeof salvo === 'object' && !Array.isArray(salvo);
  return Object.fromEntries(CATEGORIAS.map((c) => [c, valido && typeof salvo[c] === 'boolean' ? salvo[c] : false]));
}

export function gravarCategorias(storage, recolhidas) {
  if (!storage) return false;
  try {
    storage.setItem(CHAVE_CATEGORIAS, JSON.stringify(Object.fromEntries(CATEGORIAS.map((c) => [c, recolhidas?.[c] === true]))));
    return true;
  } catch {
    return false;
  }
}
