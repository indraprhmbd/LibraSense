param(
    [switch]$SkipImage
)

$ErrorActionPreference = 'Stop'
$root = Split-Path -Parent $PSScriptRoot
$output = Join-Path $root 'dist/librasense'

Push-Location $root
try {
    New-Item -ItemType Directory -Force -Path (Join-Path $root 'dist') | Out-Null

    Write-Host '[build] cross compile linux/amd64'
    $env:CGO_ENABLED = '0'
    $env:GOOS = 'linux'
    $env:GOARCH = 'amd64'
    go build -trimpath -ldflags='-s -w' -o $output ./cmd/librasense
    if ($LASTEXITCODE -ne 0) { throw 'go build gagal' }

    $size = [math]::Round((Get-Item $output).Length / 1MB, 2)
    Write-Host "[build] binary $size MB"

    if (-not $SkipImage) {
        Write-Host '[build] image localhost/librasense:latest'
        podman build -f deploy/Containerfile -t localhost/librasense:latest .
        if ($LASTEXITCODE -ne 0) { throw 'podman build gagal' }
    }
} finally {
    Remove-Item Env:GOOS, Env:GOARCH, Env:CGO_ENABLED -ErrorAction SilentlyContinue
    Pop-Location
}
