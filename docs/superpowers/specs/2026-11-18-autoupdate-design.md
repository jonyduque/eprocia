# Atualização do eprocIA pelo próprio programa

Data: 2026-11-18. Base lida: worktree `revisao-2026-11-10` (main com a v0.4.2). Revisada por um revisor sem contexto em 2026-11-18 (aprovada com ajustes, todos aplicados ou respondidos na seção 11). Decisões do dono a1 a a4 (2026-11-19) e e7 (2026-11-20) aplicadas (seção 10); não há pontos em aberto.

Pedido do dono: "Outro recurso é o autoupdate."

**Resumo das decisões:** o programa nunca procura versão nova sozinho. Toda conexão de rede começa com um clique do usuário em "Procurar atualização". Havendo versão nova, "Atualizar" troca o executável (instalação por usuário) ou baixa, confere e abre o instalador (instalação NSIS, que pede administrador e, na página final, oferece "Abrir o eprocIA"). A conferência é pelo `SHA256SUMS.txt`; a atestação de build fica para uma segunda etapa.

## 1. Problema e comportamento esperado

Hoje a única forma de atualizar é rodar de novo o `install.ps1` (`irm | iex`) ou baixar o instalador NSIS da página de releases. O usuário comum não sabe que saiu versão nova e fica meses com correções de leitura de PDF atrasadas.

Comportamento esperado:

- Em Configurações, um bloco "Atualizações": "Versão 0.4.2", o botão "Procurar atualização" e, quando houver uma guardada, "Voltar para a versão anterior (0.4.1)".
- **Nenhuma procura automática** (a1, a2): nem ao abrir, nem periódica, nem pergunta na primeira abertura. Nenhuma conexão sai do programa sem o clique em "Procurar atualização" (ou, depois dele, em "Atualizar").
- O clique consulta, no GitHub, qual é a release mais recente publicada. A consulta não envia nada sobre processos.
- O resultado aparece no próprio bloco: "O eprocIA está em dia (0.4.2)" ou "Versão 0.5.0 disponível", com "Ver novidades" e "Atualizar". Não há aviso no rodapé nem notificação do Windows: quem pediu está com Configurações aberta.
- **Instalação por usuário** (`install.ps1`): "Atualizar" baixa o executável da release, confere o SHA-256 contra o `SHA256SUMS.txt` da mesma release e, só se conferir, troca o executável e reabre o programa na versão nova. Durante o download, o bloco mostra o progresso e "Cancelar".
- **Instalação NSIS** (a4): "Atualizar" baixa o instalador da release, confere o SHA-256 e o abre; o Windows pede administrador. O eprocIA fecha logo depois que o instalador abre, para ele poder trocar os arquivos. Na página final, o instalador oferece "Abrir o eprocIA", marcado por padrão (e7), e o programa volta já na versão nova.
- Se houver processos na fila (`pending` ou `processing`), a troca (ou a abertura do instalador) espera: o botão vira "Atualizar quando a fila terminar". Nunca se interrompe uma conversão para atualizar.
- Se a versão nova não abrir (instalação por usuário), o programa volta sozinho para a anterior e diz isso.

## 2. Onde o eprocIA pode estar instalado

Isso determina o que "Atualizar" faz.

- **Instalação por usuário** (`install.ps1`): por padrão `%LOCALAPPDATA%\Programs\eprocIA\eprocia.exe`, ou a pasta dada em `-Dir` (`install.ps1:257`), sempre com a chave `HKCU\Software\Microsoft\Windows\CurrentVersion\Uninstall\eprocIA` e o `InstallLocation` gravado nela (`install.ps1:469`). O usuário pode escrever na pasta: a troca é possível. É o caso principal.
- **Instalador NSIS** (`eprocia-<versão>-windows-amd64-instalador.exe`): instala com elevação, por padrão em Program Files (`build/windows/installer/project.nsi:75`; a pasta pode ser trocada na página de diretório), e grava a chave `HKLM\Software\Microsoft\Windows\CurrentVersion\Uninstall\<empresa><produto>` (`wails_tools.nsh:28` e `:118-127`), sem `InstallLocation`, com `DisplayIcon` apontando para `$INSTDIR\eprocia.exe`. O programa não troca o próprio exe sem UAC: segue a4 (seção 3.1, `instalador.go`). O instalador passa a oferecer "Abrir o eprocIA" na página final (e7, seção 3.4).
- **Executável solto** (baixado à mão, rodando de Downloads) **ou build de desenvolvimento** (`wails dev`, `build/bin`): a procura funciona, mas o resultado só diz que há versão nova, com "Ver novidades"; nunca troca o arquivo.

Detecção (`Instalacao()` no pacote novo), em ordem:

1. **Por usuário:** lê `InstallLocation` da chave HKCU; a pasta do `os.Executable()` (após `EvalSymlinks`, comparação sem caixa) precisa ser igual a ela, e a pasta precisa aceitar escrita (cria e apaga um arquivo de teste com nome aleatório).
2. **NSIS:** lê `DisplayIcon` da chave HKLM (visão de 64 bits); o caminho, sem aspas, precisa ser igual ao `os.Executable()` (mesma comparação).
3. Qualquer outra situação é "só avisar", com o motivo.

## 3. O que muda em cada camada

### 3.1 Go

**Versão em tempo de execução.** O Go hoje não conhece a própria versão (só o frontend, via `__APP_VERSION__` no `vite.config.js`). `main.go` passa a embutir `wails.json` (`//go:embed wails.json`, mesmo diretório) e expor `versaoAtual()`, lida de `info.productVersion`. Build sem versão válida (vazia ou 0.0.0) desliga o recurso inteiro.

**Pacote novo `internal/atualizacao`:**

- `cliente.go`: `http.Client` próprio. Regras para **toda** URL, a inicial e cada redirecionamento:
  - esquema `https`, sempre (um redirecionamento para `http` é recusado);
  - host na lista `api.github.com`, `github.com`, `objects.githubusercontent.com`, `release-assets.githubusercontent.com` (o `browser_download_url` do GitHub redireciona para estes dois últimos);
  - a URL inicial de download (que vem do JSON da API) precisa começar com `https://github.com/jonyduque/eprocia/releases/download/`; a da API, com `https://api.github.com/repos/jonyduque/eprocia/`.
  - Sem cookies, sem credenciais, sem token. Proxy: `http.ProxyFromEnvironment` (ver seção 4). TLS com as raízes do Windows (padrão do Go no Windows).
- `consulta.go`: `Verificar(ctx, atual) (*Release, error)`, chamada só a partir do clique (seção 3.1, `app.go`). `GET https://api.github.com/repos/jonyduque/eprocia/releases/latest`, com `Accept: application/vnd.github+json`, `User-Agent: eprocIA/<versão>` (a API exige User-Agent; nada mais que identifique a máquina), prazo de 15 s, resposta limitada a 1 MiB. Aceita a release só se: não for draft nem prerelease (o endpoint `latest` já garante; conferir mesmo assim); tag no formato `vX.Y.Z`; versão maior que a atual (comparação numérica); exatamente um asset `eprocia-X.Y.Z-windows-amd64.exe` e exatamente um `eprocia-X.Y.Z-windows-amd64-instalador.exe`, com X.Y.Z igual ao da tag; exatamente um `SHA256SUMS.txt`. Devolve tag, versão, URL das notas (`html_url`), URLs e tamanhos dos três assets (o tipo de instalação escolhe qual baixar).
- `download.go`: `Baixar(ctx, rel, asset, pasta, progresso) (caminho string, err error)`. Cria `<pasta>\<nome do asset>.download-<aleatório>` com `O_EXCL`. Teto de 300 MiB, conferido no `Content-Length` e na contagem durante a cópia; tamanho final igual ao do asset. Baixa o `SHA256SUMS.txt` (teto 64 KiB) e acha a linha `<64 hex> *<nome>` do asset (mesma regra do `install.ps1:344`, que aceita maiúsculas, asterisco opcional e CRLF; duas linhas do mesmo nome são recusadas). Depois de fechar o arquivo baixado, **recalcula o SHA-256 sobre o arquivo fechado** e compara. Confere ainda: começa com `MZ`; o recurso de versão do PE (`GetFileVersionInfo`/`VerQueryValue` de `golang.org/x/sys/windows`, campos numéricos, como o `install.ps1` faz) é igual à versão da tag (o instalador NSIS também tem recurso de versão: `VIProductVersion`, `project.nsi:38`). Qualquer falha ou cancelamento apaga o temporário.
- `aplicar.go` (instalação por usuário): `Aplicar(pastaInstalacao, baixado)` e `Reverter()`. O temporário fica na pasta de instalação, no mesmo volume do destino (rename atômico). Antes do passo 1: a fila está sem `pending`/`processing`; o watcher é parado (para os dois processos não pegarem o mesmo ZIP); o gravador do histórico (spec do histórico, se ligado) grava e **fecha**, para que o `Shutdown` final do processo antigo não regrave por cima do que o novo gravar. Sequência:
  1. rename `eprocia.exe` para `eprocia.anterior.exe` (o Windows permite renomear o exe em execução; apagar, não). Um `eprocia.anterior.exe` de atualização antiga é apagado antes (não está em execução).
  2. rename do baixado para `eprocia.exe`. Se falhar, desfaz o passo 1 na hora e aborta (a pasta nunca fica sem `eprocia.exe`).
  3. inicia o novo `eprocia.exe` com `--apos-atualizar=<pid antigo>` (e com o argumento de iniciar oculto, `autostart.IniciarOculto`, se a janela estiver oculta).
  4. espera até 30 s pelo sinal de vida do novo processo (protocolo abaixo). Se o novo processo terminar antes ou o prazo passar: encerra o novo pelo PID e **espera ele sair** (o novo ainda não ligou fila nem watcher, então não há `tesseract`/`pdftoppm` filhos nem temporários `eprocia_pdf_*`); desfaz os renames (`eprocia.exe` para `eprocia.falhou.exe`, `eprocia.anterior.exe` para `eprocia.exe`); reabre o gravador do histórico; religa o watcher; mostra "A versão X não abriu; o eprocIA continua na versão Y".
  5. com o sinal recebido, o processo antigo chama `App.Quit` (que marca `saindo` e passa pelo encerramento normal).
- **Comportamento do processo novo com `--apos-atualizar=<pid>`:** abre a janela (ou fica oculto), carrega config e histórico, mas **não liga a fila nem o watcher**. Quando o frontend termina de carregar (`OnDomReady`), grava o sinal de vida e espera o processo `<pid>` sair (até 60 s). Só então liga a fila, o watcher (com a varredura inicial) e o gravador do histórico, e atualiza `DisplayVersion` na chave HKCU de desinstalação (o `install.ps1` grava ali na linha 466; a escrita fica junto de `internal/autostart`, com a mesma troca por fake nos testes). Se o antigo não sair no prazo, segue mesmo assim e registra no log. Durante essa espera curta há dois ícones na bandeja; o antigo esconde a janela assim que recebe o sinal. O processo novo não faz nenhuma conexão de rede.
- `instalador.go` (instalação NSIS, a4): `AbrirInstalador(baixado)`. O temporário fica em `%LOCALAPPDATA%\eprocIA\atualizacao\` (pasta do usuário criada sob demanda; a pasta de instalação não aceita escrita sem administrador). Para que o arquivo conferido seja o arquivo executado, o instalador é reaberto com compartilhamento só de leitura (`FILE_SHARE_READ`, sem `FILE_SHARE_WRITE` nem `FILE_SHARE_DELETE`), o SHA-256 é recalculado sobre esse handle, e o handle fica aberto até o `ShellExecuteEx` (verbo `open`; o manifesto do instalador pede a elevação) devolver. Se o usuário recusar o UAC (`ERROR_CANCELLED`), o bloco diz "A instalação foi cancelada; o eprocIA continua na versão Y" e nada muda. Se o instalador abrir, o eprocIA para o watcher, fecha o gravador do histórico e chama `App.Quit`; o NSIS troca os arquivos e termina na página final dele, que oferece "Abrir o eprocIA" (e7, seção 3.4). A caixa vem marcada; o programa reabre sozinho se o usuário a deixar assim. O instalador baixado é apagado na abertura seguinte (limpeza abaixo). Não há reversão automática nem "Voltar para a versão anterior" nesse modo: quem quiser voltar usa o instalador da versão anterior.
- **Estado em disco:** não há `atualizacao.json`. Sem procura automática, não há "última verificação" nem "ignorar esta versão" para guardar. A versão de "Voltar para a versão anterior" é lida do recurso de versão do PE de `eprocia.anterior.exe`. O sinal de vida vai em `%APPDATA%\eprocIA\atualizacao-sinal-<pid antigo>`, criado pelo novo processo com o texto da versão e apagado pelo antigo depois de lê-lo, de modo que os dois processos nunca escrevem o mesmo arquivo. Nunca se guarda URL nem conteúdo de resposta.
- **Limpeza na abertura normal:** sempre que o programa roda da pasta de instalação (com ou sem `--apos-atualizar`), apaga `eprocia.falhou.exe`, restos `eprocia.exe.download-*`, sinais `atualizacao-sinal-*` esquecidos e o conteúdo de `%LOCALAPPDATA%\eprocIA\atualizacao\`. `eprocia.anterior.exe` fica (é o que permite voltar).

**Protocolo de troca é contrato entre versões.** "Voltar para a versão anterior" roda o protocolo dentro da versão velha, e a versão nova é iniciada pela velha. Por isso estas peças não podem mudar sem plano de compatibilidade: a flag `--apos-atualizar=<pid>`, o arquivo `atualizacao-sinal-<pid>`, os nomes `eprocia.anterior.exe`, `eprocia.falhou.exe` e `eprocia.exe.download-*`, e o comportamento de esperar o antigo sair antes de ligar a fila. O contrato fica escrito no `CLAUDE.md` e na skill `eprocia-release`, ao lado do contrato dos assets.

**`app.go`** (camada fina; a lógica fica no pacote):

- `GetVersao() string`
- `VerificarAtualizacao() (*atualizacao.Info, error)`: consulta agora; é o único ponto que inicia uma consulta. Devolve `{versao, notasURL, modo, motivo}`, com `modo` em `trocar` (por usuário), `instalador` (NSIS) ou `so_avisar`, e o motivo quando for `so_avisar` ("executável fora da pasta de instalação", "pasta de instalação sem permissão de escrita").
- `InstalarAtualizacao() error`: baixa (exe ou instalador, conforme o modo) e, se a fila estiver ociosa, aplica ou abre o instalador; senão agenda para quando ela esvaziar (a `Queue` ganha um aviso de "fila ociosa", derivado do ponto que emite `queue:status`). Só funciona depois de uma `VerificarAtualizacao` na mesma sessão; não consulta de novo por conta própria.
- `CancelarAtualizacao()`, `VoltarVersaoAnterior() error`.
- Evento Wails `atualizacao:estado` com `{fase: verificando|disponivel|baixando|conferindo|aguardando_fila|aplicando|abrindo_instalador|erro|em_dia, versao, progresso, mensagem}`.
- `startup`: nada de rede. Só a limpeza da abertura e, com `--apos-atualizar`, o arranque em dois tempos.
- "Ver novidades" abre o `html_url` no navegador padrão (`runtime.BrowserOpenURL`), só se começar com `https://github.com/jonyduque/eprocia/releases/`. A conexão é do navegador, iniciada pelo clique.

**`internal/models` e `internal/config`:** nenhuma mudança (não há opção de procura automática).

### 3.2 Bindings

Os métodos novos acima; rodar `wails generate module`.

### 3.3 Frontend

- `frontend/src/lib/atualizacao.js`: estado puro (fase para texto, rótulos de botão, se mostra progresso, texto por modo), com teste em `frontend/atualizacao.test.mjs`.
- `Settings.svelte`: bloco "Atualizações" (versão, "Procurar atualização", resultado com "Ver novidades" e "Atualizar", progresso com "Cancelar", "Voltar para a versão anterior (X)"). No modo `instalador`, o botão diz "Baixar e abrir o instalador" e a frase "O Windows vai pedir permissão de administrador; o eprocIA fecha para a instalação. Na última tela, deixe marcado 'Abrir o eprocIA' para ele voltar sozinho". No modo `so_avisar`, só "Ver novidades" e o motivo.
- `App.svelte`: assina `atualizacao:estado` e repassa a Settings. Sem aviso no rodapé.
- `DESIGN.md`: bloco de Configurações.

### 3.4 Instalador

- `install.ps1`: a desinstalação já apaga a pasta inteira (`install.ps1:517`), o que leva `eprocia.anterior.exe` e os restos. Mudanças pequenas: `Install-App` apaga `eprocia.anterior.exe` e `eprocia.falhou.exe` ao instalar por cima; `Assert-Closed` continua valendo. Regras do arquivo: compatível com o 5.1, sem BOM, sem caracteres cujos bytes UTF-8 tenham 0x91-0x94.
- **NSIS (e7):** `build/windows/installer/project.nsi` passa a oferecer "Abrir o eprocIA" na página final, marcado por padrão. Vale para qualquer instalação pelo instalador, também a feita à mão, não só a do autoupdate. A página de diretório continua; a detecção usa `DisplayIcon`, não Program Files. A mudança:
  - antes de `!insertmacro MUI_PAGE_FINISH`, define `MUI_FINISHPAGE_RUN` (com o texto "Abrir o eprocIA") e `MUI_FINISHPAGE_RUN_FUNCTION` apontando para uma função própria; a opção de abrir fica marcada por padrão (é o comportamento do `MUI_FINISHPAGE_RUN`; sem `MUI_FINISHPAGE_RUN_NOTCHECKED`);
  - a função **não** executa o exe direto: o instalador roda elevado, e o programa aberto por ele herdaria o administrador, o que impede arrastar arquivos do Explorer para a janela (isolamento de privilégios do Windows) e grava a config e o "Iniciar com o Windows" no perfil errado. A função abre o exe por `explorer.exe "$INSTDIR\${PRODUCT_EXECUTABLE}"`, que roda com a conta do usuário;
  - instalação silenciosa (`/S`) não mostra a página final e não abre o programa, como hoje;
  - nada mais do NSIS muda (seções, atalhos, desinstalador, `wails_tools.nsh`).
  - **Restrição:** o dono tem mudanças locais sem commit nesse arquivo, no checkout principal (atalho na área de trabalho e a opção `-Desktop`/`-NoDesktop`). A implementação parte da versão do arquivo na `main` e não pode sobrescrever nem depender dessas mudanças locais. O plano deve avisar o dono antes de mexer no arquivo, para ele commitar ou guardar o que tem lá e não haver conflito na hora de juntar.
- `release.yml`: sem mudança. O formato dos assets e do `SHA256SUMS.txt` (skill `eprocia-release`; hoje com as linhas dos dois exe, `release.yml:7`) vira contrato do programa: mudar o nome do exe ou do instalador, ou o formato do `SHA256SUMS`, quebra a atualização de todas as versões instaladas.

### 3.5 Documentação

`PRD.md` (RNF01 com a exceção; RNF05, seção 6), `CLAUDE.md` (pacote `atualizacao`, contrato dos assets e do protocolo de troca), skill `eprocia-release` (os dois contratos e "retirar uma release não desfaz instalações"), README (a procura só acontece pelo botão).

## 4. Casos de borda e falhas

- **Sem rede:** a consulta falha: "Não foi possível consultar o GitHub agora", com "Abrir a página de versões no navegador". Nova tentativa só por outro clique.
- **Proxy corporativo** (comum em redes de tribunal e órgãos públicos): o Go usa só `HTTP_PROXY`/`HTTPS_PROXY`, não o proxy do sistema (WinINET/PAC). Nessas redes a consulta tende a falhar. Mitigação nesta etapa: a mesma mensagem de erro com "Abrir a página de versões no navegador", que usa o proxy do sistema. Ler o proxy do Windows exigiria dependência nova (ex.: `mattn/go-ieproxy`); fica para depois, se houver queixa.
- **Limite da API do GitHub** (60 requisições por hora por IP sem token): consultas só por clique não chegam perto. Em rede com NAT de muitas máquinas, o 403 de limite vira "O GitHub recusou a consulta agora; tente mais tarde".
- **Download corrompido ou interrompido:** SHA-256 ou tamanho divergem; o temporário é apagado, nada foi renomeado nem aberto: "O download não conferiu; nada foi alterado". "Atualizar" pode ser tentado de novo.
- **`SHA256SUMS.txt`** sem a linha do asset, com duas linhas do mesmo nome ou fora do formato: recusa, como o `install.ps1`.
- **Release com asset faltando ou com nome fora do padrão:** tratada como inexistente ("em dia"), com log.
- **Tag maior, mas recurso de versão do exe ou do instalador diferente:** recusa (release montada errado).
- **Downgrade:** a atualização nunca instala versão menor ou igual à atual. Voltar só por "Voltar para a versão anterior" (instalação por usuário), pelo instalador da versão anterior (NSIS) ou `install.ps1 -Version`.
- **eprocIA em uso:**
  - fila com itens: espera a fila esvaziar (`aguardando_fila`);
  - fila pausada com pendentes: não está ociosa; o bloco diz "Retome ou limpe a fila para atualizar";
  - segunda instância aberta (não há `SingleInstanceLock` em `main.go`): na instalação por usuário, o rename funciona, mas a outra instância segue na versão velha até fechar; na NSIS, o instalador não consegue trocar o exe em uso e mostra o erro dele. Limitação registrada.
- **Falha no rename do passo 2:** desfaz o passo 1 na hora; a pasta nunca fica sem `eprocia.exe`.
- **UAC recusado (NSIS):** nada muda; o eprocIA continua aberto. **Instalador cancelado depois do UAC** ou com erro: o eprocIA já fechou; o exe antigo fica (o NSIS só troca os arquivos na página de instalação) e o usuário reabre pelo atalho. **Usuário que desmarca "Abrir o eprocIA"** na página final: o programa fica fechado até ele abrir pelo atalho, como antes de e7.
- **Antivírus:** pode segurar ou pôr em quarentena o arquivo recém-baixado. O rename falha ou o arquivo some: aborta antes de mexer no `eprocia.exe` atual, ou desfaz se já tiver renomeado. Também pode estranhar um exe que se troca; o binário não é assinado (Authenticode), então o risco existe e deve ser observado na primeira release.
- **SmartScreen:** arquivo gravado pelo próprio programa não recebe a marca de download (`Zone.Identifier`), então não há bloqueio a cada abertura; o `install.ps1` faz `Unblock-File` pelo mesmo motivo. O instalador NSIS sem assinatura mostra no UAC "Editor desconhecido", como hoje quando baixado à mão.
- **Disco cheio:** o download falha, temporário apagado.
- **Pasta sem permissão de escrita** (política da empresa) numa instalação por usuário: `Instalacao()` detecta e cai para só aviso.
- **Atalhos, "Iniciar com o Windows" (HKCU Run) e chave de desinstalação** apontam para o mesmo `eprocia.exe`: continuam válidos.
- **Reverter** (instalação por usuário):
  - automático, se a versão nova não abrir em 30 s (passo 4);
  - manual, por "Voltar para a versão anterior": a mesma troca com `eprocia.anterior.exe` (sem download nem SHA: é o arquivo que já estava instalado e não sai rede nenhuma);
  - fora do programa: `install.ps1 -Version v0.4.2`.
- **Defeito que só aparece depois:** "Voltar para a versão anterior" fica disponível enquanto existir `eprocia.anterior.exe` (até a próxima atualização). Sem procura automática, a versão revertida não volta a ser oferecida sozinha; só aparece de novo se o usuário clicar em "Procurar atualização".
- **Formato da config mudou:** o merge com padrões (`config.Load`) já tolera campos novos e ausentes nos dois sentidos.
- **Release retirada depois de publicada:** a próxima consulta devolve a anterior como `latest`; quem já atualizou fica na retirada até sair outra.

## 5. Segurança

- **Origem única:** só HTTPS, só os quatro hosts do GitHub, repositório fixo no código (`jonyduque/eprocia`), prefixo de URL conferido na URL inicial e host conferido em cada redirecionamento. Nenhuma URL vem de configuração, de variável de ambiente ou do frontend. O frontend só pede "verifique", "instale", "cancele".
- **Nunca executar sem verificar:** o arquivo baixado só é renomeado para `eprocia.exe`, ou aberto como instalador, depois de conferir tamanho, SHA-256 (recalculado sobre o arquivo fechado; no instalador, sobre o handle que fica aberto sem compartilhamento de escrita até a abertura) contra o `SHA256SUMS.txt` da mesma release, cabeçalho `MZ` e versão do PE.
- **Limite do `SHA256SUMS`:** ele vem da mesma release do exe. Protege contra download corrompido e adulteração em trânsito (que o HTTPS já evita), não contra quem controlar a conta ou o repositório no GitHub e trocar os dois arquivos. No modo NSIS essa brecha pesa mais, porque o arquivo roda como administrador.
- **Atestação** (`actions/attest-build-provenance`, `release.yml:113`), **segunda etapa** (a3): prova que o exe saiu do workflow `release.yml` do repositório, a partir de uma tag. Verificá-la no programa fecha boa parte da brecha acima (as tags `v*` são imutáveis pelo ruleset), mas exige a biblioteca `sigstore-go` (muitas dependências novas, mais superfície para o `govulncheck`, alguns MB no exe) e uma raiz de confiança do Sigstore embutida, que só se renova com uma release nova do eprocIA (o TUF falaria com servidores fora do GitHub). A atestação em si vem da API do GitHub (`GET /repos/jonyduque/eprocia/attestations/sha256:<hex>`). Fica fora desta primeira versão; a etapa seguinte terá spec própria. Para não fechar a porta: a conferência fica atrás de uma função `conferir(rel, arquivo)` única, onde a atestação entra depois sem mudar o fluxo.
- **Sem token, credenciais nem cookies:** o repositório é público desde 2026-10-26.
- **Temporário** do exe na pasta de instalação, com nome aleatório e `O_EXCL`; nunca em `%TEMP%`. O do instalador fica em `%LOCALAPPDATA%\eprocIA\atualizacao\`, com nome aleatório e `O_EXCL`, protegido entre a conferência e a abertura pelo handle sem compartilhamento de escrita.
- **`--apos-atualizar`** recebe só um PID; a limpeza mexe só em nomes fixos na própria pasta de instalação e na pasta de atualização, e não aceita caminho.

## 6. Privacidade e conflito com o RNF01

O RNF01 diz "Todo o processamento deve ser Local (Offline). Nenhum dado pode trafegar para APIs externas de conversão". A consulta de versão não processa nem envia dado de processo, mas é tráfego de rede, e o produto se apresenta como offline. O que o GitHub vê, a cada clique: o IP da máquina, a hora e o User-Agent `eprocIA/<versão>` (logo, que alguém naquele IP usa o eprocIA).

Resolução (a2):

- `PRD.md` ganha o RNF05 "Atualização": a única conexão de rede do programa é a consulta e o download de versões no GitHub, e ela só acontece quando o usuário clica em "Procurar atualização" (e, depois, em "Atualizar"). Não há procura automática, periódica nem na abertura. A consulta não envia conteúdo, nome nem caminho de processo.
- O RNF01 ganha "exceto a consulta de atualização iniciada pelo botão, regida pelo RNF05".
- Nenhuma telemetria, nem contagem de atualizações.
- O log local registra as consultas (hora e resultado), nada além.

## 7. Conflito com a rodada de correção

Grupos conforme `docs/revisao/2026-11-18-plano-correcao.md` no commit 05e7e7f:

- **F3** (F-VI-08 componente único de aviso com três tipos; F-AN-06 retorno ao salvar; F-AC-07 fim do `alert`): as mensagens do bloco "Atualizações" usam esse componente, não criam um quarto.
- **F2** (F-VI-01, F-VI-07, F-AC-11, painel lateral) e **F5** (F-AC-03, F-AC-09, F-AC-08): o bloco mora em Configurações, dentro do painel que F2 reorganiza; o rodapé não muda.
- **F18** (F-AC-14, menu da barra recolhida) e **F15** (F-AN-10, janela na bandeja): deixam de conflitar, porque não há aviso no rodapé nem notificação.

Recomendação: o backend (`internal/atualizacao`, com testes) pode ser feito em paralelo à rodada; a interface entra depois dela.

## 8. Testes que provam cada comportamento

`internal/atualizacao` (servidor `httptest.NewTLSServer`, cliente com o certificado do servidor e lista de hosts e prefixos injetáveis; nenhum teste fala com a rede real):

- **consulta:** versão maior aceita; igual ou menor devolve "em dia"; prerelease e draft ignorados; tag fora do formato; nenhum ou dois assets `eprocia-*-windows-amd64.exe`; instalador ausente; versão do nome diferente da tag; `SHA256SUMS` ausente; resposta acima de 1 MiB recusada;
- **comparação de versões:** 0.10.0 > 0.9.9, 1.0.0 > 0.99.99, sufixos recusados;
- **cliente:** URL inicial `http://` recusada; URL inicial de download fora do prefixo do repositório recusada; URL inicial em host fora da lista recusada; redirecionamento para host fora da lista recusado; redirecionamento para `http` recusado; redirecionamento para `objects.githubusercontent.com` aceito;
- **download:** SHA certo grava o arquivo; SHA errado apaga o temporário e não deixa `.download-*`; arquivo alterado depois da cópia e antes do rename é pego pelo recálculo; `Content-Length` acima do teto aborta antes de gravar; corpo maior que o anunciado aborta; cancelamento no meio apaga o temporário; linha do `SHA256SUMS` em maiúsculas, com e sem asterisco, com CRLF; duas linhas do mesmo nome recusadas;
- **versão do PE:** exe de teste com recurso de versão (em `testdata`, ou compilado no teste com um `.syso`) com versão certa e errada;
- **aplicar,** numa pasta temporária, com um exe auxiliar compilado no teste (um `main` que cria o arquivo de sinal, ou sai com código 1, ou fica parado):
  - troca bem-sucedida deixa `eprocia.exe` novo e `eprocia.anterior.exe`;
  - processo novo que sai com erro faz os nomes voltarem;
  - prazo estourado encerra o novo, espera ele sair e reverte;
  - falha injetada no rename do passo 2 deixa `eprocia.exe` de volta no lugar;
  - rename de exe em execução funciona (o auxiliar fica rodando durante o rename);
  - o auxiliar, iniciado com `--apos-atualizar`, não liga fila nem watcher antes de o "antigo" sair (o auxiliar registra a ordem num arquivo);
- **instalador** (com `ShellExecuteEx` trocado por fake): SHA conferido abre o arquivo com o handle ainda aberto e sem compartilhamento de escrita (uma escrita concorrente no arquivo falha); `ERROR_CANCELLED` devolve "cancelada" e não fecha o programa; sucesso para o watcher, fecha o gravador e pede `Quit`; SHA errado não abre nada;
- **`Instalacao()`:** pasta igual ao `InstallLocation` da chave HKCU (fake), inclusive uma pasta personalizada de `-Dir`, devolve `trocar`; `DisplayIcon` da chave HKLM (fake) igual ao exe, em Program Files ou em outra pasta, devolve `instalador`; Downloads e `build\bin` devolvem `so_avisar`; pasta por usuário sem escrita devolve `so_avisar`;
- **versão anterior:** lida do PE de `eprocia.anterior.exe`; sem o arquivo, o botão não aparece;
- **limpeza na abertura:** `eprocia.falhou.exe`, `.download-*`, sinais esquecidos e o conteúdo da pasta de atualização são apagados também sem `--apos-atualizar`; `eprocia.anterior.exe` fica.

`app_test.go`: **nenhuma conexão sem clique**: `startup` (com e sem `--apos-atualizar`) e o arranque em dois tempos não chamam o cliente (cliente falso que falha o teste se chamado); `InstalarAtualizacao` sem `VerificarAtualizacao` antes devolve erro sem consultar; `InstalarAtualizacao` com fila ocupada fica em `aguardando_fila` e aplica quando a fila esvazia (fila e aplicador falsos); antes do passo 1 o gravador do histórico é fechado, e reaberto na reversão; URL de notas fora do prefixo não é aberta.

`frontend/instalador-nsis.test.mjs` (teste de texto sobre `build/windows/installer/project.nsi`, no mesmo glob que a CI já roda): `MUI_FINISHPAGE_RUN` e `MUI_FINISHPAGE_RUN_FUNCTION` definidos antes de `MUI_PAGE_FINISH`; sem `MUI_FINISHPAGE_RUN_NOTCHECKED` (marcado por padrão); a função usa `explorer.exe` e não `Exec` direto do exe; o arquivo continua com o atalho e o desinstalador de antes. A prova de compilação é `wails build -clean -nsis` no ensaio manual (abaixo).

`frontend/atualizacao.test.mjs`: textos e botões de cada fase e de cada modo (inclusive a frase do modo `instalador` sobre a caixa "Abrir o eprocIA"); teste por regex de que nenhum componente monta URL de download e de que nada chama `VerificarAtualizacao` fora do clique do botão (nenhuma chamada em `onMount` ou em assinatura de evento).

**Manual,** antes da primeira release com o recurso: um ensaio com tag rc não serve (prerelease é ignorada pela consulta). O ensaio é instalar a versão N pelo `install.ps1` numa conta de teste do Windows e pelo instalador NSIS em outra, publicar N+1 e ver, em cada uma, a procura pelo botão, a troca (ou o instalador com UAC, aceito e recusado, com "Abrir o eprocIA" marcado e desmarcado, conferindo que o programa aberto por ele roda sem administrador e aceita arrastar um ZIP do Explorer), o sinal de vida, a reversão e o "Voltar para a versão anterior". O primeiro lançamento com o recurso só se prova no lançamento seguinte; vale planejar uma 0.x.1 pequena logo depois.

## 9. Estimativa

Grande. Go: cerca de 780 linhas de código e 1.000 de teste (consulta, download, cliente restrito, versão do PE, troca e reversão com processo auxiliar, arranque em dois tempos com `--apos-atualizar`, abertura do instalador NSIS, detecção HKCU e HKLM). Frontend: cerca de 150 linhas e 90 de teste. Mais PRD, CLAUDE.md, skill de release e ensaio manual em duas releases e dois tipos de instalação. Uma semana de agente com revisão (sem mudança no total: saem a procura automática, o estado em `atualizacao.json`, o aviso no rodapé e a notificação; entra o modo NSIS). A mudança do NSIS (e7) soma cerca de 15 linhas em `project.nsi` e 30 de teste, meio dia de agente, já incluídos no total (a semana). A atestação (a3) fica para uma etapa seguinte, de dois a três dias.

## 10. Decisões do dono

De 2026-11-19:

- **a1.** O que fazer com versão nova: "Nunca procurar sozinho; só pelo botão manual". Aplicado nas seções 1, 3.1 e 4.
- **a2.** Procura e o offline do PRD: "O usuário clica na opção de buscar atualização para iniciar". Sem procura automática nem pergunta na primeira abertura; nenhuma conexão sai sem o clique; a exceção ao RNF01 cobre só a conexão iniciada pelo botão. Aplicado nas seções 1, 3.1, 6 e 8.
- **a3.** Verificação: "Só SHA256SUMS agora; atestação depois". Aplicado na seção 5 e na estimativa.
- **a4.** Instalação NSIS com administrador: "Baixar o instalador, conferir SHA-256 e abrir (pede admin)". Aplicado nas seções 1, 2, 3.1, 4, 5 e 8.

De 2026-11-20:

- **e7.** Depois do instalador NSIS: mudar `build/windows/installer/project.nsi` para oferecer "Abrir o eprocIA" na página final. Aplicado nas seções 1, 2, 3.1, 3.4, 4 e 8 e na estimativa (seção 9). Restrição do dono: ele tem mudanças locais sem commit nesse arquivo no checkout principal; a implementação parte da versão da `main`, não as sobrescreve nem depende delas, e o plano avisa o dono antes de mexer no arquivo (seção 3.4).

## 11. Resposta à revisão

Todos os achados foram aceitos: A1 (gravador do histórico fechado antes do passo 1, reaberto na reversão), A2 (processo novo só liga fila e watcher depois do sinal e da saída do antigo), A3 (protocolo como contrato; sinal em arquivo separado; o `atualizacao.json` deixou de existir com a1), A4 (falha no passo 2 desfaz o passo 1, com teste; espera o novo sair antes de reverter), A5 (grupos renumerados; a forma recolhida do aviso deixou de existir com a1), A6 (SHA recalculado sobre o arquivo fechado), A7 (regras da URL inicial e do redirecionamento), A8 (`InstallLocation` como referência), A9, A10 (a2 condicionada à a1; custo da a3 reescrito) e A11 (limpeza na abertura normal). Nenhuma recusa.
