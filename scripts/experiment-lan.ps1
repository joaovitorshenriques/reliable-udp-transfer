param([string]$Desktop = '192.168.2.128', [string]$Notebook = '192.168.2.150', [string]$Resume = '')
$ErrorActionPreference = 'Stop'
Set-Location (Split-Path $PSScriptRoot -Parent)
$source = Get-Item data/server/arquivo.bin
if ($source.Length -lt 80MB) { throw 'Require source >=80 MiB.' }
$expected = (Get-FileHash $source.FullName -Algorithm SHA256).Hash
$out = if ($Resume) { $Resume } else { Join-Path 'results/lan' ('campaign-' + [guid]::NewGuid().ToString('N')) }
New-Item -ItemType Directory -Path $out -Force | Out-Null
$exe = Join-Path $out 'rc2.exe'
if ($Resume) {
    $saved = Get-Content (Join-Path $out 'manifest.json') -Raw | ConvertFrom-Json
    if ($saved.sha256 -ne $expected -or $saved.desktop -ne $Desktop -or $saved.notebook -ne $Notebook -or -not (Test-Path $exe)) { throw 'Resume manifest/file/endpoints mismatch or missing original executable.' }
} else {
& go build -o $exe ./cmd/rc2
if ($LASTEXITCODE -ne 0) { throw 'Build failed.' }
$commit = & git rev-parse HEAD
@{desktop=$Desktop;notebook=$Notebook;client=$env:COMPUTERNAME;commit="$commit";goVersion=(& go version);bytes=$source.Length;sha256=$expected;repetitions=5;window=64;startedUTC=[DateTime]::UtcNow.ToString('o')} | ConvertTo-Json | Set-Content (Join-Path $out 'manifest.json') -Encoding UTF8
}
Write-Host "Results: $out"
$modes = @('desktop','notebook','cdn')
for ($rep=1; $rep -le 5; $rep++) {
    $waited = $false
    for ($offset=0; $offset -lt 3; $offset++) {
        $mode = $modes[($rep-1+$offset)%3]
        $prefix = Join-Path $out "$mode-$rep"
        $destination = "$prefix.bin"
        if (Test-Path $destination) {
            if ((Get-FileHash $destination -Algorithm SHA256).Hash -ne $expected) { throw "Existing output has wrong hash: $destination" }
            Write-Host "Already verified: $mode repetition $rep"
            continue
        }
        if (-not $waited) {
            Write-Host 'Waiting 125 seconds for previous endpoint leases to expire (default server settings).'
            for ($tick=0; $tick -lt 25; $tick++) { Start-Sleep -Seconds 5 }
            $waited = $true
        }
        # Preserve logs from unsuccessful attempts before retrying the same cell.
        $attempt = [guid]::NewGuid().ToString('N')
        foreach ($suffix in @('-stdout.json','-stderr.txt','-metrics.jsonl','-selection.json')) {
            $old = "$prefix$suffix"
            if (Test-Path -LiteralPath $old) { Move-Item -LiteralPath $old -Destination "$prefix-attempt-$attempt$suffix" }
        }
        "$( [DateTime]::UtcNow.ToString('o') ) repetition=$rep mode=$mode wait_per_round_seconds=125" | Add-Content (Join-Path $out 'execution.log')
        $common = @('-password-file','password.txt','-name','arquivo.bin','-file',$destination,'-window','64')
        Write-Host "Repetition $rep/5: $mode"
        if ($mode -eq 'cdn') {
            $cli = @('cdn','-servers',"${Desktop}:9000,${Notebook}:9000",'-samples','5','-selection',"$prefix-selection.json") + $common
        } else {
            $address = if ($mode -eq 'desktop') { $Desktop } else { $Notebook }
            $cli = @('download','-addr',"${address}:9000",'-metrics',"$prefix-metrics.jsonl") + $common
        }
        # Windows PowerShell turns native stderr into an ErrorRecord under Stop.
        $ErrorActionPreference = 'Continue'
        try { & $exe @cli 1> "$prefix-stdout.json" 2> "$prefix-stderr.txt"; $exitCode = $LASTEXITCODE }
        finally { $ErrorActionPreference = 'Stop' }
        if ($exitCode -ne 0) { throw "Failed: $mode repetition $rep. Inspect $prefix-stderr.txt. Partial results preserved." }
        if ((Get-FileHash $destination -Algorithm SHA256).Hash -ne $expected) { throw "Hash mismatch: $destination" }
        Write-Host 'Hash OK'
    }
}
'All 15 transfers completed with matching SHA256.' | Set-Content (Join-Path $out 'COMPLETE') -Encoding UTF8
Write-Host "Completed: $out"
