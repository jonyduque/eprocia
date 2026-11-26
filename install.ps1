<#
.SYNOPSIS
    Instala, atualiza ou remove o eprocIA (a instalacao por usuario ou a para todos).

.DESCRIPTION
    Baixa o eprocia-<versão>-windows-amd64.exe da release no GitHub, confere com o
    SHA256SUMS.txt publicado e copia para %LOCALAPPDATA%\Programs\eprocIA, com atalhos no
    menu Iniciar e na área de trabalho e entrada em "Aplicativos instalados" (HKCU).
    Rodar de novo atualiza no lugar a versao ja instalada: a por usuario ou, se so houver
    ela, a instalacao para todos os usuarios (feita pelo instalador), que e atualizada pelo
    instalador da release em silencio. So essa (atualizar ou remover a instalacao para
    todos) pede permissao de administrador (UAC); a instalacao por usuario nunca pede.
    Com o eprocIA aberto, o script pergunta antes de encerra-lo e so o encerra na hora de
    trocar o arquivo.

    Num terminal elevado a instalação continua sendo do usuário: o script roda de novo, sem
    privilégios, como o dono da sessão do Windows (o dono do explorer.exe), por uma tarefa
    agendada de uso único. Assim nada vai para o perfil da conta de administrador.
    Antes, pergunta se pode rodar como esse usuario numa janela oculta.
    Funciona no Windows PowerShell 5.1 e no 7.

    irm https://raw.githubusercontent.com/jonyduque/eprocia/main/install.ps1 | iex

    Com parâmetros:
    & ([scriptblock]::Create((irm https://raw.githubusercontent.com/jonyduque/eprocia/main/install.ps1))) -Uninstall

    Repositório privado: o script usa $env:GITHUB_TOKEN, $env:GH_TOKEN ou `gh auth token`
    quando o acesso anônimo falha. Para obter o próprio script nesse caso:
    & ([scriptblock]::Create((gh api repos/jonyduque/eprocia/contents/install.ps1 -H 'Accept: application/vnd.github.raw')))

.PARAMETER Version
    Tag da release (ex.: v0.3.0). Padrão: a mais recente.

.PARAMETER Dir
    Pasta de instalação. Padrão: %LOCALAPPDATA%\Programs\eprocIA.

.PARAMETER From
    Instala este eprocia.exe local em vez de baixar a release.

.PARAMETER Uninstall
    Remove o executável, os atalhos, a entrada de desinstalação e o "Iniciar com o Windows".
    Configurações em %APPDATA%\eprocIA ficam, a não ser com -Purge.

.PARAMETER Purge
    Com -Uninstall: apaga também configurações, histórico de falhas, dados do WebView2 e o
    cache do OCR.

.PARAMETER NoDesktop
    Não cria (e remove, se houver) o atalho na área de trabalho.

.PARAMETER NoStart
    Não abre o eprocIA ao terminar.

.PARAMETER DryRun
    Mostra o que seria feito sem alterar nada e depois pergunta se deve fazer de verdade.

.PARAMETER Yes
    Não pergunta antes de instalar, atualizar ou remover. Sem console para perguntar (entrada
    redirecionada, tarefa agendada), o script só segue com -Yes.
    Com -Yes o script tambem encerra o eprocIA aberto sem perguntar.

.PARAMETER Log
    Interno: arquivo de log da cópia sem privilégios iniciada por um terminal elevado.

.PARAMETER Self
    Interno: caminho do script quando o 5.1 o relê como UTF-8.

.PARAMETER Machine
    Interno: o filho elevado age so na instalacao para todos os usuarios (so vale com -Log).

.PARAMETER ClosePids
    Interno: PIDs do eprocIA que o usuario aceitou encerrar, separados por virgula ('-' se
    nenhum).

.PARAMETER CloseAny
    Interno: encerra o eprocIA aberto sem lista; so e passado quando o usuario deu -Yes.
#>
# eprocia-install: elevacao-v1
# Sem parâmetros posicionais: uma palavra desconhecida é erro antes de qualquer ação.
[CmdletBinding(PositionalBinding = $false)]
param(
    [string]$Version = '',
    [string]$Dir = '',
    [string]$From = '',
    [switch]$Uninstall,
    [switch]$Purge,
    [switch]$NoDesktop,
    [switch]$NoStart,
    [Alias('dry-run')]
    [switch]$DryRun,
    [switch]$Yes,
    [string]$Log = '',
    [string]$Self = '',
    [switch]$Machine,
    [string]$ClosePids = '',
    [switch]$CloseAny
)

# O arquivo não tem BOM, porque o BOM quebra o `irm | iex`. Rodado com -File, o 5.1 o lê
# como ANSI e estraga os acentos; então ele se relê como UTF-8 e roda essa cópia.
if ($PSCommandPath -and -not $Self -and $PSVersionTable.PSVersion.Major -lt 6) {
    $bytes = [IO.File]::ReadAllBytes($PSCommandPath)
    if (-not ($bytes.Length -ge 3 -and $bytes[0] -eq 0xEF -and $bytes[1] -eq 0xBB -and $bytes[2] -eq 0xBF)) {
        $PSBoundParameters['Self'] = $PSCommandPath
        & ([scriptblock]::Create([Text.Encoding]::UTF8.GetString($bytes))) @PSBoundParameters
        if ($Log) { exit $LASTEXITCODE }
        return
    }
}
$selfPath = if ($Self) { $Self } elseif ($PSCommandPath) { $PSCommandPath } else { '' }
# Texto do bloco em execucao (o da branch, quando rodado por [scriptblock]::Create). Lido
# aqui, no nivel do script: dentro de uma funcao, $MyInvocation seria o da funcao.
$selfText = ''; try { if ($MyInvocation.MyCommand.ScriptBlock) { $selfText = $MyInvocation.MyCommand.ScriptBlock.ToString() } } catch { }

Set-StrictMode -Version 2.0
$ErrorActionPreference = 'Stop'
$ProgressPreference = 'SilentlyContinue'

$repo = 'jonyduque/eprocia'
$app = 'eprocIA'
$exeName = 'eprocia.exe'
# Sempre pelo caminho completo: o ShellExecuteEx (-Verb RunAs) e o agendador procurariam
# antes na pasta atual ou no PATH.
$psExe = Join-Path $env:SystemRoot 'System32\WindowsPowerShell\v1.0\powershell.exe'
$flagTray = '--tray' # internal/autostart.FlagTray
$scriptUrl = "https://raw.githubusercontent.com/$repo/main/install.ps1"
$uninstallKey = 'HKCU:\Software\Microsoft\Windows\CurrentVersion\Uninstall\eprocIA'
$runKey = 'HKCU:\Software\Microsoft\Windows\CurrentVersion\Run'

# Console de verdade: banner, cores e barra de progresso. Não há com a saída redirecionada
# nem na cópia da tarefa agendada (-Log), que roda oculta. NO_COLOR (no-color.org) tira as cores.
$fancy = $false
try { $fancy = -not $Log -and -not [Console]::IsOutputRedirected } catch { }
$useColor = $fancy -and -not $env:NO_COLOR

# Estilos ANSI montados com o caractere ESC: `$PSStyle` e `` `e `` só existem no 7.
$esc = [char]27
function Get-Rgb([int]$R, [int]$G, [int]$B) { if ($useColor) { "$esc[38;2;$R;$G;${B}m" } else { '' } }
$cPurple = Get-Rgb 154 92 219
$cPeach  = Get-Rgb 255 117 117
$cGray   = Get-Rgb 128 128 128
$cPink   = Get-Rgb 224 91 191
$cBlue   = Get-Rgb 26 124 240
$cDim    = if ($useColor) { "$esc[2m" } else { '' }
$cBold   = if ($useColor) { "$esc[1m" } else { '' }
$cReset  = if ($useColor) { "$esc[0m" } else { '' }

$cGreen  = Get-Rgb 80 200 120
$cYellow = Get-Rgb 230 190 60
$cRed    = Get-Rgb 235 80 80
# Intensidade normal: tira o negrito sem perder a cor do resto da linha.
$cNoBold = if ($useColor) { "$esc[22m" } else { '' }

# Marcadores fora do ASCII montados com [char] (o 5.1 lê o arquivo sem BOM como ANSI).
$iconOk   = [string][char]0x2705
$iconFail = [string][char]0x274C
$bullet   = [string][char]0x203A
$ellipsis = [string][char]0x2026

# Texto de mais de $Max caracteres é cortado com reticências (o log guarda o texto inteiro).
# Texto com caminho (C:\...) sai inteiro: cortado, não mostraria onde as coisas ficam.
function Limit-Text([string]$Text, [int]$Max = 49) {
    if ($Text.Length -le $Max -or $Text -match '[A-Za-z]:\\') { return $Text }
    $Text.Substring(0, $Max - 1) + $ellipsis
}
# O nome do app sempre em negrito.
function Format-Name([string]$Text) {
    if ($cBold) { $Text -creplace 'eprocIA', "${cBold}eprocIA$cNoBold" } else { $Text }
}

# UTF-8 para as setas, o ponto e os ícones; o valor anterior volta no fim, porque sob
# `irm | iex` a mudança ficaria valendo no terminal de quem rodou.
$prevOutputEncoding = [Console]::OutputEncoding
try { [Console]::OutputEncoding = [System.Text.Encoding]::UTF8 } catch { }
function Restore-Console { try { [Console]::OutputEncoding = $prevOutputEncoding } catch { } }

# Erro: linha vermelha com o ícone, sem corte (a mensagem inteira importa).
function Write-Fail([string]$Text) { Write-Host "  $iconFail $cRed$(Format-Name $Text)$cReset" }

if ($Version -and $Version -notmatch '^v?\d+\.\d+\.\d+([-.][0-9A-Za-z.]+)?$') {
    Write-Fail "Versão inválida: '$Version' (use uma tag de release, ex.: v0.3.0)"
    Restore-Console
    return
}
if ($Purge -and -not $Uninstall) {
    Write-Fail '-Purge só vale junto com -Uninstall'
    Restore-Console
    return
}

# Título da janela durante a execução; Restore-Window devolve o anterior.
$script:prevTitle = $null
function Set-WindowTitle([string]$Title) {
    try {
        if ($null -eq $script:prevTitle) { $script:prevTitle = $Host.UI.RawUI.WindowTitle }
        $Host.UI.RawUI.WindowTitle = $Title
    } catch { }
}
function Restore-Window {
    try { if ($null -ne $script:prevTitle) { $Host.UI.RawUI.WindowTitle = $script:prevTitle; $script:prevTitle = $null } } catch { }
}

# Banner: o ícone do app (quadrado azul em degradê com o "e" e a estrela brancos) e o nome,
# em pixels: '.' vazio, 'b' azul, 'w' branco, '#' texto na cor padrão, '@' texto azul ("IA").
# Cada linha da tela junta duas linhas de pixels com meios-blocos, montados com [char]
# porque o 5.1 lê este arquivo sem BOM como ANSI.
$bannerPixels = @(
    '..bbbbbbbbbb...............................................'
    '.bbbbbbbbbwbb..............................................'
    'bbbbbbbbbwwwbb..................................@@@@@...@..'
    'bbbbwwwwbbwbbb......###...........................@....@.@.'
    'bbbwwbbwwbbbbb.....#...#.####..#.##..###...###....@...@...@'
    'bbwwbbbbwwbbbb.....#####.#...#.##...#...#.#...#...@...@@@@@'
    'bbwwwwwwwwbbbb.....#.....#...#.#....#...#.#.......@...@...@'
    'bbwwbbbbbbbbbb.....#...#.####..#....#...#.#...#...@...@...@'
    'bbbwwbbwwbbbbb......###..#.....#.....###...###..@@@@@.@...@'
    'bbbbwwwwbbbbbb...........#.................................'
    '.bbbbbbbbbbbb............#.................................'
    '..bbbbbbbbbb...............................................'
)

# Cor de um pixel como "r;g;b", 'def' (cor padrão do terminal) ou $null (vazio). O azul do
# ícone vai em degradê na diagonal, de #1A7CF0 a #0056B3.
function Get-BannerColor([char]$P, [int]$X, [int]$Y) {
    switch ($P) {
        'b' {
            $k = ($X + $Y) / 30.0
            return '{0};{1};{2}' -f [int](26 - 26 * $k), [int](124 - 38 * $k), [int](240 - 61 * $k)
        }
        'w' { return '255;255;255' }
        '#' { return 'def' }
        '@' { return '26;124;240' }
    }
    $null
}

function Get-BannerLines {
    $up = [char]0x2580; $down = [char]0x2584; $full = [char]0x2588
    for ($r = 0; $r -lt $bannerPixels.Count; $r += 2) {
        $top = $bannerPixels[$r]; $bottom = $bannerPixels[$r + 1]
        $sb = New-Object Text.StringBuilder '  '
        for ($x = 0; $x -lt $top.Length; $x++) {
            $a = Get-BannerColor $top[$x] $x $r
            $b = Get-BannerColor $bottom[$x] $x ($r + 1)
            $fa = if ($a -eq 'def') { "$esc[39m" } else { "$esc[38;2;${a}m" }
            $fb = if ($b -eq 'def') { "$esc[39m" } else { "$esc[38;2;${b}m" }
            if (-not $a -and -not $b) { [void]$sb.Append("$esc[0m ") }
            elseif (-not $b) { [void]$sb.Append("$esc[0m$fa$up") }
            elseif (-not $a) { [void]$sb.Append("$esc[0m$fb$down") }
            elseif ($a -eq $b) { [void]$sb.Append("$esc[0m$fa$full") }
            else { [void]$sb.Append("$esc[0m$fa$esc[48;2;${b}m$up") }
        }
        [void]$sb.Append("$esc[0m")
        $sb.ToString()
    }
}

# No início de cada execução visível. Janela mais estreita que o banner, ou sem cor: só o nome.
function Get-WindowWidth { try { [int]$Host.UI.RawUI.WindowSize.Width } catch { 0 } }

function Show-Banner {
    if (-not $fancy) { return }
    $width = Get-WindowWidth
    Write-Host ''
    if ($useColor -and $width -ge $bannerPixels[0].Length + 3) {
        foreach ($line in Get-BannerLines) { Write-Host $line }
    } else {
        Write-Host "  ${cBold}eproc$cBlue${cBold}IA$cReset"
    }
}

# A cópia sem privilégios grava cada linha no -Log (tipo, texto, valor) e o terminal
# elevado as reapresenta com os mesmos estilos.
function Add-Log([string]$Kind, [string]$Text, [string]$Value = '') {
    if ($Log) { Add-Content -LiteralPath $Log -Value "$Kind`t$Text`t$Value" -Encoding UTF8 }
}
function Write-Title([string]$Text) {
    Write-Host "$cBold$cPink`n$Text$cReset`n"
    Add-Log 'title' $Text
}
# Uma linha de passo: o texto em cinza e o valor (um caminho, uma versão) em roxo. Texto e
# valor juntos cabem em 49 caracteres: o valor é cortado primeiro.
function Write-Step([string]$Text, [string]$Value = '') {
    $t = $Text; $v = $Value
    if ($v) {
        $room = 49 - $t.Length - 1
        if ($room -lt 8) { $t = Limit-Text "$Text $Value"; $v = '' } else { $v = Limit-Text $v $room }
    } else {
        $t = Limit-Text $t
    }
    $line = "  $bullet $cGray$(Format-Name $t)"
    if ($v) { $line += " $cPurple$(Format-Name $v)" }
    Write-Host "$line$cReset"
    Add-Log 'step' $Text $Value
}
function Write-Ok([string]$Text) { Write-Host "  $iconOk $cGreen$(Format-Name (Limit-Text $Text))$cReset"; Add-Log 'ok' $Text }
function Write-Note([string]$Text) { Write-Host "  $cGray$(Format-Name (Limit-Text $Text))$cReset"; Add-Log 'note' $Text }
function Write-Warn([string]$Text) { Write-Host "  ! $cYellow$(Format-Name (Limit-Text $Text))$cReset"; Add-Log 'warn' $Text }
function Write-Done([string]$Text) { Write-Host ''; Write-Host "  $cGreen$(Format-Name (Limit-Text $Text))$cReset"; Add-Log 'done' $Text }

function Show-Log([string]$Path) {
    if (-not (Test-Path -LiteralPath $Path)) { return }
    $saved = $Log
    $script:Log = ''
    try {
        foreach ($line in Get-Content -LiteralPath $Path -Encoding UTF8) {
            $kind, $text, $value = $line -split "`t", 3
            switch ($kind) {
                'title' { Write-Title $text }
                'step'  { Write-Step $text $value }
                'ok'    { Write-Ok $text }
                'note'  { Write-Note $text }
                'warn'  { Write-Warn $text }
                'done'  { Write-Done $text }
                'fail'  { Write-Fail $text }
            }
        }
    } finally { $script:Log = $saved }
}

# Atalhos: chave = tecla; valor = ação.
function Get-Footer([System.Collections.IDictionary]$Keybinds) {
    $sKey = "$esc[1m$(Get-Rgb 150 150 150)"
    $sBind = "$esc[22m$(Get-Rgb 110 110 110)"
    $parts = foreach ($k in @($Keybinds.Keys)) { "$sKey$($k.ToString().Trim()) $sBind$($Keybinds[$k].ToString().Trim())" }
    Write-Host ("  " + ($parts -join " $([char]0x00B7) ") + $cReset)
}

# Escolha com as setas: cima/baixo (ou esquerda/direita, Tab) movem, Enter escolhe, Esc
# cancela ($null). Sem console para ler teclas, devolve $Default.
function Read-Choice([string]$Question, [System.Collections.IDictionary]$Options, [int]$Default = 0) {
    try { if ([Console]::IsInputRedirected) { return $Default } } catch { return $Default }
    if ($Options.Count -eq 0) { return $null }

    Write-Title $Question
    # [Math]::Clamp não existe no .NET Framework do 5.1.
    $sel = [Math]::Max(0, [Math]::Min($Default, $Options.Count - 1))
    $keyOptions = @($Options.Keys)
    $keybinds = [ordered]@{
        # Setas como [char]: os bytes UTF-8 delas viram aspas tipográficas no 5.1 sem BOM.
        "$([char]0x2193)$([char]0x2191)" = 'movem'
        'Enter' = 'escolhe'
        'Esc'   = 'cancela'
    }

    # A primeira linha só é conhecida depois do primeiro desenho: escrever no fim da janela
    # rola o buffer e uma posição lida antes ficaria velha.
    $startTop = -1
    $cursorVisibleOriginal = [Console]::CursorVisible
    # A explicação começa 5 colunas depois da opção mais longa, com inicial maiúscula.
    $keyWidth = 5
    foreach ($k in $keyOptions) { $keyWidth = [Math]::Max($keyWidth, $k.ToString().Length + 5) }
    $descs = foreach ($k in $keyOptions) {
        $d = Limit-Text ([string]$Options[$k])
        if ($d) { $d.Substring(0, 1).ToUpper() + $d.Substring(1) } else { $d }
    }
    $descs = @($descs)
    $draw = {
        if ($startTop -ge 0) { [Console]::SetCursorPosition(0, $startTop) }
        for ($i = 0; $i -lt $Options.Count; $i++) {
            $k = $keyOptions[$i].ToString().PadRight($keyWidth)
            $v = $descs[$i]
            if ($i -eq $sel) {
                $colArrow = "$cPurple$cBold{0,-2}$cReset" -f " $([char]0x2192)"
                $colKey   = "$cPurple$cBold$k$cReset"
                $colDesc  = "$cPurple$cDim$v$cReset"
            } else {
                $colArrow = "$cGray{0,-2}$cReset" -f '  '
                $colKey   = $k
                $colDesc  = "$cDim$v$cReset"
            }
            Write-Host "  $colArrow $colKey$colDesc"
        }
        Get-Footer $keybinds
    }

    try {
        [Console]::CursorVisible = $false
        & $draw
        $startTop = [Math]::Max(0, [Console]::CursorTop - $Options.Count - 1)
        while ($true) {
            $k = [Console]::ReadKey($true)
            switch ($k.Key) {
                { $_ -in 'UpArrow', 'LeftArrow' }           { $sel = ($sel + $Options.Count - 1) % $Options.Count }
                { $_ -in 'DownArrow', 'RightArrow', 'Tab' } { $sel = ($sel + 1) % $Options.Count }
                'Enter'  { return $sel }
                'Escape' { return $null }
            }
            & $draw
        }
    } finally {
        [Console]::CursorVisible = $cursorVisibleOriginal
    }
}

# Erros terminam com uma linha vermelha; `exit` fecharia o terminal sob `irm | iex`.
function Fail([string]$Text) { throw $Text }

# Há um console para perguntar? Não há na tarefa agendada nem com a entrada redirecionada.
# Sob `irm | iex` a entrada é o console, e o Read-Host lê normalmente.
function Test-Interactive {
    try { [Environment]::UserInteractive -and -not [Console]::IsInputRedirected } catch { $false }
}

# Verdadeiro depois do "Sim" da simulação: a execução real que vem em seguida não pergunta de novo.
$script:confirmed = $false
# PIDs do eprocIA que o usuario aceitou encerrar (Confirm-Close); o Stop-App so encerra estes.
$script:closePids = @()
# PIDs que a simulacao mostrou (e o "Sim" dela aceitou).
$script:dryRunPids = @()

# Pergunta antes de mudar a máquina, com a escolha por setas: a ação ($Action, descrita por
# $Detail) ou Cancelar; Enter escolhe, Esc cancela. Com -Yes não pergunta; na simulação só
# mostra o que perguntaria; sem console, exige -Yes.
# -Fresh: pergunta de novo mesmo depois do "Sim" da simulacao (algo que ela nao mostrou).
function Confirm-Plan([string]$Question, [string]$Action, [string]$Detail, [switch]$Fresh) {
    if ($Yes) { return $true }
    if ($script:confirmed -and -not $Fresh) { return $true }
    if ($DryRun -and -not $script:confirmed) { Write-Note "Perguntaria: $Question ($Action ou Cancelar)"; return $true }
    if (-not (Test-Interactive)) { Fail 'Sem console para confirmar. Rode de novo com -Yes para seguir sem pergunta.' }
    $options = [ordered]@{}
    $options[$Action] = $Detail
    $options['Cancelar'] = 'Sai sem alterar nada'
    (Read-Choice $Question $options) -eq 0
}

function Write-Cancelled { Write-Warn 'Cancelado. Nada foi alterado.' }

function Test-Elevated {
    $id = [Security.Principal.WindowsIdentity]::GetCurrent()
    (New-Object Security.Principal.WindowsPrincipal $id).IsInRole([Security.Principal.WindowsBuiltInRole]::Administrator)
}

# Dono do explorer.exe da sessão deste processo (ou de qualquer sessão, se esta não tiver
# shell, como num SSH elevado).
function Get-SessionUser {
    $session = (Get-Process -Id $PID).SessionId
    $shells = @(Get-CimInstance Win32_Process -Filter "Name = 'explorer.exe'")
    if ($shells.Count -eq 0) { Fail 'Nenhum usuário com sessão aberta no Windows. Entre com a conta que vai usar o eprocIA e rode de novo.' }
    $mine = @($shells | Where-Object { $_.SessionId -eq $session })
    if ($mine.Count -eq 0) { $mine = $shells }
    $owner = Invoke-CimMethod -InputObject $mine[0] -MethodName GetOwner
    if ($owner.ReturnValue -ne 0) { Fail 'Não foi possível identificar o usuário da sessão.' }
    "$($owner.Domain)\$($owner.User)"
}

function Get-InstallDir { if ($Dir) { $Dir } else { Join-Path $env:LOCALAPPDATA "Programs\$app" } }

# Pasta de instalação de outro usuário (terminal elevado), pelo perfil registrado dele.
function Get-UserInstallDir([string]$User) {
    if ($Dir -or $User -eq [Security.Principal.WindowsIdentity]::GetCurrent().Name) { return Get-InstallDir }
    try {
        $sid = (New-Object Security.Principal.NTAccount $User).Translate([Security.Principal.SecurityIdentifier]).Value
        $userHome = (Get-ItemProperty -LiteralPath "HKLM:\SOFTWARE\Microsoft\Windows NT\CurrentVersion\ProfileList\$sid").ProfileImagePath
        if ($userHome) { return Join-Path $userHome "AppData\Local\Programs\$app" }
    } catch { }
    "%LOCALAPPDATA%\Programs\$app ($User)"
}

# Com -Purge: configurações e falhas.json, dados do WebView2 e binários do OCR extraídos.
function Get-PurgeDirs {
    @((Join-Path $env:APPDATA $app), (Join-Path $env:APPDATA $exeName), (Join-Path ([IO.Path]::GetTempPath()) 'eprocIA_bin'))
}

function Enable-Tls12 {
    if ($PSVersionTable.PSVersion.Major -lt 6) {
        # O 5.1 pode usar TLS 1.0 por padrão, que o GitHub recusa.
        [Net.ServicePointManager]::SecurityProtocol = [Net.ServicePointManager]::SecurityProtocol -bor [Net.SecurityProtocolType]::Tls12
    }
}

function Get-Shortcuts {
    @((Join-Path ([Environment]::GetFolderPath('Programs')) "$app.lnk"),
      (Join-Path ([Environment]::GetFolderPath('Desktop')) "$app.lnk"))
}

# --- GitHub -----------------------------------------------------------------------------

$script:token = $null
function Get-Token {
    if ($null -ne $script:token) { return $script:token }
    $script:token = ''
    if ($env:GITHUB_TOKEN) { $script:token = $env:GITHUB_TOKEN }
    elseif ($env:GH_TOKEN) { $script:token = $env:GH_TOKEN }
    elseif (Get-Command gh -ErrorAction SilentlyContinue) {
        $t = & gh auth token 2>$null
        if ($LASTEXITCODE -eq 0 -and $t) { $script:token = ([string]$t).Trim() }
    }
    $script:token
}

# Código HTTP do erro, ou 0 se a conexão caiu. Um erro de um método .NET (HttpWebRequest)
# chega embrulhado: a resposta fica numa exceção interna.
# Queda de conexão: erro de rede sem código HTTP. Os erros locais saem por Fail e não repetem.
function Test-Dropped($ErrorRecord) {
    if ((Get-Status $ErrorRecord) -ne 0) { return $false }
    for ($e = $ErrorRecord.Exception; $e; $e = $e.InnerException) {
        if ($e -is [Net.WebException] -or $e -is [IO.IOException] -or $e -is [Net.Sockets.SocketException] -or $e -is [TimeoutException] -or
            $e.GetType().FullName -in 'System.Net.Http.HttpRequestException', 'System.Threading.Tasks.TaskCanceledException') { return $true }
    }
    $false
}

function Get-Status($ErrorRecord) {
    for ($e = $ErrorRecord.Exception; $e; $e = $e.InnerException) {
        try { if ($e.Response) { return [int]$e.Response.StatusCode } } catch { }
    }
    0
}

# Linha da barra de download, que cabe em $Width colunas:
# "  Baixando  [blocos cheios e vazios]  45%  60,1 de 132,7 MB". Sem o tamanho total
# ($Total <= 0), só os MB baixados.
function Format-Bar([long]$Done, [long]$Total, [int]$Width) {
    if ($Width -le 0) { $Width = 80 }
    if ($Total -le 0) { return "  Baixando  $cPeach$('{0:N1} MB' -f ($Done / 1MB))$cReset" }
    $pct = [int][Math]::Floor([Math]::Min(100, $Done * 100.0 / $Total))
    $tail = ' {0}%  {1:N1} de {2:N1} MB' -f $pct, ($Done / 1MB), ($Total / 1MB)
    $head = '  Baixando  '
    # Uma coluna de folga: escrever na última coluna quebra a linha em alguns terminais.
    $size = [Math]::Min(30, $Width - 1 - $head.Length - 2 - $tail.Length - 1)
    if ($size -lt 5) {
        $short = '  {0}%  {1:N1} MB' -f $pct, ($Done / 1MB)
        if ($short.Length -lt $Width) { return $short }
        return ''
    }
    $fill = [int][Math]::Floor($size * $pct / 100)
    $full = ([string][char]0x2588) * $fill
    $empty = ([string][char]0x2591) * ($size - $fill)
    "$head[$cBlue$full$cGray$empty$cReset]$tail"
}

# Baixa um arquivo em blocos, com barra de progresso desenhada na linha corrente (a do
# Write-Progress aparece em outro lugar no Windows Terminal e deixa o 5.1 lento). Sem console
# não há barra, só a linha de fim. Os redirecionamentos (o GitHub manda os assets para outro
# host) são seguidos aqui, para o token não ir junto a outro servidor.
function Save-WithProgress([hashtable]$Request) {
    $uri = [Uri]$Request.Uri
    $headers = @{}
    if ($Request.ContainsKey('Headers')) { foreach ($k in $Request.Headers.Keys) { $headers[$k] = $Request.Headers[$k] } }
    for ($hop = 0; ; $hop++) {
        $req = [Net.HttpWebRequest][Net.WebRequest]::Create($uri)
        $req.AllowAutoRedirect = $false
        $req.UserAgent = 'eprocIA-install'
        $req.ReadWriteTimeout = 60000
        foreach ($k in $headers.Keys) {
            if ($k -eq 'Accept') { $req.Accept = $headers[$k] } else { $req.Headers[$k] = $headers[$k] }
        }
        try { $resp = $req.GetResponse() }
        catch {
            for ($e = $_.Exception; $e; $e = $e.InnerException) {
                if ($e -is [Net.WebException] -and $e.Response) { try { $e.Response.GetResponseStream().Dispose() } catch { } }
            }
            throw
        }
        $code = [int]$resp.StatusCode
        if ($code -lt 300 -or $code -ge 400) { break }
        $location = $resp.Headers['Location']
        $resp.Close()
        if (-not $location -or $hop -ge 5) { Fail "Redirecionamento inválido ao baixar $($Request.Uri)" }
        $next = New-Object Uri($uri, $location)
        if ($next.Host -ne $uri.Host) { $headers.Remove('Authorization') }
        $uri = $next
    }
    try {
        $total = $resp.ContentLength
        $in = $resp.GetResponseStream()
        try { $out = [IO.File]::Create($Request.OutFile) }
        catch { $in.Dispose(); Fail "Não foi possível gravar $($Request.OutFile): $($_.Exception.InnerException.Message)" }
        try {
            $buf = New-Object byte[] 262144
            $done = [long]0
            $clock = [Diagnostics.Stopwatch]::StartNew()
            $cursor = $null
            if ($fancy) { try { $cursor = [Console]::CursorVisible; [Console]::CursorVisible = $false } catch { } }
            while (($n = $in.Read($buf, 0, $buf.Length)) -gt 0) {
                try { $out.Write($buf, 0, $n) }
                catch { Fail "Não foi possível gravar $($Request.OutFile): $($_.Exception.InnerException.Message)" }
                $done += $n
                if ($fancy -and $clock.ElapsedMilliseconds -ge 250) {
                    $clock.Restart()
                    # `r volta ao início da linha; ESC[K apaga o resto da anterior.
                    Write-Host "`r$(Format-Bar $done $total (Get-WindowWidth))$esc[K" -NoNewline
                }
            }
            # Conexão fechada antes do fim: conta como queda e Invoke-Download repete.
            if ($total -ge 0 -and $done -ne $total) { throw (New-Object IO.IOException "Download incompleto ($done de $total bytes)") }
        } finally {
            $out.Dispose()
            $in.Dispose()
            # A linha da barra some; a nota de nova tentativa ou o ok começam numa linha limpa.
            if ($fancy) {
                Write-Host "`r$esc[K" -NoNewline
                if ($null -ne $cursor) { try { [Console]::CursorVisible = $cursor } catch { } }
            }
        }
        Write-Ok ('Baixado: {0:N1} MB' -f ($done / 1MB))
    } finally { $resp.Close() }
}

# Repete até 3 vezes quando a conexão cai (o exe tem mais de 100 MB); erro HTTP não repete.
# O arquivo grande (OutFile) vai em blocos com progresso; as chamadas pequenas, pelo
# Invoke-WebRequest.
function Invoke-Download([hashtable]$Request) {
    for ($try = 1; ; $try++) {
        try {
            if ($Request.ContainsKey('OutFile')) { return Save-WithProgress $Request }
            return Invoke-WebRequest @Request
        }
        catch {
            if (-not (Test-Dropped $_) -or $try -ge 3) { throw }
            Write-Note "Conexão interrompida. Tentando de novo ($($try + 1)/3)"
            Start-Sleep -Seconds (2 * $try)
        }
    }
}

# Tenta sem credencial; se o GitHub negar (repositório privado), repete com o token.
function Invoke-GitHub([string]$Uri, [string]$Accept = 'application/vnd.github+json', [string]$OutFile = '', [string]$AuthUri = '') {
    $request = @{ UseBasicParsing = $true; Uri = $Uri }
    if ($OutFile) { $request.OutFile = $OutFile }
    try {
        return Invoke-Download $request
    } catch {
        $status = Get-Status $_
        if ($status -notin 401, 403, 404) { throw }
        $tok = Get-Token
        if (-not $tok) { Fail "O GitHub negou o acesso a $Uri (código $status)." }
        if ($AuthUri) { $request.Uri = $AuthUri }
        $request.Headers = @{ Authorization = "Bearer $tok"; Accept = $Accept; 'X-GitHub-Api-Version' = '2022-11-28' }
        try { return Invoke-Download $request }
        catch { Fail "Não foi possível baixar $($request.Uri) nem com o token do GitHub (código $(Get-Status $_))" }
    }
}

function Get-Text($Response) {
    # Assets vêm como application/octet-stream: Content é byte[], não texto.
    $raw = $Response.Content
    if ($raw -is [byte[]]) { [Text.Encoding]::UTF8.GetString($raw) } else { [string]$raw }
}

function Get-Release {
    $uri = if ($Version) { "https://api.github.com/repos/$repo/releases/tags/$Version" } else { "https://api.github.com/repos/$repo/releases/latest" }
    $rel = (Get-Text (Invoke-GitHub $uri)) | ConvertFrom-Json
    $exe = @($rel.assets | Where-Object { $_.name -match '^eprocia-.+-windows-amd64\.exe$' })
    $sums = @($rel.assets | Where-Object { $_.name -eq 'SHA256SUMS.txt' })
    if ($exe.Count -ne 1) { Fail "A release $($rel.tag_name) não tem exatamente um eprocia-*-windows-amd64.exe" }
    if ($sums.Count -ne 1) { Fail "A release $($rel.tag_name) não tem SHA256SUMS.txt" }
    # O instalador (NSIS) so e obrigatorio para a instalacao para todos (Get-Target).
    $inst = @($rel.assets | Where-Object { $_.name -match '^eprocia-.+-windows-amd64-instalador\.exe$' })
    $installer = if ($inst.Count -eq 1) { $inst[0] } else { $null }
    [pscustomobject]@{ Tag = $rel.tag_name; Exe = $exe[0]; Sums = $sums[0]; Installer = $installer }
}

function Get-Sha256([string]$Path) {
    # .NET em vez de Get-FileHash: o 5.1 aberto a partir do 7 herda um PSModulePath em que
    # o Get-FileHash não carrega.
    $sha = [Security.Cryptography.SHA256]::Create()
    $stream = [IO.File]::OpenRead($Path)
    try { ([BitConverter]::ToString($sha.ComputeHash($stream)) -replace '-', '').ToUpperInvariant() }
    finally { $stream.Dispose(); $sha.Dispose() }
}

# Baixa o exe da release ($Rel, de Get-Release) para $Target e confere o SHA-256. Devolve a tag.
function Save-ReleaseExe([string]$Target, $Rel = $null) {
    $rel = if ($Rel) { $Rel } else { Get-Release }
    Save-ReleaseAsset $Target $rel $rel.Exe
}

# Baixa um asset da release ($Asset, de $Rel) para $Target e confere com o SHA256SUMS.txt.
function Save-ReleaseAsset([string]$Target, $Rel, $Asset) {
    Write-Step 'Baixando' "$($Asset.name) ($($Rel.Tag))"
    Invoke-GitHub $Asset.browser_download_url 'application/octet-stream' $Target $Asset.url | Out-Null
    $sumText = Get-Text (Invoke-GitHub $Rel.Sums.browser_download_url 'application/octet-stream' '' $Rel.Sums.url)
    $pattern = "^\s*([0-9a-fA-F]{64})\s+\*?$([regex]::Escape($Asset.name))\s*$"
    $line = @($sumText -split "`r?`n" | Where-Object { $_ -match $pattern })
    if ($line.Count -eq 0 -or $line[0] -notmatch $pattern) { Fail "o SHA256SUMS.txt nao lista $($Asset.name)" }
    $expected = $Matches[1].ToUpperInvariant()
    $actual = Get-Sha256 $Target
    if ($actual -ne $expected) { Fail "SHA-256 nao confere (esperado $expected, baixado $actual)" }
    Write-Ok 'SHA-256 conferido'
    $Rel.Tag
}

# Baixa o instalador (NSIS) da release para $Target e confere o SHA-256. Devolve a tag.
function Save-ReleaseInstaller([string]$Target, $Rel) {
    if (-not $Rel.Installer) { Fail "A release $($Rel.Tag) nao tem o instalador" }
    Save-ReleaseAsset $Target $Rel $Rel.Installer
}

# Texto deste script: o do bloco em execucao, se tiver o marcador de protocolo; senao o do
# arquivo; so em ultimo caso o da main (sob `irm | iex`, que e a mesma origem). Para o filho
# elevado (-ForElevation), o texto precisa do marcador: um script antigo nao conhece -Machine.
function Get-ScriptText([switch]$ForElevation) {
    $marker = '# eprocia-install: elevacao-v1'
    if ($selfText -and $selfText.Contains($marker)) { $text = $selfText }
    elseif ($selfPath) { $text = [IO.File]::ReadAllText($selfPath, [Text.Encoding]::UTF8) }
    else { $text = Get-Text (Invoke-GitHub $scriptUrl 'application/vnd.github.raw' '' "https://api.github.com/repos/$repo/contents/install.ps1") }
    if ($ForElevation -and -not $text.Contains($marker)) {
        Fail 'O install.ps1 obtido nao suporta a acao como administrador; baixe a versao atual e rode de novo.'
    }
    $text
}

# Com BOM: a cópia local roda com -File no 5.1 (desinstalação, tarefa agendada).
function Save-ScriptCopy([string]$Path, [string]$Text = '') {
    if (-not $Text) { $Text = Get-ScriptText }
    [IO.File]::WriteAllText($Path, $Text.TrimStart([char]0xFEFF), (New-Object Text.UTF8Encoding $true))
}

# --- Estado da máquina --------------------------------------------------------------------

# Os eprocia.exe abertos com o caminho $Exe. Sem elevacao, o ExecutablePath de um processo
# de outra conta vem vazio e ele fica de fora (nao roda da pasta deste usuario; no alvo para
# todos, quem decide sobre ele e o filho elevado, que le todos os caminhos).
function Get-AppProcesses([string]$Exe) {
    @(Get-CimInstance Win32_Process -Filter "Name = 'eprocia.exe'" | ForEach-Object {
        [pscustomobject]@{ ProcessId = [int]$_.ProcessId; SessionId = [int]$_.SessionId; ExecutablePath = [string]$_.ExecutablePath }
    } | Where-Object { $_.ExecutablePath -and $_.ExecutablePath -eq $Exe })
}

# Filhos de OCR (Tesseract e Poppler) do processo: ha um processamento em curso.
function Get-OcrChildren([int]$ProcessId) {
    @(Get-CimInstance Win32_Process -Filter "ParentProcessId = $ProcessId" | Where-Object {
        $_.Name -in 'tesseract.exe', 'pdftoppm.exe', 'pdfinfo.exe', 'pdftotext.exe'
    })
}

# Pergunta se pode encerrar o eprocIA aberto do alvo; nunca encerra (quem encerra e o
# Stop-App, na hora da troca). Sim: guarda os PIDs aceitos. A copia interna (-Log) nao
# pergunta nem acrescenta PIDs: a lista aceita dela vem de -ClosePids.
function Confirm-Close([string]$Exe) {
    if ($Log) { return $true }
    $procs = @(Get-AppProcesses $Exe)
    if ($procs.Count -eq 0) { return $true }
    $ocr = $false
    foreach ($p in $procs) {
        Write-Step 'eprocIA aberto (PID)' ([string]$p.ProcessId)
        if (@(Get-OcrChildren $p.ProcessId).Count -gt 0) { $ocr = $true }
    }
    if ($ocr) { Write-Warn 'Ha um processamento em curso (OCR); encerrar agora interrompe o processamento' }
    else { Write-Note 'Um processamento em curso seria interrompido' }
    $ids = @($procs | ForEach-Object { [int]$_.ProcessId })
    # O "Sim" da simulacao so vale para os PIDs que ela mostrou; um eprocIA aberto depois
    # disso faz perguntar de novo (sem console, para sem mudar nada).
    $unseen = @($ids | Where-Object { $script:dryRunPids -notcontains $_ })
    $fresh = $script:confirmed -and $unseen.Count -gt 0
    if (-not (Confirm-Plan 'Encerrar o eprocIA aberto?' 'Encerrar' 'Fecha o eprocIA na hora de trocar' -Fresh:$fresh)) { return $false }
    if ($DryRun -and -not $script:confirmed) { $script:dryRunPids = $ids }
    $script:closePids = $ids
    $true
}

# PIDs de -ClosePids ('-' ou vazio = nenhum).
function Get-ClosePidList {
    if (-not $ClosePids -or $ClosePids -eq '-') { return @() }
    if ($ClosePids -notmatch '^\d+(,\d+)*$') { Fail "-ClosePids invalido: $ClosePids" }
    @($ClosePids -split ',' | ForEach-Object { [int]$_ })
}

# Valor de -ClosePids para a copia interna: os PIDs aceitos, ou '-' se nenhum. Vai sempre na
# forma -ClosePids:<valor>: com -File, o powershell.exe 5.1 recusa um '-' solto como valor
# (sai com 5 sem rodar o script).
function Get-ClosePidText {
    $ids = @($script:closePids)
    if ($ids.Count -gt 0) { $ids -join ',' } else { '-' }
}

# Logo antes da troca: confere de novo e encerra so o que o usuario aceitou. Um eprocIA do
# alvo em outra sessao, ou aberto depois da pergunta (fora da lista), faz parar antes de
# encerrar qualquer coisa. Encerrar sem lista so vale com o -Yes do proprio usuario ou com
# -CloseAny; o -Yes interno (com -Log) nunca autoriza.
function Stop-App([string]$Exe) {
    $session = (Get-Process -Id $PID).SessionId
    $procs = @(Get-AppProcesses $Exe)
    if (@($procs | Where-Object { $_.SessionId -ne $session }).Count -gt 0) {
        Fail 'O eprocIA esta aberto em outra sessao; feche-o la e rode de novo. Nada foi alterado.'
    }
    $accepted = @($script:closePids) + @(Get-ClosePidList)
    $closeAny = $CloseAny -or ($Yes -and -not $Log)
    $extra = @($procs | Where-Object { $accepted -notcontains $_.ProcessId })
    if ($extra.Count -gt 0 -and -not $closeAny) {
        Fail 'O eprocIA foi aberto depois da pergunta; feche-o e rode de novo. Nada foi alterado.'
    }
    if ($procs.Count -eq 0) { return }
    $ids = @()
    foreach ($p in $procs) {
        $ids += $p.ProcessId
        $ids += @(Get-OcrChildren $p.ProcessId | ForEach-Object { [int]$_.ProcessId })
    }
    Write-Step 'Encerrando o eprocIA (PID)' (@($procs | ForEach-Object { $_.ProcessId }) -join ', ')
    Stop-Process -Id $ids -Force -ErrorAction SilentlyContinue
    try { Wait-Process -Id $ids -Timeout 15 -ErrorAction SilentlyContinue } catch { }
    if (@(Get-Process -Id $ids -ErrorAction SilentlyContinue).Count -gt 0) { Fail 'O eprocIA nao fechou em 15 s; nada foi trocado.' }
}

# Troca o exe: o Windows segura a imagem de um processo recem-encerrado por um instante,
# entao repete ate 10 vezes, 200 ms entre elas, so em erro de arquivo em uso ou de acesso.
function Move-ExeWithRetry([string]$Source, [string]$Destination) {
    for ($try = 1; ; $try++) {
        try {
            Move-Item -LiteralPath $Source -Destination $Destination -Force
            return
        } catch [System.IO.IOException], [System.UnauthorizedAccessException] {
            if ($try -ge 10) { throw }
            Start-Sleep -Milliseconds 200
        }
    }
}

function Test-WebView2 {
    $id = '{F3017226-FE2A-4295-8BDF-00C3A9A7E4C5}'
    foreach ($key in @("HKLM:\SOFTWARE\WOW6432Node\Microsoft\EdgeUpdate\Clients\$id",
            "HKLM:\SOFTWARE\Microsoft\EdgeUpdate\Clients\$id",
            "HKCU:\Software\Microsoft\EdgeUpdate\Clients\$id")) {
        $pv = (Get-ItemProperty -LiteralPath $key -ErrorAction SilentlyContinue).pv
        if ($pv -and $pv -ne '0.0.0.0') { return $true }
    }
    $false
}

# A instalação do NSIS (Arquivos de Programas) deixaria duas cópias e dois atalhos.
function Get-MachineInstall {
    foreach ($root in @('HKLM:\SOFTWARE\Microsoft\Windows\CurrentVersion\Uninstall',
            'HKLM:\SOFTWARE\WOW6432Node\Microsoft\Windows\CurrentVersion\Uninstall')) {
        Get-ChildItem -LiteralPath $root -ErrorAction SilentlyContinue | ForEach-Object {
            $p = Get-ItemProperty -LiteralPath $_.PSPath -ErrorAction SilentlyContinue
            if ($p -and ($p.PSObject.Properties.Name -contains 'DisplayName') -and $p.DisplayName -like '*eprocia*') { $p }
        }
    }
}

# O bloco de strings do Wails usa o idioma 0000, que o FileVersionInfo não lê; a parte fixa
# traz a versão.
function Get-ExeVersion([string]$Exe) {
    $v = (Get-Item -LiteralPath $Exe).VersionInfo
    '{0}.{1}.{2}' -f $v.FileMajorPart, $v.FileMinorPart, $v.FileBuildPart
}

function Assert-From {
    if ($From -and -not (Test-Path -LiteralPath $From -PathType Leaf)) { Fail "Arquivo não encontrado: $From" }
}

# Versão que será instalada: a do -From ou a da tag da release ($Rel), sem o "v".
function Get-NewVersion($Rel) {
    if ($From) {
        try { return Get-ExeVersion (Resolve-Path -LiteralPath $From).Path } catch { return 'desconhecida' }
    }
    ([string]$Rel.Tag) -replace '^v', ''
}

function Get-CurrentVersion([string]$InstallDir) {
    $exe = Join-Path $InstallDir $exeName
    if (-not (Test-Path -LiteralPath $exe)) { return '' }
    try { Get-ExeVersion $exe } catch { 'desconhecida' }
}

# X.Y.Z como [version] (sem "v" nem sufixo de pré-release), ou $null se não der para ler.
function ConvertTo-Version([string]$Text) {
    $m = [regex]::Match($Text, '^v?(\d+\.\d+\.\d+)')
    if ($m.Success) { [version]$m.Groups[1].Value } else { $null }
}

# Pergunta, ação e detalhe da confirmação de instalação, conforme a versão instalada.
function Get-InstallPrompt([string]$NewVersion, [string]$InstallDir) {
    $current = Get-CurrentVersion $InstallDir
    $label = if ($NewVersion) { "a $NewVersion" } else { 'a release mais recente' }
    if (-not $NewVersion) { $NewVersion = 'da release mais recente' }
    $a = ConvertTo-Version $current
    $b = ConvertTo-Version $NewVersion
    if (-not $current) { $q = "Instalar o eprocIA ${NewVersion}?"; $act = 'Instalar'; $det = "Instala $label" }
    elseif ($current -eq $NewVersion) { $q = "Reinstalar o eprocIA ${current}?"; $act = 'Reinstalar'; $det = "Instala de novo $label" }
    elseif ($a -and $b -and $b -lt $a) { $q = "Voltar o eprocIA de $current para ${NewVersion}?"; $act = 'Voltar'; $det = "Troca a $current pela $NewVersion" }
    else { $q = "Atualizar o eprocIA de $current para ${NewVersion}?"; $act = 'Atualizar'; $det = "Troca a $current pela $NewVersion" }
    [pscustomobject]@{ Question = $q; Action = $act; Detail = $det }
}

function Get-UninstallPrompt {
    if ($Purge) { [pscustomobject]@{ Question = 'Remover o eprocIA e as configurações?'; Action = 'Remover'; Detail = 'Apaga o app e as configurações' } }
    else { [pscustomobject]@{ Question = 'Remover o eprocIA?'; Action = 'Remover'; Detail = 'Apaga o app; configurações ficam' } }
}

# Mostra a versão (a atual também, numa atualização) e a pasta, e pergunta.
function Confirm-Install([string]$NewVersion, [string]$InstallDir) {
    $p = Get-InstallPrompt $NewVersion $InstallDir
    if ($Log) { return (Confirm-Plan $p.Question $p.Action $p.Detail) }
    $current = Get-CurrentVersion $InstallDir
    if ($current) {
        Write-Step 'Versão instalada' $current
        Write-Step 'Nova versão' $NewVersion
    } else {
        Write-Step 'Versão' $NewVersion
    }
    Write-Step 'Pasta' $InstallDir
    Confirm-Plan $p.Question $p.Action $p.Detail
}

# Mostra o que será apagado e pergunta.
function Confirm-Uninstall([string]$InstallDir) {
    $p = Get-UninstallPrompt
    if ($Log) { return (Confirm-Plan $p.Question $p.Action $p.Detail) }
    if (Test-Path -LiteralPath $InstallDir) { Write-Step 'Apaga a pasta' $InstallDir } else { Write-Step 'Não há eprocIA em' $InstallDir }
    Write-Step 'Apaga os atalhos, a entrada em Aplicativos instalados e o "Iniciar com o Windows"'
    if ($Purge) {
        foreach ($d in Get-PurgeDirs) { Write-Step 'Apaga também' $d }
    } else {
        Write-Note "Configurações em $(Join-Path $env:APPDATA $app) ficam (-Purge apaga)"
    }
    Confirm-Plan $p.Question $p.Action $p.Detail
}

function New-Shortcut([string]$Path, [string]$Target) {
    $lnk = (New-Object -ComObject WScript.Shell).CreateShortcut($Path)
    $lnk.TargetPath = $Target
    $lnk.WorkingDirectory = Split-Path $Target
    $lnk.IconLocation = "$Target,0"
    $lnk.Description = 'Transforma os ZIPs do Eproc num texto para IA'
    $lnk.Save()
}

# --- Instalar / remover -------------------------------------------------------------------

function Install-App {
    $installDir = Get-InstallDir
    $exe = Join-Path $installDir $exeName
    Enable-Tls12
    $label = if ($From) { 'arquivo local' } elseif ($Version) { $Version } else { 'última release' }
    Write-Title "Instalando o eprocIA ($label, só para você)"
    $rel = if ($From) { $null } else { Get-Release }
    if (-not (Confirm-Install (Get-NewVersion $rel) $installDir)) { Write-Cancelled; return }
    # So pergunta: o app aberto e encerrado pelo Stop-App, depois do download, na troca.
    if (-not (Confirm-Close $exe)) { Write-Cancelled; return }

    New-Item -ItemType Directory -Force -Path $installDir | Out-Null
    # Tudo é baixado antes de trocar qualquer arquivo: uma falha no meio não muda nada.
    $suffix = '.download-' + [Guid]::NewGuid().ToString('N')
    $tmp = Join-Path $installDir "$exeName$suffix"
    $localScript = Join-Path $installDir 'install.ps1'
    $copyScript = -not ($selfPath -and ((Resolve-Path -LiteralPath $selfPath).Path -eq $localScript))
    $tmpScript = "$localScript$suffix"
    try {
        if ($From) {
            $source = (Resolve-Path -LiteralPath $From).Path
            Write-Step 'Copiando' $source
            Copy-Item -LiteralPath $source -Destination $tmp
        } else {
            Save-ReleaseExe $tmp $rel | Out-Null
        }
        # A cópia local do script atende a desinstalação por Aplicativos instalados.
        if ($copyScript) { Save-ScriptCopy $tmpScript }
        Stop-App $exe
        Move-ExeWithRetry $tmp $exe
        if ($copyScript) { Move-Item -LiteralPath $tmpScript -Destination $localScript -Force }
    } finally {
        foreach ($f in $tmp, $tmpScript) { if (Test-Path -LiteralPath $f) { Remove-Item -LiteralPath $f -Force } }
    }
    # Sobras da atualizacao pelo proprio app: o exe antigo e o que nao abriu.
    Remove-Item -LiteralPath (Join-Path $installDir 'eprocia.anterior.exe') -Force -ErrorAction SilentlyContinue
    Remove-Item -LiteralPath (Join-Path $installDir 'eprocia.falhou.exe') -Force -ErrorAction SilentlyContinue
    # Sem a marca de download, o SmartScreen não barra cada abertura.
    Unblock-File -LiteralPath $exe
    $version = Get-ExeVersion $exe
    Write-Step 'Salvo em' $exe

    $shortcuts = Get-Shortcuts
    New-Shortcut $shortcuts[0] $exe
    Write-Step 'Atalho no menu Iniciar' $shortcuts[0]
    if ($NoDesktop) {
        Remove-Item -LiteralPath $shortcuts[1] -Force -ErrorAction SilentlyContinue
    } else {
        New-Shortcut $shortcuts[1] $exe
        Write-Step 'Atalho na área de trabalho' $shortcuts[1]
    }

    # Caminho completo do powershell.exe, como nos outros lugares (sem busca na pasta atual).
    $uninstall = "`"$psExe`" -NoProfile -ExecutionPolicy Bypass -File `"$localScript`" -Uninstall"
    New-Item -Path $uninstallKey -Force | Out-Null
    $values = [ordered]@{
        DisplayName          = $app
        DisplayVersion       = $version
        Publisher            = $app
        DisplayIcon          = "$exe,0"
        InstallLocation      = $installDir
        UninstallString      = $uninstall
        QuietUninstallString = "$uninstall -Yes"
        HelpLink             = "https://github.com/$repo"
    }
    foreach ($k in $values.Keys) { Set-ItemProperty -LiteralPath $uninstallKey -Name $k -Value $values[$k] }
    foreach ($k in 'NoModify', 'NoRepair') { Set-ItemProperty -LiteralPath $uninstallKey -Name $k -Value 1 -Type DWord }
    $kb = [int][Math]::Ceiling((Get-Item -LiteralPath $exe).Length / 1KB)
    Set-ItemProperty -LiteralPath $uninstallKey -Name EstimatedSize -Value $kb -Type DWord
    Write-Step 'Registrado em Aplicativos instalados'

    # Quem usava a versão portátil com "Iniciar com o Windows" passa a abrir a instalada.
    if (Get-ItemProperty -LiteralPath $runKey -Name $app -ErrorAction SilentlyContinue) {
        Set-ItemProperty -LiteralPath $runKey -Name $app -Value "`"$exe`" $flagTray"
        Write-Step '"Iniciar com o Windows" agora abre' $exe
    }

    if (-not (Test-WebView2)) {
        Write-Warn 'Microsoft Edge WebView2 Runtime não encontrado. Instale em https://go.microsoft.com/fwlink/p/?LinkId=2124703 ou com o comando "winget install -e --id Microsoft.EdgeWebView2Runtime"'
    }
    foreach ($other in @(Get-MachineInstall)) {
        Write-Warn "Há também uma instalação para todos os usuários ($($other.DisplayName)).`nDesinstale em Configurações > Aplicativos para não ficar com duas cópias"
    }

    Write-Done "Pronto: eprocIA $version instalado só para você."
    if (-not $NoStart) {
        Start-Process -FilePath $exe -WorkingDirectory $installDir
        Write-Note 'eprocIA aberto. Ele fica no ícone da bandeja ao fechar a janela.'
    }
}

function Uninstall-App {
    $installDir = Get-InstallDir
    $exe = Join-Path $installDir $exeName
    Write-Title 'Removendo o eprocIA (só para você)'
    if (-not (Confirm-Uninstall $installDir)) { Write-Cancelled; return }
    if (-not (Confirm-Close $exe)) { Write-Cancelled; return }
    Stop-App $exe
    # A pasta pode ser a atual (desinstalação por Aplicativos instalados).
    Set-Location -LiteralPath ([IO.Path]::GetTempPath())

    $run = Get-ItemProperty -LiteralPath $runKey -Name $app -ErrorAction SilentlyContinue
    if ($run -and $run.$app -like "*$exe*") {
        Remove-ItemProperty -LiteralPath $runKey -Name $app
        Write-Step '"Iniciar com o Windows" desligado'
    }
    foreach ($s in Get-Shortcuts) {
        if (Test-Path -LiteralPath $s) { Remove-Item -LiteralPath $s -Force; Write-Step 'Atalho removido' $s }
    }
    Remove-Item -LiteralPath $uninstallKey -Recurse -Force -ErrorAction SilentlyContinue
    if (Test-Path -LiteralPath $installDir) { Remove-Item -LiteralPath $installDir -Recurse -Force }
    Write-Ok "Removido: $installDir"

    $config = Join-Path $env:APPDATA $app
    if ($Purge) {
        foreach ($d in Get-PurgeDirs) {
            if (Test-Path -LiteralPath $d) { Remove-Item -LiteralPath $d -Recurse -Force; Write-Step 'Apagado' $d }
        }
    } else {
        Write-Note "Configurações mantidas em $config (-Purge apaga)"
    }
    Write-Done 'Pronto: eprocIA removido'
}

# Num terminal elevado, roda este script de novo sem privilégios como o usuário da sessão,
# por uma tarefa agendada de uso único, e reapresenta o log dele.
function Invoke-AsUser {
    $user = Get-SessionUser
    $hiddenDetail = if ($Uninstall) { "Remove so para $user, sem privilegios" } else { "Instala so para $user, sem privilegios" }
    if (-not (Confirm-Plan "Rodar como $user numa janela oculta?" 'Continuar' $hiddenDetail)) { Write-Cancelled; return }
    Write-Note "Terminal elevado: a instalação continua só para $user, sem privilégios."
    # A pergunta é feita aqui: a cópia sem privilégios roda sem console e recebe -Yes.
    $userDir = Get-UserInstallDir $user
    $rel = $null
    if ($Uninstall) {
        if (-not (Confirm-Uninstall $userDir)) { Write-Cancelled; return }
    } else {
        Enable-Tls12
        if (-not $From) { $rel = Get-Release }
        if (-not (Confirm-Install (Get-NewVersion $rel) $userDir)) { Write-Cancelled; return }
    }
    if (-not (Confirm-Close (Join-Path $userDir $exeName))) { Write-Cancelled; return }
    $id = [Guid]::NewGuid().ToString('N')
    $stage = Join-Path $env:PUBLIC "eprocia-install-$id"
    New-Item -ItemType Directory -Path $stage | Out-Null
    try {
        # O usuário pode não ter acesso ao perfil do administrador: tudo que a tarefa lê ou
        # escreve fica numa pasta pública com permissão para ele.
        & icacls.exe $stage /grant "${user}:(OI)(CI)M" | Out-Null
        if ($LASTEXITCODE -ne 0) { Fail "icacls falhou ao dar acesso a $user" }
        $stagedScript = Join-Path $stage 'install.ps1'
        Save-ScriptCopy $stagedScript
        $stagedLog = Join-Path $stage 'install.log'

        $argv = @('-NoProfile', '-ExecutionPolicy', 'Bypass', '-WindowStyle', 'Hidden', '-File', "`"$stagedScript`"", '-Log', "`"$stagedLog`"", '-Yes')
        # O -Yes interno nao autoriza encerrar: so os PIDs aceitos, ou qualquer um com o -Yes
        # do usuario (-CloseAny).
        $argv += "-ClosePids:$(Get-ClosePidText)"
        if ($Yes -and -not $Log) { $argv += '-CloseAny' }
        if ($Dir) { $argv += @('-Dir', "`"$Dir`"") }
        if ($Uninstall) {
            $argv += '-Uninstall'
            if ($Purge) { $argv += '-Purge' }
        } else {
            # O exe é obtido aqui: o token do gh e o -From podem só existir para o administrador.
            $stagedExe = Join-Path $stage $exeName
            if ($From) { Copy-Item -LiteralPath (Resolve-Path -LiteralPath $From).Path -Destination $stagedExe }
            else { Save-ReleaseExe $stagedExe $rel | Out-Null }
            $argv += @('-From', "`"$stagedExe`"")
            if ($NoDesktop) { $argv += '-NoDesktop' }
            if ($NoStart) { $argv += '-NoStart' }
        }

        $task = "eprocIA-install-$id"
        $action = New-ScheduledTaskAction -Execute $psExe -Argument ($argv -join ' ')
        $principal = New-ScheduledTaskPrincipal -UserId $user -LogonType Interactive -RunLevel Limited
        $settings = New-ScheduledTaskSettingsSet -AllowStartIfOnBatteries -DontStopIfGoingOnBatteries -ExecutionTimeLimit (New-TimeSpan -Minutes 10)
        Register-ScheduledTask -TaskName $task -Action $action -Principal $principal -Settings $settings | Out-Null
        try {
            Start-ScheduledTask -TaskName $task
            $deadline = (Get-Date).AddMinutes(10)
            do {
                Start-Sleep -Milliseconds 500
                $info = Get-ScheduledTaskInfo -TaskName $task
                # 267009 = em execução; 267011 = ainda não rodou.
                $finished = $info.LastTaskResult -notin 267009, 267011
            } until ($finished -or (Get-Date) -gt $deadline)
        } finally {
            Unregister-ScheduledTask -TaskName $task -Confirm:$false -ErrorAction SilentlyContinue
        }
        Show-Log $stagedLog
        if (-not $finished) { Fail 'A cópia sem privilégios não terminou em 10 minutos.' }
        if ($info.LastTaskResult -ne 0) { Fail "A cópia sem privilégios terminou com erro (código $($info.LastTaskResult))" }
    } finally {
        Remove-Item -LiteralPath $stage -Recurse -Force -ErrorAction SilentlyContinue
    }
}

# -DryRun: as mesmas decisões, mostradas em vez de feitas. Devolve $true se o usuário pedir
# a execução real em seguida.
function Invoke-DryRun([bool]$Elevated, $Target) {
    $action = if ($Uninstall) { 'remover' } else { 'instalar' }
    if ($Target.Kind -eq 'machine') {
        Write-Title "Simulacao: $action o eprocIA (para todos os usuarios)"
        Write-Step 'Instalacao para todos os usuarios em' $Target.Dir
        if ($Elevated) { Write-Step 'Terminal elevado: faria aqui mesmo' }
        else { Write-Step 'Pediria permissao de administrador (UAC)' }
        if ($Uninstall) {
            Write-Step 'Rodaria o desinstalador de' $Target.Dir
            if ($Purge) { foreach ($d in Get-PurgeDirs) { Write-Step 'Apagaria tambem' $d } }
        } else {
            if ($From) { Write-Step 'Copiaria' $From }
            else {
                $label = if ($Version) { "release $Version" } else { 'release mais recente' }
                Write-Step 'Baixaria o instalador' "eprocia-*-windows-amd64-instalador.exe ($label de $repo)"
                Write-Step 'Conferiria o SHA-256 do instalador com o SHA256SUMS.txt da release'
            }
            Write-Step 'Rodaria o instalador em silencio sobre' $Target.Dir
            if ($NoDesktop) { Write-Note 'Na instalacao para todos, o atalho da area de trabalho fica como estava (-NoDesktop nao vale)' }
            if (-not $NoStart) { Write-Step 'Abriria o eprocIA' }
        }
        return (Complete-DryRun $Target.Dir $Target.Exe)
    }
    Write-Title "Simulação: $action o eprocIA (só para você)"
    $installDir = Get-InstallDir
    if ($Elevated) {
        $user = Get-SessionUser
        $hiddenDetail = if ($Uninstall) { "Remove so para $user, sem privilegios" } else { "Instala so para $user, sem privilegios" }
        Confirm-Plan "Rodar como $user numa janela oculta?" 'Continuar' $hiddenDetail | Out-Null
        Write-Step 'Terminal elevado: rodaria sem privilégios, por tarefa agendada, como' $user
        Write-Note 'Os caminhos abaixo seriam os do perfil desse usuário'
        $installDir = Get-UserInstallDir $user
    }
    $exe = Join-Path $installDir $exeName
    $shortcuts = Get-Shortcuts
    if ($Uninstall) {
        if (Test-Path -LiteralPath $installDir) { Write-Step 'Apagaria' $installDir } else { Write-Step 'Não há eprocIA em' $installDir }
        Write-Step 'Apagaria os atalhos e a entrada de Aplicativos instalados'
        if ($Purge) { Write-Step 'Apagaria as configurações em' (Join-Path $env:APPDATA $app) }
        else { Write-Note "Configurações em $(Join-Path $env:APPDATA $app) ficariam" }
    } else {
        if ($From) { Write-Step 'Copiaria' $From }
        else {
            $label = if ($Version) { "release $Version" } else { 'release mais recente' }
            Write-Step 'Baixaria' "eprocia-*-windows-amd64.exe ($label de $repo)"
            Write-Step 'Conferiria o SHA-256 com o SHA256SUMS.txt da release'
        }
        if (Test-Path -LiteralPath $exe) { Write-Step "Substituiria a $(Get-ExeVersion $exe) em" $exe } else { Write-Step 'Salvaria em' $exe }
        Write-Step 'Criaria o atalho' $shortcuts[0]
        if (-not $NoDesktop) { Write-Step 'Criaria o atalho' $shortcuts[1] }
        Write-Step 'Registraria em Aplicativos instalados'
        if (-not $NoStart) { Write-Step 'Abriria o eprocIA' }
    }
    return (Complete-DryRun $installDir $exe)
}

# Fim da simulacao, comum aos dois alvos: as perguntas que faria (a acao e o app aberto) e a
# escolha de fazer de verdade. Devolve $true se o usuario pedir a execucao real.
function Complete-DryRun([string]$InstallDir, [string]$Exe) {
    # A versão nova vem da API do GitHub; sem rede, a pergunta sai sem ela.
    if ($Uninstall) { $p = Get-UninstallPrompt }
    else {
        $newVersion = ''
        try { Enable-Tls12; $newVersion = Get-NewVersion $(if ($From) { $null } else { Get-Release }) } catch { }
        $p = Get-InstallPrompt $newVersion $installDir
    }
    Confirm-Plan $p.Question $p.Action $p.Detail | Out-Null
    # Com o app aberto, mostra os PIDs, a nota de OCR e "Perguntaria: Encerrar ...".
    Confirm-Close $Exe | Out-Null
    Write-Host ''
    Write-Note 'Simulação: nada foi alterado.'
    $verb = if ($Uninstall) { 'remoção' } else { 'instalação' }
    # Padrão "Não": sem console para perguntar, uma simulação nunca vira execução real.
    $go = Read-Choice "Iniciar a $verb real?" ([ordered]@{
        'Sim' = "Faz a $verb agora"
        'Não' = 'Sai sem alterar nada'
    }) -Default 1
    $go -eq 0
}

# Instalacao para todos (NSIS) com DisplayIcon legivel: pasta e exe vem dele, sem aspas e
# sem o ",N" do fim. $null se nao houver.
function Get-MachineTarget {
    foreach ($m in @(Get-MachineInstall)) {
        if (-not ($m.PSObject.Properties.Name -contains 'DisplayIcon') -or -not $m.DisplayIcon) { continue }
        $icon = ([string]$m.DisplayIcon).Trim() -replace ',\s*-?\d+$', ''
        $icon = $icon.Trim().Trim([char]34)
        if (-not $icon -or -not (Test-Path -LiteralPath $icon -PathType Leaf)) { continue }
        return [pscustomobject]@{ Kind = 'machine'; Dir = (Split-Path -Parent $icon); Exe = $icon; Key = $m.PSPath }
    }
    $null
}

# Sobre o que o script age: Kind 'user' (pasta por usuario) ou 'machine' (instalacao para
# todos). Com -Machine (filho elevado), sempre a para todos; -Dir e sempre por usuario; a por
# usuario que ja existe vem antes; senao a para todos; nenhuma, instalacao nova por usuario.
# O desinstalador da para todos e sempre <Dir>\uninstall.exe.
function Get-Target {
    if ($Machine) {
        $m = Get-MachineTarget
        if (-not $m) { Fail 'Nao ha instalacao para todos os usuarios' }
        return $m
    }
    $userDir = Get-InstallDir
    if (-not $Dir -and (Test-Elevated)) {
        $userDir = ''
        try { $userDir = Get-UserInstallDir (Get-SessionUser) } catch { }
    }
    $user = [pscustomobject]@{ Kind = 'user'; Dir = $userDir; Exe = $(if ($userDir) { Join-Path $userDir $exeName } else { '' }); Key = '' }
    if ($Dir) { return $user }
    if ($user.Exe -and (Test-Path -LiteralPath $user.Exe -PathType Leaf)) { return $user }
    $m = Get-MachineTarget
    if ($m) { return $m }
    $user
}

# --- Instalacao para todos (NSIS): elevacao sob demanda ---------------------------------

# Dono da pasta, como SID (o nome de Administradores muda com o idioma do Windows).
function Get-OwnerSid([string]$Path) {
    (Get-DirAcl $Path).GetOwner([Security.Principal.SecurityIdentifier]).Value
}

# Dono e DACL da pasta (.NET em vez de Get-Acl, como em Get-Sha256: o 5.1 aberto a partir do
# 7 pode nao carregar o modulo).
function Get-DirAcl([string]$Path) {
    $sections = [Security.AccessControl.AccessControlSections]'Owner, Access'
    if ($PSVersionTable.PSVersion.Major -ge 6) {
        [IO.FileSystemAclExtensions]::GetAccessControl((New-Object IO.DirectoryInfo $Path), $sections)
    } else {
        [IO.Directory]::GetAccessControl($Path, $sections)
    }
}

# Cria (num processo ja elevado) uma pasta so de Administradores e SYSTEM, sem herdar do pai,
# mais leitura para $ReaderSid quando houver. A ACL vai na criacao; depois confere que a pasta
# e nova, vazia e de Administradores ou SYSTEM (C:\ProgramData deixa qualquer usuario criar
# pastas: uma pasta plantada antes nao passa).
function New-ProtectedDir([string]$Path, [string]$ReaderSid) {
    $dirty = 'A pasta temporaria do administrador nao estava limpa. Nada foi alterado.'
    if (Test-Path -LiteralPath $Path) { Fail $dirty }
    $sec = New-Object Security.AccessControl.DirectorySecurity
    $sec.SetAccessRuleProtection($true, $false)
    $rules = @(@('S-1-5-32-544', 'FullControl'), @('S-1-5-18', 'FullControl'))
    if ($ReaderSid) { $rules += , @($ReaderSid, 'ReadAndExecute') }
    foreach ($r in $rules) {
        $sid = New-Object Security.Principal.SecurityIdentifier $r[0]
        $rights = [Security.AccessControl.FileSystemRights]$r[1]
        $sec.AddAccessRule((New-Object Security.AccessControl.FileSystemAccessRule($sid, $rights, 'ContainerInherit,ObjectInherit', 'None', 'Allow')))
    }
    if ($PSVersionTable.PSVersion.Major -ge 6) { $null = [IO.FileSystemAclExtensions]::CreateDirectory($sec, $Path) }
    else { $null = [IO.Directory]::CreateDirectory($Path, $sec) }
    # CreateDirectory nao falha com uma pasta (ou juncao) ja existente: confere tudo depois.
    if (-not (Test-AdminDir $Path $ReaderSid)) { Fail $dirty }
    if (@(Get-ChildItem -Force -LiteralPath $Path).Count -ne 0) { Fail $dirty }
}

# A pasta e de administrador de verdade: nao e link (juncao), o dono e Administradores ou
# SYSTEM (por SID) e a DACL e protegida, so com regras de Administradores, SYSTEM e, se
# houver, $ReaderSid. Usada antes de confiar numa pasta de %ProgramData%.
function Test-AdminDir([string]$Path, [string]$ReaderSid = '') {
    $admins = @('S-1-5-32-544', 'S-1-5-18')
    $allowed = $admins + @(@($ReaderSid) | Where-Object { $_ })
    try {
        if ([IO.File]::GetAttributes($Path) -band [IO.FileAttributes]::ReparsePoint) { return $false }
        if ((Get-OwnerSid $Path) -notin $admins) { return $false }
        $acl = Get-DirAcl $Path
        if (-not $acl.AreAccessRulesProtected) { return $false }
        $other = @($acl.GetAccessRules($true, $true, [Security.Principal.SecurityIdentifier]) | Where-Object { $_.IdentityReference.Value -notin $allowed })
        return ($other.Count -eq 0)
    } catch { return $false }
}

# Sobras de %ProgramData%\eprocia-elevar-* (queda de energia no meio de uma acao): so as de
# Administradores ou SYSTEM com mais de um dia, nunca um link nem a pasta desta execucao.
# [IO.Directory]::Delete nao segue links; o Remove-Item -Recurse do 5.1 segue.
function Remove-StaleElevationDirs {
    if (-not $env:ProgramData) { return }
    $current = if ($selfPath) { Split-Path -Parent $selfPath } else { '' }
    $limit = (Get-Date).AddDays(-1)
    $dirs = @(Get-ChildItem -LiteralPath $env:ProgramData -Directory -Force -ErrorAction SilentlyContinue | Where-Object { $_.Name -like 'eprocia-elevar-*' })
    foreach ($d in $dirs) {
        if ($d.Attributes -band [IO.FileAttributes]::ReparsePoint) { continue }
        if ($current -and $d.FullName -eq $current) { continue }
        if ($d.CreationTime -gt $limit) { continue }
        try {
            if ((Get-OwnerSid $d.FullName) -notin 'S-1-5-32-544', 'S-1-5-18') { continue }
            [IO.Directory]::Delete($d.FullName, $true)
        } catch { }
    }
}

# Texto do -Command do processo elevado. O modelo abaixo (ASCII, sem aspas duplas, com
# aliases e nomes curtos para caber no limite de cerca de 2.048 caracteres do ShellExecuteEx)
# vira uma linha so, com as linhas juntadas por ';' e os marcadores trocados por literais em
# aspas simples. Ele: cria %ProgramData%\eprocia-elevar-<guid> ja com a ACL (Administradores
# e SYSTEM, leitura para o usuario que pediu); confere que e nova, vazia e de Administradores
# ou SYSTEM (senao sai com 4); copia o script e o arquivo da acao da pasta do pai e confere o
# SHA-256 das copias (diferente: 3, sem rodar nada); roda o script de la com -Machine e o log
# na pasta protegida (-From = a ultima copia, a do arquivo da acao, quando houver); grava o
# codigo em 'fim'; espera ate 30 s pelo 'lido' do pai; apaga a pasta e sai com o codigo.
# Encurtado para caber: -PSPath e o alias de -LiteralPath; (131241,2032127)[bool] escolhe
# ReadAndExecute ou FullControl; 0,0 = PropagationFlags None e AccessControlType Allow;
# -nop -ex = -NoProfile -ExecutionPolicy; sc = Set-Content; $g diz se $d e link (1024 =
# ReparsePoint); o powershell.exe filho vem do $PSHOME, nao do PATH. O nome da pasta
# protegida e o da pasta do pai (eprocia-elevar-<guid>), tirado de $p.
# Esse guid e previsivel (a pasta do pai aparece no %TEMP% antes do UAC): por isso, depois
# de criar, uma juncao plantada (CreateDirectory nao falha com pasta existente) sai com 4, e
# a DACL e conferida (protegida, so as tres regras) alem do dono. A remocao final e por
# [IO.Directory]::Delete (nao segue links) e so se nao for link. Com hash diferente, as
# copias saem antes do 'fim' (a pasta e legivel pelo usuario).
function Get-ElevationBootstrap([string]$Parent, [string]$ScriptHash, [string]$PayloadName, [string]$PayloadHash, [string]$UserSid, [string[]]$ActionArgs) {
    $template = @'
$ErrorActionPreference='Stop'
$p=_PAI_
$d=$env:ProgramData+'\'+(Split-Path -Leaf $p)
$i=[Security.Principal.SecurityIdentifier]
$v='S-1-5-32-544','S-1-5-18'
$u=$v+_SID_
$g={[IO.File]::GetAttributes($d)-band 1024}
if(Test-Path -PSPath $d){exit 4}
$s=New-Object Security.AccessControl.DirectorySecurity
$s.SetAccessRuleProtection(1,0)
foreach($r in $u){$s.AddAccessRule([Security.AccessControl.FileSystemAccessRule]::new(($r -as $i),(131241,2032127)[$r -in $v],'ContainerInherit,ObjectInherit',0,0))}
$null=[IO.Directory]::CreateDirectory($d,$s)
if(&$g){exit 4}
$q=[IO.Directory]::GetAccessControl($d)
if(@(Get-ChildItem -Force -PSPath $d).Count -or $q.GetOwner($i) -notin $v -or !$q.AreAccessRulesProtected -or @($q.GetAccessRules(1,1,$i)|?{$_.IdentityReference -notin $u}).Count){exit 4}
$c=3
try{$k=1;foreach($x in ('install.ps1',_HS_),(_NOME_,_HA_)){if($x[0]){$t=$d+'\'+$x[0];Copy-Item -PSPath ($p+'\'+$x[0]) $t;if([Convert]::ToBase64String([Security.Cryptography.SHA256]::Create().ComputeHash([IO.File]::ReadAllBytes($t))) -cne $x[1]){$k=0}}};$a=@(_ARGS_);if($x[0]){$a+='-From',$t};if($k){& $PSHOME\powershell.exe -nop -ex Bypass -File ($d+'\install.ps1') -Machine -Log ($d+'\install.log') -Yes @a;$c=$LASTEXITCODE}else{del ($d+'\*')};sc ($d+'\fim') $c;$w=0;while(![IO.File]::Exists($p+'\lido') -and $w++ -lt 150){sleep -m 200}}finally{if(!(&$g)){[IO.Directory]::Delete($d,1)}}
exit $c
'@
    $lit = { param([string]$Value) "'" + $Value.Replace("'", "''") + "'" }
    # Os hashes (hex de Get-Sha256) vao em Base64: 44 caracteres em vez de 64 cada.
    $b64 = { param([string]$Hex) if (-not $Hex) { return '' }; [Convert]::ToBase64String([byte[]]@(for ($n = 0; $n -lt $Hex.Length; $n += 2) { [Convert]::ToByte($Hex.Substring($n, 2), 16) })) }
    $values = @{
        PAI  = & $lit $Parent
        HS   = & $lit (& $b64 $ScriptHash)
        NOME = & $lit $PayloadName
        HA   = & $lit (& $b64 $PayloadHash)
        SID  = & $lit $UserSid
        ARGS = (@($ActionArgs | ForEach-Object { & $lit $_ }) -join ',')
    }
    $line = @($template -split "`r?`n" | ForEach-Object { $_.Trim() } | Where-Object { $_ }) -join ';'
    # Uma passada so: um valor (a pasta do pai) que contenha um marcador nao e trocado de novo.
    [regex]::Replace($line, '_(PAI|HS|NOME|HA|SID|ARGS)_', [Text.RegularExpressions.MatchEvaluator] { param($m) $values[$m.Groups[1].Value] })
}

# Roda a acao para todos num powershell.exe elevado (UAC) e mostra o log dele. O pai grava
# numa pasta sua o script (com o marcador de protocolo) e o $Payload (instalador ja baixado e
# conferido, ou o exe do -From) com nome fixo; o bootstrap copia os dois para a pasta
# protegida e confere os hashes antes de rodar. O log fica na pasta protegida (o
# administrador nunca escreve numa pasta do usuario); o pai o le ao ver 'fim' e cria 'lido'.
# UAC recusado: devolve $false sem erro. Sucesso: $true.
function Invoke-Elevated([string[]]$ChildArgs, [string]$Payload) {
    $id = [Guid]::NewGuid().ToString('N')
    $parent = Join-Path ([IO.Path]::GetTempPath()) "eprocia-elevar-$id"
    $dst = Join-Path $env:ProgramData "eprocia-elevar-$id"
    New-Item -ItemType Directory -Path $parent | Out-Null
    try {
        $scriptCopy = Join-Path $parent 'install.ps1'
        Save-ScriptCopy $scriptCopy (Get-ScriptText -ForElevation)
        $scriptHash = Get-Sha256 $scriptCopy
        $name = ''
        $payloadHash = ''
        if ($Payload) {
            $name = if ($Payload -like '*-instalador.exe') { 'eprocia-instalador.exe' } else { $exeName }
            $staged = Join-Path $parent $name
            Copy-Item -LiteralPath $Payload -Destination $staged
            $payloadHash = Get-Sha256 $staged
        }
        # O -Yes interno do filho nao autoriza encerrar: vao os PIDs aceitos e, so com o -Yes
        # do usuario, -CloseAny.
        $all = @($ChildArgs) + @("-ClosePids:$(Get-ClosePidText)")
        if ($Yes -and -not $Log) { $all += '-CloseAny' }
        $sid = [Security.Principal.WindowsIdentity]::GetCurrent().User.Value
        $bootstrap = Get-ElevationBootstrap $parent $scriptHash $name $payloadHash $sid $all
        # Uma string so: no 5.1 um array e juntado sem aspas e o -Command colapsaria espacos.
        $uacArgs = "-NoProfile -ExecutionPolicy Bypass -WindowStyle Hidden -Command `"$bootstrap`""
        # O ShellExecuteEx nao garante mais que cerca de 2.048 caracteres: confere antes do UAC.
        if ($uacArgs.Length -ge 2000) { Fail 'Caminho da pasta temporaria longo demais para a acao como administrador. Nada foi alterado.' }
        Write-Step 'Pedindo permissao de administrador (UAC)'
        try {
            $p = Start-Process -FilePath $psExe -ArgumentList $uacArgs -Verb RunAs -WindowStyle Hidden -PassThru
        } catch {
            for ($e = $_.Exception; $e; $e = $e.InnerException) {
                if ($e -is [ComponentModel.Win32Exception] -and $e.NativeErrorCode -eq 1223) {
                    Write-Warn 'Permissao de administrador recusada. Nada foi alterado.'
                    return $false
                }
            }
            throw
        }
        # Fixa o handle: sem isso o ExitCode do processo elevado pode vir vazio no 5.1.
        $null = $p.Handle
        $fim = Join-Path $dst 'fim'
        $deadline = (Get-Date).AddMinutes(15)
        $code = $null
        while ($true) {
            if (Test-Path -LiteralPath $fim) {
                # Um 'fim' numa pasta que nao e de administrador (plantada antes, ou um link)
                # nao vale: seria anunciar um resultado que o filho nao deu.
                if (-not (Test-AdminDir $dst $sid)) { Fail 'A pasta temporaria do administrador nao estava limpa. Nada foi alterado.' }
                $raw = ''
                try { $raw = [IO.File]::ReadAllText($fim).Trim() } catch { }
                if ($raw -match '^-?\d+$') { $code = [int]$raw; break }
            }
            $exited = $false
            try { $exited = $p.HasExited } catch { }
            if ($exited) { break }
            if ((Get-Date) -gt $deadline) { Fail 'A acao como administrador nao terminou em 15 minutos; confira em Configuracoes > Aplicativos.' }
            Start-Sleep -Milliseconds 200
        }
        $logText = ''
        if ($null -ne $code) {
            $childLog = Join-Path $dst 'install.log'
            try { if (Test-Path -LiteralPath $childLog) { $logText = [IO.File]::ReadAllText($childLog) } } catch { }
            try { Show-Log $childLog } finally { New-Item -ItemType File -Path (Join-Path $parent 'lido') -Force | Out-Null }
        } else {
            $p.WaitForExit()
            $code = $p.ExitCode
            if ($null -eq $code) { Fail 'A acao como administrador terminou sem registro (codigo desconhecido). Confira em Configuracoes > Aplicativos.' }
        }
        if ($code -eq 3) { Fail 'O arquivo mudou antes de rodar como administrador. Nada foi alterado.' }
        if ($code -eq 4) { Fail 'A pasta temporaria do administrador nao estava limpa. Nada foi alterado.' }
        if (-not $logText.Trim()) { Fail "A acao como administrador terminou sem registro (codigo $code). Confira em Configuracoes > Aplicativos." }
        if ($code -ne 0) { Fail "A acao como administrador falhou (codigo $code)" }
        $true
    } finally {
        Remove-Item -LiteralPath $parent -Recurse -Force -ErrorAction SilentlyContinue
    }
}

# Atualiza ou reinstala a instalacao para todos. Roda no filho elevado (-Machine, com o
# -From ja na pasta protegida) ou num terminal ja elevado, que cria antes uma pasta protegida
# nova e baixa o instalador direto nela, conferido contra o SHA256SUMS.txt (o %TEMP% do
# administrador, num token dividido, e gravavel pelo mesmo usuario sem elevacao), ou copia o
# -From para la, e roda de la. Um -From que termina em -instalador.exe e o instalador (NSIS,
# em silencio, /D= por ultimo e sem aspas); outro exe e copiado sobre o eprocia.exe, com
# DisplayVersion gravado na chave HKLM. Nunca abre o app.
function Install-Machine($Target, $Rel = $null) {
    Remove-StaleElevationDirs
    $guard = ''
    $tmp = ''
    try {
        $src = $From
        if (-not $Machine) {
            $protected = Join-Path $env:ProgramData ('eprocia-elevar-' + [Guid]::NewGuid().ToString('N'))
            New-ProtectedDir $protected ''
            $guard = $protected
            $name = if (-not $From -or $From -like '*-instalador.exe') { 'eprocia-instalador.exe' } else { $exeName }
            $src = Join-Path $guard $name
            if ($From) { Copy-Item -LiteralPath (Resolve-Path -LiteralPath $From).Path -Destination $src }
            else { Save-ReleaseInstaller $src $Rel | Out-Null }
        }
        if (-not $src -or -not (Test-Path -LiteralPath $src -PathType Leaf)) { Fail "Arquivo nao encontrado: $src" }
        $isInstaller = $src -like '*-instalador.exe'
        if (-not $isInstaller) {
            $tmp = Join-Path $Target.Dir ("$exeName.download-" + [Guid]::NewGuid().ToString('N'))
            Copy-Item -LiteralPath $src -Destination $tmp
        }
        Stop-App $Target.Exe
        if ($isInstaller) {
            Write-Step 'Rodando o instalador em silencio sobre' $Target.Dir
            $proc = Start-Process -FilePath $src -ArgumentList @('/S', "/D=$($Target.Dir)") -Wait -PassThru
            if ($proc.ExitCode -ne 0) { Fail "O instalador terminou com erro (codigo $($proc.ExitCode))" }
        } else {
            Move-ExeWithRetry $tmp $Target.Exe
            Unblock-File -LiteralPath $Target.Exe
            if ($Target.Key) { Set-ItemProperty -LiteralPath $Target.Key -Name DisplayVersion -Value (Get-ExeVersion $Target.Exe) }
        }
        Write-Done "Pronto: eprocIA $(Get-ExeVersion $Target.Exe) instalado para todos os usuarios."
    } finally {
        if ($tmp -and (Test-Path -LiteralPath $tmp)) { Remove-Item -LiteralPath $tmp -Force -ErrorAction SilentlyContinue }
        # So a pasta que este processo criou e conferiu, e nunca um link.
        if ($guard -and (Test-AdminDir $guard)) { [IO.Directory]::Delete($guard, $true) }
    }
}

# Remove a instalacao para todos pelo desinstalador da pasta. Com _?= o NSIS espera de
# verdade (sem isso ele se copia para o temporario e volta na hora) e deixa o uninstall.exe,
# que sai depois, com a pasta se ficar vazia. Nada do perfil do usuario e apagado aqui.
function Uninstall-Machine($Target) {
    Remove-StaleElevationDirs
    $u = Join-Path $Target.Dir 'uninstall.exe'
    if (-not (Test-Path -LiteralPath $u -PathType Leaf)) { Fail "Desinstalador nao encontrado: $u" }
    Stop-App $Target.Exe
    Write-Step 'Rodando o desinstalador de' $Target.Dir
    $proc = Start-Process -FilePath $u -ArgumentList @('/S', "_?=$($Target.Dir)") -Wait -PassThru
    if ($proc.ExitCode -ne 0) { Fail "O desinstalador terminou com erro (codigo $($proc.ExitCode))" }
    Remove-Item -LiteralPath $u -Force -ErrorAction SilentlyContinue
    if ((Test-Path -LiteralPath $Target.Dir) -and @(Get-ChildItem -Force -LiteralPath $Target.Dir).Count -eq 0) {
        [IO.Directory]::Delete($Target.Dir)
    }
    if (Test-Path -LiteralPath $Target.Dir) { Write-Warn "Sobraram arquivos em $($Target.Dir)" }
    Write-Done 'Pronto: eprocIA removido para todos os usuarios'
}

function Invoke-Main {
    # Filho elevado: so a instalacao para todos, da pasta protegida; nunca o ramo por usuario,
    # a tarefa agendada nem o Invoke-Elevated.
    if ($Machine) {
        if (-not $Log) { Fail '-Machine e interno e so vale com -Log' }
        if (-not $Uninstall) { Assert-From }
        $machineTarget = Get-Target
        if ($Uninstall) { Uninstall-Machine $machineTarget } else { Install-Machine $machineTarget }
        return
    }
    $elevated = Test-Elevated
    $target = Get-Target
    if (-not $Uninstall) { Assert-From }
    # A cópia da tarefa agendada roda numa janela oculta: título e banner só no terminal visível.
    if (-not $Log) {
        Set-WindowTitle "eprocIA - $(if ($Uninstall) { 'remoção' } else { 'instalação' })"
        Show-Banner
    }
    if ($DryRun) {
        if (-not (Invoke-DryRun $elevated $target)) { return }
        # O "Sim" da simulação já confirmou; a cópia elevada passa -Yes adiante.
        $script:confirmed = $true
    }
    if ($target.Kind -eq 'machine') {
        $rel = $null
        if ($Uninstall) {
            Write-Title 'Removendo o eprocIA (para todos os usuarios)'
            if (-not (Confirm-Uninstall $target.Dir)) { Write-Cancelled; return }
        } else {
            Enable-Tls12
            $label = if ($From) { 'arquivo local' } elseif ($Version) { $Version } else { 'ultima release' }
            Write-Title "Instalando o eprocIA ($label, para todos os usuarios)"
            if (-not $From) {
                $rel = Get-Release
                if (-not $rel.Installer) { Fail "A release $($rel.Tag) nao tem o instalador" }
            }
            if (-not (Confirm-Install (Get-NewVersion $rel) $target.Dir)) { Write-Cancelled; return }
        }
        if (-not (Confirm-Close $target.Exe)) { Write-Cancelled; return }
        if ($NoDesktop -and -not $Uninstall) { Write-Note 'Na instalacao para todos, o atalho da area de trabalho fica como estava (-NoDesktop nao vale)' }
        if ($elevated) {
            # Ja ha administrador: sem UAC, sem bootstrap e sem tarefa agendada.
            if ($Uninstall) { Uninstall-Machine $target } else { Install-Machine $target $rel }
        } else {
            $download = ''
            try {
                $payload = ''
                if (-not $Uninstall) {
                    if ($From) { $payload = (Resolve-Path -LiteralPath $From).Path }
                    else {
                        $download = Join-Path ([IO.Path]::GetTempPath()) ('eprocia-' + [Guid]::NewGuid().ToString('N') + '-instalador.exe')
                        Save-ReleaseInstaller $download $rel | Out-Null
                        $payload = $download
                    }
                }
                $childArgs = @()
                if ($Uninstall) { $childArgs += '-Uninstall' }
                if (-not (Invoke-Elevated $childArgs $payload)) { return }
            } finally {
                if ($download -and (Test-Path -LiteralPath $download)) { Remove-Item -LiteralPath $download -Force -ErrorAction SilentlyContinue }
            }
        }
        if ($Uninstall) {
            # As pastas do -Purge sao do usuario: quem apaga e este processo, nunca o filho.
            if ($Purge) {
                foreach ($d in Get-PurgeDirs) {
                    if (Test-Path -LiteralPath $d) { Remove-Item -LiteralPath $d -Recurse -Force; Write-Step 'Apagado' $d }
                }
            }
        } elseif (-not $NoStart) {
            # Num terminal elevado, o explorer.exe abre o programa com a conta do usuario.
            if ($elevated) { Start-Process -FilePath 'explorer.exe' -ArgumentList ([string][char]34 + $target.Exe + [char]34) }
            else { Start-Process -FilePath $target.Exe -WorkingDirectory $target.Dir }
            Write-Note 'eprocIA aberto. Ele fica no icone da bandeja ao fechar a janela.'
        }
        return
    }
    if ($elevated) { Invoke-AsUser; return }
    if ($Uninstall) { Uninstall-App } else { Install-App }
}

$failed = $false
try {
    Invoke-Main
} catch {
    $failed = $true
    Write-Fail $_.Exception.Message
    Add-Log 'fail' $_.Exception.Message
} finally { Restore-Console; Restore-Window }
# A cópia da tarefa agendada roda com -File: o código de saída chega ao terminal elevado.
if ($Log -and $failed) { exit 1 }
