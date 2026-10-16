import { describe, it } from 'node:test';
import assert from 'node:assert/strict';
import { spawnSync } from 'node:child_process';
import path from 'node:path';
import { RAIZ, normalizar } from './lib.mjs';
import * as gerados from './proteger-gerados.mjs';
import * as dados from './proteger-dados.mjs';
import * as bench from './guarda-bench.mjs';
import * as goCheck from './go-check.mjs';
import * as front from './front-check.mjs';
import * as ambiente from './ambiente.mjs';

const edit = (file_path) => ({ tool_name: 'Edit', tool_input: { file_path } });
const bash = (command) => ({ tool_name: 'Bash', tool_input: { command } });

describe('lib.normalizar', () => {
  it('torna relativo à raiz e usa barras normais', () => {
    assert.equal(normalizar('C:\\Repo\\Frontend\\src\\App.svelte', 'C:\\Repo'), 'frontend/src/app.svelte');
    assert.equal(normalizar('./internal/x.go', 'C:\\Repo'), 'internal/x.go');
    assert.equal(normalizar('', 'C:\\Repo'), '');
  });
});

describe('proteger-gerados', () => {
  const raiz = 'C:\\Users\\u\\eprocia';
  for (const p of [
    'frontend/src/wailsjs/go/main/App.js',
    'C:\\Users\\u\\eprocia\\frontend\\dist\\index.html',
    'internal/ocr/embed/tesseract/tesseract.exe',
    'build/bin/eprocia.exe',
    'frontend/src/../dist/x.js',
  ]) {
    it(`bloqueia ${p}`, () => {
      const r = gerados.avaliar(edit(p), { raiz });
      assert.equal(r.codigo, 2);
      assert.match(r.stderr, /Bloqueado/);
    });
  }
  it('deixa editar código-fonte', () => {
    assert.equal(gerados.avaliar(edit('frontend/src/App.svelte'), { raiz }).codigo, 0);
    assert.equal(gerados.avaliar(edit('C:\\Users\\u\\eprocia\\internal\\etl\\queue.go'), { raiz }).codigo, 0);
  });
  it('sai 0 sem tool_input', () => {
    assert.equal(gerados.avaliar({}).codigo, 0);
  });
});

describe('proteger-dados', () => {
  const ctx = { benchDir: 'D:\\corpus', listar: () => [] };
  for (const c of [
    'git add a.zip',
    'git add "pasta com espaço/y.PDF"',
    'git commit -m ok && git add saida.txt',
    'cd x; git add D:\\corpus\\proc.json',
    'git status\ngit add x.zip',
    'git status & git add x.zip',
    '(git add x.zip)',
    'git -C . add x.zip',
    'git -c core.x=1 --no-pager add x.zip',
    'git commit -m "msg" dados.zip',
    'git add frontend/../x.pdf',
  ]) {
    it(`bloqueia: ${JSON.stringify(c)}`, () => assert.equal(dados.avaliar(bash(c), ctx).codigo, 2));
  }
  for (const c of ['git add -A', 'git add .', 'git add ./', 'git add *', 'git add dir/', 'git add internal', 'git commit -am "x"', 'git commit --all -m x']) {
    it(`expande e bloqueia amplo com dado real: ${c}`, () => {
      const vistos = [];
      const r = dados.avaliar(bash(c), { ...ctx, listar: (p) => { vistos.push(p); return ['README.md', 'Processo 1.zip']; } });
      assert.equal(r.codigo, 2);
      assert.match(r.stderr, /Processo 1\.zip/);
      assert.equal(vistos.length, 1);
    });
  }
  it('falha fechada quando não consegue listar', () => {
    const r = dados.avaliar(bash('git add -A'), { ...ctx, listar: () => null });
    assert.equal(r.codigo, 2);
    assert.match(r.stderr, /não consegui listar/);
  });
  it('deixa git add -A sem dados reais', () => {
    assert.equal(dados.avaliar(bash('git add -A'), { ...ctx, listar: () => ['a.go'] }).codigo, 0);
  });
  it('pathspec amplo é repassado ao git', () => {
    let visto;
    dados.avaliar(bash('git add docs/'), { ...ctx, listar: (p) => { visto = p; return []; } });
    assert.deepEqual(visto, ['docs/']);
  });
  for (const c of ['git add README.md', 'git add scripts/bench/fixtures/misto.zip scripts/bench/fixtures/nativo.pdf',
    'git add docs/revisao/notas.txt', 'git status', 'ls *.zip', 'git commit -m "remove a.zip"', 'echo "git add a.zip"']) {
    it(`deixa: ${JSON.stringify(c)}`, () => assert.equal(dados.avaliar(bash(c), { ...ctx, listar: () => { throw new Error('não devia listar'); } }).codigo, 0));
  }
  it('sai 0 sem comando', () => assert.equal(dados.avaliar({ tool_input: {} }, ctx).codigo, 0));
});

describe('guarda-bench', () => {
  it('avisa sem -count', () => assert.match(bench.avaliar(bash('go test ./scripts/bench -bench .')).contexto, /-count=1/));
  it('avisa com -count=3', () => assert.ok(bench.avaliar(bash('go test -bench=. -count=3 ./...')).contexto));
  it('não avisa com -count=10', () => assert.equal(bench.avaliar(bash('go test -bench . -count=10 ./x')).contexto, undefined));
  it('ignora go test sem bench', () => assert.equal(bench.avaliar(bash('go test ./...')).contexto, undefined));
  it('nunca bloqueia', () => assert.equal(bench.avaliar(bash('go test -bench .')).codigo, 0));
});

describe('go-check', () => {
  const chamadas = [];
  const rodar = async (cmd, args) => {
    chamadas.push([cmd, ...args].join(' '));
    if (cmd === 'gofmt') return { ok: true, saida: args[0] };
    return { ok: false, saida: 'internal/etl/x.go:3:1: unreachable code' };
  };
  it('roda gofmt e vet no pacote do arquivo .go', async () => {
    const r = await goCheck.avaliar(edit(path.join(RAIZ, 'internal', 'etl', 'x.go')), { rodar });
    assert.equal(r.codigo, 0);
    assert.match(r.contexto, /gofmt/);
    assert.match(r.contexto, /unreachable code/);
    assert.ok(chamadas.some((c) => c === 'go vet ./internal/etl'));
  });
  it('ignora não-Go', async () => {
    assert.deepEqual(await goCheck.avaliar(edit('README.md'), { rodar }), { codigo: 0 });
  });
});

describe('front-check', () => {
  it('reconhece alvos', () => {
    assert.ok(front.ehAlvo('frontend/src/app.svelte'));
    assert.ok(front.ehAlvo('frontend/src/assets/css/style.css'));
    assert.ok(!front.ehAlvo('frontend/src/wailsjs/go/main/app.js'));
    assert.ok(!front.ehAlvo('frontend/package.json'));
  });
  it('informa que está inativo sem svelte-check', async () => {
    const r = await front.avaliar(edit('frontend/src/App.svelte'), { instalado: false });
    assert.match(r.contexto, /inativo/);
  });
  it('filtra a saída pelo arquivo editado', async () => {
    const rodar = async () => ({ ok: false, saida: '1 ERROR "src/App.svelte" 3:1 "x"\n1 ERROR "src/outro.svelte" 1:1 "y"' });
    const r = await front.avaliar(edit('frontend/src/App.svelte'), { instalado: true, rodar });
    assert.match(r.contexto, /App\.svelte/);
    assert.doesNotMatch(r.contexto, /outro/);
  });
});

describe('ambiente', () => {
  it('detecta Wails CLI incompatível', () => {
    const l = ambiente.montarLinha({ goVersao: '1.27.1', xtools: 'v0.30.0' });
    assert.match(l, /incompatível/);
    assert.match(l, /EPROCIA_BENCH_DIR não definido/);
  });
  it('linha completa com tudo ok', () => {
    const l = ambiente.montarLinha({ goVersao: '1.27.1', xtools: 'v0.40.0', pid: '42', benchDir: 'D:\\c' });
    assert.match(l, /Wails CLI ok/);
    assert.match(l, /PID 42/);
  });
  it('sem wails', () => assert.match(ambiente.montarLinha({}), /não encontrado/));
  it('wails achado mas versão não lida não vira "não encontrado"', () => {
    const l = ambiente.montarLinha({ goVersao: '1.27.1', wails: 'C:\\go\\bin\\wails.exe' });
    assert.doesNotMatch(l, /não encontrado/);
    assert.match(l, /versão do x\/tools não lida/);
  });
});

describe('execução como script', () => {
  const rodarHook = (nome, entrada) =>
    spawnSync(process.execPath, [path.join(RAIZ, '.claude', 'hooks', nome)], { input: entrada, encoding: 'utf8' });
  it('JSON inválido sai 0', () => {
    assert.equal(rodarHook('proteger-gerados.mjs', 'não é json').status, 0);
  });
  it('bloqueio sai 2 com mensagem', () => {
    const r = rodarHook('proteger-gerados.mjs', JSON.stringify(edit('frontend/dist/x.js')));
    assert.equal(r.status, 2);
    assert.match(r.stderr, /Bloqueado/);
  });
  it('aviso sai 0 com JSON de contexto', () => {
    const r = rodarHook('guarda-bench.mjs', JSON.stringify(bash('go test -bench .')));
    assert.equal(r.status, 0);
    assert.equal(JSON.parse(r.stdout).hookSpecificOutput.hookEventName, 'PreToolUse');
  });
});
