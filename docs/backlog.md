# Backlog

Ideias e pedidos do dono ainda não planejados. Cada item sai daqui quando vira spec ou plano.

## Barra lateral (configurações)

Pedido em 2026-11-09:

- Aumentar um pouco o padding da barra lateral.
- Rodar um subagente para um brainstorm do visual da barra lateral, com foco em:
  - melhorar a usabilidade;
  - tirar os comboboxes. Eles são feios, e o usuário só vê as opções depois de abrir.
- Repensar a omissão de tipos de documento. Hoje o usuário digita os tipos e precisa lembrar de todos. Ideia: a lista já vem com alguns tipos, e cresce com os tipos de documento que já apareceram nos processos.

## Estudos feitos, aguardando decisão

Os relatórios dos estudos de 2026-11-09 ficaram no scratchpad da sessão e não foram versionados. O resumo de cada um:

- **Atualização dentro do app (G).** Fase 1: checagem opcional, uma vez por dia, de `releases/latest`, que avisa na janela e na bandeja. Não envia dado do usuário (registrar em PRD RNF01). Fase 2: baixar a versão nova e trocar o exe renomeando o que está em uso, só com a fila ociosa e só depois de verificar a atestação com `sigstore-go`. O NSIS silencioso foi descartado porque pede admin.
- **Extensão do eproc (M).** MV3 em `extensao/`. Grava com `chrome.downloads` em `Downloads\eprocIA`, e o app monitora essa pasta. Native messaging fica para uma segunda fase. Servidor HTTP local fica descartado. Antes de começar, falta a captura do fluxo de "Download completo" no DevTools: descobrir se o resultado é ZIP ou PDF e se a geração é assíncrona.
- **Arrastar um item processado para o navegador (G).** `SHDoDragDrop` com o IDataObject do shell, iniciado por um binding. Fazer antes um spike da captura do mouse fora da thread da janela. Complemento barato: botão "Copiar arquivo" (CF_HDROP).

## Pendências menores da higienização

- Uma cauda depois da chancela que seja um marcador de página ("Página 1 de 3") não passa pelos filtros de marcador.
- HTML com BOM UTF-8 mantém o U+FEFF no início do texto.
- O exemplo de saída do `README.md` está no formato antigo.

## Pendências menores da bandeja (revisão de 2026-11-09)

- O `Animador` segura o mutex durante chamadas Win32 síncronas. Se um dia Show ou Quit rodarem na thread de UI, isso vira deadlock.
- Há uma corrida estreita entre o `go Hide()` do `OnBeforeClose` e o `Quit`. Ela não trava a saída.
- O deslize pode tirar a janela do Snap ou cruzar a divisa entre monitores com DPI diferente.
- Falta conferir na tela como o WebView2 pinta dentro da janela em camadas.
