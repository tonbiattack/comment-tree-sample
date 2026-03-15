param(
    [switch]$Recreate
)

$ErrorActionPreference = "Stop"

$repoRoot = Resolve-Path (Join-Path $PSScriptRoot "..")
Set-Location $repoRoot

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

$portOutput = docker compose port mysql 3306
if (-not $portOutput) {
    throw "MySQL published port could not be resolved."
}

$publishedPort = ($portOutput.Trim() -split ":")[-1]
Write-Host "MySQL is ready on localhost:$publishedPort"
