import { describe, it } from 'node:test';
import assert from 'node:assert/strict';
import { ATUALIZACAO as T } from './src/lib/textos.js';
import { textoFase, acoes, mostraProgresso, fraseModo, aceitaEntrada } from './src/lib/atualizacao.js';

const e = (o) => ({ ativo: true, fase: '', versao: '', progresso: 0, mensagem: '', modo: '', motivo: '', atual: '0.4.4', ...o });
const nenhuma = { procurar: false, atualizar: null, reiniciar: null, cancelar: false, novidades: false, paginaVersoes: false };
const A = (o, fila = false) => acoes(e(o), fila);

describe('textos da atualização', () => {
  it('textos exatos', () => {
    assert.equal(T.titulo, 'Atualizações');
    assert.equal(T.versao('1.0.0'), 'Versão 1.0.0');
    assert.equal(T.procurar, 'Procurar atualização');
    assert.equal(T.procurando, 'Procurando...');
    assert.equal(T.emDia('1.0.0'), 'O eprocIA está em dia (1.0.0)');
    assert.equal(T.disponivel('1.0.1'), 'Versão 1.0.1 disponível');
    assert.equal(T.novidades, 'Ver novidades');
    assert.equal(T.atualizar, 'Atualizar');
    assert.equal(T.baixarInstalador, 'Baixar e abrir o instalador');
    assert.equal(T.quandoFilaTerminar, 'Atualizar quando a fila terminar');
    assert.equal(T.cancelar, 'Cancelar');
    assert.equal(T.baixando(42), 'Baixando... 42%');
    assert.equal(T.conferindo, 'Conferindo o download...');
    assert.equal(T.aguardandoFila, 'A atualização começa quando a fila terminar');
    assert.equal(T.abrindoInstalador, 'Abrindo o instalador...');
    assert.equal(T.paginaVersoes, 'Abrir a página de versões no navegador');
    assert.equal(T.instalando, 'Instalando a versão nova...');
    assert.equal(T.instalada('1.0.1'), 'Versão 1.0.1 instalada');
    assert.equal(T.reiniciar, 'Reiniciar agora');
    assert.equal(T.reiniciarQuandoFilaTerminar, 'Reiniciar quando a fila terminar');
    assert.equal(T.aguardandoReinicio, 'O eprocIA reinicia quando a fila terminar');
    assert.equal(
      T.fraseInstalador,
      "O Windows vai pedir permissão de administrador; o eprocIA fecha para a instalação. Na última tela, deixe marcado 'Abrir o eprocIA' para ele voltar sozinho",
    );
    assert.equal(T.incompleta('1.0.1'), 'A versão 1.0.1 foi publicada, mas está incompleta; tente de novo mais tarde');
    assert.equal(
      T.soltarDepois,
      'O eprocIA está sendo atualizado; solte os arquivos de novo depois que ele reabrir',
    );
  });
  it('sem Voltar nem anterior', () => {
    for (const k of Object.keys(T)) {
      assert.doesNotMatch(k, /voltar|anterior/i);
    }
  });
});

describe('atualizacao', () => {
  it('fase vazia só procura', () => {
    assert.deepEqual(A({}), { ...nenhuma, procurar: true });
    assert.equal(textoFase(e({})), '');
  });
  it('verificando', () => {
    assert.deepEqual(A({ fase: 'verificando' }), nenhuma);
    assert.equal(textoFase(e({ fase: 'verificando' })), 'Procurando...');
  });
  it('em_dia', () => {
    assert.equal(textoFase(e({ fase: 'em_dia', versao: '0.4.4' })), 'O eprocIA está em dia (0.4.4)');
    assert.equal(A({ fase: 'em_dia' }).procurar, true);
    assert.equal(A({ fase: 'em_dia' }).atualizar, null);
  });
  it('disponivel trocar', () => {
    const s = { fase: 'disponivel', versao: '0.5.0', modo: 'trocar' };
    assert.equal(textoFase(e(s)), 'Versão 0.5.0 disponível');
    assert.equal(A(s).atualizar, 'Atualizar');
    assert.equal(A(s, true).atualizar, 'Atualizar quando a fila terminar');
    assert.equal(A(s).reiniciar, null);
    assert.equal(fraseModo(e(s)), null);
  });
  it('disponivel instalador', () => {
    const s = { fase: 'disponivel', versao: '0.5.0', modo: 'instalador' };
    assert.equal(A(s).atualizar, 'Baixar e abrir o instalador');
    assert.equal(A(s, true).atualizar, 'Atualizar quando a fila terminar');
    assert.equal(fraseModo(e(s)), T.fraseInstalador);
  });
  it('disponivel so_avisar', () => {
    const s = { fase: 'disponivel', versao: '0.5.0', modo: 'so_avisar', motivo: 'Instalação em pasta protegida' };
    assert.equal(A(s).atualizar, null);
    assert.equal(A(s).novidades, true);
    assert.equal(fraseModo(e(s)), 'Instalação em pasta protegida');
  });
  it('incompleta', () => {
    const s = { fase: 'incompleta', versao: '0.5.0' };
    assert.equal(textoFase(e(s)), 'A versão 0.5.0 foi publicada, mas está incompleta; tente de novo mais tarde');
    assert.equal(A(s).novidades, true);
    assert.equal(A(s).atualizar, null);
  });
  it('baixando', () => {
    const s = { fase: 'baixando', progresso: 30 };
    assert.equal(textoFase(e(s)), 'Baixando... 30%');
    assert.equal(mostraProgresso(e(s)), true);
    assert.equal(A(s).cancelar, true);
    assert.equal(A(s).procurar, false);
  });
  it('conferindo', () => {
    const s = { fase: 'conferindo' };
    assert.equal(textoFase(e(s)), 'Conferindo o download...');
    assert.equal(A(s).cancelar, false);
    assert.equal(mostraProgresso(e(s)), false);
  });
  it('instalando', () => {
    assert.equal(textoFase(e({ fase: 'instalando' })), 'Instalando a versão nova...');
    assert.deepEqual(A({ fase: 'instalando' }), nenhuma);
  });
  it('instalada', () => {
    const s = { fase: 'instalada', versao: '0.5.0' };
    assert.equal(textoFase(e(s)), 'Versão 0.5.0 instalada');
    assert.deepEqual(A(s), { ...nenhuma, reiniciar: 'Reiniciar agora' });
    assert.equal(A(s, true).reiniciar, 'Reiniciar quando a fila terminar');
  });
  it('instalada com mensagem', () => {
    const s = { fase: 'instalada', versao: '0.5.0', mensagem: 'Abre na próxima vez.' };
    assert.equal(textoFase(e(s)), 'Versão 0.5.0 instalada Abre na próxima vez.');
    assert.equal(A(s).reiniciar, null);
    assert.equal(A(s).atualizar, null);
  });
  it('aguardando_fila', () => {
    assert.equal(textoFase(e({ fase: 'aguardando_fila', mensagem: 'Espera' })), 'Espera');
    assert.equal(textoFase(e({ fase: 'aguardando_fila', modo: 'trocar' })), 'O eprocIA reinicia quando a fila terminar');
    assert.equal(
      textoFase(e({ fase: 'aguardando_fila', modo: 'instalador' })),
      'A atualização começa quando a fila terminar',
    );
    assert.equal(A({ fase: 'aguardando_fila' }).cancelar, true);
  });
  it('reiniciando e abrindo_instalador', () => {
    assert.equal(textoFase(e({ fase: 'reiniciando', mensagem: 'Reiniciando' })), 'Reiniciando');
    assert.equal(textoFase(e({ fase: 'abrindo_instalador' })), 'Abrindo o instalador...');
    assert.equal(textoFase(e({ fase: 'abrindo_instalador', mensagem: 'X' })), 'X');
    assert.deepEqual(A({ fase: 'reiniciando' }), nenhuma);
    assert.deepEqual(A({ fase: 'abrindo_instalador' }), nenhuma);
  });
  it('erro sem modo', () => {
    const s = { fase: 'erro', mensagem: 'Sem conexão' };
    assert.equal(textoFase(e(s)), 'Sem conexão');
    assert.equal(A(s).paginaVersoes, true);
    assert.equal(A(s).atualizar, null);
    assert.equal(A(s).novidades, false);
  });
  it('erro com modo', () => {
    for (const modo of ['trocar', 'instalador']) {
      const s = { fase: 'erro', mensagem: 'Falhou', modo };
      assert.equal(textoFase(e(s)), 'Falhou');
      assert.equal(A(s).novidades, true);
      assert.notEqual(A(s).atualizar, null);
      assert.equal(A(s).paginaVersoes, false);
    }
  });
  it('inativo', () => {
    const s = e({ ativo: false, fase: 'disponivel', modo: 'trocar', versao: '1' });
    assert.deepEqual(acoes(s, false), nenhuma);
    assert.equal(textoFase(s), '');
  });
  it('reiniciar só em instalada', () => {
    for (const fase of ['', 'verificando', 'em_dia', 'disponivel', 'incompleta', 'baixando', 'conferindo', 'instalando',
      'aguardando_fila', 'reiniciando', 'abrindo_instalador', 'erro']) {
      assert.equal(A({ fase, modo: 'trocar' }).reiniciar, null, fase);
    }
  });
  it('aceitaEntrada', () => {
    assert.equal(aceitaEntrada('reiniciando'), false);
    assert.equal(aceitaEntrada('abrindo_instalador'), false);
    for (const f of ['', 'verificando', 'em_dia', 'disponivel', 'incompleta', 'baixando', 'conferindo', 'instalando',
      'instalada', 'aguardando_fila', 'erro']) {
      assert.equal(aceitaEntrada(f), true, f);
    }
  });
});
