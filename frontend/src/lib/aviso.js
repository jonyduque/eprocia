// Espelham --dur-fast e --dur-base de style.css: transições do Svelte rodam
// em JS e não leem variáveis CSS.
export const DUR_FAST = 150;
export const DUR_BASE = 220;

export function duracaoMovimento(ms, janela = globalThis) {
  const reduzir = janela?.matchMedia?.('(prefers-reduced-motion: reduce)')?.matches;
  return reduzir ? 0 : ms;
}

// Um único timer por aviso: cada mensagem nova cancela o anterior, para que
// o prazo de uma mensagem antiga não apague a atual.
export function criarAviso(definir, duracao = 3000, relogio = globalThis) {
  let id = null;

  function cancelar() {
    if (id !== null) {
      relogio.clearTimeout(id);
      id = null;
    }
  }

  return {
    mostrar(texto, tipo) {
      cancelar();
      definir({ texto, tipo });
      id = relogio.setTimeout(() => {
        id = null;
        definir(null);
      }, duracao);
    },
    limpar: cancelar,
  };
}
