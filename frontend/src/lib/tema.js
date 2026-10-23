// Preferência de tema: 'system' segue o Windows; 'light' e 'dark' fixam.
// O script inline de index.html repete esta regra para aplicar a classe
// antes da montagem; mantenha os dois em sincronia.
export const TEMAS = ['system', 'light', 'dark'];

export const CONSULTA_ESCURO = '(prefers-color-scheme: dark)';

// Ler window.localStorage pode lançar (armazenamento bloqueado pelo WebView
// ou pela política do sistema); nesses casos o tema só não persiste.
export function armazenamento(janela = globalThis) {
  try {
    return janela.localStorage ?? null;
  } catch {
    return null;
  }
}

// Tema salvo válido, ou null se não houver.
export function lerSalvo(storage) {
  try {
    const salvo = storage?.getItem('theme');
    return TEMAS.includes(salvo) ? salvo : null;
  } catch {
    return null;
  }
}

export function lerPreferencia(storage) {
  return lerSalvo(storage) ?? 'system';
}

export function gravarSalvo(storage, tema) {
  if (!storage) return false;
  try {
    storage.setItem('theme', tema);
    return true;
  } catch {
    return false;
  }
}

// O localStorage aplica o tema antes da montagem; o config.json pinta o fundo
// da janela nativa. Na abertura, o localStorage vence e o config é alinhado;
// se o localStorage estiver vazio (limpo ou bloqueado), vale o do config.
// doConfig undefined significa config ilegível: não sobrescreve.
export function conciliarTema(local, doConfig) {
  const configValido = TEMAS.includes(doConfig);
  const tema = local ?? (configValido ? doConfig : 'system');
  return {
    tema,
    gravarConfig: configValido && tema !== doConfig,
    gravarLocal: local === null && tema !== 'system',
  };
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
