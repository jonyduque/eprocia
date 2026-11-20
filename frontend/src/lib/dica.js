// Dica dos botões de ícone (use:dica={texto}). O title nativo demora e nunca
// aparece no foco por teclado; aqui o ponteiro mostra depois de --dur-fast e o
// foco por teclado mostra na hora. Um único elemento na camada superior
// (Popover API), porque .list-card tem overflow: hidden. O nome acessível
// continua no aria-label do botão: a dica é só visual (aria-hidden).
import { DUR_FAST, duracaoMovimento } from './aviso.js';

export const MARGEM_DICA = 4;
export const DISTANCIA_DICA = 6;

// Abaixo do alvo, centralizada; sobe quando não cabe e fica dentro da janela.
export function posicaoDica(alvo, largura, altura, larguraJanela, alturaJanela) {
  let top = alvo.bottom + DISTANCIA_DICA;
  if (top + altura > alturaJanela - MARGEM_DICA) top = alvo.top - DISTANCIA_DICA - altura;
  const centro = alvo.left + alvo.width / 2;
  const maximo = larguraJanela - MARGEM_DICA - largura;
  const left = Math.max(MARGEM_DICA, Math.min(centro - largura / 2, maximo));
  return { left: Math.round(left), top: Math.round(Math.max(MARGEM_DICA, top)) };
}

let elemento = null;
let dono = null;
let timerEsconder;

function garantirElemento() {
  if (elemento) return elemento;
  elemento = document.createElement('div');
  elemento.className = 'dica';
  elemento.setAttribute('popover', 'manual');
  elemento.setAttribute('aria-hidden', 'true');
  document.body.appendChild(elemento);
  return elemento;
}

function mostrar(no, texto) {
  if (!texto || !no.isConnected) return;
  const el = garantirElemento();
  clearTimeout(timerEsconder);
  dono = no;
  el.textContent = texto;
  if (!el.matches(':popover-open')) el.showPopover();
  const r = no.getBoundingClientRect();
  const p = posicaoDica(r, el.offsetWidth, el.offsetHeight, window.innerWidth, window.innerHeight);
  el.style.left = `${p.left}px`;
  el.style.top = `${p.top}px`;
  requestAnimationFrame(() => {
    if (dono === no) el.classList.add('visivel');
  });
}

function esconder(no) {
  if (!elemento || dono !== no) return;
  dono = null;
  const el = elemento;
  el.classList.remove('visivel');
  clearTimeout(timerEsconder);
  timerEsconder = setTimeout(() => {
    if (dono === null && el.matches(':popover-open')) el.hidePopover();
  }, duracaoMovimento(DUR_FAST));
}

export function dica(no, texto) {
  let atual = texto;
  let timerMostrar;

  const aoEntrar = () => {
    clearTimeout(timerMostrar);
    timerMostrar = setTimeout(() => mostrar(no, atual), DUR_FAST);
  };
  const aoSair = () => {
    clearTimeout(timerMostrar);
    // O foco por teclado continua no botão: a dica fica.
    if (!no.matches(':focus-visible')) esconder(no);
  };
  const aoFocar = () => {
    if (no.matches(':focus-visible')) mostrar(no, atual);
  };
  const aoDesfocar = () => {
    clearTimeout(timerMostrar);
    esconder(no);
  };
  const aoTeclar = (e) => {
    if (e.key === 'Escape' && dono === no) esconder(no);
  };

  no.addEventListener('pointerenter', aoEntrar);
  no.addEventListener('pointerleave', aoSair);
  no.addEventListener('pointerdown', aoDesfocar);
  no.addEventListener('focus', aoFocar);
  no.addEventListener('blur', aoDesfocar);
  no.addEventListener('keydown', aoTeclar);

  return {
    update(novo) {
      atual = novo;
      if (dono === no && elemento) {
        if (atual) elemento.textContent = atual;
        else esconder(no);
      }
    },
    destroy() {
      clearTimeout(timerMostrar);
      no.removeEventListener('pointerenter', aoEntrar);
      no.removeEventListener('pointerleave', aoSair);
      no.removeEventListener('pointerdown', aoDesfocar);
      no.removeEventListener('focus', aoFocar);
      no.removeEventListener('blur', aoDesfocar);
      no.removeEventListener('keydown', aoTeclar);
      esconder(no);
    }
  };
}
