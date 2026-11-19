param(
    [string]$Ssh = 'vps-librasense'
)

# Fallback build path. Only used when the workstation has no container runtime
# (no podman machine, no WSL, no Docker). The Go binary is still cross compiled
# on the workstation; the VPS only assembles the image from that binary. No
# `go build` runs on the host.

$ErrorActionPreference = 'Stop'
$root = Split-Path -Parent $PSScriptRoot
$image = 'localhost/librasense:latest'
$remote = '/tmp/librasense-build'

function Assert-Exit {
    param([string]$What)
    if ($LASTEXITCODE -ne 0) { throw "gagal: $What" }
}

& (Join-Path $PSScriptRoot 'build.ps1') -SkipImage
Assert-Exit 'build'

$stage = Join-Path $env:TEMP 'librasense-build'
if (Test-Path $stage) { Remove-Item -Recurse -Force $stage }
New-Item -ItemType Directory -Force -Path (Join-Path $stage 'dist') | Out-Null
Copy-Item (Join-Path $PSScriptRoot 'Containerfile') (Join-Path $stage 'Containerfile')
Copy-Item (Join-Path $root 'dist/librasense') (Join-Path $stage 'dist/librasense')

Write-Host "[build-remote] cek akses ke $Ssh"
ssh -o BatchMode=yes $Ssh 'id -un'
Assert-Exit 'ssh'

Write-Host "[build-remote] simpan image lama sebagai :prev"
ssh -o BatchMode=yes $Ssh "sudo podman tag $image localhost/librasense:prev 2>/dev/null || echo 'belum ada image sebelumnya'"

Write-Host "[build-remote] unggah konteks build"
ssh -o BatchMode=yes $Ssh "rm -rf $remote"
Assert-Exit 'bersihkan konteks lama'
scp -r -q $stage "${Ssh}:$remote"
Assert-Exit 'scp konteks'

Write-Host "[build-remote] podman build di host"
ssh -o BatchMode=yes $Ssh "sudo podman build -f $remote/Containerfile -t $image $remote"
Assert-Exit 'podman build'
ssh -o BatchMode=yes $Ssh "rm -rf $remote"

Write-Host "[build-remote] image $image siap di host"
