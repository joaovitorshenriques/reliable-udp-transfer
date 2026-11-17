$ErrorActionPreference = 'Stop'
Set-Location (Split-Path $PSScriptRoot -Parent)
$staged = git diff --cached --name-only
if ($LASTEXITCODE -ne 0 -or $staged) { throw 'Start with an empty index.' }
function Commit-Group($message, $paths) {
    & git add -- @paths
    if ($LASTEXITCODE -ne 0) { throw 'git add failed' }
    & git commit -m $message
    if ($LASTEXITCODE -ne 0) { throw 'Commit failed; inspect git status before proceeding.' }
}
Commit-Group ':sparkles: feat(service): adicionar sondagens autenticadas' @('internal/service/service.go','internal/service/probe.go','internal/service/probe_test.go')
Commit-Group ':sparkles: feat(cdn): selecionar replica por RTT e baixar arquivo' @('internal/cdn','cmd/rc2/main.go','cmd/rc2/cdn.go')
Commit-Group ':memo: docs: preparar teste da mini-CDN na rede local' @('docs/COMMITS.md','docs/ETAPA5.md','scripts/commit-stage5.ps1','scripts/prepare-lan.ps1')
git log -3 --oneline
git status --short
