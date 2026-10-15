// Captures the eprocIA UI with fictitious data (skill eprocia-visual).
//
//   node scripts/visual/capturar.mjs --saida docs/revisao/capturas/2026-10-15/antes
//   node scripts/visual/capturar.mjs --saida <dir> --rapido   (1 size, 1 theme)
//
// Starts only the Vite dev server (no Go backend, so no real data can show up),
// stubs the Wails bindings, and saves one PNG per size × theme × state, plus the
// inner divider dragged to its minimum and maximum and the collapsed sidebar.
import { execFileSync, execSync, spawn } from 'node:child_process';
import { existsSync, mkdirSync } from 'node:fs';
import { createRequire } from 'node:module';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { ESTADOS, instalarStubs } from './estados.mjs';

const RAIZ = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..', '..');
const PLAYWRIGHT = '1.63.0';
const PORTA = 5199;

function opcoes(argv) {
  const o = { saida: null, rapido: false, estados: null };
  for (let i = 0; i < argv.length; i++) {
    if (argv[i] === '--saida') o.saida = argv[++i];
    else if (argv[i] === '--rapido') o.rapido = true;
    else if (argv[i] === '--estados') o.estados = argv[++i].split(',');
  }
  if (!o.saida) throw new Error('uso: node scripts/visual/capturar.mjs --saida <dir> [--rapido] [--estados a,b]');
  return o;
}

// carregarPlaywright installs Playwright once in a cache outside the repo, so it
// never enters frontend/package.json.
function carregarPlaywright() {
  const base = process.env.LOCALAPPDATA ?? path.join(process.env.HOME ?? '.', '.cache');
  const cache = path.join(base, 'eprocia-harness', `playwright-${PLAYWRIGHT}`);
  if (!existsSync(path.join(cache, 'node_modules', 'playwright'))) {
    console.log(`[visual] instalando playwright@${PLAYWRIGHT} em ${cache}`);
    mkdirSync(cache, { recursive: true });
    execSync(`npm install --prefix "${cache}" --no-save --no-audit --no-fund playwright@${PLAYWRIGHT}`, { stdio: 'inherit' });
  }
  return createRequire(path.join(cache, 'package.json'))('playwright');
}

async function subirVite() {
  // A leftover server on the port would answer the readiness check below and
  // serve stale code, so refuse to start instead.
  try {
    await fetch(`http://localhost:${PORTA}/`);
    throw new Error(`porta ${PORTA} já está em uso (Vite de uma execução anterior?); encerre o processo e rode de novo`);
  } catch (err) {
    if (err.message.startsWith('porta')) throw err;
  }
  const proc = spawn(`npx vite --port ${PORTA} --strictPort`, {
    cwd: path.join(RAIZ, 'frontend'), shell: true, stdio: ['ignore', 'pipe', 'pipe'],
  });
  const url = `http://localhost:${PORTA}/`;
  for (let i = 0; i < 100; i++) {
    try {
      if ((await fetch(url)).ok) return { proc, url };
    } catch {}
    await new Promise((r) => setTimeout(r, 200));
  }
  encerrar(proc);
  throw new Error('Vite não respondeu em 20 s');
}

function encerrar(proc) {
  if (!proc?.pid) return;
  if (process.platform === 'win32') {
    try { execFileSync('taskkill', ['/PID', String(proc.pid), '/T', '/F'], { stdio: 'ignore' }); } catch {}
  } else {
    proc.kill('SIGTERM');
  }
}

async function abrirNavegador(chromium) {
  try {
    return await chromium.launch({ channel: 'chrome' });
  } catch {
    return await chromium.launch();
  }
}

// arrastarDivisor drags the sidebar resize handle to an x coordinate.
async function arrastarDivisor(page, x) {
  const h = await page.$('.resize-handle');
  if (!h) return false;
  const b = await h.boundingBox();
  await page.mouse.move(b.x + b.width / 2, b.y + b.height / 2);
  await page.mouse.down();
  await page.mouse.move(x, b.y + b.height / 2, { steps: 12 });
  await page.mouse.up();
  return true;
}

async function main() {
  const o = opcoes(process.argv.slice(2));
  const saida = path.resolve(RAIZ, o.saida);
  mkdirSync(saida, { recursive: true });

  const tamanhos = o.rapido ? [[1024, 768]] : [[800, 600], [1024, 768], [1440, 900], [1920, 1080]];
  const temas = o.rapido ? ['escuro'] : ['escuro', 'claro'];
  const estados = o.estados ?? Object.keys(ESTADOS);

  const { chromium } = carregarPlaywright();
  let vite = null;
  let navegador = null;
  // Ctrl+C or a killed parent must not leave Vite listening on the port.
  const aoSair = () => encerrar(vite?.proc);
  process.once('exit', aoSair);
  for (const sinal of ['SIGINT', 'SIGTERM']) {
    process.once(sinal, () => {
      aoSair();
      process.exit(130);
    });
  }
  const gerados = [];
  try {
    vite = await subirVite();
    navegador = await abrirNavegador(chromium);
    for (const [w, h] of tamanhos) {
      for (const tema of temas) {
        for (const nome of estados) {
          const ctx = await navegador.newContext({ viewport: { width: w, height: h }, reducedMotion: 'reduce' });
          const page = await ctx.newPage();
          await page.addInitScript(instalarStubs, { estado: ESTADOS[nome], tema });
          await page.goto(vite.url);
          await page.waitForLoadState('networkidle');
          await page.waitForTimeout(300);

          const salvar = async (sufixo) => {
            const arquivo = path.join(saida, `${w}x${h}-${tema}-${nome}${sufixo}.png`);
            await page.screenshot({ path: arquivo });
            gerados.push(arquivo);
          };
          await salvar('');

          // The divider and collapsed sidebar only vary with the layout, so one state is enough.
          if (nome === 'processando') {
            if (await arrastarDivisor(page, 20)) await salvar('-divisor-minimo');
            if (await arrastarDivisor(page, w - 20)) await salvar('-divisor-maximo');
            const recolher = await page.$('.btn-toggle-sidebar');
            if (recolher) {
              await recolher.click();
              await page.waitForTimeout(300);
              await salvar('-recolhido');
            }
          }
          await ctx.close();
        }
      }
    }
  } finally {
    await navegador?.close();
    encerrar(vite?.proc);
  }
  console.log(`[visual] ${gerados.length} capturas em ${saida}`);
}

main().catch((err) => {
  console.error(`[visual] erro: ${err.message}`);
  process.exit(1);
});
