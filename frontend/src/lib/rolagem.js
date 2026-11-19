// Estado de rolagem de uma área com overflow: há conteúdo escondido acima e/ou abaixo?
// A folga de 1px cobre o meio pixel que sobra com zoom fracionário do Windows.
export function estadoRolagem({ scrollTop, scrollHeight, clientHeight }) {
  return { acima: scrollTop > 0, abaixo: scrollTop + clientHeight < scrollHeight - 1 };
}

// Ação Svelte: chama aoMudar({ acima, abaixo }) na montagem e sempre que o estado
// muda, seja por rolagem, por mudança de tamanho da área ou do conteúdo.
export function rolagem(node, aoMudar) {
  let atual = null;
  const medir = () => {
    const e = estadoRolagem(node);
    if (atual && e.acima === atual.acima && e.abaixo === atual.abaixo) return;
    atual = e;
    aoMudar(e);
  };
  // O conteúdo cresce sem mudar o tamanho da área: observa também os filhos,
  // e troca a lista de filhos observados quando um bloco {#if} entra ou sai.
  const tamanho = new ResizeObserver(medir);
  const observarFilhos = () => {
    tamanho.disconnect();
    tamanho.observe(node);
    for (const filho of node.children) tamanho.observe(filho);
  };
  const filhos = new MutationObserver(() => {
    observarFilhos();
    medir();
  });
  observarFilhos();
  filhos.observe(node, { childList: true });
  node.addEventListener('scroll', medir, { passive: true });
  medir();
  return {
    update(novo) {
      aoMudar = novo;
    },
    destroy() {
      node.removeEventListener('scroll', medir);
      tamanho.disconnect();
      filhos.disconnect();
    },
  };
}
