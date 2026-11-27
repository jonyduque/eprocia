// Categorias recolhidas do painel de Configurações, salvas entre sessões.
// Sem storage (ou com o valor salvo ilegível), todas começam recolhidas.

export const CHAVE_CATEGORIAS = 'eprocia.categorias';
export const CATEGORIAS = Object.freeze(['monitoramento', 'processamento', 'sistema', 'atualizacoes']);

// { monitoramento, processamento, sistema, atualizacoes }: true = recolhida.
// Chave ausente no valor salvo (de antes de uma categoria nova) fica recolhida,
// como as demais sem valor salvo; o que já foi salvo vale como está.
export function lerCategorias(storage) {
  let salvo = null;
  try {
    salvo = JSON.parse(storage?.getItem(CHAVE_CATEGORIAS) ?? 'null');
  } catch {
    salvo = null;
  }
  const valido = salvo && typeof salvo === 'object' && !Array.isArray(salvo);
  return Object.fromEntries(CATEGORIAS.map((c) => [c, valido && typeof salvo[c] === 'boolean' ? salvo[c] : true]));
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

// Atualizações abre sozinha quando o estado passa a "versão nova disponível",
// uma vez por versão nesta sessão: se o usuário recolher, só reabre com outra
// versão. É só em memória; a preferência salva continua a do usuário.
// memoria = { versao (a última que abriu a categoria), visto (já chegou um
// estado ativo) }. Se a versão já vem disponível no primeiro estado ativo
// (abertura do painel), abre sem animar; depois, anima como o clique.
export const ABERTURA_INICIAL = Object.freeze({ versao: '', visto: false });

export function aberturaAtualizacoes(estado, memoria = ABERTURA_INICIAL) {
  if (!estado || !estado.ativo) return { abrir: false, animar: false, memoria };
  const primeiro = !memoria.visto;
  if (estado.fase === 'disponivel' && estado.versao && estado.versao !== memoria.versao) {
    return { abrir: true, animar: !primeiro, memoria: { versao: estado.versao, visto: true } };
  }
  return { abrir: false, animar: false, memoria: { versao: memoria.versao, visto: true } };
}
