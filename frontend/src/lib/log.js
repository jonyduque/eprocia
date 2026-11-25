// O que as Configurações mostram sobre o log (decisão Q8 do dono): com o log
// desligado, nada; ligado, o botão da pasta de logs e o aviso de que os
// arquivos ficam só neste computador.
import { ROTULO_ABRIR_LOGS, TEXTO_LOG_ATIVO } from './textos.js';

export function itensLog(logAtivo) {
  if (logAtivo !== true) return [];
  return [
    { tipo: 'botao', texto: ROTULO_ABRIR_LOGS },
    { tipo: 'texto', texto: TEXTO_LOG_ATIVO }
  ];
}
