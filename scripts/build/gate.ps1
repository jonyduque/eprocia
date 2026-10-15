<#
.SYNOPSIS
  Gate de build do harness: vet, testes com -race, wails build e smoke test do exe.

.PARAMETER SemBuild
  Pula wails build e o smoke test (validação rápida).

.PARAMETER SemRace
  Roda os testes sem -race (o detector exige CGO e gcc no PATH).

.EXAMPLE
  pwsh scripts/build/gate.ps1
  pwsh scripts/build/gate.ps1 -SemBuild
#>
param(
  [switch]$SemBuild,
  [switch]$SemRace
)
$ErrorActionPreference = 'Stop'
$raiz = Resolve-Path (Join-Path $PSScriptRoot '..' '..')
Set-Location $raiz

function Etapa($nome, [scriptblock]$bloco) {
  Write-Host "[gate] $nome"
  $inicio = Get-Date
  & $bloco
  if ($LASTEXITCODE -ne 0) { Write-Host "[gate] FALHOU: $nome"; exit 1 }
  Write-Host ("[gate] ok: {0} ({1:n1}s)" -f $nome, ((Get-Date) - $inicio).TotalSeconds)
}

Etapa 'go vet ./...' { go vet ./... }

$race = -not $SemRace
if ($race -and -not (Get-Command gcc -ErrorAction SilentlyContinue)) {
  Write-Host '[gate] aviso: gcc não encontrado; testes sem -race'
  $race = $false
}
if ($race) {
  $env:CGO_ENABLED = '1'
  Etapa 'go test ./... -race -count=1' { go test ./... -race -count=1 }
} else {
  Etapa 'go test ./... -count=1' { go test ./... -count=1 }
}

if ($SemBuild) { Write-Host '[gate] build e smoke pulados (-SemBuild)'; exit 0 }

$exe = [IO.Path]::GetFullPath((Join-Path $raiz 'build\bin\eprocia.exe'))
$rodando = Get-Process eprocia -ErrorAction SilentlyContinue |
  Where-Object { $_.Path -and [string]::Equals([IO.Path]::GetFullPath($_.Path), $exe, [StringComparison]::OrdinalIgnoreCase) }
if ($rodando) {
  Write-Host "[gate] FALHOU: eprocia.exe (PID $($rodando.Id -join ', ')) está aberto e trava o build. Feche-o pela bandeja (Sair) e rode de novo."
  exit 1
}

Etapa 'wails build' { wails build }

Write-Host '[gate] smoke test: iniciando o exe por 5 s'
$proc = Start-Process -FilePath $exe -PassThru
Start-Sleep -Seconds 5
$vivo = -not $proc.HasExited
if ($vivo) {
  # Only the process this script started is stopped.
  Stop-Process -Id $proc.Id -Force
  Write-Host "[gate] ok: smoke (PID $($proc.Id) respondeu por 5 s e foi encerrado)"
} else {
  Write-Host "[gate] FALHOU: smoke (exe saiu com código $($proc.ExitCode))"
  exit 1
}
