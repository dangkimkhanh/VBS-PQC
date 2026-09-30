param(
    [string]$Version = "",
    [string]$Image = "kmasc-chaincode",
    [switch]$SkipDocker
)

$ErrorActionPreference = "Stop"
$moduleRoot = (Resolve-Path (Join-Path $PSScriptRoot "..")).Path

if ([string]::IsNullOrWhiteSpace($Version)) {
    $Version = (Get-Content -LiteralPath (Join-Path $moduleRoot "VERSION") -Raw).Trim()
}
if ($Version -notmatch '^\d+\.\d+\.\d+([-.][0-9A-Za-z.-]+)?$') {
    throw "Version '$Version' is invalid. Use a version such as 1.0.1."
}

function Invoke-Checked {
    param([string]$Command, [string[]]$Arguments)
    & $Command @Arguments
    if ($LASTEXITCODE -ne 0) {
        throw "Command failed ($LASTEXITCODE): $Command $($Arguments -join ' ')"
    }
}

Push-Location $moduleRoot
try {
    $goFiles = Get-ChildItem -LiteralPath "contractapi" -Filter "*.go" | ForEach-Object { $_.FullName }
    Invoke-Checked "gofmt" (@("-w") + $goFiles)
    Invoke-Checked "go" @("mod", "tidy")
    Invoke-Checked "go" @("mod", "verify")
    Invoke-Checked "go" @("vet", "./...")
    Invoke-Checked "go" @("build", "./...")
    Invoke-Checked "go" @("test", "./...")

    New-Item -ItemType Directory -Force -Path "dist" | Out-Null
    $oldCgo = $env:CGO_ENABLED
    $oldGoos = $env:GOOS
    $oldGoarch = $env:GOARCH
    try {
        $env:CGO_ENABLED = "0"
        $env:GOOS = "linux"
        $env:GOARCH = "amd64"
        Invoke-Checked "go" @("build", "-mod=vendor", "-buildvcs=false", "-trimpath", "-ldflags=-s -w", "-o", "dist/kmasc-chaincode", "./contractapi")
    }
    finally {
        $env:CGO_ENABLED = $oldCgo
        $env:GOOS = $oldGoos
        $env:GOARCH = $oldGoarch
    }

    if (-not $SkipDocker) {
        Invoke-Checked "docker" @("build", "--build-arg", "CHAINCODE_VERSION=$Version", "-t", "${Image}:$Version", ".")
    }

    Write-Host "Build completed: dist/kmasc-chaincode"
    if (-not $SkipDocker) {
        Write-Host "Docker image: ${Image}:$Version"
    }
}
finally {
    Pop-Location
}
