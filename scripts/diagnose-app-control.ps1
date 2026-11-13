# Read-only: no registry, policy or protection settings are changed.
$ErrorActionPreference = 'Stop'
try {
    $events = Get-WinEvent -FilterHashtable @{
        LogName = 'Microsoft-Windows-CodeIntegrity/Operational'
        Id = 3077
        StartTime = (Get-Date).AddDays(-1)
    } -MaxEvents 200
    $relevant = $events | Where-Object {
        $_.Message -match '(service\.test\.exe|transfer\.test\.exe|rc2\.exe)'
    } | Select-Object -First 6
    if (!$relevant) {
        Write-Output 'Nenhum evento 3077 do projeto encontrado nas últimas 24 horas.'
    } else {
        $relevant | Select-Object TimeCreated, Id, Message | Format-List
    }
} catch {
    Write-Error "Não foi possível ler o log. Execute este diagnóstico em PowerShell como administrador. Detalhe: $($_.Exception.Message)"
}
