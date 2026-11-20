// Estado do monitoramento com um vocabulário só, usado no painel, no rodapé
// e na barra recolhida (os ícones ficam nos componentes).
export const ROTULO_MONITORAMENTO = {
  monitorando: 'Monitorando',
  pausado: 'Pausado',
  sem_pasta: 'Sem pasta'
};

// Sem pasta não há o que monitorar, mesmo com watchdog_enabled gravado true.
export function estadoMonitoramento(config) {
  if (!config?.monitored_path) return 'sem_pasta';
  return config.watchdog_enabled ? 'monitorando' : 'pausado';
}

export function textoMonitoramento(config) {
  const estado = estadoMonitoramento(config);
  const rotulo = ROTULO_MONITORAMENTO[estado];
  return estado === 'sem_pasta' ? rotulo : `${rotulo}: ${config.monitored_path}`;
}

// Reticências no meio, guardando mais do fim: em caminhos é o nome da pasta
// que diferencia um do outro. `cabe` é o máximo de caracteres ou uma função
// que diz se o texto cabe; procura o maior corte que ainda cabe.
export function truncarMeio(texto, cabe) {
  const caberia = typeof cabe === 'number' ? (s) => s.length <= cabe : cabe;
  if (!texto || caberia(texto)) return texto;
  const montar = (n) => {
    const ini = Math.floor(n * 0.4);
    return texto.slice(0, ini) + '…' + texto.slice(texto.length - (n - ini));
  };
  let lo = 0;
  let hi = texto.length - 1;
  while (lo < hi) {
    const meio = Math.ceil((lo + hi) / 2);
    if (caberia(montar(meio))) lo = meio;
    else hi = meio - 1;
  }
  return montar(lo);
}

// Ação Svelte: escreve o texto truncado no meio para caber na largura do nó,
// medindo com a fonte calculada; refaz quando o nó muda de tamanho.
export function reticenciasMeio(node, texto) {
  const ctx = document.createElement('canvas').getContext('2d');
  let atual = texto;
  const aplicar = () => {
    const largura = node.clientWidth;
    if (!ctx || !largura) {
      node.textContent = atual ?? '';
      return;
    }
    ctx.font = getComputedStyle(node).font;
    node.textContent = truncarMeio(atual ?? '', (s) => ctx.measureText(s).width <= largura);
  };
  const obs = typeof ResizeObserver === 'function' ? new ResizeObserver(aplicar) : null;
  obs?.observe(node);
  aplicar();
  return {
    update(novo) {
      atual = novo;
      aplicar();
    },
    destroy() {
      obs?.disconnect();
    }
  };
}
