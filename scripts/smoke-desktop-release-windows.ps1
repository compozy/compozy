param(
    [Parameter(Mandatory = $true)][string]$Artifact,
    [Parameter(Mandatory = $true)][string]$Version,
    [switch]$RequireSignature
)
$ErrorActionPreference = 'Stop'
$smokeGate = $env:COMPOZY_RELEASE_SMOKE_GATE -eq '1'
$smokeToken = $env:COMPOZY_RELEASE_SMOKE_ACCOUNT_TOKEN
$env:COMPOZY_RELEASE_SMOKE_ACCOUNT_TOKEN = $null
$env:COMPOZY_ACCOUNT_TOKEN = $null
$env:COMPOZY_INTERNAL_ACCOUNT_TOKEN = $null
if ($smokeGate -and [string]::IsNullOrWhiteSpace($smokeToken)) {
    throw 'Set COMPOZY_RELEASE_SMOKE_ACCOUNT_TOKEN for the gated desktop smoke.'
}

function Get-SmokePort {
    $listener = [System.Net.Sockets.TcpListener]::new([System.Net.IPAddress]::Loopback, 0)
    $listener.Start()
    try {
        $listener.LocalEndpoint.Port
    } finally {
        $listener.Stop()
    }
}

function Test-SignedOutRuntime($RuntimePath, $SignedOutHome) {
    $port = Get-SmokePort
    $url = "http://127.0.0.1:$port/api"
    New-Item -ItemType Directory -Path $SignedOutHome | Out-Null
    Set-Content -Path (Join-Path $SignedOutHome 'config.toml') -Encoding utf8NoBOM `
        -Value "[http]`nhost = `"127.0.0.1`"`nport = $port`n"
    $previousHome = $env:COMPOZY_HOME
    try {
        $env:COMPOZY_HOME = $SignedOutHome
        & $RuntimePath daemon start | Out-Null
        if ($LASTEXITCODE -ne 0) { throw 'The signed-out smoke daemon failed to start.' }

        $account = Invoke-RestMethod -Uri "$url/account" -TimeoutSec 5
        $workspaces = Invoke-WebRequest -Uri "$url/workspaces" -TimeoutSec 5 -SkipHttpErrorCheck
        if ($account.gate -eq 'grace' -and $workspaces.StatusCode -eq 401) {
            # Grace can expire between requests.
            $account = Invoke-RestMethod -Uri "$url/account" -TimeoutSec 5
        }
        if ($account.state -ne 'signed_out' -or $account.signed_in -ne $false) {
            throw 'The smoke daemon did not report a signed-out account.'
        }
        if ($account.gate -eq 'grace' -and $workspaces.StatusCode -eq 200) { return }
        if ($account.gate -eq 'enforced' -and $workspaces.StatusCode -eq 401 -and
            ($workspaces.Content | ConvertFrom-Json).code -eq 'sign_in_required') { return }
        throw "The signed-out gate '$($account.gate)' returned workspaces HTTP $($workspaces.StatusCode)."
    } finally {
        try {
            if (Test-Path (Join-Path $SignedOutHome 'daemon.json')) {
                & $RuntimePath daemon stop | Out-Null
                if ($LASTEXITCODE -ne 0) { throw 'The signed-out smoke daemon failed to stop.' }
            }
        } finally {
            $env:COMPOZY_HOME = $previousHome
        }
    }
}

$smokeRoot = Join-Path $env:RUNNER_TEMP ("compozyds." + [Guid]::NewGuid().ToString('N'))
$runtimeHome = Join-Path $smokeRoot 'runtime'
$installation = Join-Path $smokeRoot 'Compozy OS São Paulo'
$executable = Join-Path $installation 'CompozyOS.exe'
$runtime = Join-Path $runtimeHome 'bin/compozy.exe'
$previousHome = $env:COMPOZY_HOME
$desktopProcess = $null
New-Item -ItemType Directory -Path $runtimeHome -Force | Out-Null
$port = Get-SmokePort
$config = "[app]`nupdate_check = false`n[http]`nhost = `"127.0.0.1`"`nport = $port`n"
Set-Content -Path (Join-Path $runtimeHome 'config.toml') -Value $config -Encoding utf8NoBOM
try {
    $env:COMPOZY_HOME = $runtimeHome
    if ($RequireSignature -and (Get-AuthenticodeSignature $Artifact).Status -ne 'Valid') {
        throw 'The release installer has no valid Authenticode signature.'
    }
    $installed = Start-Process -FilePath (Resolve-Path $Artifact) -ArgumentList "/S /D=$installation" -Wait -PassThru
    if ($installed.ExitCode -ne 0) { throw "Installer exited with $($installed.ExitCode)." }
    try {
        if ($smokeGate) { $env:COMPOZY_ACCOUNT_TOKEN = $smokeToken }
        $desktopProcess = Start-Process -FilePath $executable -PassThru
    } finally {
        $env:COMPOZY_ACCOUNT_TOKEN = $null
    }
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
    if ($smokeGate) {
        $account = Invoke-RestMethod -Uri "http://127.0.0.1:$port/api/account" -TimeoutSec 5
        if ($account.gate -ne 'enforced' -or $account.state -ne 'signed_in' -or $account.source -ne 'token') {
            throw 'The packaged app did not sign in with the smoke account token.'
        }
        Invoke-WebRequest -Uri "http://127.0.0.1:$port/api/workspaces" -TimeoutSec 5 | Out-Null
    }
    $status = & $runtime app status -o json | ConvertFrom-Json
    if ($LASTEXITCODE -ne 0 -or !$status.running -or $status.state -ne 'product') { throw 'Desktop status failed.' }
    & $runtime app diagnose -o json
    if ($LASTEXITCODE -ne 0) { throw 'Named-pipe desktop diagnostics failed.' }
    & $runtime app open /settings/general
    if ($LASTEXITCODE -ne 0) { throw 'Named-pipe navigation failed.' }
    $response = Invoke-WebRequest -Uri "http://127.0.0.1:$port/" -TimeoutSec 10
    if ($response.StatusCode -ne 200 -or $response.Content -notmatch 'CompozyOS') { throw 'The product HTML was not served.' }
    if ($smokeGate) { Test-SignedOutRuntime $runtime (Join-Path $smokeRoot 'signed-out') }
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
            $deadline = [DateTime]::UtcNow.AddSeconds(60)
            while ((Test-Path $executable) -and [DateTime]::UtcNow -lt $deadline) {
                Start-Sleep -Milliseconds 250
            }
            if (Test-Path $executable) { throw 'The smoke installation left CompozyOS.exe after uninstall.' }
        }
        if ((Get-Content -Raw (Join-Path $runtimeHome 'config.toml')).Trim() -ne $config.Trim()) {
            throw 'Uninstall changed the CompozyOS configuration.'
        }
    } finally {
        $env:COMPOZY_HOME = $previousHome
        Write-Output "Windows desktop smoke evidence: $smokeRoot"
    }
}
