param([string]$Campaign = 'results/etapa4-774158857')
$ErrorActionPreference = 'Stop'
Set-Location (Split-Path $PSScriptRoot -Parent)
$manifest = Get-Content (Join-Path $Campaign 'manifest.json') -Raw | ConvertFrom-Json
if ($manifest.Repetitions -ne 5 -or $manifest.MiB -ne 80 -or $manifest.Groups.Count -ne 4 -or @($manifest.Groups | Where-Object Window -ne 16).Count -ne 0) {
    throw 'Expected a complete four-scenario plan: window 16, five repeats, 80 MiB.'
}
$marker = Join-Path $Campaign 'window256-started.lock'
Write-Host "Waiting for COMPLETE in $Campaign. Keep this terminal open; Ctrl+C cancels this watcher only."
$deadline = (Get-Date).AddHours(48)
while (-not (Test-Path (Join-Path $Campaign 'COMPLETE'))) {
    if ((Get-Date) -gt $deadline) { throw '48-hour wait expired; window 256 was not started.' }
    Start-Sleep -Seconds 30
}
$rows = @(Import-Csv (Join-Path $Campaign 'metrics.csv'))
if ($rows.Count -ne 80 -or @($rows | Where-Object success -ne 'true').Count -ne 0) {
    throw 'Window 16 metrics incomplete or failed; window 256 was not started.'
}
# Exclusive marker prevents two watchers from launching duplicate campaigns.
$lock = [System.IO.File]::Open((Join-Path $PWD $marker), [System.IO.FileMode]::CreateNew, [System.IO.FileAccess]::Write, [System.IO.FileShare]::None)
$lock.Dispose()
Write-Host 'Window 16 completed successfully. Starting window 256.'
& go run ./cmd/experiment -stage4 -repeats 5 -mib 80 -window 256
if ($LASTEXITCODE -ne 0) { throw 'Window 256 failed. Inspect the output; no automatic retry will be performed.' }
Write-Host 'Window 256 completed. Preserve the results directory printed above.'
