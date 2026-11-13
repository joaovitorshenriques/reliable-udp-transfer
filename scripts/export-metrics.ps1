param(
    [Parameter(Mandatory = $true)][string[]]$InputPaths,
    [Parameter(Mandatory = $true)][string]$OutputPath
)
$ErrorActionPreference = 'Stop'
$rows = foreach ($inputPath in $InputPaths) {
    foreach ($line in Get-Content -LiteralPath $inputPath) {
        if ([string]::IsNullOrWhiteSpace($line)) { continue }
        $record = $line | ConvertFrom-Json
        $m = $record.metrics
        [pscustomobject]@{
            timestamp = $record.timestamp
            endpoint = $record.endpoint
            operation = $record.operation
            name = $record.name
            success = $record.success
            error = $record.error
            session = $m.session
            role = $m.role
            bytes = $m.bytes
            seconds = $m.seconds
            goodput_mbps = $m.goodput_mbps
            data_retransmissions = $m.data_retransmissions
            rtt_samples = $m.rtt_samples
            mean_data_rtt_ms = $m.mean_data_rtt_ms
            adaptive_rto = $m.adaptive_rto
            data_rto_ms = $m.data_rto_ms
            max_packet_rto_ms = $m.max_packet_rto_ms
            window_packets = $m.window_packets
            transport_bytes_sent = $m.transport_bytes_sent
            transport_bytes_received = $m.transport_bytes_received
            sha256 = $m.sha256
        }
    }
}
$rows | Export-Csv -LiteralPath $OutputPath -NoTypeInformation -Encoding UTF8
