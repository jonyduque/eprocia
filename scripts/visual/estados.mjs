// Fictitious data for each captured UI state. Nothing here may resemble a real
// process: numbers use the 5000000-00.2026.4.00.xxxx pattern, paths are made up.

const BASE_TS = 1767322800000; // 2026-01-02 03:00 UTC, fixed so captures are stable

const CONFIG = {
  monitored_path: 'C:\\Usuarios\\exemplo\\Downloads\\Processos',
  ocr_mode: 'normal',
  cleaning_level: 'aggressive',
  auto_start: true,
  watchdog_enabled: true,
};

export function processo(n) {
  return `5000000-00.2026.4.00.${String(n).padStart(4, '0')}`;
}

function item(n, status, extra = {}) {
  const nome = `${processo(n)}.zip`;
  return {
    id: `00000000-0000-4000-8000-${String(n).padStart(12, '0')}`,
    name: nome,
    path: `${CONFIG.monitored_path}\\${nome}`,
    status,
    progress: 0,
    message: '',
    output_path: '',
    docs_count: 0,
    pages_count: 0,
    processed_docs: 0,
    total_docs: 0,
    processed_pages: 0,
    total_pages: 0,
    timestamp: BASE_TS + n * 60000,
    ...extra,
  };
}

const sucesso = (n) => item(n, 'success', {
  progress: 100,
  message: 'Concluído',
  output_path: `${CONFIG.monitored_path}\\${processo(n)}.txt`,
  docs_count: 57, pages_count: 142, processed_docs: 57, total_docs: 57, processed_pages: 142, total_pages: 142,
});

const processando = (n, pct) => item(n, 'processing', {
  progress: pct,
  message: `Processando documento ${Math.round(57 * pct / 100)} de 57`,
  processed_docs: Math.round(57 * pct / 100), total_docs: 57,
  processed_pages: Math.round(142 * pct / 100), total_pages: 142,
});

export const ESTADOS = {
  vazio: { config: { ...CONFIG, monitored_path: '', watchdog_enabled: false }, fila: [], pausado: false },
  fila: {
    config: CONFIG,
    fila: [sucesso(1), sucesso(2), item(3, 'pending'), item(4, 'pending'), item(5, 'cancelled', { message: 'Cancelado pelo usuário' })],
    pausado: false,
  },
  processando: {
    config: CONFIG,
    fila: [sucesso(1), processando(2, 42), item(3, 'pending'), item(4, 'pending')],
    pausado: false,
  },
  erro: {
    config: CONFIG,
    fila: [
      item(1, 'error', { progress: 12, message: 'Falha', error: 'Arquivo ZIP corrompido ou incompleto: zip: not a valid zip file' }),
      sucesso(2),
      processando(3, 75),
    ],
    pausado: true,
  },
  configuracoes: {
    config: { ...CONFIG, monitored_path: 'C:\\Usuarios\\exemplo\\OneDrive - Empresa Exemplo\\Área de Trabalho\\Pasta com um nome bem longo\\Processos baixados', ocr_mode: 'best', cleaning_level: 'basic' },
    fila: [],
    pausado: false,
  },
};

// instalarStubs runs in the browser (page.addInitScript) before any app code.
// It replaces the Wails bindings (window.go.main.App) and runtime (window.runtime)
// with fakes that serve the given state. Unknown calls resolve to undefined.
export function instalarStubs({ estado, tema }) {
  localStorage.setItem('theme', tema === 'claro' ? 'light' : 'dark');
  const dados = {
    GetConfig: estado.config,
    GetQueue: estado.fila,
    IsQueuePaused: estado.pausado,
    SelectZipFiles: [],
    SelectDirectory: '',
    SelectZipFile: '',
  };
  const app = new Proxy({}, {
    get: (_, nome) => () => Promise.resolve(structuredClone(dados[nome])),
  });
  window.go = { main: { App: app } };
  window.runtime = new Proxy({}, {
    get: (_, nome) => {
      if (nome === 'EventsOnMultiple') return () => () => {};
      return () => undefined;
    },
  });
  window.alert = () => {};
}
