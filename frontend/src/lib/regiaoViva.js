// Região viva: esvazia antes de escrever (o mesmo texto de novo é lido de novo)
// e junta os anúncios que chegam no mesmo tick, para que nenhum se perca.
export function criarAnunciador(escrever, proximoTick) {
  let fila = [];
  let ciclo = null;
  return function anunciar(texto) {
    if (!texto) return Promise.resolve();
    fila.push(texto);
    if (ciclo) return ciclo;
    escrever('');
    // finally: um tick que falha não pode deixar o ciclo preso e calar a região.
    ciclo = (async () => {
      try {
        await proximoTick();
      } finally {
        const junto = fila.join('. ');
        fila = [];
        ciclo = null;
        escrever(junto);
      }
    })();
    return ciclo;
  };
}
