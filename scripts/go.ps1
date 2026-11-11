$ErrorActionPreference = 'Stop'
$projectRoot = Split-Path $PSScriptRoot -Parent
$goExecutable = Join-Path $projectRoot '.tools\go\bin\go.exe'
if (!(Test-Path -LiteralPath $goExecutable)) {
    throw 'Go portátil não encontrado. Consulte README.md.'
}
$env:GOCACHE = Join-Path $projectRoot '.cache\go-build'
$env:GOMODCACHE = Join-Path $projectRoot '.cache\go-mod'
$env:GOTMPDIR = Join-Path $projectRoot '.cache\go-tmp'
New-Item -ItemType Directory -Force $env:GOTMPDIR | Out-Null
$env:GOTOOLCHAIN = 'local'
$env:GOPROXY = 'off'
& $goExecutable @args
exit $LASTEXITCODE
