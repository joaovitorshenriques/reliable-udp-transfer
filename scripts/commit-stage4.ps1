$ErrorActionPreference = 'Stop'
Set-Location (Split-Path $PSScriptRoot -Parent)
$staged = git diff --cached --name-only
if ($LASTEXITCODE -ne 0 -or $staged) { throw 'Index must be empty before starting.' }
function Commit-Group($message, $paths) {
    & git add -- @paths
    if ($LASTEXITCODE -ne 0) { throw 'git add failed' }
    & git commit -m $message
    if ($LASTEXITCODE -ne 0) { throw 'git commit failed; inspect status before continuing' }
}
Commit-Group ':sparkles: feat(netem): emular atraso e perda UDP com contadores' @('internal/netem')
Commit-Group ':sparkles: feat(experiment): executar quatro cenarios controlados' @('cmd/experiment/main.go','cmd/experiment/scenarios.go','cmd/experiment/scenarios_test.go','cmd/validate/main.go')
Commit-Group ':sparkles: feat(analysis): comparar janelas e inspecionar capturas RC2' @('scripts/analyze-stage4.py','scripts/compare-stage4.py','scripts/plot-stage4.py','scripts/inspect-capture.py','scripts/rc2.lua','scripts/continue-stage4.ps1','.gitignore')
Commit-Group ':memo: docs: consolidar resultados e validacao da etapa 4' @('README.md','docs/PLANO.md','docs/VALIDACAO.md','docs/ETAPA4.md','docs/VERIFICAR-ETAPA4.md','docs/CAPTURA-S3.md','docs/capture-s3','docs/comparacao-etapa4','docs/results-etapa4-w64','scripts/commit-stage4.ps1')
git log -4 --oneline
git status --short
