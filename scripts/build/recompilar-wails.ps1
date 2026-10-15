<#
.SYNOPSIS
  Passo 0 do toolchain: recompila o Wails CLI v2.12.0 com golang.org/x/tools atual.

.DESCRIPTION
  O Wails CLI v2.12.0 publicado foi compilado com x/tools v0.30.0, que falha com
  Go 1.27 ("package "bytes" without types was imported"). Este script copia o
  módulo do cache para um diretório temporário, atualiza x/tools e instala o CLI
  em $(go env GOPATH)\bin. Não altera o go.mod do projeto.

.EXAMPLE
  pwsh scripts/build/recompilar-wails.ps1
#>
param(
  [string]$Versao = 'v2.12.0'
)
$ErrorActionPreference = 'Stop'

function Passo($msg) { Write-Host "[recompilar-wails] $msg" }

$cache = (go env GOMODCACHE).Trim()
$origem = Join-Path $cache "github.com/wailsapp/wails/v2@$Versao"
if (-not (Test-Path $origem)) {
  Passo "módulo ausente do cache; baixando github.com/wailsapp/wails/v2@$Versao"
  go mod download "github.com/wailsapp/wails/v2@$Versao"
  if ($LASTEXITCODE -ne 0) { throw 'falha ao baixar o módulo do Wails' }
}

$destino = Join-Path ([IO.Path]::GetTempPath()) "wails-cli-$Versao"
if (Test-Path $destino) { Remove-Item -Recurse -Force $destino }
Passo "copiando $origem -> $destino"
Copy-Item -Recurse $origem $destino
# Files in the module cache are read-only.
Get-ChildItem -Recurse -File $destino | ForEach-Object { $_.IsReadOnly = $false }

Push-Location $destino
try {
  Passo 'atualizando golang.org/x/tools'
  go get golang.org/x/tools@latest
  if ($LASTEXITCODE -ne 0) { throw 'go get golang.org/x/tools falhou' }
  go mod tidy
  if ($LASTEXITCODE -ne 0) { throw 'go mod tidy falhou' }
  Passo 'instalando ./cmd/wails'
  go install ./cmd/wails
  if ($LASTEXITCODE -ne 0) { throw 'go install ./cmd/wails falhou' }
} finally {
  Pop-Location
}

$exe = Join-Path (go env GOPATH).Trim() 'bin/wails.exe'
$tools = (go version -m $exe | Select-String 'golang.org/x/tools\s+(v\S+)').Matches.Groups[1].Value
Passo "instalado: $exe (x/tools $tools, $(go version))"
Remove-Item -Recurse -Force $destino
