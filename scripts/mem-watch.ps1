# Watch process working set (RSS) in GB. Usage: .\scripts\mem-watch.ps1 -PID <pid>

param(
    [Parameter(Mandatory = $true)]
    [int]$PID
)

while ($true) {
    Clear-Host
    try {
        $p = Get-Process -Id $PID -ErrorAction Stop
        $rssGB = [math]::Round($p.WorkingSet64 / 1GB, 2)
        Write-Host "PID    ProcessName    WorkingSet64    rss_GB"
        Write-Host "$($p.Id)    $($p.ProcessName)    $($p.WorkingSet64)    $rssGB"
    } catch {
        Write-Host "Process $PID not found or exited."
    }
    Start-Sleep -Seconds 1
}
