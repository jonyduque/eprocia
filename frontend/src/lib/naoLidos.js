// Documentos que o eprocIA não conseguiu ler (unread_docs do item da fila).

export function rotuloNaoLidos(n) {
  if (!n || n < 1) return '';
  return n === 1 ? '1 não lido' : `${n} não lidos`;
}

export function avisoCopiaNaoLidos(n) {
  if (!n || n < 1) return '';
  return n === 1
    ? 'Copiado. Atenção: 1 documento não foi lido; o texto está incompleto.'
    : `Copiado. Atenção: ${n} documentos não foram lidos; o texto está incompleto.`;
}

export function tituloNaoLidos(n) {
  if (!n || n < 1) return '';
  return `${n === 1 ? '1 documento não foi lido' : `${n} documentos não foram lidos`}. Abra o item para ver quais.`;
}
