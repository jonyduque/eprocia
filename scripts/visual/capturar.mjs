// Captures the eprocIA UI with fictitious data (skill eprocia-visual).
//
//   node scripts/visual/capturar.mjs --saida docs/revisao/capturas/2026-10-15/antes
//   node scripts/visual/capturar.mjs --saida <dir> --rapido   (1 size, 1 theme)
//
// Starts only the Vite dev server (no Go backend, so no real data can show up),
// stubs the Wails bindings, and saves one PNG per size × theme × state, plus the
// inner divider dragged to its minimum and maximum and the collapsed sidebar,
// the processing report panel, the log block and the queue button breakpoint.
// The states in visual.alto_contraste of .claude/review.json are also captured once with forced colors on
// (Windows high contrast), at 1024x768, with the activity panel open.
import { execFileSync, execSync, spawn } from 'node:child_process';
import { existsSync, mkdirSync, readFileSync } from 'node:fs';
import { createRequire } from 'node:module';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { ESTADOS, VERSAO_ATUAL, instalarStubs, listaAltoContraste } from './estados.mjs';

const RAIZ = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..', '..');
const PLAYWRIGHT = '1.63.0';
const PORTA = 5199;
// @container (max-width) que tira o rótulo dos botões da fila (ProcessList.svelte);
// estados.test.mjs confere que é o mesmo número.
const LIMITE_BOTOES = 660;
const ALTO_CONTRASTE = listaAltoContraste(JSON.parse(readFileSync(path.join(RAIZ, '.claude', 'review.json'), 'utf8')));

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
          await page.addInitScript(instalarStubs, { estado: ESTADOS[nome], tema, versao: VERSAO_ATUAL });
          await page.goto(vite.url);
          await page.waitForLoadState('networkidle');
          await page.waitForTimeout(300);

          const salvar = async (sufixo) => {
            const arquivo = path.join(saida, `${w}x${h}-${tema}-${nome}${sufixo}.png`);
            await page.screenshot({ path: arquivo });
            gerados.push(arquivo);
          };
          if (nome.startsWith('atualizacao-')) {
            // O bloco Atualizações pode vir recolhido (eprocia.categorias): expande
            // a categoria e só então rola o bloco até a vista.
            const cab = page.locator('[aria-controls="grupo-atualizacoes"]');
            if ((await cab.getAttribute('aria-expanded')) === 'false') {
              // Com versão nova disponível a categoria abre sozinha (lib/categorias.js).
              if (ESTADOS[nome].atualizacao?.fase === 'disponivel') {
                console.log(`[visual] ${w}x${h} ${tema} ${nome}: Atualizações não abriu sozinha com versão nova`);
              }
              await cab.click();
              await page.locator('#grupo-atualizacoes').waitFor({ state: 'visible' });
              await page.waitForTimeout(300);
            }
            await page.evaluate(() => document.querySelector('[data-bloco="atualizacoes"]')?.scrollIntoView({ block: 'center' }));
            await page.waitForTimeout(200);
          }
          await salvar('');

          // Painéis que abrem dentro da linha: a atividade dos documentos do item
          // em processamento e o formulário de reprocessar de um item concluído.
          if (nome === 'processando') {
            const linha = await page.$('.item-row.processing .info-btn');
            if (linha) {
              await linha.click();
              await page.waitForTimeout(300);
              await salvar('-atividade');
              await linha.click();
            }
          }
          if (nome === 'nao-lidos') {
            // Primeiro o painel (sem aviso na tela), depois fecha-o e copia para o aviso.
            const linha = await page.$('.item-row.success .info-btn');
            if (linha) {
              await linha.click();
              await page.waitForTimeout(300);
              await salvar('-atividade');
              await linha.click();
              await page.waitForTimeout(300);
            }
            const copiar = await page.$('.item-row.success button[data-acao="copiar"]');
            if (copiar) {
              await copiar.click();
              await page.waitForTimeout(300);
              await salvar('-copiado');
            }
          }
          if (nome === 'fila') {
            const botao = await page.$('.item-row.success button[data-acao="reprocessar"]');
            if (botao) {
              await botao.click();
              await page.waitForTimeout(300);
              await salvar('-reprocessar');
            }
          }

          if (nome === 'relatorio') {
            // Item com avisos: o painel de detalhes aberto, depois fechado e o
            // foco (de teclado) no "Ler com OCR".
            const comAviso = ESTADOS.relatorio.fila.find((i) => i.relatorio?.atencao?.length > 0);
            const linhaAviso = `.item-row[data-id="${comAviso.id}"]`;
            const resumo = await page.$(`${linhaAviso} .resumo-relatorio`);
            // O mouse sai de cima da linha para não sublinhar o resumo (hover).
            const tirarMouse = () => page.mouse.move(w - 30, h - 60);
            if (resumo) {
              await resumo.click();
              await tirarMouse();
              await page.waitForTimeout(300);
              await salvar('-relatorio');
              // O fim do painel (documentos para conferir) fica abaixo da dobra.
              await page.evaluate(() => document.querySelector('.painel-relatorio')?.scrollIntoView({ block: 'end' }));
              await page.waitForTimeout(200);
              await salvar('-relatorio-fim');
              await resumo.click();
              await page.waitForTimeout(300);
            }
            if (await page.$(`${linhaAviso} .btn-ler-ocr`)) {
              // Uma tecla antes do foco faz o Chromium tratá-lo como de teclado
              // (:focus-visible); sem ela, o clique anterior esconde o anel.
              await tirarMouse();
              await page.keyboard.press('Shift');
              await page.focus(`${linhaAviso} .btn-ler-ocr`);
              await page.waitForTimeout(200);
              await salvar('-ler-ocr');
              await page.evaluate(() => document.activeElement?.blur());
            }
            // Fila nos dois lados do limite em que os botões perdem o rótulo
            // (@container max-width: LIMITE_BOTOES em ProcessList.svelte): o card
            // no limite (só ícones) e 1 px acima (com rótulo), quando o divisor
            // alcança (com 660, só a 1024 px); a largura medida vai para o log. O
            // arrasto com o mouse não acerta o pixel: a largura do painel vai
            // direto para a chave que o app lê ao abrir (lib/painel.js) e a
            // página recarrega.
            // A consulta @container mede a caixa de conteúdo do .list-card (sem
            // a borda), não a largura total.
            const medir = () => page.evaluate(() => {
              const c = document.querySelector('.list-card');
              const p = document.querySelector('.sidebar')?.getBoundingClientRect();
              if (!c || !p) return null;
              const s = getComputedStyle(c);
              const conteudo = c.clientWidth - parseFloat(s.paddingLeft) - parseFloat(s.paddingRight);
              return { card: Math.round(conteudo), painel: Math.round(p.width) };
            });
            const base = await medir();
            for (const alvo of base ? [LIMITE_BOTOES, LIMITE_BOTOES + 1] : []) {
              const largura = base.painel + base.card - alvo;
              await page.evaluate((l) => localStorage.setItem('eprocia.sidebar', JSON.stringify({ largura: l, recolhido: false })), largura);
              await page.reload();
              await page.waitForLoadState('networkidle');
              await page.waitForTimeout(300);
              const m = await medir();
              console.log(`[visual] ${w}x${h} ${tema}: .list-card alvo ${alvo} -> ${m?.card} px (painel ${m?.painel})`);
              if (m?.card === alvo) await salvar(`-fila-${alvo}`);
            }
          }
          if (nome === 'configuracoes-log') {
            // O bloco do log fica no fim do painel: rola até ele, na largura
            // padrão e com o painel no mínimo (280 px).
            const medirLog = async (rotulo) => {
              const m = await page.evaluate(() => {
                const b = document.querySelector('.btn-log');
                if (!b) return null;
                b.scrollIntoView({ block: 'center' });
                const r = b.getBoundingClientRect();
                const pai = b.parentElement.getBoundingClientRect();
                return { btn: Math.round(r.width), pai: Math.round(pai.width), cortado: b.scrollWidth > b.clientWidth };
              });
              console.log(`[visual] ${w}x${h} ${tema} ${rotulo}: .btn-log ${JSON.stringify(m)}`);
              return m;
            };
            if (await medirLog('padrão')) {
              await page.waitForTimeout(200);
              await salvar('-log');
              if (await arrastarDivisor(page, 20)) {
                await page.waitForTimeout(300);
                await medirLog('280');
                await page.waitForTimeout(200);
                await salvar('-log-280');
              }
            }
          }

          // The divider and collapsed sidebar only vary with the layout, so one state is enough.
          // O nao-lidos também ganha o divisor no máximo, para conferir o rótulo
          // "N não lidos" numa linha estreita.
          if (nome === 'nao-lidos' && (await arrastarDivisor(page, w - 20))) await salvar('-divisor-maximo');
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

    for (const nome of estados.filter((e) => ALTO_CONTRASTE.includes(e))) {
      const ctx = await navegador.newContext({ viewport: { width: 1024, height: 768 }, reducedMotion: 'reduce', forcedColors: 'active' });
      const page = await ctx.newPage();
      await page.addInitScript(instalarStubs, { estado: ESTADOS[nome], tema: 'escuro', versao: VERSAO_ATUAL });
      await page.goto(vite.url);
      await page.waitForLoadState('networkidle');
      await page.waitForTimeout(300);
      const linha = await page.$('.item-row.processing .info-btn');
      if (linha) {
        await linha.click();
        await page.waitForTimeout(300);
      }
      const arquivo = path.join(saida, `1024x768-alto-contraste-${nome}.png`);
      await page.screenshot({ path: arquivo });
      gerados.push(arquivo);
      await ctx.close();
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
