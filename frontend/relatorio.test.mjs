// Relatório de processamento de um ZIP: linha-resumo, avisos, "Ler com OCR"
// e painel de detalhes (spec 2026-11-22, Parte 1).
// Rodar com: node --test frontend/relatorio.test.mjs
import { describe, it } from 'node:test';
import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import {
  resumoPaginas,
  avisosPaginas,
  mostrarLerComOCR,
  reducaoLimpeza,
  formatarDuracao,
  formatarBytes,
  linhasAtencao
} from './src/lib/relatorio.js';
import {
  textoPaginasSemOCR,
  textoPaginasComErro,
  textoOCRFalhouComReserva,
  textoListaPaginas,
  textoDocsSemOCR,
  EXPLICACAO_OCR,
  ROTULO_LER_COM_OCR,
  textoPaginasForaDoResumo
} from './src/lib/textos.js';

const ler = (p) => readFileSync(new URL(p, import.meta.url), 'utf8');

function rel(paginas = {}, extra = {}) {
  return {
    documentos: { total: 1, lidos: 1, ...(extra.documentos ?? {}) },
    paginas: {
      total: 0, lida_nativa: 0, lida_ocr: 0, vazia: 0, sem_ocr: 0, com_erro: 0,
      ocr_sem_ganho: 0, ocr_falhou_com_reserva: 0, de_nao_lidos: 0, de_omitidos: 0, duplicadas: 0,
      ...paginas
    },
    tempo: { total_ms: 0, ocr_trabalho_ms: 0, ...(extra.tempo ?? {}) },
    ocr: { pagina_estimada_ms: 0, ...(extra.ocr ?? {}) },
    limpeza: { nivel: 'standard', bytes_antes: 0, bytes_depois: 0, ...(extra.limpeza ?? {}) },
    atencao: extra.atencao
  };
}

const ZERADO = rel();

describe('textos', () => {
  it('plural e singular das frases de aviso', () => {
    assert.equal(textoPaginasSemOCR(6), '6 páginas têm imagem e o OCR estava desligado');
    assert.equal(textoPaginasSemOCR(1), '1 página tem imagem e o OCR estava desligado');
    assert.equal(textoPaginasComErro(3), '3 páginas não puderam ser lidas');
    assert.equal(textoPaginasComErro(1), '1 página não pôde ser lida');
    assert.equal(textoOCRFalhouComReserva(2), '2 páginas ficaram com o texto do PDF porque o OCR falhou');
    assert.equal(textoOCRFalhouComReserva(1), '1 página ficou com o texto do PDF porque o OCR falhou');
  });
  it('explicação e rótulo do botão', () => {
    assert.equal(EXPLICACAO_OCR, 'Página em branco ou só com texto não vai ao OCR: isso é normal. O aviso aparece só quando a página tem imagem ou desenho e o texto do PDF não bastou.');
    assert.equal(ROTULO_LER_COM_OCR, 'Ler com OCR');
  });
  it('lista de páginas diz quantas ficaram de fora do limite', () => {
    assert.equal(textoListaPaginas([3]), 'pág. 3');
    assert.equal(textoListaPaginas([3, 7, 9], 3), 'págs. 3, 7, 9');
    assert.equal(textoListaPaginas([1, 2], 14), 'págs. 1, 2 e mais 12');
  });
});

describe('resumoPaginas', () => {
  it('resumo com total 0 é null', () => {
    assert.equal(resumoPaginas(ZERADO), null);
  });
  it('sem relatório (item antigo) é null e não lança', () => {
    assert.equal(resumoPaginas(undefined), null);
    assert.equal(resumoPaginas(null), null);
    assert.equal(resumoPaginas({}), null);
  });
  it('plural com as três partes', () => {
    const r = rel({ total: 142, lida_nativa: 120, lida_ocr: 18, vazia: 4 });
    assert.equal(resumoPaginas(r), '142 páginas: 120 do texto do PDF, 18 por OCR, 4 em branco');
  });
  it('omite partes zeradas', () => {
    const r = rel({ total: 10, lida_nativa: 10 });
    assert.equal(resumoPaginas(r), '10 páginas: 10 do texto do PDF');
    const s = rel({ total: 5, lida_ocr: 3, vazia: 2 });
    assert.equal(resumoPaginas(s), '5 páginas: 3 por OCR, 2 em branco');
  });
  it('singular', () => {
    assert.equal(resumoPaginas(rel({ total: 1, lida_nativa: 1 })), '1 página: 1 do texto do PDF');
  });
  it('só páginas de aviso: total sem partes', () => {
    assert.equal(resumoPaginas(rel({ total: 6, sem_ocr: 6 })), '6 páginas');
  });
  // O contador da linha (pages_count) inclui as páginas dos não lidos e dos
  // omitidos; o resumo diz de onde vem a diferença.
  it('páginas de não lidos e omitidos entram como trecho à parte', () => {
    const r = rel({ total: 139, lida_nativa: 131, lida_ocr: 8, de_nao_lidos: 3 });
    assert.equal(resumoPaginas(r), '139 páginas: 131 do texto do PDF, 8 por OCR; + 3 de documentos não lidos ou omitidos');
    const s = rel({ total: 10, lida_nativa: 10, de_nao_lidos: 2, de_omitidos: 4 });
    assert.equal(resumoPaginas(s), '10 páginas: 10 do texto do PDF; + 6 de documentos não lidos ou omitidos');
    assert.equal(resumoPaginas(rel({ total: 6, sem_ocr: 6, de_omitidos: 1 })), '6 páginas; + 1 de documento não lido ou omitido');
  });
  it('texto do trecho: singular e plural', () => {
    assert.equal(textoPaginasForaDoResumo(1), '+ 1 de documento não lido ou omitido');
    assert.equal(textoPaginasForaDoResumo(3), '+ 3 de documentos não lidos ou omitidos');
  });
});

describe('avisosPaginas', () => {
  it('nenhum aviso com relatório zerado ou ausente', () => {
    assert.deepEqual(avisosPaginas(ZERADO), []);
    assert.deepEqual(avisosPaginas(undefined), []);
    assert.deepEqual(avisosPaginas({}), []);
  });
  it('nunca para páginas em branco', () => {
    assert.deepEqual(avisosPaginas(rel({ total: 4, vazia: 4 })), []);
  });
  it('só as três categorias, na ordem', () => {
    const r = rel({ total: 20, lida_nativa: 9, vazia: 2, sem_ocr: 6, com_erro: 3, ocr_falhou_com_reserva: 1 });
    assert.deepEqual(avisosPaginas(r), [
      { tipo: 'sem_ocr', texto: '6 páginas têm imagem e o OCR estava desligado' },
      { tipo: 'com_erro', texto: '3 páginas não puderam ser lidas' },
      { tipo: 'ocr_falhou_com_reserva', texto: '1 página ficou com o texto do PDF porque o OCR falhou' }
    ]);
  });
  it('documento digitalizado não lido por falta de OCR vira o primeiro aviso, com plural certo', () => {
    const um = rel({ total: 4, lida_nativa: 4, de_nao_lidos: 3 }, { documentos: { nao_lidos: 1, por_leitura: { lido: 1, sem_ocr: 1 } } });
    assert.deepEqual(avisosPaginas(um), [
      { tipo: 'docs_sem_ocr', texto: '1 documento digitalizado não lido: o OCR estava desligado' }
    ]);
    const dois = rel({ total: 6, sem_ocr: 2, lida_nativa: 4 }, { documentos: { nao_lidos: 2, por_leitura: { sem_ocr: 2, senha: 1 } } });
    assert.deepEqual(avisosPaginas(dois).map((a) => a.tipo), ['docs_sem_ocr', 'sem_ocr']);
    assert.equal(avisosPaginas(dois)[0].texto, '2 documentos digitalizados não lidos: o OCR estava desligado');
    // Não lido por outro motivo (senha) não gera este aviso.
    assert.deepEqual(avisosPaginas(rel({ total: 1, lida_nativa: 1 }, { documentos: { por_leitura: { senha: 1 } } })), []);
  });
  it('textoDocsSemOCR: singular e plural', () => {
    assert.equal(textoDocsSemOCR(1), '1 documento digitalizado não lido: o OCR estava desligado');
    assert.equal(textoDocsSemOCR(3), '3 documentos digitalizados não lidos: o OCR estava desligado');
  });
});

describe('mostrarLerComOCR', () => {
  it('com página sem OCR', () => {
    assert.equal(mostrarLerComOCR(rel({ total: 2, sem_ocr: 1, lida_nativa: 1 })), true);
  });
  it('sem página sem OCR', () => {
    assert.equal(mostrarLerComOCR(rel({ total: 2, com_erro: 2 })), false);
    assert.equal(mostrarLerComOCR(ZERADO), false);
  });
  it('documento inteiro não lido por falta de OCR também mostra (páginas sem_ocr = 0)', () => {
    const r = rel({ total: 0, de_nao_lidos: 3 }, { documentos: { nao_lidos: 1, por_leitura: { sem_ocr: 1 } } });
    assert.equal(mostrarLerComOCR(r), true);
  });
  it('outros motivos de não lido não mostram', () => {
    const r = rel({ total: 0 }, { documentos: { nao_lidos: 1, por_leitura: { senha: 1 } } });
    assert.equal(mostrarLerComOCR(r), false);
  });
  it('relatório ausente', () => {
    assert.equal(mostrarLerComOCR(undefined), false);
    assert.equal(mostrarLerComOCR({}), false);
  });
});

describe('reducaoLimpeza', () => {
  it('null com nível none', () => {
    assert.equal(reducaoLimpeza(rel({}, { limpeza: { nivel: 'none', bytes_antes: 400, bytes_depois: 400 } })), null);
  });
  it('null com antes 0', () => {
    assert.equal(reducaoLimpeza(ZERADO), null);
  });
  it('25 para 400 -> 300', () => {
    assert.equal(reducaoLimpeza(rel({}, { limpeza: { nivel: 'standard', bytes_antes: 400, bytes_depois: 300 } })), 25);
  });
  it('fica entre 0 e 100', () => {
    assert.equal(reducaoLimpeza(rel({}, { limpeza: { nivel: 'basic', bytes_antes: 100, bytes_depois: 120 } })), 0);
    assert.equal(reducaoLimpeza(rel({}, { limpeza: { nivel: 'basic', bytes_antes: 100, bytes_depois: -5 } })), 100);
  });
  it('relatório ausente', () => {
    assert.equal(reducaoLimpeza(undefined), null);
    assert.equal(reducaoLimpeza({}), null);
  });
});

describe('formatação do painel', () => {
  it('duração', () => {
    assert.equal(formatarDuracao(0), '0 s');
    assert.equal(formatarDuracao(450), '0,5 s');
    assert.equal(formatarDuracao(1234), '1,2 s');
    assert.equal(formatarDuracao(59_900), '59,9 s');
    assert.equal(formatarDuracao(125_000), '2 min 5 s');
    assert.equal(formatarDuracao(undefined), '0 s');
  });
  it('bytes', () => {
    assert.equal(formatarBytes(0), '0 B');
    assert.equal(formatarBytes(900), '900 B');
    assert.equal(formatarBytes(12_600), '12,3 KB');
    assert.equal(formatarBytes(3 * 1024 * 1024), '3,0 MB');
    assert.equal(formatarBytes(undefined), '0 B');
  });
  it('linhas de atenção por documento, com o total além do limite de 50', () => {
    const r = rel({}, {
      atencao: [{
        nome: 'Evento 3 - PET1.pdf', evento: 3,
        sem_ocr: [1, 2], total_sem_ocr: 60,
        com_erro: [5], total_com_erro: 1
      }]
    });
    const l = linhasAtencao(r);
    assert.equal(l.length, 1);
    assert.equal(l[0].nome, 'Evento 3 - PET1.pdf');
    assert.deepEqual(l[0].casos.map((c) => c.tipo), ['sem_ocr', 'com_erro']);
    assert.equal(l[0].casos[0].paginas, 'págs. 1, 2 e mais 58');
    assert.equal(l[0].casos[1].paginas, 'pág. 5');
    assert.deepEqual(linhasAtencao(undefined), []);
    assert.deepEqual(linhasAtencao(ZERADO), []);
  });
  it('relatório zerado não gera NaN em nenhuma função', () => {
    const saidas = [
      resumoPaginas(ZERADO), JSON.stringify(avisosPaginas(ZERADO)), mostrarLerComOCR(ZERADO),
      reducaoLimpeza(ZERADO), formatarDuracao(ZERADO.tempo.total_ms), formatarBytes(ZERADO.limpeza.bytes_antes),
      JSON.stringify(linhasAtencao(ZERADO))
    ];
    for (const s of saidas) assert.doesNotMatch(String(s), /NaN/);
  });
});

describe('convenções dos componentes', () => {
  const lista = ler('./src/components/ProcessList.svelte');
  const painel = ler('./src/components/RelatorioItem.svelte');

  it('ProcessList importa o painel e mostra o botão dentro de mostrarLerComOCR', () => {
    assert.match(lista, /import RelatorioItem from '\.\/RelatorioItem\.svelte'/);
    // O resultado de mostrarLerComOCR fica num {@const ler} reaproveitado pelo bloco e pelo botão.
    assert.match(lista, /\{@const ler = mostrarLerComOCR\(item\.relatorio\)\}/);
    assert.match(lista, /\{#if ler\}[\s\S]*?\{ROTULO_LER_COM_OCR\}[\s\S]*?\{\/if\}/);
  });
  it('a linha-resumo abre o painel com aria-expanded', () => {
    assert.match(lista, /resumoPaginas\(item\.relatorio\)/);
    assert.match(lista, /class="resumo-relatorio"[^>]*aria-expanded=/);
    assert.match(lista, /<RelatorioItem/);
  });
  it('Ler com OCR chama ReprocessQueueItem com forcar=false via argumentosLerComOCR', () => {
    assert.match(lista, /argumentosLerComOCR\(item, config\)/);
    assert.match(lista, /ReprocessQueueItem\(item\.id, a\.ocr, a\.limpeza, a\.forcar\)/);
  });
  it('Ler com OCR tem trava contra duplo envio', () => {
    const f = lista.slice(lista.indexOf('async function lerComOCR'));
    const corpo = f.slice(0, f.search(/\r?\n  \}\r?\n/));
    assert.match(corpo, /if \(lendoOCR\[item\.id\]\) return;/);
    assert.match(corpo, /finally \{[\s\S]*lendoOCR = \{ \.\.\.lendoOCR, \[item\.id\]: false \}/);
    // Também travado durante a troca de versão (Task 17 do autoupdate).
    assert.match(lista, /class="btn-ler-ocr"[^>]*aria-disabled=\{!!lendoOCR\[item\.id\] \|\| entradaBloqueada\}/);
  });
  it('Ler com OCR não usa disabled: num erro o foco fica no botão, e não cai no body', () => {
    const botao = lista.match(/<button\b[^>]*class="btn-ler-ocr"[^>]*>/)?.[0] ?? '';
    assert.ok(botao, 'falta o botão btn-ler-ocr');
    assert.doesNotMatch(botao, /(^|\s)disabled=/);
    assert.doesNotMatch(lista, /\.btn-ler-ocr:disabled/);
    assert.match(lista, /\.btn-ler-ocr\[aria-disabled='true'\]/);
  });
  it('linha-resumo e Ler com OCR entram no foco itinerante', () => {
    assert.match(lista, /class="resumo-relatorio"[^>]*data-parada="relatorio"[^>]*tabindex=\{tabParada\(item\.id, 'relatorio', paradaAtiva\)\}/);
    assert.match(lista, /class="btn-ler-ocr"[^>]*data-parada="lerocr"[^>]*tabindex=\{tabParada\(item\.id, 'lerocr', paradaAtiva\)\}/);
  });
  it('Ler com OCR fecha o formulário do Reprocessar e esquece a origem, como o Reprocessar', () => {
    const f = lista.slice(lista.indexOf('async function lerComOCR'));
    const corpo = f.slice(0, f.search(/\r?\n  \}\r?\n/));
    const ok = corpo.slice(corpo.indexOf('try {'), corpo.indexOf('} catch'));
    assert.match(ok, /reprocessarAberto = \{ \.\.\.reprocessarAberto, \[item\.id\]: false \}/);
    assert.match(ok, /origemReprocessar = \{ \.\.\.origemReprocessar, \[item\.id\]: null \}/);
  });
  it('Reprocessar comum também fecha o painel do relatório', () => {
    const f = lista.slice(lista.indexOf('async function confirmarReprocessar'));
    assert.match(f.slice(0, 900), /relatorioAberto = \{ \.\.\.relatorioAberto, \[item\.id\]: false \}/);
  });
  it('painel de detalhes: uma só coluna de valores para todas as seções (subgrid)', () => {
    const css = painel.slice(painel.indexOf('<style>'));
    assert.match(css, /\.painel-relatorio\s*\{[^}]*display:\s*grid[^}]*grid-template-columns:\s*max-content minmax\(0, 1fr\)/);
    assert.match(css, /section\s*\{[^}]*grid-column:\s*1 \/ -1[^}]*grid-template-columns:\s*subgrid/);
    assert.match(css, /\.grade\s*\{[^}]*grid-column:\s*1 \/ -1[^}]*grid-template-columns:\s*subgrid/);
  });
  it('painel de detalhes: uma só regra .painel-relatorio no @container estreito', () => {
    const css = painel.slice(painel.indexOf('<style>'));
    const bloco = css.slice(css.indexOf('@container (max-width: 420px)'));
    assert.equal((bloco.match(/\.painel-relatorio\s*\{/g) ?? []).length, 1);
  });
  it('painel de detalhes: subtítulos com um só estilo (h3, peso 600)', () => {
    const css = painel.slice(painel.indexOf('<style>'));
    assert.equal((css.match(/(^|[\s,}])h3[^{]*\{/g) ?? []).length, 1, 'mais de uma regra para h3');
    assert.match(css, /h3\s*\{[^}]*font-weight:\s*600/);
    assert.doesNotMatch(painel, /<h3 class=/);
  });
  it('RelatorioItem não tem cor literal fora de var()', () => {
    const semVar = painel.replace(/var\([^)]*\)/g, '');
    assert.doesNotMatch(semVar, /#[0-9a-fA-F]{3,6}\b/);
    assert.doesNotMatch(semVar, /rgba?\(/);
  });
  it('textos novos vêm de textos.js, não de literais nos .svelte', () => {
    for (const f of [lista, painel]) {
      assert.doesNotMatch(f, /OCR estava desligado/);
      assert.doesNotMatch(f, /não puderam ser lidas/);
      assert.doesNotMatch(f, /Ler com OCR/);
    }
  });
});
