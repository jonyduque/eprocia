// Preferência de tema: 'system' segue o Windows; 'light' e 'dark' fixam.
// O script inline de index.html repete esta regra para aplicar a classe
// antes da montagem; mantenha os dois em sincronia.
export const TEMAS = ['system', 'light', 'dark'];

export const CONSULTA_ESCURO = '(prefers-color-scheme: dark)';

export function lerPreferencia(storage) {
  const salvo = storage?.getItem('theme');
  return TEMAS.includes(salvo) ? salvo : 'system';
}

export function proximoTema(atual) {
  const i = TEMAS.indexOf(atual);
  return TEMAS[(i + 1) % TEMAS.length];
}

export function temaEfetivo(preferencia, sistemaEscuro) {
  if (preferencia === 'system') return sistemaEscuro ? 'dark' : 'light';
  return preferencia;
}

export const NOME_TEMA = { system: 'sistema', light: 'claro', dark: 'escuro' };
