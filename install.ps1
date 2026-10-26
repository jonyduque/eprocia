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

if ($Version -and $Version -notmatch '^v?\d+\.\d+\.\d+([-.][0-9A-Za-z.]+)?$') {
    Write-Host "x versão inválida: '$Version' (use uma tag de release, ex.: v0.3.0)" -ForegroundColor Red
    return
}
if ($Purge -and -not $Uninstall) {
    Write-Host 'x -Purge só vale junto com -Uninstall' -ForegroundColor Red
    return
}

# Estilos ANSI montados com o caractere ESC: `$PSStyle` e `` `e `` só existem no 7.
$esc = [char]27
function Get-Rgb([int]$R, [int]$G, [int]$B) { "$esc[38;2;$R;$G;${B}m" }
$cPurple = Get-Rgb 154 92 219
$cPeach  = Get-Rgb 255 117 117
$cGray   = Get-Rgb 128 128 128
$cPink   = Get-Rgb 224 91 191
$cDim    = "$esc[2m"
$cBold   = "$esc[1m"
$cReset  = "$esc[0m"

# UTF-8 para as setas e o ponto; o valor anterior volta no fim, porque sob `irm | iex` a
# mudança ficaria valendo no terminal de quem rodou.
$prevOutputEncoding = [Console]::OutputEncoding
try { [Console]::OutputEncoding = [System.Text.Encoding]::UTF8 } catch { }
function Restore-Console { try { [Console]::OutputEncoding = $prevOutputEncoding } catch { } }

# A cópia sem privilégios grava cada linha no -Log (tipo, texto, valor) e o terminal
# elevado as reapresenta com os mesmos estilos.
function Add-Log([string]$Kind, [string]$Text, [string]$Value = '') {
    if ($Log) { Add-Content -LiteralPath $Log -Value "$Kind`t$Text`t$Value" -Encoding UTF8 }
}
function Write-Title([string]$Text) {
    Write-Host "$cBold$cPink`n$Text$cReset`n"
    Add-Log 'title' $Text
}
# Uma linha de passo: o texto na cor padrão e o valor (um caminho, uma versão) em destaque.
function Write-Step([string]$Text, [string]$Value = '') {
    Write-Host "  - " -NoNewline
    if ($Value) {
        Write-Host "$cGray$Text$cReset " -NoNewline
        Write-Host "$cPeach$Value$cReset"
    } else {
        Write-Host $Text
    }
    Add-Log 'step' $Text $Value
}
function Write-Ok([string]$Text) { Write-Host "  ok $Text" -ForegroundColor Green; Add-Log 'ok' $Text }
function Write-Note([string]$Text) { Write-Host "  $cGray$Text$cReset"; Add-Log 'note' $Text }
function Write-Warn([string]$Text) { Write-Host "  ! $Text" -ForegroundColor Yellow; Add-Log 'warn' $Text }
function Write-Done([string]$Text) { Write-Host ''; Write-Host $Text -ForegroundColor Green; Add-Log 'done' $Text }

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
                'fail'  { Write-Host "x $text" -ForegroundColor Red }
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
    $draw = {
        if ($startTop -ge 0) { [Console]::SetCursorPosition(0, $startTop) }
        for ($i = 0; $i -lt $Options.Count; $i++) {
            $k = $keyOptions[$i]
            $v = $Options[$k]
            if ($i -eq $sel) {
                $colArrow = "$cPurple$cBold{0,-2}$cReset" -f " $([char]0x2192)"
                $colKey   = "$cPurple$cBold{0,-25}$cReset" -f $k
                $colDesc  = "$cPurple$cDim{0,-30}$cReset" -f $v
            } else {
                $colArrow = "$cGray{0,-2}$cReset" -f '  '
                $colKey   = '{0,-25}' -f $k
                $colDesc  = "$cDim{0,-30}$cReset" -f $v
            }
            Write-Host "  $colArrow $colKey $colDesc"
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

function Test-Elevated {
    $id = [Security.Principal.WindowsIdentity]::GetCurrent()
    (New-Object Security.Principal.WindowsPrincipal $id).IsInRole([Security.Principal.WindowsBuiltInRole]::Administrator)
}

# Dono do explorer.exe da sessão deste processo (ou de qualquer sessão, se esta não tiver
# shell, como num SSH elevado).
function Get-SessionUser {
    $session = (Get-Process -Id $PID).SessionId
    $shells = @(Get-CimInstance Win32_Process -Filter "Name = 'explorer.exe'")
    if ($shells.Count -eq 0) { Fail 'nenhum usuário com sessão aberta no Windows; entre com a conta que vai usar o eprocIA e rode de novo' }
    $mine = @($shells | Where-Object { $_.SessionId -eq $session })
    if ($mine.Count -eq 0) { $mine = $shells }
    $owner = Invoke-CimMethod -InputObject $mine[0] -MethodName GetOwner
    if ($owner.ReturnValue -ne 0) { Fail 'não foi possível identificar o usuário da sessão' }
    "$($owner.Domain)\$($owner.User)"
}

function Get-InstallDir { if ($Dir) { $Dir } else { Join-Path $env:LOCALAPPDATA "Programs\$app" } }

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

function Get-Status($ErrorRecord) {
    try { [int]$ErrorRecord.Exception.Response.StatusCode } catch { 0 }
}

# Repete até 3 vezes quando a conexão cai (o exe tem mais de 100 MB); erro HTTP não repete.
function Invoke-Download([hashtable]$Request) {
    for ($try = 1; ; $try++) {
        try { return Invoke-WebRequest @Request }
        catch {
            if ((Get-Status $_) -ne 0 -or $try -ge 3) { throw }
            Write-Note "conexão interrompida; tentando de novo ($($try + 1)/3)"
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
        if (-not $tok) { Fail "o GitHub negou o acesso a $Uri (código $status); se o repositório for privado, defina GITHUB_TOKEN ou entre com 'gh auth login'" }
        if ($AuthUri) { $request.Uri = $AuthUri }
        $request.Headers = @{ Authorization = "Bearer $tok"; Accept = $Accept; 'X-GitHub-Api-Version' = '2022-11-28' }
        try { return Invoke-Download $request }
        catch { Fail "não foi possível baixar $($request.Uri) nem com o token do GitHub (código $(Get-Status $_))" }
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
    if ($exe.Count -ne 1) { Fail "a release $($rel.tag_name) não tem exatamente um eprocia-*-windows-amd64.exe" }
    if ($sums.Count -ne 1) { Fail "a release $($rel.tag_name) não tem SHA256SUMS.txt" }
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

# Baixa o exe da release para $Target e confere o SHA-256. Devolve a tag.
function Save-ReleaseExe([string]$Target) {
    $rel = Get-Release
    Write-Step 'baixando' "$($rel.Exe.name) ($($rel.Tag))"
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
    if ($running.Count -gt 0) { Fail 'o eprocIA está aberto; feche pelo ícone da bandeja (Sair) e rode de novo' }
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
    if ($PSVersionTable.PSVersion.Major -lt 6) {
        # O 5.1 pode usar TLS 1.0 por padrão, que o GitHub recusa.
        [Net.ServicePointManager]::SecurityProtocol = [Net.ServicePointManager]::SecurityProtocol -bor [Net.SecurityProtocolType]::Tls12
    }
    $label = if ($From) { 'arquivo local' } elseif ($Version) { $Version } else { 'última release' }
    Write-Title "Instalando o eprocIA ($label, só para você)"
    Assert-Closed $exe

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
            Write-Step 'copiando' $source
            Copy-Item -LiteralPath $source -Destination $tmp
        } else {
            Save-ReleaseExe $tmp | Out-Null
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
    Write-Step 'salvo em' $exe

    $shortcuts = Get-Shortcuts
    New-Shortcut $shortcuts[0] $exe
    Write-Step 'atalho no menu Iniciar' $shortcuts[0]
    if ($NoDesktop) {
        Remove-Item -LiteralPath $shortcuts[1] -Force -ErrorAction SilentlyContinue
    } else {
        New-Shortcut $shortcuts[1] $exe
        Write-Step 'atalho na área de trabalho' $shortcuts[1]
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
        QuietUninstallString = $uninstall
        HelpLink             = "https://github.com/$repo"
    }
    foreach ($k in $values.Keys) { Set-ItemProperty -LiteralPath $uninstallKey -Name $k -Value $values[$k] }
    foreach ($k in 'NoModify', 'NoRepair') { Set-ItemProperty -LiteralPath $uninstallKey -Name $k -Value 1 -Type DWord }
    $kb = [int][Math]::Ceiling((Get-Item -LiteralPath $exe).Length / 1KB)
    Set-ItemProperty -LiteralPath $uninstallKey -Name EstimatedSize -Value $kb -Type DWord
    Write-Step 'registrado em Aplicativos instalados'

    # Quem usava a versão portátil com "Iniciar com o Windows" passa a abrir a instalada.
    if (Get-ItemProperty -LiteralPath $runKey -Name $app -ErrorAction SilentlyContinue) {
        Set-ItemProperty -LiteralPath $runKey -Name $app -Value "`"$exe`" $flagTray"
        Write-Step '"Iniciar com o Windows" agora abre' $exe
    }

    if (-not (Test-WebView2)) {
        Write-Warn 'Microsoft Edge WebView2 Runtime não encontrado; instale em https://go.microsoft.com/fwlink/p/?LinkId=2124703'
    }
    foreach ($other in @(Get-MachineInstall)) {
        Write-Warn "há também uma instalação para todos os usuários ($($other.DisplayName)); desinstale em Configurações > Aplicativos para não ficar com duas cópias"
    }

    Write-Done "Pronto: eprocIA $version (só para você)"
    if (-not $NoStart) {
        Start-Process -FilePath $exe -WorkingDirectory $installDir
        Write-Note 'eprocIA aberto; ele fica no ícone da bandeja ao fechar a janela'
    }
}

function Uninstall-App {
    $installDir = Get-InstallDir
    $exe = Join-Path $installDir $exeName
    Write-Title 'Removendo o eprocIA (só para você)'
    Assert-Closed $exe
    # A pasta pode ser a atual (desinstalação por Aplicativos instalados).
    Set-Location -LiteralPath ([IO.Path]::GetTempPath())

    $run = Get-ItemProperty -LiteralPath $runKey -Name $app -ErrorAction SilentlyContinue
    if ($run -and $run.$app -like "*$exe*") {
        Remove-ItemProperty -LiteralPath $runKey -Name $app
        Write-Step '"Iniciar com o Windows" desligado'
    }
    foreach ($s in Get-Shortcuts) {
        if (Test-Path -LiteralPath $s) { Remove-Item -LiteralPath $s -Force; Write-Step 'atalho removido' $s }
    }
    Remove-Item -LiteralPath $uninstallKey -Recurse -Force -ErrorAction SilentlyContinue
    if (Test-Path -LiteralPath $installDir) { Remove-Item -LiteralPath $installDir -Recurse -Force }
    Write-Ok "removido: $installDir"

    $config = Join-Path $env:APPDATA $app
    if ($Purge) {
        # Configurações e falhas.json, dados do WebView2 e binários do OCR extraídos.
        foreach ($d in @($config, (Join-Path $env:APPDATA $exeName), (Join-Path ([IO.Path]::GetTempPath()) 'eprocIA_bin'))) {
            if (Test-Path -LiteralPath $d) { Remove-Item -LiteralPath $d -Recurse -Force; Write-Step 'apagado' $d }
        }
    } else {
        Write-Note "configurações mantidas em $config (-Purge apaga)"
    }
    Write-Done 'Pronto: eprocIA removido'
}

# Num terminal elevado, roda este script de novo sem privilégios como o usuário da sessão,
# por uma tarefa agendada de uso único, e reapresenta o log dele.
function Invoke-AsUser {
    $user = Get-SessionUser
    Write-Note "terminal elevado: a instalação continua só para $user, sem privilégios"
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

        $argv = @('-NoProfile', '-ExecutionPolicy', 'Bypass', '-WindowStyle', 'Hidden', '-File', "`"$stagedScript`"", '-Log', "`"$stagedLog`"")
        if ($Dir) { $argv += @('-Dir', "`"$Dir`"") }
        if ($Uninstall) {
            $argv += '-Uninstall'
            if ($Purge) { $argv += '-Purge' }
        } else {
            # O exe é obtido aqui: o token do gh e o -From podem só existir para o administrador.
            $stagedExe = Join-Path $stage $exeName
            if ($From) { Copy-Item -LiteralPath (Resolve-Path -LiteralPath $From).Path -Destination $stagedExe }
            else {
                if ($PSVersionTable.PSVersion.Major -lt 6) {
                    [Net.ServicePointManager]::SecurityProtocol = [Net.ServicePointManager]::SecurityProtocol -bor [Net.SecurityProtocolType]::Tls12
                }
                Save-ReleaseExe $stagedExe | Out-Null
            }
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
        if (-not $finished) { Fail 'a cópia sem privilégios não terminou em 10 minutos' }
        if ($info.LastTaskResult -ne 0) { Fail "a cópia sem privilégios terminou com erro (código $($info.LastTaskResult))" }
    } finally {
        Remove-Item -LiteralPath $stage -Recurse -Force -ErrorAction SilentlyContinue
    }
}

# -DryRun: as mesmas decisões, mostradas em vez de feitas. Devolve $true se o usuário pedir
# a execução real em seguida.
function Invoke-DryRun([bool]$Elevated) {
    $action = if ($Uninstall) { 'remover' } else { 'instalar' }
    Write-Title "Simulação: $action o eprocIA (só para você)"
    if ($Elevated) {
        Write-Step 'terminal elevado: rodaria sem privilégios, por tarefa agendada, como' (Get-SessionUser)
        Write-Note 'os caminhos abaixo seriam os do perfil desse usuário'
    }
    $installDir = Get-InstallDir
    $exe = Join-Path $installDir $exeName
    $shortcuts = Get-Shortcuts
    if ($Uninstall) {
        if (Test-Path -LiteralPath $installDir) { Write-Step 'apagaria' $installDir } else { Write-Step 'não há eprocIA em' $installDir }
        Write-Step 'apagaria os atalhos e a entrada de Aplicativos instalados'
        if ($Purge) { Write-Step 'apagaria as configurações em' (Join-Path $env:APPDATA $app) }
        else { Write-Note "configurações em $(Join-Path $env:APPDATA $app) ficariam" }
    } else {
        if ($From) { Write-Step 'copiaria' $From }
        else {
            $label = if ($Version) { "release $Version" } else { 'release mais recente' }
            Write-Step 'baixaria' "eprocia-*-windows-amd64.exe ($label de $repo)"
            Write-Step 'conferiria o SHA-256 com o SHA256SUMS.txt da release'
        }
        if (Test-Path -LiteralPath $exe) { Write-Step "substituiria a $(Get-ExeVersion $exe) em" $exe } else { Write-Step 'salvaria em' $exe }
        Write-Step 'criaria o atalho' $shortcuts[0]
        if (-not $NoDesktop) { Write-Step 'criaria o atalho' $shortcuts[1] }
        Write-Step 'registraria em Aplicativos instalados'
        if (-not $NoStart) { Write-Step 'abriria o eprocIA' }
    }
    Write-Host ''
    Write-Note 'Simulação: nada foi alterado.'
    $verb = if ($Uninstall) { 'remoção' } else { 'instalação' }
    # Padrão "Não": sem console para perguntar, uma simulação nunca vira execução real.
    $go = Read-Choice "Iniciar a $verb real?" ([ordered]@{
        'Sim' = "faz a $verb agora"
        'Não' = 'sai sem alterar nada'
    }) -Default 1
    $go -eq 0
}

function Invoke-Main {
    $elevated = Test-Elevated
    if ($DryRun -and -not (Invoke-DryRun $elevated)) { return }
    if ($elevated) { Invoke-AsUser; return }
    if ($Uninstall) { Uninstall-App } else { Install-App }
}

$failed = $false
try {
    Invoke-Main
} catch {
    $failed = $true
    Write-Host "x $($_.Exception.Message)" -ForegroundColor Red
    Add-Log 'fail' $_.Exception.Message
} finally { Restore-Console }
# A cópia da tarefa agendada roda com -File: o código de saída chega ao terminal elevado.
if ($Log -and $failed) { exit 1 }
