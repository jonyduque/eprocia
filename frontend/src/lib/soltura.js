// Estado do overlay de soltar arquivos (F-AN-01): null, arrastando, soltando
// (esperando o OnFileDrop do Wails) ou aceito (confirmação curta).

// Texto selecionado e links também disparam dragenter; só arquivos ligam o overlay.
export function temArquivos(dt) {
  const tipos = dt?.types;
  if (!tipos) return false;
  if (typeof tipos.includes === 'function') return tipos.includes('Files');
  if (typeof tipos.contains === 'function') return tipos.contains('Files');
  return Array.from(tipos).includes('Files');
}

export function textoAceitos(n) {
  return n === 1 ? '1 arquivo adicionado' : `${n} arquivos adicionados`;
}

// O drop do DOM chega antes do OnFileDrop; sem resposta em `espera` ms
// (soltura recusada pelo Wails), o overlay fecha sozinho.
export function criarSoltura(definir, relogio = globalThis, duracaoAceito = 220, espera = 1000) {
  // dragenter/dragleave disparam a cada filho cruzado: o contador evita
  // depender de clientX/clientY para saber se o ponteiro saiu da janela.
  let profundidade = 0;
  let id = null;

  function cancelar() {
    if (id !== null) {
      relogio.clearTimeout(id);
      id = null;
    }
  }

  function fechar() {
    cancelar();
    profundidade = 0;
    definir(null);
  }

  function depois(ms) {
    cancelar();
    id = relogio.setTimeout(() => {
      id = null;
      definir(null);
    }, ms);
  }

  return {
    entrar(dt) {
      if (!temArquivos(dt)) return false;
      cancelar();
      if (profundidade++ === 0) definir({ fase: 'arrastando' });
      return true;
    },
    sair(dt) {
      if (!temArquivos(dt) || profundidade === 0) return;
      if (--profundidade === 0) fechar();
    },
    soltar(dt) {
      if (!temArquivos(dt)) return;
      profundidade = 0;
      definir({ fase: 'soltando' });
      depois(espera);
    },
    recebido(aceitos) {
      profundidade = 0;
      if (!aceitos) {
        fechar();
        return;
      }
      definir({ fase: 'aceito', texto: textoAceitos(aceitos) });
      depois(duracaoAceito);
    },
    limpar: cancelar,
  };
}
