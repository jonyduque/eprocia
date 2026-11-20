// Funções puras da lista da fila (ProcessList.svelte).

// Criado uma vez: toLocaleTimeString monta um formatador novo a cada chamada,
// e a lista inteira é redesenhada a cada queue:update.
const FORMATO_HORA = new Intl.DateTimeFormat('pt-BR', {
  hour: '2-digit',
  minute: '2-digit',
  second: '2-digit',
});

export function formatarHora(timestamp) {
  if (!timestamp) return '';
  const data = new Date(timestamp);
  if (Number.isNaN(data.getTime())) return '';
  return FORMATO_HORA.format(data);
}

const FORMATO_CURTO = new Intl.NumberFormat('pt-BR', { notation: 'compact' });
const FORMATO_EXATO = new Intl.NumberFormat('pt-BR');

export function abreviarContagem(n) {
  return FORMATO_CURTO.format(n || 0);
}

export function nomeExibido(nome) {
  return (nome ?? '').replace(/\.zip$/i, '');
}

// Nome acessível de um botão de ícone da linha: a ação seguida do arquivo, para
// o leitor de tela não ouvir N botões "Remover da lista" iguais.
export function rotuloAcao(acao, nome) {
  const n = nomeExibido(nome);
  return n ? `${acao}: ${n}` : acao;
}

// Dica dos contadores da linha: os números exatos que o selo abrevia.
export function dicaContadores(item) {
  const n = (v) => FORMATO_EXATO.format(v || 0);
  if (item.status === 'processing') {
    return `Documentos: ${n(item.processed_docs)} de ${n(item.total_docs)}; páginas: ${n(item.processed_pages)} de ${n(item.total_pages)}`;
  }
  const qtd = (v, um, varios) => `${n(v)} ${v === 1 ? um : varios}`;
  let dica = `${qtd(item.docs_count, 'documento', 'documentos')}, ${qtd(item.pages_count, 'página', 'páginas')}`;
  if (item.estimated_tokens > 0) {
    dica += `, ~${n(item.estimated_tokens)} tokens (estimativa local: 1 token a cada 4 caracteres)`;
  }
  return dica;
}

export function filtrarItens(items, filtro) {
  const q = (filtro ?? '').toLowerCase();
  if (!q) return items;
  return items.filter(
    (item) =>
      item.name.toLowerCase().includes(q) ||
      (item.message && item.message.toLowerCase().includes(q)),
  );
}

// Para onde vai o foco depois de uma ação na linha `id`: a própria linha, se
// ela continua na lista; senão a seguinte que sobrou; senão a anterior; senão
// null (o chamador foca o título da lista).
export function alvoFoco(idsAntes, id, idsDepois) {
  if (id == null) return null;
  const restantes = new Set(idsDepois);
  if (restantes.has(id)) return id;
  const i = idsAntes.indexOf(id);
  if (i < 0) return idsDepois[0] ?? null;
  for (let j = i + 1; j < idsAntes.length; j++) {
    if (restantes.has(idsAntes[j])) return idsAntes[j];
  }
  for (let j = i - 1; j >= 0; j--) {
    if (restantes.has(idsAntes[j])) return idsAntes[j];
  }
  return null;
}

// Botões de linha têm data-acao; cada estado mostra um conjunto diferente.
// Prefere a mesma ação, depois Remover e Cancelar, depois a primeira que houver.
export function escolherAcao(disponiveis, preferida) {
  for (const acao of [preferida, 'remover', 'cancelar']) {
    if (acao && disponiveis.includes(acao)) return acao;
  }
  return disponiveis[0] ?? null;
}

// As transições das linhas leem isto quando começam, depois do $: que chama
// atualizar: só o que chegou pelo queue:update entra animado, só o que saiu
// da fila sai com fade, e o filtro não move nada. terminou(id) fica verdadeiro
// enquanto o item segue concluído ou com erro depois de mudar de estado nesta
// sessão: a classe do realce não sai, e a animação roda uma vez só.
const REALCE = new Set(['success', 'error']);

export function criarMovimentoFila() {
  let presentes = new Set();
  let novos = new Set();
  let filtro = '';
  let filtrou = false;
  let estados = new Map();
  const terminados = new Set();

  return {
    atualizar(items, textoFiltro = '') {
      const ids = new Set(items.map((i) => i.id));
      novos = new Set();
      for (const id of ids) if (!presentes.has(id)) novos.add(id);
      presentes = ids;
      filtrou = textoFiltro !== filtro;
      filtro = textoFiltro;
      const proximos = new Map();
      for (const item of items) {
        const antes = estados.get(item.id);
        if (antes !== undefined && antes !== item.status) {
          if (REALCE.has(item.status)) terminados.add(item.id);
          else terminados.delete(item.id);
        }
        proximos.set(item.id, item.status);
      }
      for (const id of terminados) if (!proximos.has(id)) terminados.delete(id);
      estados = proximos;
    },
    novo: (id) => novos.has(id),
    terminou: (id) => terminados.has(id),
    removido: (id) => !presentes.has(id),
    porFiltro: () => filtrou,
  };
}

const CONCLUIDO = new Set(['success', 'error', 'cancelled']);

export function contarConcluidos(items) {
  return items.filter((i) => CONCLUIDO.has(i.status)).length;
}

// Pergunta da confirmação de "Limpar tudo": conta a lista inteira, não só o
// que o filtro mostra, porque ClearQueue apaga tudo.
export function textoConfirmarLimpeza(items) {
  const n = items.length;
  const itens = n === 1 ? '1 item' : `${n} itens`;
  const andamento = items.filter((i) => i.status === 'processing').length;
  if (andamento > 0) return `Cancelar ${andamento} em andamento e remover ${itens}?`;
  return `Remover ${itens} da lista?`;
}

// Foco itinerante (F-AC-15): a lista é uma parada de Tab só. Cada linha tem
// suas paradas na ordem da tela; espelha os botões de ProcessList.svelte.
export function paradasItem(item, expansivel) {
  const p = expansivel ? ['info'] : [];
  if (item.status === 'processing' || item.status === 'pending') return [...p, 'cancelar'];
  if (item.status === 'success' && item.output_path) return [...p, 'reprocessar', 'copiar', 'abrir', 'remover'];
  if (item.status === 'error') p.push('detalhes');
  return [...p, 'reprocessar', 'remover'];
}

function naLinha(linha, chave, coluna) {
  const i = linha.paradas.indexOf(chave);
  if (i >= 0) return { id: linha.id, chave };
  return { id: linha.id, chave: linha.paradas[Math.min(coluna, linha.paradas.length - 1)] };
}

// grade: [{ id, paradas }] na ordem da lista; atual: { id, chave }.
// Devolve a próxima parada, a mesma nas pontas, ou null se a tecla não é de navegação.
export function proximaParada(grade, atual, tecla) {
  const l = atual ? grade.findIndex((g) => g.id === atual.id) : -1;
  if (l < 0) return null;
  const c = Math.max(0, grade[l].paradas.indexOf(atual.chave));
  const ultima = grade.length - 1;
  switch (tecla) {
    case 'ArrowUp':
      return naLinha(grade[Math.max(0, l - 1)], atual.chave, c);
    case 'ArrowDown':
      return naLinha(grade[Math.min(ultima, l + 1)], atual.chave, c);
    case 'ArrowLeft':
      return { id: atual.id, chave: grade[l].paradas[Math.max(0, c - 1)] };
    case 'ArrowRight':
      return { id: atual.id, chave: grade[l].paradas[Math.min(grade[l].paradas.length - 1, c + 1)] };
    case 'Home':
      return { id: grade[0].id, chave: grade[0].paradas[0] };
    case 'End':
      return { id: grade[ultima].id, chave: grade[ultima].paradas[0] };
    default:
      return null;
  }
}

// A parada com tabindex 0: a última focada, enquanto a linha e a ação existirem.
export function resolverParada(grade, ativa) {
  if (grade.length === 0) return null;
  const linha = ativa ? grade.find((g) => g.id === ativa.id) : null;
  if (!linha) return { id: grade[0].id, chave: grade[0].paradas[0] };
  return naLinha(linha, ativa.chave, 0);
}

// Delete segue os botões (F-AC-12): o X do pendente cancela, a lixeira do
// terminado remove; o item em andamento só cancela pelo botão, sem atalho.
export function acaoDelete(status) {
  if (status === 'pending') return 'cancelar';
  if (CONCLUIDO.has(status)) return 'remover';
  return null;
}

export function copiavel(item) {
  return item.status === 'success' && !!item.output_path;
}
