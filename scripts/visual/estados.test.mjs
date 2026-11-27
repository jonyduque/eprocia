import { describe, it } from 'node:test';
import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import { ESTADOS, instalarStubs, listaAltoContraste } from './estados.mjs';

// Fields of etl.QueueItem (internal/etl/queue.go) the UI reads.
const CAMPOS = ['id', 'name', 'path', 'status', 'progress', 'message', 'output_path', 'docs_count', 'pages_count', 'estimated_tokens',
  'processed_docs', 'total_docs', 'processed_pages', 'total_pages', 'timestamp', 'unread_docs', 'kind'];
const STATUS = new Set(['pending', 'processing', 'paused', 'cancelled', 'success', 'error', 'interrupted']);

// Chaves de models.RelatorioZip (internal/models, frontend/src/wailsjs/go/models.ts),
// sem as opcionais (omitempty).
const CHAVES_RELATORIO = {
  '': ['documentos', 'paginas', 'tempo', 'ocr', 'limpeza', 'bytes_txt', 'tokens'],
  documentos: ['total', 'lidos', 'nao_lidos', 'omitidos', 'duplicados_h7', 'pdf', 'html'],
  paginas: ['total', 'lida_nativa', 'lida_ocr', 'vazia', 'sem_ocr', 'com_erro', 'ocr_sem_ganho', 'ocr_falhou_com_reserva',
    'de_nao_lidos', 'de_omitidos', 'duplicadas'],
  tempo: ['total_ms', 'abrir_ms', 'pre_contagem_ms', 'leitura_parede_ms', 'leitura_soma_ms', 'ocr_trabalho_ms', 'ocr_parede_ms',
    'formatacao_ms', 'gravacao_ms'],
  ocr: ['processos', 'blocos', 'paginas_em_bloco', 'refacoes', 'prazos', 'bloco_medio_ms', 'bloco_mediana_ms', 'bloco_p95_ms',
    'pagina_estimada_ms'],
  limpeza: ['nivel', 'bytes_antes', 'bytes_depois', 'bytes_duplicatas', 'tokens_antes', 'tokens_depois'],
};
// As cinco categorias disjuntas da spec (2026-11-22-relatorio-processamento-design.md).
const CATEGORIAS = ['lida_nativa', 'lida_ocr', 'vazia', 'sem_ocr', 'com_erro'];
const ATUALIZACAO = ['atualizacao-em-dia', 'atualizacao-disponivel', 'atualizacao-instalador', 'atualizacao-so-avisar',
  'atualizacao-incompleta', 'atualizacao-baixando', 'atualizacao-instalada', 'atualizacao-aguardando-fila', 'atualizacao-erro',
  'atualizacao-reiniciando'];
const NOVOS = ['historico', 'relatorio', 'fila-pdf', 'configuracoes-log', 'configuracoes-none', 'configuracoes-recolhida', ...ATUALIZACAO];

describe('estados visuais', () => {
  it('cobre os estados da review.json', () => {
    assert.deepEqual(Object.keys(ESTADOS).sort(),
      ['configuracoes', 'configuracoes-log', 'configuracoes-none', 'configuracoes-recolhida', 'erro', 'fila', 'fila-pdf', 'historico', 'nao-lidos', 'processando', 'relatorio', 'vazio', ...ATUALIZACAO].sort());
  });

  it('relatorio: três concluídos, um só nativo, um com OCR e em branco, um com os três avisos e atencao', () => {
    const fila = ESTADOS.relatorio.fila;
    assert.equal(fila.length, 3);
    for (const i of fila) {
      assert.equal(i.status, 'success');
      assert.ok(i.relatorio, `${i.name}: sem relatorio`);
    }
    const [nativo, comOCR, avisos] = fila.map((i) => i.relatorio.paginas);
    assert.equal(nativo.lida_nativa, nativo.total);
    assert.ok(comOCR.lida_ocr > 0 && comOCR.vazia > 0);
    assert.equal(comOCR.sem_ocr + comOCR.com_erro + comOCR.ocr_falhou_com_reserva, 0);
    assert.ok(avisos.sem_ocr > 0 && avisos.com_erro > 0 && avisos.ocr_falhou_com_reserva > 0);
    assert.ok(fila[2].relatorio.atencao?.length > 0, 'atencao vazia');
  });

  it('historico: um item de hoje e três da sessão anterior em outro dia (concluído, TXT ausente, interrompido)', () => {
    const fila = ESTADOS.historico.fila;
    assert.equal(fila.length, 4);
    const dia = (ms) => new Date(ms).toISOString().slice(0, 10);
    const quando = (i) => i.finished_at || i.timestamp;
    const [hoje, ...anteriores] = fila;
    for (const i of anteriores) assert.notEqual(dia(quando(i)), dia(quando(hoje)), `${i.name}: mesmo dia de hoje`);
    const concluido = anteriores.find((i) => i.status === 'success' && !i.output_missing);
    const ausente = anteriores.find((i) => i.status === 'success' && i.output_missing === true);
    const interrompido = anteriores.find((i) => i.status === 'interrupted');
    assert.ok(concluido && concluido.finished_at > 0, 'concluído com finished_at');
    assert.ok(ausente, 'concluído com output_missing');
    assert.ok(interrompido, 'interrompido');
    // O histórico grava o relatório só com contagens (decisão d3 de 2026-11-26):
    // o concluído restaurado traz relatorio, sem atencao.
    assert.ok(concluido.relatorio?.paginas?.total > 0, 'concluído restaurado com relatorio');
    assert.equal(concluido.relatorio.atencao, undefined, 'relatorio restaurado sem atencao');
    for (const i of fila) assert.match(i.name, /^5000000-00\.2026\.4\.00\.\d{4}(\.zip)?$/);
  });

  it('fila-pdf: um ZIP, um PDF avulso e um item de três PDFs, todos concluídos', () => {
    const [zip, pdf, lote] = ESTADOS['fila-pdf'].fila;
    assert.equal(ESTADOS['fila-pdf'].fila.length, 3);
    for (const i of [zip, pdf, lote]) assert.equal(i.status, 'success');
    assert.equal(zip.kind, 'zip');
    assert.match(zip.name, /\.zip$/);
    assert.equal(pdf.kind, 'pdf');
    assert.match(pdf.name, /^5000000-00\.2026\.4\.00\.\d{4}$/);
    assert.equal(pdf.paths.length, 1);
    assert.equal(lote.kind, 'pdf');
    assert.match(lote.name, / \(3 PDFs\)$/);
    assert.equal(lote.paths.length, 3);
    for (const [nome, e] of Object.entries(ESTADOS)) {
      for (const i of e.fila) assert.ok(i.kind === 'zip' || i.kind === 'pdf', `${nome}/${i.name}: kind`);
    }
  });

  it('fila e nao-lidos: todo concluído tem relatorio, como no app real; nao-lidos mostra "Ler com OCR"', () => {
    for (const nome of ['fila', 'nao-lidos']) {
      for (const i of ESTADOS[nome].fila.filter((x) => x.status === 'success')) {
        assert.ok(i.relatorio, `${nome}/${i.name}: sem relatorio`);
        assert.equal(i.relatorio.documentos.total, i.docs_count, `${nome}/${i.name}: docs_count`);
        assert.equal(i.relatorio.documentos.nao_lidos, i.unread_docs, `${nome}/${i.name}: unread_docs`);
      }
    }
    assert.ok(ESTADOS['nao-lidos'].fila[0].relatorio.documentos.por_leitura.sem_ocr > 0);
  });

  it('atualização: versão atual do wails.json e a nova um minor acima', () => {
    const atual = JSON.parse(readFileSync(new URL('../../wails.json', import.meta.url), 'utf8')).info.productVersion;
    const [ma, mi] = atual.split('.').map(Number);
    for (const nome of ['configuracoes', 'configuracoes-recolhida', 'atualizacao-disponivel', 'atualizacao-baixando']) {
      const a = ESTADOS[nome].atualizacao;
      assert.equal(a.ativo, true, nome);
      assert.equal(a.atual, atual, nome);
      assert.equal(a.versao, [ma, mi + 1, 0].join('.'), nome);
    }
  });

  it('atualizacao-*: ativo, sem a chave anterior, e cada um com a fase e o modo do brief', () => {
    const esperado = {
      'atualizacao-em-dia': { fase: 'em_dia' },
      'atualizacao-disponivel': { fase: 'disponivel', modo: 'trocar' },
      'atualizacao-instalador': { fase: 'disponivel', modo: 'instalador' },
      'atualizacao-so-avisar': { fase: 'disponivel', modo: 'so_avisar' },
      'atualizacao-incompleta': { fase: 'incompleta' },
      'atualizacao-baixando': { fase: 'baixando', progresso: 42 },
      'atualizacao-instalada': { fase: 'instalada', modo: 'trocar', mensagem: '' },
      'atualizacao-aguardando-fila': { fase: 'aguardando_fila', modo: 'trocar' },
      'atualizacao-erro': { fase: 'erro', modo: '', mensagem: 'Não foi possível consultar o GitHub agora' },
      'atualizacao-reiniciando': { fase: 'reiniciando' },
    };
    assert.deepEqual(Object.keys(esperado).sort(), [...ATUALIZACAO].sort());
    for (const [nome, campos] of Object.entries(esperado)) {
      const a = ESTADOS[nome].atualizacao;
      assert.equal(a.ativo, true, nome);
      for (const [k, v] of Object.entries(campos)) assert.equal(a[k], v, nome + '.' + k);
    }
    assert.match(ESTADOS['atualizacao-so-avisar'].atualizacao.motivo, /fora da pasta de instalação/);
    // Atualizações salva recolhida: a captura mostra a categoria aberta sozinha
    // pela versão nova (lib/categorias.js, aberturaAtualizacoes).
    assert.deepEqual(ESTADOS['atualizacao-disponivel'].categorias, { atualizacoes: true });
    assert.ok(ESTADOS['atualizacao-aguardando-fila'].fila.some((i) => i.status === 'processing'));
    assert.ok(ESTADOS['atualizacao-reiniciando'].fila.some((i) => i.status === 'success'));
    for (const [nome, e] of Object.entries(ESTADOS)) {
      assert.ok(!('anterior' in e) && !('anterior' in (e.atualizacao ?? {})), nome + ': chave anterior');
    }
  });

  it('configuracoes: LogAtivo e forcar_ocr explícitos em cada variação', () => {
    const c = ESTADOS.configuracoes;
    assert.equal(c.LogAtivo, false);
    assert.equal(c.config.forcar_ocr, false);
    const log = ESTADOS['configuracoes-log'];
    assert.equal(log.LogAtivo, true);
    assert.equal(log.config.forcar_ocr, true);
    assert.equal(log.config.ocr_mode, 'normal');
    const none = ESTADOS['configuracoes-none'];
    assert.equal(none.config.forcar_ocr, true);
    assert.equal(none.config.ocr_mode, 'none');
  });

  for (const nome of NOVOS) {
    it(`${nome}: só processo(n) e C:\\Usuarios\\exemplo`, () => {
      const texto = JSON.stringify(ESTADOS[nome]);
      for (const m of texto.matchAll(/[A-Za-z]:\\\\[^"]*/g)) {
        assert.ok(m[0].startsWith('C:\\\\Usuarios\\\\exemplo'), `caminho fora do exemplo: ${m[0]}`);
      }
      for (const i of ESTADOS[nome].fila) {
        assert.match(i.name, /^5000000-00\.2026\.4\.00\.\d{4}(\.zip| \(\d+ PDFs\))?$/, `nome fora de processo(n): ${i.name}`);
        if (i.kind === 'zip') assert.match(i.name, /\.zip$/, `ZIP sem .zip no nome: ${i.name}`);
        for (const p of i.paths ?? []) assert.ok(p.startsWith('C:\\Usuarios\\exemplo'), `caminho fora do exemplo: ${p}`);
      }
    });
  }

  for (const [nome, e] of Object.entries(ESTADOS)) {
    it(`${nome}: itens com o formato de etl.QueueItem`, () => {
      for (const item of e.fila) {
        for (const c of CAMPOS) assert.ok(c in item, `${nome}: campo ${c} ausente`);
        assert.ok(STATUS.has(item.status), `${nome}: status ${item.status}`);
      }
      assert.equal(new Set(e.fila.map((i) => i.id)).size, e.fila.length, 'ids repetidos');
    });

    it(`${nome}: relatorio com as chaves de models.RelatorioZip e categorias que somam o total`, () => {
      for (const item of e.fila.filter((i) => i.relatorio !== undefined)) {
        const r = item.relatorio;
        for (const [parte, chaves] of Object.entries(CHAVES_RELATORIO)) {
          const obj = parte ? r[parte] : r;
          assert.ok(obj && typeof obj === 'object', `${item.name}: ${parte} ausente`);
          for (const k of chaves) assert.ok(k in obj, `${item.name}: ${parte}.${k} ausente`);
        }
        const soma = CATEGORIAS.reduce((s, k) => s + r.paginas[k], 0);
        assert.equal(soma, r.paginas.total, `${item.name}: soma das categorias`);
        assert.ok(r.paginas.ocr_sem_ganho + r.paginas.ocr_falhou_com_reserva <= r.paginas.lida_nativa,
          `${item.name}: sub-contadores maiores que lida_nativa`);
      }
    });

    it(`${nome}: só dados fictícios`, () => {
      const texto = JSON.stringify(e);
      for (const m of texto.matchAll(/\d{7}-\d{2}\.\d{4}\.\d\.\d{2}\.\d{4}/g)) {
        assert.match(m[0], /^5000000-00\.2026\.4\.00\./, `número de processo não fictício: ${m[0]}`);
      }
      assert.doesNotMatch(texto, /jonyd|Users\\\\[a-z]+\\\\/i, 'caminho de usuário real');
    });
  }

  it('stubs servem o estado e ignoram chamadas desconhecidas', async () => {
    const store = {};
    globalThis.window = globalThis;
    // Node 24 exposes its own localStorage getter; shadow it for the test.
    const original = Object.getOwnPropertyDescriptor(globalThis, 'localStorage');
    Object.defineProperty(globalThis, 'localStorage', { value: { getItem: (k) => (k in store ? store[k] : null), setItem: (k, v) => { store[k] = v; } }, configurable: true });
    try {
      instalarStubs({ estado: ESTADOS.fila, tema: 'claro', versao: '0.4.4' });
      assert.equal(store.theme, 'light');
      assert.equal((await window.go.main.App.GetQueue()).length, ESTADOS.fila.fila.length);
      assert.equal(await window.go.main.App.PauseQueue(), undefined);
      assert.equal(await window.go.main.App.GetQueueActivity('x'), null);
      assert.equal(typeof window.runtime.EventsOnMultiple('x', () => {}, -1), 'function');
      assert.equal(window.runtime.WindowMinimise(), undefined);
      // LogAtivo devolve um booleano (false sem a chave no estado); os outros
      // bindings do plano existem e resolvem sem erro.
      assert.equal(await window.go.main.App.LogAtivo(), false);
      // Sem a chave atualizacao, o atualizador fica ligado e sem consulta (fase vazia).
      assert.deepEqual(await window.go.main.App.EstadoAtualizacao(), { ativo: true, fase: '', versao: '', progresso: 0, mensagem: '', modo: '', motivo: '', atual: '0.4.4' });
      assert.equal(await window.go.main.App.GetVersao(), '0.4.4');
      assert.equal(await window.go.main.App.AbrirPastaLogs(), undefined);
      assert.equal(await window.go.main.App.ReprocessQueueItem('x', 'normal', 'standard', true), undefined);
      instalarStubs({ estado: ESTADOS.configuracoes, tema: 'escuro' });
      assert.deepEqual(await window.go.main.App.EstadoAtualizacao(), ESTADOS.configuracoes.atualizacao);
      instalarStubs({ estado: ESTADOS['configuracoes-log'], tema: 'escuro' });
      assert.equal(await window.go.main.App.LogAtivo(), true);
      // O app abre com o painel recolhido e as categorias fechadas quando não há
      // nada salvo: as capturas gravam o painel aberto e as categorias
      // expandidas, e só recolhem uma categoria quando o estado pede.
      const ABERTAS = { monitoramento: false, processamento: false, sistema: false, atualizacoes: false };
      assert.deepEqual(JSON.parse(store['eprocia.sidebar']), { largura: 320, recolhido: false });
      assert.deepEqual(JSON.parse(store['eprocia.categorias']), ABERTAS);
      for (const k of Object.keys(store)) if (k !== 'theme') delete store[k];
      instalarStubs({ estado: ESTADOS['configuracoes-recolhida'], tema: 'escuro' });
      assert.deepEqual(JSON.parse(store['eprocia.categorias']), { ...ABERTAS, processamento: true });
      // O init script roda de novo no page.reload(): o que a captura gravou fica.
      store['eprocia.sidebar'] = JSON.stringify({ largura: 400, recolhido: false });
      store['eprocia.categorias'] = JSON.stringify({ ...ABERTAS, sistema: true });
      instalarStubs({ estado: ESTADOS.configuracoes, tema: 'escuro' });
      assert.deepEqual(JSON.parse(store['eprocia.sidebar']), { largura: 400, recolhido: false });
      assert.deepEqual(JSON.parse(store['eprocia.categorias']), { ...ABERTAS, sistema: true });
    } finally {
      delete globalThis.go;
      delete globalThis.runtime;
      delete globalThis.localStorage;
      if (original) Object.defineProperty(globalThis, 'localStorage', original);
    }
  });
});

describe('lista de alto contraste', () => {
  const review = JSON.parse(readFileSync(new URL('../../.claude/review.json', import.meta.url), 'utf8'));

  it('toda entrada do review.json é um estado', () => {
    const lista = listaAltoContraste(review);
    assert.ok(lista.length > 0);
    for (const e of lista) assert.ok(e in ESTADOS, `estado desconhecido: ${e}`);
  });

  it('config falso: lista presente, ausente e chave visual ausente', () => {
    assert.deepEqual(listaAltoContraste({ visual: { alto_contraste: ['fila'] } }), ['fila']);
    assert.deepEqual(listaAltoContraste({ visual: {} }), []);
    assert.deepEqual(listaAltoContraste({}), []);
  });

  it('capturar.mjs tem as variações do relatório e do log', () => {
    const src = readFileSync(new URL('./capturar.mjs', import.meta.url), 'utf8');
    for (const s of ["'-relatorio'", "'-relatorio-fim'", "'-ler-ocr'", '.btn-ler-ocr', '.resumo-relatorio', '.btn-log', '[LIMITE_BOTOES, LIMITE_BOTOES + 1]', '`-fila-${alvo}`']) {
      assert.ok(src.includes(s), `capturar.mjs sem ${s}`);
    }
  });

  it('capturar.mjs fixa o relógio e o fuso, e o historico ganha a lista estreita', () => {
    const src = readFileSync(new URL('./capturar.mjs', import.meta.url), 'utf8');
    assert.match(src, /page\.clock\.setFixedTime\(BASE_TS \+ 2 \* 60 \* 60000\)/);
    assert.equal(src.match(/timezoneId: 'America\/Sao_Paulo'/g)?.length, 2, 'fuso nos dois newContext');
    assert.match(src, /\['relatorio', 'historico'\]\.includes\(nome\)/);
  });

  it('capturar.mjs usa o mesmo limite dos botões da fila que o ProcessList.svelte', () => {
    const src = readFileSync(new URL('./capturar.mjs', import.meta.url), 'utf8');
    const lista = readFileSync(new URL('../../frontend/src/components/ProcessList.svelte', import.meta.url), 'utf8');
    const css = lista.match(/@container \(max-width: (\d+)px\) \{\s*\.rotulo-acao/)?.[1];
    assert.ok(css, 'regra do .rotulo-acao não encontrada');
    assert.match(src, new RegExp(`const LIMITE_BOTOES = ${css};`));
  });

  it('capturar.mjs expande grupo-atualizacoes antes de rolar o bloco das atualizações', () => {
    const src = readFileSync(new URL('./capturar.mjs', import.meta.url), 'utf8');
    const expande = src.indexOf('grupo-atualizacoes');
    const rola = src.indexOf('scrollIntoView', src.indexOf('data-bloco="atualizacoes"') - 80);
    assert.ok(expande > 0 && src.includes('data-bloco="atualizacoes"'), 'sem grupo-atualizacoes ou data-bloco');
    assert.ok(expande < src.indexOf('data-bloco="atualizacoes"') && rola > expande, 'rola antes de expandir');
    assert.ok(src.includes("'aria-expanded')) === 'false'"), 'não confere o aria-expanded');
  });

  it('capturar.mjs não tem a lista literal e lê alto_contraste', () => {
    const src = readFileSync(new URL('./capturar.mjs', import.meta.url), 'utf8');
    assert.doesNotMatch(src, /ALTO_CONTRASTE\s*=\s*\[/);
    assert.match(src, /listaAltoContraste/);
    assert.match(src, /review\.json/);
  });
});
