// Espelham --dur-fast e --dur-base de style.css: transições do Svelte rodam
// em JS e não leem variáveis CSS.
export const DUR_FAST = 150;
export const DUR_BASE = 220;

export function duracaoMovimento(ms, janela = globalThis) {
  const reduzir = janela?.matchMedia?.('(prefers-reduced-motion: reduce)')?.matches;
  return reduzir ? 0 : ms;
}

// Um único timer por aviso: cada mensagem nova cancela o anterior, para que
// o prazo de uma mensagem antiga não apague a atual. Erros não têm prazo
// (WCAG 2.2.1): ficam até o usuário fechar ou chegar outra mensagem.
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
      if (tipo === 'error') return;
      id = relogio.setTimeout(() => {
        id = null;
        definir(null);
      }, duracao);
    },
    fechar() {
      cancelar();
      definir(null);
    },
    limpar: cancelar,
  };
}

// Confirmação curta ao lado do controle salvo. O n muda a cada chamada para
// que salvar de novo o mesmo campo, com o sinal ainda na tela, o refaça.
export function criarConfirmacao(definir, duracao = 1500, relogio = globalThis) {
  let id = null;
  let n = 0;

  function cancelar() {
    if (id !== null) {
      relogio.clearTimeout(id);
      id = null;
    }
  }

  return {
    mostrar(campo) {
      cancelar();
      definir({ campo, n: ++n });
      id = relogio.setTimeout(() => {
        id = null;
        definir(null);
      }, duracao);
    },
    limpar: cancelar,
  };
}

// ZIP do Eproc e PDF avulso, sem diferença de caixa. O Go recusa o resto de
// novo (AddFilesComOrigem); aqui é só para avisar sem esperar a fila.
export function separarEntradas(caminhos) {
  const lista = caminhos || [];
  const aceitos = lista.filter((c) => /\.(zip|pdf)$/i.test(c));
  return { aceitos, ignorados: lista.length - aceitos.length };
}

// Nada aceito é erro; parte aceita é só atenção, porque os outros seguiram.
export function avisoSoltura(aceitos, ignorados) {
  if (!ignorados) return null;
  if (!aceitos) return { texto: `Arraste arquivos ZIP do Eproc ou PDF. Ignorados: ${ignorados}`, tipo: 'error' };
  const qtd = ignorados === 1 ? '1 arquivo ignorado' : `${ignorados} arquivos ignorados`;
  return { texto: `${qtd}: só ZIP do Eproc ou PDF`, tipo: 'aviso' };
}
