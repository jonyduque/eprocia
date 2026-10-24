// Limites do divisor entre o painel e a fila. O máximo desconta a largura
// mínima da fila para que, na janela mínima (800px, main.go), o cabeçalho da
// fila ainda caiba com todos os botões.

export const LARGURA_MIN_PAINEL = 280;
export const LARGURA_MAX_PAINEL = 480;
export const LARGURA_MIN_FILA = 340;

export function larguraMaxPainel(larguraJanela) {
  return Math.max(LARGURA_MIN_PAINEL, Math.min(LARGURA_MAX_PAINEL, larguraJanela - LARGURA_MIN_FILA));
}

export function limitarPainel(largura, larguraJanela) {
  return Math.min(larguraMaxPainel(larguraJanela), Math.max(LARGURA_MIN_PAINEL, largura));
}

export const PASSO_TECLADO = 10;

// Ação do teclado no divisor: nova largura ou recolher o painel; null deixa a
// tecla seguir o comportamento padrão (Tab, por exemplo).
export function teclaDivisor(tecla, largura, larguraJanela) {
  switch (tecla) {
    case 'ArrowLeft':
      return { largura: limitarPainel(largura - PASSO_TECLADO, larguraJanela) };
    case 'ArrowRight':
      return { largura: limitarPainel(largura + PASSO_TECLADO, larguraJanela) };
    case 'Home':
      return { largura: LARGURA_MIN_PAINEL };
    case 'End':
      return { largura: larguraMaxPainel(larguraJanela) };
    case 'Enter':
      return { recolher: true };
    default:
      return null;
  }
}

// Junta os movimentos do ponteiro num por quadro: a largura muda no máximo uma
// vez por pintura, sempre com a posição mais recente. Soltar aplica na hora a
// posição que ainda esperava o quadro, para a largura salva ser a final.
export function criarArrasto(aplicar, relogio = globalThis) {
  let id = 0;
  let x = 0;
  return {
    mover(novoX) {
      x = novoX;
      if (id) return;
      id = relogio.requestAnimationFrame(() => {
        id = 0;
        aplicar(x);
      });
    },
    soltar() {
      if (!id) return;
      relogio.cancelAnimationFrame(id);
      id = 0;
      aplicar(x);
    },
  };
}
