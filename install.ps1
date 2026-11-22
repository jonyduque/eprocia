<#
.SYNOPSIS
    Instala (ou atualiza, ou remove) o eprocIA, sempre só para o usuário atual.

.DESCRIPTION
    Baixa o eprocia-<versão>-windows-amd64.exe da release no GitHub, confere com o
    SHA256SUMS.txt publicado e copia para %LOCALAPPDATA%\Programs\eprocIA, com atalhos no
    menu Iniciar e na área de trabalho e entrada em "Aplicativos instalados" (HKCU).
    Nunca pede administrador.

    Num terminal elevado a instalação continua sendo do usuário: o script roda de novo, sem
    privilégios, como o dono da sessão do Windows (o dono do explorer.exe), por uma tarefa
    agendada de uso único. Assim nada vai para o perfil da conta de administrador.
    Rodar de novo atualiza para a release mais recente. Funciona no Windows PowerShell 5.1 e no 7.

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

.PARAMETER Log
    Interno: arquivo de log da cópia sem privilégios iniciada por um terminal elevado.

.PARAMETER Self
    Interno: caminho do script quando o 5.1 o relê como UTF-8.
#>
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
    [string]$Self = ''
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

Set-StrictMode -Version 2.0
$ErrorActionPreference = 'Stop'
$ProgressPreference = 'SilentlyContinue'

$repo = 'jonyduque/eprocia'
$app = 'eprocIA'
$exeName = 'eprocia.exe'
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

# Pergunta antes de mudar a máquina, com a escolha por setas: a ação ($Action, descrita por
# $Detail) ou Cancelar; Enter escolhe, Esc cancela. Com -Yes não pergunta; na simulação só
# mostra o que perguntaria; sem console, exige -Yes.
function Confirm-Plan([string]$Question, [string]$Action, [string]$Detail) {
    if ($Yes -or $script:confirmed) { return $true }
    if ($DryRun) { Write-Note "Perguntaria: $Question ($Action ou Cancelar)"; return $true }
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
    [pscustomobject]@{ Tag = $rel.tag_name; Exe = $exe[0]; Sums = $sums[0] }
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
    Write-Step 'Baixando' "$($rel.Exe.name) ($($rel.Tag))"
    Invoke-GitHub $rel.Exe.browser_download_url 'application/octet-stream' $Target $rel.Exe.url | Out-Null
    $sumText = Get-Text (Invoke-GitHub $rel.Sums.browser_download_url 'application/octet-stream' '' $rel.Sums.url)
    $pattern = "^\s*([0-9a-fA-F]{64})\s+\*?$([regex]::Escape($rel.Exe.name))\s*$"
    $line = @($sumText -split "`r?`n" | Where-Object { $_ -match $pattern })
    if ($line.Count -eq 0 -or $line[0] -notmatch $pattern) { Fail "o SHA256SUMS.txt não lista $($rel.Exe.name)" }
    $expected = $Matches[1].ToUpperInvariant()
    $actual = Get-Sha256 $Target
    if ($actual -ne $expected) { Fail "SHA-256 não confere (esperado $expected, baixado $actual)" }
    Write-Ok 'SHA-256 conferido'
    $rel.Tag
}

# Texto deste script: do arquivo, ou baixado de novo quando veio por `irm | iex`.
function Get-ScriptText {
    if ($selfPath) { return [IO.File]::ReadAllText($selfPath, [Text.Encoding]::UTF8) }
    Get-Text (Invoke-GitHub $scriptUrl 'application/vnd.github.raw' '' "https://api.github.com/repos/$repo/contents/install.ps1")
}

# Com BOM: a cópia local roda com -File no 5.1 (desinstalação, tarefa agendada).
function Save-ScriptCopy([string]$Path) {
    [IO.File]::WriteAllText($Path, (Get-ScriptText).TrimStart([char]0xFEFF), (New-Object Text.UTF8Encoding $true))
}

# --- Estado da máquina --------------------------------------------------------------------

function Assert-Closed([string]$Exe) {
    $running = @(Get-Process -Name 'eprocia' -ErrorAction SilentlyContinue | Where-Object {
        try { $_.Path -eq $Exe } catch { $false }
    })
    if ($running.Count -gt 0) { Fail 'O eprocIA está aberto. Feche pelo ícone da bandeja (Sair) e rode de novo' }
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
    Assert-Closed $exe
    $rel = if ($From) { $null } else { Get-Release }
    if (-not (Confirm-Install (Get-NewVersion $rel) $installDir)) { Write-Cancelled; return }

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
        Move-Item -LiteralPath $tmp -Destination $exe -Force
        if ($copyScript) { Move-Item -LiteralPath $tmpScript -Destination $localScript -Force }
    } finally {
        foreach ($f in $tmp, $tmpScript) { if (Test-Path -LiteralPath $f) { Remove-Item -LiteralPath $f -Force } }
    }
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

    $uninstall = "powershell.exe -NoProfile -ExecutionPolicy Bypass -File `"$localScript`" -Uninstall"
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
    Assert-Closed $exe
    if (-not (Confirm-Uninstall $installDir)) { Write-Cancelled; return }
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
    Write-Note "Terminal elevado: a instalação continua só para $user, sem privilégios."
    # A pergunta é feita aqui: a cópia sem privilégios roda sem console e recebe -Yes.
    $userDir = Get-UserInstallDir $user
    Assert-Closed (Join-Path $userDir $exeName)
    $rel = $null
    if ($Uninstall) {
        if (-not (Confirm-Uninstall $userDir)) { Write-Cancelled; return }
    } else {
        Enable-Tls12
        if (-not $From) { $rel = Get-Release }
        if (-not (Confirm-Install (Get-NewVersion $rel) $userDir)) { Write-Cancelled; return }
    }
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
        $action = New-ScheduledTaskAction -Execute 'powershell.exe' -Argument ($argv -join ' ')
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
function Invoke-DryRun([bool]$Elevated) {
    $action = if ($Uninstall) { 'remover' } else { 'instalar' }
    Write-Title "Simulação: $action o eprocIA (só para você)"
    $installDir = Get-InstallDir
    if ($Elevated) {
        $user = Get-SessionUser
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
    # A versão nova vem da API do GitHub; sem rede, a pergunta sai sem ela.
    if ($Uninstall) { $p = Get-UninstallPrompt }
    else {
        $newVersion = ''
        try { Enable-Tls12; $newVersion = Get-NewVersion $(if ($From) { $null } else { Get-Release }) } catch { }
        $p = Get-InstallPrompt $newVersion $installDir
    }
    Confirm-Plan $p.Question $p.Action $p.Detail | Out-Null
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

function Invoke-Main {
    $elevated = Test-Elevated
    if (-not $Uninstall) { Assert-From }
    # A cópia da tarefa agendada roda numa janela oculta: título e banner só no terminal visível.
    if (-not $Log) {
        Set-WindowTitle "eprocIA - $(if ($Uninstall) { 'remoção' } else { 'instalação' })"
        Show-Banner
    }
    if ($DryRun) {
        if (-not (Invoke-DryRun $elevated)) { return }
        # O "Sim" da simulação já confirmou; a cópia elevada passa -Yes adiante.
        $script:confirmed = $true
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
