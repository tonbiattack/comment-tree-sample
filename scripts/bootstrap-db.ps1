param(
    [switch]$Recreate
)

$ErrorActionPreference = "Stop"

function Get-EnvMap {
    param([string]$Path)

    $envMap = @{}
    Get-Content $Path | ForEach-Object {
        if ($_ -match "^\s*#" -or $_ -match "^\s*$") {
            return
        }
        $parts = $_ -split "=", 2
        if ($parts.Count -eq 2) {
            $envMap[$parts[0]] = $parts[1]
        }
    }
    return $envMap
}

function Set-EnvValue {
    param(
        [string]$Path,
        [string]$Key,
        [string]$Value
    )

    $lines = Get-Content $Path
    $updated = $false
    for ($i = 0; $i -lt $lines.Count; $i++) {
        if ($lines[$i] -match "^$Key=") {
            $lines[$i] = "$Key=$Value"
            $updated = $true
        }
    }

    if (-not $updated) {
        $lines += "$Key=$Value"
    }

    Set-Content -Path $Path -Value $lines
}

function Test-PortInUse {
    param([int]$Port)

    $listeners = [System.Net.NetworkInformation.IPGlobalProperties]::GetIPGlobalProperties().GetActiveTcpListeners()
    return $listeners.Port -contains $Port
}

function Find-FreePort {
    param([int[]]$Candidates)

    foreach ($candidate in $Candidates) {
        if (-not (Test-PortInUse -Port $candidate)) {
            return $candidate
        }
    }

    throw "No free port found in candidate range."
}

$repoRoot = Resolve-Path (Join-Path $PSScriptRoot "..")
Set-Location $repoRoot

if (-not (Test-Path ".env") -and (Test-Path ".env.example")) {
    Copy-Item ".env.example" ".env"
    Write-Host "Created .env from .env.example"
}

$envMap = Get-EnvMap -Path ".env"
$hostPort = $envMap["MYSQL_HOST_PORT"]
if (-not $hostPort) {
    $hostPort = "33306"
}

$hostPortNumber = [int]$hostPort
if (Test-PortInUse -Port $hostPortNumber) {
    $candidatePorts = @($hostPortNumber) + (33306..33320 | Where-Object { $_ -ne $hostPortNumber })
    $freePort = Find-FreePort -Candidates $candidatePorts
    Set-EnvValue -Path ".env" -Key "MYSQL_HOST_PORT" -Value "$freePort"
    $hostPortNumber = $freePort
    Write-Host "Updated MYSQL_HOST_PORT to $freePort because port $hostPort was already in use."
}

if ($Recreate) {
    docker compose down -v
}

docker compose up -d mysql

$containerId = docker compose ps -q mysql
if (-not $containerId) {
    throw "MySQL container ID could not be resolved."
}

$status = ""
for ($i = 0; $i -lt 30; $i++) {
    $status = docker inspect --format "{{if .State.Health}}{{.State.Health.Status}}{{else}}{{.State.Status}}{{end}}" $containerId
    if ($status -eq "healthy" -or $status -eq "running") {
        break
    }
    Start-Sleep -Seconds 2
}

if ($status -ne "healthy" -and $status -ne "running") {
    throw "MySQL container did not become healthy."
}

Write-Host "MySQL is ready on localhost:$hostPortNumber"
