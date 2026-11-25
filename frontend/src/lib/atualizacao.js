// Lógica pura da atualização pelo aplicativo: o que a tela mostra e quais
// botões existem para cada Estado do backend (internal/atualizacao/estado.go).
// As frases de erro chegam prontas em `mensagem`; aqui não se reescrevem.
import { ATUALIZACAO as T } from './textos.js';

const MODOS_INSTALAVEIS = ['trocar', 'instalador'];

const instalavel = (modo) => MODOS_INSTALAVEIS.includes(modo);

export function textoFase(estado) {
  if (!estado || !estado.ativo) return '';
  const { fase, versao, progresso, mensagem, modo } = estado;
  switch (fase) {
    case 'verificando':
      return T.procurando;
    case 'em_dia':
      return T.emDia(versao);
    case 'disponivel':
      return T.disponivel(versao);
    case 'incompleta':
      return T.incompleta(versao);
    case 'baixando':
      return T.baixando(progresso);
    case 'conferindo':
      return T.conferindo;
    case 'instalando':
      return T.instalando;
    case 'instalada':
      return mensagem || T.instalada(versao);
    case 'aguardando_fila':
      if (mensagem) return mensagem;
      return modo === 'instalador' ? T.aguardandoFila : T.aguardandoReinicio;
    case 'reiniciando':
      return mensagem || T.reiniciando;
    case 'abrindo_instalador':
      return mensagem || T.abrindoInstalador;
    case 'erro':
      return mensagem || '';
    default:
      return '';
  }
}

export function acoes(estado, filaOcupada) {
  const nenhuma = { procurar: false, atualizar: null, reiniciar: null, cancelar: false, novidades: false, paginaVersoes: false };
  if (!estado || !estado.ativo) return nenhuma;
  const { fase, modo, mensagem } = estado;
  const a = { ...nenhuma };
  // Só a abertura do instalador espera a fila; a troca dos arquivos (trocar)
  // não fecha o programa, e quem espera a fila nela é o Reiniciar.
  const rotuloAtualizar = () => {
    if (modo !== 'instalador') return T.atualizar;
    return filaOcupada ? T.quandoFilaTerminar : T.baixarInstalador;
  };
  switch (fase) {
    case '':
    case 'em_dia':
      a.procurar = true;
      break;
    case 'disponivel':
      a.procurar = true;
      a.novidades = true;
      if (instalavel(modo)) a.atualizar = rotuloAtualizar();
      break;
    case 'incompleta':
      a.procurar = true;
      a.novidades = true;
      break;
    case 'baixando':
    case 'aguardando_fila':
      a.cancelar = true;
      break;
    case 'instalada':
      if (!mensagem) a.reiniciar = filaOcupada ? T.reiniciarQuandoFilaTerminar : T.reiniciar;
      break;
    case 'erro':
      a.procurar = true;
      if (instalavel(modo)) {
        a.novidades = true;
        a.atualizar = rotuloAtualizar();
      } else {
        a.paginaVersoes = true;
      }
      break;
    default:
      break;
  }
  return a;
}

export function mostraProgresso(estado) {
  return !!estado && !!estado.ativo && estado.fase === 'baixando';
}

export function fraseModo(estado) {
  if (!estado || !estado.ativo) return null;
  if (estado.fase === 'disponivel') {
    if (estado.modo === 'instalador') return T.fraseInstalador;
    if (estado.modo === 'so_avisar') return estado.motivo || null;
  }
  if (estado.fase === 'erro' && estado.modo === 'instalador') return T.fraseInstalador;
  return null;
}

// A troca dos arquivos (instalando, instalada) não fecha o programa: só
// reiniciando e abrindo_instalador recusam soltar ZIPs.
export function aceitaEntrada(fase) {
  return fase !== 'reiniciando' && fase !== 'abrindo_instalador';
}
