param(
    [Parameter(Mandatory = $true)][string]$Artifact,
    [Parameter(Mandatory = $true)][string]$Version,
    [switch]$RequireSignature
)
$ErrorActionPreference = 'Stop'
$smokeRoot = Join-Path $env:RUNNER_TEMP ("compozyds." + [Guid]::NewGuid().ToString('N'))
$runtimeHome = Join-Path $smokeRoot 'runtime'
$installation = Join-Path $smokeRoot 'Compozy OS São Paulo'
$executable = Join-Path $installation 'CompozyOS.exe'
$runtime = Join-Path $runtimeHome 'bin/compozy.exe'
$previousHome = $env:COMPOZY_HOME
$desktopProcess = $null
New-Item -ItemType Directory -Path $runtimeHome -Force | Out-Null
$listener = [System.Net.Sockets.TcpListener]::new([System.Net.IPAddress]::Loopback, 0)
$listener.Start()
$port = $listener.LocalEndpoint.Port
$listener.Stop()
$config = "[app]`nupdate_check = false`n[http]`nhost = `"127.0.0.1`"`nport = $port`n"
Set-Content -Path (Join-Path $runtimeHome 'config.toml') -Value $config -Encoding utf8NoBOM
try {
    $env:COMPOZY_HOME = $runtimeHome
    if ($RequireSignature -and (Get-AuthenticodeSignature $Artifact).Status -ne 'Valid') {
        throw 'The release installer has no valid Authenticode signature.'
    }
    $installed = Start-Process -FilePath (Resolve-Path $Artifact) -ArgumentList "/S /D=$installation" -Wait -PassThru
    if ($installed.ExitCode -ne 0) { throw "Installer exited with $($installed.ExitCode)." }
    $desktopProcess = Start-Process -FilePath $executable -PassThru
    $deadline = [DateTime]::UtcNow.AddSeconds(90)
    $record = $null
    do {
        $recordPath = Join-Path $runtimeHome 'app.json'
        if (Test-Path $recordPath) {
            $record = Get-Content -Raw $recordPath | ConvertFrom-Json
            if ($record.state -eq 'error') { throw 'The packaged app failed to start; inspect desktop-bootstrap.jsonl.' }
            if ($record.state -eq 'product') { break }
        }
        Start-Sleep -Milliseconds 250
    } while ([DateTime]::UtcNow -lt $deadline)
    if (!$record -or $record.state -ne 'product' -or $record.app_version -ne $Version) {
        throw 'The packaged app did not reach the expected product version.'
    }
    if ($record.diagnostic_report.runtime_version.TrimStart('v') -ne $Version -or !$record.diagnostic_report.runtime_owned) {
        throw 'The packaged app did not provision its matching owned runtime.'
    }
    $desktopProcess = Get-Process -Id $record.pid
    if ($desktopProcess.Path -ne $executable) { throw 'The observed desktop is outside the smoke installation.' }
    $status = & $runtime app status -o json | ConvertFrom-Json
    if ($LASTEXITCODE -ne 0 -or !$status.running -or $status.state -ne 'product') { throw 'Desktop status failed.' }
    & $runtime app diagnose -o json
    if ($LASTEXITCODE -ne 0) { throw 'Named-pipe desktop diagnostics failed.' }
    & $runtime app open /settings/general
    if ($LASTEXITCODE -ne 0) { throw 'Named-pipe navigation failed.' }
    $response = Invoke-WebRequest -Uri "http://127.0.0.1:$port/" -TimeoutSec 10
    if ($response.StatusCode -ne 200 -or $response.Content -notmatch 'CompozyOS') { throw 'The product HTML was not served.' }
    Write-Output "Windows desktop ${Version}: installed, provisioned, rendered and controlled successfully."
} finally {
    try {
        if ($desktopProcess -and !$desktopProcess.HasExited -and $desktopProcess.Path -eq $executable) {
            $desktopProcess.CloseMainWindow() | Out-Null
            if (!$desktopProcess.WaitForExit(10000)) { $desktopProcess.Kill(); $desktopProcess.WaitForExit() }
        }
        if (Test-Path $runtime) {
            & $runtime daemon stop
            if ($LASTEXITCODE -ne 0) { throw 'The smoke daemon did not stop cleanly.' }
        }
        $uninstaller = Join-Path $installation 'Uninstall CompozyOS.exe'
        if (Test-Path $uninstaller) {
            $removed = Start-Process -FilePath $uninstaller -ArgumentList '/S' -Wait -PassThru
            if ($removed.ExitCode -ne 0) { throw 'The smoke installation did not uninstall cleanly.' }
        }
        if ((Get-Content -Raw (Join-Path $runtimeHome 'config.toml')).Trim() -ne $config.Trim()) {
            throw 'Uninstall changed the CompozyOS configuration.'
        }
    } finally {
        $env:COMPOZY_HOME = $previousHome
        Write-Output "Windows desktop smoke evidence: $smokeRoot"
    }
}
