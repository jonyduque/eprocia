<#
.SYNOPSIS
  Gera o ícone do eprocIA a partir do SVG mestre (skill eprocia-icone).

.DESCRIPTION
  Rasteriza o SVG com o ImageMagick 7 em cada tamanho e monta:
    - build/windows/icon.ico com 16, 24, 32, 48, 64 e 256 px (bandeja e executável);
    - build/appicon.png com 1024 px (usado pelo Wails).
  Se existir um SVG simplificado para tamanhos pequenos (build/icon-pequeno.svg),
  ele é usado em 16 e 24 px, onde detalhes finos viram borrão.

.EXAMPLE
  pwsh scripts/icone/gerar.ps1
  pwsh scripts/icone/gerar.ps1 -Svg $env:TEMP/teste.svg -Saida $env:TEMP/icone-teste
#>
param(
  [string]$Svg,
  [string]$SvgPequeno,
  [string]$Saida
)
$ErrorActionPreference = 'Stop'
$raiz = Resolve-Path (Join-Path $PSScriptRoot '..' '..')
if (-not $Svg) { $Svg = Join-Path $raiz 'build/icon.svg' }
if (-not $SvgPequeno) { $SvgPequeno = Join-Path $raiz 'build/icon-pequeno.svg' }
if (-not (Test-Path $Svg)) { throw "SVG mestre não encontrado: $Svg" }
if (-not (Get-Command magick -ErrorAction SilentlyContinue)) { throw 'ImageMagick 7 (magick) não está no PATH' }

if ($Saida) {
  New-Item -ItemType Directory -Force $Saida | Out-Null
  $ico = Join-Path $Saida 'icon.ico'
  $png = Join-Path $Saida 'appicon.png'
} else {
  $ico = Join-Path $raiz 'build/windows/icon.ico'
  $png = Join-Path $raiz 'build/appicon.png'
}

$tmp = Join-Path ([IO.Path]::GetTempPath()) ("eprocia-icone-" + [guid]::NewGuid())
New-Item -ItemType Directory $tmp | Out-Null
try {
  function Rasterizar($origem, $px, $destino) {
    # High density keeps edges crisp before downscaling; the background stays transparent.
    magick -background none -density 1536 $origem -resize "${px}x${px}" -gravity center -extent "${px}x${px}" "PNG32:$destino"
    if ($LASTEXITCODE -ne 0) { throw "falha ao rasterizar $origem em $px px" }
  }

  $camadas = @()
  foreach ($px in 256, 64, 48, 32, 24, 16) {
    $origem = if ($px -le 24 -and (Test-Path $SvgPequeno)) { $SvgPequeno } else { $Svg }
    $arq = Join-Path $tmp "$px.png"
    Rasterizar $origem $px $arq
    $camadas += $arq
  }
  magick @camadas $ico
  if ($LASTEXITCODE -ne 0) { throw 'falha ao montar o .ico' }
  Rasterizar $Svg 1024 $png

  Write-Host "[icone] $ico"
  magick identify $ico | ForEach-Object { Write-Host "  $_" }
  Write-Host "[icone] $png"
} finally {
  Remove-Item -Recurse -Force $tmp
}
