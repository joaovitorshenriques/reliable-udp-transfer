$ErrorActionPreference = 'Stop'
Set-Location (Split-Path $PSScriptRoot -Parent)
New-Item -ItemType Directory -Force data/server,results/lan | Out-Null
$path = Join-Path $PWD 'data/server/arquivo.bin'
if (-not (Test-Path -LiteralPath $path)) {
    $buffer = New-Object byte[] (1MB)
    for ($i = 0; $i -lt $buffer.Length; $i++) { $buffer[$i] = $i % 251 }
    $stream = [System.IO.File]::Open($path, [System.IO.FileMode]::CreateNew)
    try { for ($i = 0; $i -lt 80; $i++) { $stream.Write($buffer,0,$buffer.Length) } }
    finally { $stream.Dispose() }
}
if ((Get-Item -LiteralPath $path).Length -ne 80MB) { throw 'Existing file is not 80 MiB; preserved without overwriting.' }
Get-FileHash -LiteralPath $path -Algorithm SHA256
if (-not (Test-Path password.txt)) {
    $password = Read-Host 'Enter the SAME temporary test password on both computers'
    if ([string]::IsNullOrWhiteSpace($password)) { throw 'Password must not be empty.' }
    [System.IO.File]::WriteAllText((Join-Path $PWD 'password.txt'),$password,(New-Object System.Text.UTF8Encoding($false)))
}
Write-Host 'Prepared. Compare SHA256 on both machines. Existing password.txt was preserved.'
