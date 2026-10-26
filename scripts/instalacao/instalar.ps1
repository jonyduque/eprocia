<#
.SYNOPSIS
Instala, atualiza ou remove o eprocIA só para o usuário atual.

.DESCRIPTION
Copia o executável para %LOCALAPPDATA%\Programs\eprocIA, cria os atalhos do
menu Iniciar e da área de trabalho e registra a desinstalação em
"Aplicativos instalados", tudo em HKCU, sem pedir administrador.

A instalação é sempre do usuário, mesmo num terminal elevado: nesse caso o
script se executa de novo, sem privilégios, como o usuário dono da sessão
(o dono do explorer.exe), por meio de uma tarefa agendada temporária. Assim
os arquivos, os atalhos e o registro ficam no perfil de quem usa o Windows,
e não no da conta de administrador que elevou o terminal.

O executável vem, nesta ordem, de -Origem, de um eprocia*.exe ao lado do
script ou da release no GitHub (exige o gh autenticado, porque o repositório
é privado; a soma SHA-256 é conferida com o SHA256SUMS.txt da release).

.EXAMPLE
powershell -ExecutionPolicy Bypass -File instalar.ps1

.EXAMPLE
powershell -ExecutionPolicy Bypass -File instalar.ps1 -Origem .\eprocia.exe -NaoAbrir

.EXAMPLE
powershell -ExecutionPolicy Bypass -File instalar.ps1 -Desinstalar -RemoverDados
#>
[CmdletBinding()]
param(
    # Executável a instalar. Sem ele, usa o que estiver ao lado do script ou baixa a release.
    [string]$Origem,
    # Tag da release a baixar (ex.: v0.3.0). Sem ela, baixa a mais recente.
    [string]$Versao,
    [switch]$Desinstalar,
    # Com -Desinstalar: apaga também configurações, histórico de falhas e cache do OCR.
    [switch]$RemoverDados,
    [switch]$SemAtalhoNaAreaDeTrabalho,
    [switch]$NaoAbrir,
    # Uso interno: arquivo de log da execução sem privilégios.
    [string]$Log
)

Set-StrictMode -Version Latest
$ErrorActionPreference = 'Stop'
$ProgressPreference = 'SilentlyContinue'

$Nome = 'eprocIA'
$NomeExe = 'eprocia.exe'
$Repositorio = 'jonyduque/eprocia'
$FlagTray = '--tray' # internal/autostart.FlagTray
$ChaveDesinstalar = 'HKCU:\Software\Microsoft\Windows\CurrentVersion\Uninstall\eprocIA'
$ChaveRun = 'HKCU:\Software\Microsoft\Windows\CurrentVersion\Run'
$EsteScript = $PSCommandPath

function Escrever([string]$Texto, [string]$Cor = 'Gray') {
    Write-Host $Texto -ForegroundColor $Cor
    if ($Log) { Add-Content -LiteralPath $Log -Value $Texto -Encoding UTF8 }
}

function Test-Elevado {
    $identidade = [Security.Principal.WindowsIdentity]::GetCurrent()
    $principal = New-Object Security.Principal.WindowsPrincipal($identidade)
    return $principal.IsInRole([Security.Principal.WindowsBuiltInRole]::Administrator)
}

# Dono do explorer.exe da sessão deste processo (ou de qualquer sessão, se
# esta não tiver shell, como num SSH elevado).
function Get-UsuarioInterativo {
    $sessao = (Get-Process -Id $PID).SessionId
    $exploradores = @(Get-CimInstance Win32_Process -Filter "Name = 'explorer.exe'")
    if ($exploradores.Count -eq 0) {
        throw 'Nenhum usuário com sessão aberta no Windows. Entre com a conta que vai usar o eprocIA e rode o script de novo.'
    }
    $escolhido = @($exploradores | Where-Object { $_.SessionId -eq $sessao })
    if ($escolhido.Count -eq 0) { $escolhido = $exploradores }
    $dono = Invoke-CimMethod -InputObject $escolhido[0] -MethodName GetOwner
    if ($dono.ReturnValue -ne 0) { throw 'Não foi possível identificar o usuário da sessão.' }
    return "$($dono.Domain)\$($dono.User)"
}

function Get-PastaInstalacao { Join-Path $env:LOCALAPPDATA "Programs\$Nome" }

function Get-Atalhos {
    $lista = @(Join-Path ([Environment]::GetFolderPath('Programs')) "$Nome.lnk")
    $lista += Join-Path ([Environment]::GetFolderPath('Desktop')) "$Nome.lnk"
    return $lista
}

# Baixa o exe portátil da release e confere a soma SHA-256.
function Get-ExeDaRelease([string]$Destino) {
    if (-not (Get-Command gh -ErrorAction SilentlyContinue)) {
        throw "O GitHub CLI (gh) não está instalado. Baixe eprocia-<versão>-windows-amd64.exe em https://github.com/$Repositorio/releases e rode o script com -Origem <arquivo>."
    }
    $alvo = if ($Versao) { $Versao } else { 'mais recente' }
    Escrever "Baixando a release $alvo de $Repositorio..."
    $argumentos = @('release', 'download')
    if ($Versao) { $argumentos += $Versao }
    $argumentos += @('--repo', $Repositorio, '--pattern', 'eprocia-*-windows-amd64.exe', '--pattern', 'SHA256SUMS.txt', '--dir', $Destino, '--clobber')
    & gh @argumentos
    if ($LASTEXITCODE -ne 0) { throw "O gh release download falhou (código $LASTEXITCODE). Confira o login com 'gh auth status'." }

    $exe = @(Get-ChildItem -LiteralPath $Destino -Filter 'eprocia-*-windows-amd64.exe')
    if ($exe.Count -ne 1) { throw 'A release não tem exatamente um eprocia-*-windows-amd64.exe.' }
    $somas = Join-Path $Destino 'SHA256SUMS.txt'
    if (-not (Test-Path -LiteralPath $somas)) { throw 'A release não tem SHA256SUMS.txt.' }
    $padrao = "^\s*([0-9a-fA-F]{64})\s+\*?$([regex]::Escape($exe[0].Name))\s*$"
    $linha = Get-Content -LiteralPath $somas | Where-Object { $_ -match $padrao } | Select-Object -First 1
    if (-not $linha -or $linha -notmatch $padrao) { throw "O SHA256SUMS.txt não lista $($exe[0].Name)." }
    $esperado = $Matches[1].ToLowerInvariant()
    # Get-FileHash some quando o PSModulePath vem de um pwsh pai; o .NET não.
    $sha = [Security.Cryptography.SHA256]::Create()
    $fluxo = [IO.File]::OpenRead($exe[0].FullName)
    try { $obtido = -join ($sha.ComputeHash($fluxo) | ForEach-Object { $_.ToString('x2') }) } finally { $fluxo.Dispose(); $sha.Dispose() }
    if ($esperado -ne $obtido) { throw "Soma SHA-256 não confere para $($exe[0].Name): esperado $esperado, obtido $obtido." }
    Escrever "Soma SHA-256 conferida ($($exe[0].Name))."
    return $exe[0].FullName
}

function Resolve-Origem([string]$PastaTemporaria) {
    if ($Origem) {
        $caminho = (Resolve-Path -LiteralPath $Origem).Path
        if ((Get-Item -LiteralPath $caminho).PSIsContainer) { throw "-Origem aponta para uma pasta: $caminho" }
        return $caminho
    }
    if ($PSScriptRoot -and ($PSScriptRoot -ne (Get-PastaInstalacao))) {
        $vizinho = Get-ChildItem -LiteralPath $PSScriptRoot -Filter 'eprocia*.exe' -ErrorAction SilentlyContinue |
            Where-Object { $_.Name -notmatch 'instala' } | Sort-Object LastWriteTime -Descending | Select-Object -First 1
        if ($vizinho) {
            Escrever "Usando $($vizinho.FullName)."
            return $vizinho.FullName
        }
    }
    return Get-ExeDaRelease $PastaTemporaria
}

function Assert-Fechado([string]$Exe) {
    $rodando = @(Get-Process -Name 'eprocia' -ErrorAction SilentlyContinue | Where-Object {
        try { $_.Path -eq $Exe } catch { $false }
    })
    if ($rodando.Count -gt 0) {
        throw "O eprocIA está aberto. Feche pelo ícone da bandeja (Sair) e rode o script de novo."
    }
}

function New-Atalho([string]$Caminho, [string]$Alvo) {
    $shell = New-Object -ComObject WScript.Shell
    $lnk = $shell.CreateShortcut($Caminho)
    $lnk.TargetPath = $Alvo
    $lnk.WorkingDirectory = Split-Path $Alvo
    $lnk.IconLocation = "$Alvo,0"
    $lnk.Description = 'Transforma os ZIPs do Eproc num texto para IA'
    $lnk.Save()
}

function Test-WebView2 {
    $id = '{F3017226-FE2A-4295-8BDF-00C3A9A7E4C5}'
    foreach ($chave in @(
            "HKLM:\SOFTWARE\WOW6432Node\Microsoft\EdgeUpdate\Clients\$id",
            "HKLM:\SOFTWARE\Microsoft\EdgeUpdate\Clients\$id",
            "HKCU:\Software\Microsoft\EdgeUpdate\Clients\$id")) {
        $pv = (Get-ItemProperty -LiteralPath $chave -ErrorAction SilentlyContinue).pv
        if ($pv -and $pv -ne '0.0.0.0') { return $true }
    }
    return $false
}

# Instalação do NSIS (Arquivos de Programas) deixaria duas cópias e dois atalhos.
function Get-InstalacaoParaTodos {
    foreach ($raiz in @('HKLM:\SOFTWARE\Microsoft\Windows\CurrentVersion\Uninstall',
            'HKLM:\SOFTWARE\WOW6432Node\Microsoft\Windows\CurrentVersion\Uninstall')) {
        Get-ChildItem -LiteralPath $raiz -ErrorAction SilentlyContinue | ForEach-Object {
            $p = Get-ItemProperty -LiteralPath $_.PSPath -ErrorAction SilentlyContinue
            if ($p -and ($p.PSObject.Properties.Name -contains 'DisplayName') -and $p.DisplayName -like '*eprocia*') { $p }
        }
    }
}

function Install-Eprocia {
    $pasta = Get-PastaInstalacao
    $exe = Join-Path $pasta $NomeExe
    $temporaria = Join-Path ([IO.Path]::GetTempPath()) "eprocia-instalacao-$([guid]::NewGuid().ToString('N'))"
    New-Item -ItemType Directory -Path $temporaria | Out-Null
    try {
        $origemExe = Resolve-Origem $temporaria
        Assert-Fechado $exe

        New-Item -ItemType Directory -Path $pasta -Force | Out-Null
        if ($origemExe -ne $exe) {
            # Copia com outro nome e troca, para não deixar um exe pela metade.
            $parcial = "$exe.novo"
            Copy-Item -LiteralPath $origemExe -Destination $parcial -Force
            Move-Item -LiteralPath $parcial -Destination $exe -Force
        }
        # Sem a marca de download, o SmartScreen não barra cada abertura.
        Unblock-File -LiteralPath $exe
        $script = Join-Path $pasta 'instalar.ps1'
        if ($EsteScript -ne $script) { Copy-Item -LiteralPath $EsteScript -Destination $script -Force }

        # O bloco de strings do Wails usa o idioma 0000, que o FileVersionInfo não
        # lê; a parte fixa traz a versão.
        $info = (Get-Item -LiteralPath $exe).VersionInfo
        $versao = '{0}.{1}.{2}' -f $info.FileMajorPart, $info.FileMinorPart, $info.FileBuildPart
        Escrever "Instalado em $exe (versão $versao)."

        $atalhos = Get-Atalhos
        New-Atalho $atalhos[0] $exe
        if ($SemAtalhoNaAreaDeTrabalho) {
            Remove-Item -LiteralPath $atalhos[1] -Force -ErrorAction SilentlyContinue
        } else {
            New-Atalho $atalhos[1] $exe
        }
        Escrever 'Atalhos criados.'

        New-Item -Path $ChaveDesinstalar -Force | Out-Null
        $valores = @{
            DisplayName     = $Nome
            DisplayVersion  = $versao
            Publisher       = $Nome
            DisplayIcon     = "$exe,0"
            InstallLocation = $pasta
            UninstallString = "powershell.exe -NoProfile -ExecutionPolicy Bypass -File `"$script`" -Desinstalar"
            QuietUninstallString = "powershell.exe -NoProfile -ExecutionPolicy Bypass -WindowStyle Hidden -File `"$script`" -Desinstalar"
            HelpLink        = "https://github.com/$Repositorio"
        }
        foreach ($k in $valores.Keys) { Set-ItemProperty -LiteralPath $ChaveDesinstalar -Name $k -Value $valores[$k] }
        foreach ($k in @('NoModify', 'NoRepair')) { Set-ItemProperty -LiteralPath $ChaveDesinstalar -Name $k -Value 1 -Type DWord }
        $kb = [int][Math]::Ceiling((Get-Item -LiteralPath $exe).Length / 1KB)
        Set-ItemProperty -LiteralPath $ChaveDesinstalar -Name EstimatedSize -Value $kb -Type DWord
        Escrever 'Registrado em Aplicativos instalados.'

        # Quem usava a versão portátil com "Iniciar com o Windows" passa a abrir a instalada.
        $run = Get-ItemProperty -LiteralPath $ChaveRun -Name $Nome -ErrorAction SilentlyContinue
        if ($run) {
            Set-ItemProperty -LiteralPath $ChaveRun -Name $Nome -Value "`"$exe`" $FlagTray"
            Escrever '"Iniciar com o Windows" agora aponta para a cópia instalada.'
        }

        if (-not (Test-WebView2)) {
            Escrever 'Aviso: o Microsoft Edge WebView2 Runtime não foi encontrado. Instale-o em https://go.microsoft.com/fwlink/p/?LinkId=2124703' 'Yellow'
        }
        foreach ($outra in @(Get-InstalacaoParaTodos)) {
            Escrever "Aviso: há também uma instalação para todos os usuários ($($outra.DisplayName)). Desinstale-a em Configurações > Aplicativos para não ficar com duas cópias." 'Yellow'
        }

        if (-not $NaoAbrir) {
            Start-Process -FilePath $exe -WorkingDirectory $pasta
            Escrever 'eprocIA aberto.'
        }
        Escrever 'Pronto.' 'Green'
    } finally {
        Remove-Item -LiteralPath $temporaria -Recurse -Force -ErrorAction SilentlyContinue
    }
}

function Uninstall-Eprocia {
    $pasta = Get-PastaInstalacao
    $exe = Join-Path $pasta $NomeExe
    Assert-Fechado $exe
    Set-Location -LiteralPath ([IO.Path]::GetTempPath())

    $run = Get-ItemProperty -LiteralPath $ChaveRun -Name $Nome -ErrorAction SilentlyContinue
    if ($run -and $run.$Nome -like "*$exe*") {
        Remove-ItemProperty -LiteralPath $ChaveRun -Name $Nome
        Escrever '"Iniciar com o Windows" desligado.'
    }
    foreach ($atalho in Get-Atalhos) { Remove-Item -LiteralPath $atalho -Force -ErrorAction SilentlyContinue }
    Remove-Item -LiteralPath $ChaveDesinstalar -Recurse -Force -ErrorAction SilentlyContinue
    if (Test-Path -LiteralPath $pasta) { Remove-Item -LiteralPath $pasta -Recurse -Force }
    Escrever "Removido de $pasta."

    if ($RemoverDados) {
        # Configurações e falhas.json, dados do WebView2 e binários do OCR extraídos.
        foreach ($dados in @((Join-Path $env:APPDATA $Nome), (Join-Path $env:APPDATA $NomeExe), (Join-Path ([IO.Path]::GetTempPath()) 'eprocIA_bin'))) {
            Remove-Item -LiteralPath $dados -Recurse -Force -ErrorAction SilentlyContinue
        }
        Escrever 'Configurações e dados apagados.'
    } else {
        Escrever "Configurações mantidas em $(Join-Path $env:APPDATA $Nome)."
    }
    Escrever 'Pronto.' 'Green'
}

# Num terminal elevado, reexecuta este script sem privilégios como o usuário
# da sessão, por uma tarefa agendada de uso único, e repassa o log e o código.
function Invoke-ComoUsuario {
    $usuario = Get-UsuarioInterativo
    Escrever "Terminal elevado: a instalação vai rodar sem privilégios, como $usuario." 'Yellow'

    $id = [guid]::NewGuid().ToString('N')
    $palco = Join-Path $env:PUBLIC "eprocia-instalacao-$id"
    New-Item -ItemType Directory -Path $palco | Out-Null
    try {
        # O usuário pode não ter acesso ao perfil do administrador: tudo que a
        # tarefa lê ou escreve fica numa pasta pública com permissão para ele.
        & icacls.exe $palco /grant "${usuario}:(OI)(CI)M" | Out-Null
        if ($LASTEXITCODE -ne 0) { throw "icacls falhou ao dar acesso a $usuario." }
        $script = Join-Path $palco 'instalar.ps1'
        Copy-Item -LiteralPath $EsteScript -Destination $script
        $logUsuario = Join-Path $palco 'instalacao.log'

        $argumentos = @('-NoProfile', '-ExecutionPolicy', 'Bypass', '-WindowStyle', 'Hidden', '-File', "`"$script`"", '-Log', "`"$logUsuario`"")
        if ($Desinstalar) {
            $argumentos += '-Desinstalar'
            if ($RemoverDados) { $argumentos += '-RemoverDados' }
        } else {
            # Resolve o exe aqui: o gh e o -Origem podem só existir para o administrador.
            $exe = Resolve-Origem $palco
            $copia = Join-Path $palco $NomeExe
            if ($exe -ne $copia) { Copy-Item -LiteralPath $exe -Destination $copia -Force }
            $argumentos += @('-Origem', "`"$copia`"")
            if ($SemAtalhoNaAreaDeTrabalho) { $argumentos += '-SemAtalhoNaAreaDeTrabalho' }
            if ($NaoAbrir) { $argumentos += '-NaoAbrir' }
        }

        $tarefa = "eprocIA-instalacao-$id"
        $acao = New-ScheduledTaskAction -Execute 'powershell.exe' -Argument ($argumentos -join ' ')
        $principal = New-ScheduledTaskPrincipal -UserId $usuario -LogonType Interactive -RunLevel Limited
        $config = New-ScheduledTaskSettingsSet -AllowStartIfOnBatteries -DontStopIfGoingOnBatteries -ExecutionTimeLimit (New-TimeSpan -Minutes 10)
        Register-ScheduledTask -TaskName $tarefa -Action $acao -Principal $principal -Settings $config | Out-Null
        try {
            Start-ScheduledTask -TaskName $tarefa
            $limite = (Get-Date).AddMinutes(10)
            do {
                Start-Sleep -Milliseconds 500
                $info = Get-ScheduledTaskInfo -TaskName $tarefa
                # 267009 = em execução; 267011 = ainda não rodou.
                $terminou = ($info.LastTaskResult -ne 267009) -and ($info.LastTaskResult -ne 267011)
            } until ($terminou -or (Get-Date) -gt $limite)
            if (-not $terminou) { throw 'A instalação sem privilégios não terminou em 10 minutos.' }
        } finally {
            Unregister-ScheduledTask -TaskName $tarefa -Confirm:$false -ErrorAction SilentlyContinue
        }

        if (Test-Path -LiteralPath $logUsuario) {
            Get-Content -LiteralPath $logUsuario -Encoding UTF8 | ForEach-Object { Write-Host "  $_" }
        }
        return [int]$info.LastTaskResult
    } finally {
        Remove-Item -LiteralPath $palco -Recurse -Force -ErrorAction SilentlyContinue
    }
}

try {
    if (Test-Elevado) {
        exit (Invoke-ComoUsuario)
    }
    if ($Desinstalar) { Uninstall-Eprocia } else { Install-Eprocia }
    exit 0
} catch {
    Escrever "Erro: $($_.Exception.Message)" 'Red'
    exit 1
}
