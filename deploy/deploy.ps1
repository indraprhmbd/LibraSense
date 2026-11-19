param(
    [string]$Ssh = 'vps-librasense',
    [string]$EnvFile = '',
    [switch]$SkipBuild,
    [switch]$Remote
)

$ErrorActionPreference = 'Stop'
$root = Split-Path -Parent $PSScriptRoot
$unit = Join-Path $PSScriptRoot 'systemd/librasense.service'
$image = 'localhost/librasense:latest'
$health = 'http://127.0.0.1:8097/health'

function Assert-Exit {
    param([string]$What)
    if ($LASTEXITCODE -ne 0) { throw "gagal: $What" }
}

if (-not $SkipBuild) {
    if ($Remote) {
        & (Join-Path $PSScriptRoot 'build-remote.ps1') -Ssh $Ssh
    } else {
        & (Join-Path $PSScriptRoot 'build.ps1')
    }
    Assert-Exit 'build'
}

Write-Host "[deploy] cek akses ke $Ssh"
ssh -o BatchMode=yes $Ssh 'id -un'
Assert-Exit 'ssh'

if ($EnvFile -ne '') {
    if (-not (Test-Path $EnvFile)) { throw "env file tidak ada: $EnvFile" }
    Write-Host '[deploy] unggah env, hanya karena diminta eksplisit'
    scp $EnvFile "${Ssh}:/home/librasense/.env"
    Assert-Exit 'scp env'
    ssh -o BatchMode=yes $Ssh 'chmod 600 /home/librasense/.env'
    Assert-Exit 'chmod env'
}

ssh -o BatchMode=yes $Ssh 'test -f /home/librasense/.env'
if ($LASTEXITCODE -ne 0) {
    throw 'env belum ada. Salin deploy/.env.example ke /home/librasense/.env lalu ulangi dengan -EnvFile.'
}

if (-not $Remote) {
    Write-Host '[deploy] simpan image lama sebagai :prev'
    ssh -o BatchMode=yes $Ssh "sudo podman tag $image localhost/librasense:prev"
    if ($LASTEXITCODE -ne 0) {
        Write-Host '[deploy] belum ada image sebelumnya, ini deploy pertama'
    }

    $tar = Join-Path $env:TEMP 'librasense-image.tar'
    Write-Host '[deploy] simpan image'
    podman save -o $tar $image
    Assert-Exit 'podman save'

    Write-Host '[deploy] unggah image'
    scp $tar "${Ssh}:/tmp/librasense-image.tar"
    Assert-Exit 'scp image'
    Remove-Item -Force $tar -ErrorAction SilentlyContinue

    ssh -o BatchMode=yes $Ssh 'sudo podman load -i /tmp/librasense-image.tar'
    Assert-Exit 'podman load'
    ssh -o BatchMode=yes $Ssh 'rm -f /tmp/librasense-image.tar'
    Assert-Exit 'hapus tar'
} else {
    Write-Host '[deploy] image sudah dibangun di host, lewati transfer'
}

Write-Host '[deploy] pasang unit dan mulai ulang'
scp $unit "${Ssh}:/tmp/librasense.service"
Assert-Exit 'scp unit'
ssh -o BatchMode=yes $Ssh 'sudo install -m 0644 /tmp/librasense.service /etc/systemd/system/librasense.service'
Assert-Exit 'install unit'
ssh -o BatchMode=yes $Ssh 'rm -f /tmp/librasense.service'
ssh -o BatchMode=yes $Ssh 'sudo systemctl daemon-reload'
Assert-Exit 'daemon-reload'
ssh -o BatchMode=yes $Ssh 'sudo systemctl enable librasense.service'
Assert-Exit 'enable'
ssh -o BatchMode=yes $Ssh 'sudo systemctl restart librasense.service'
Assert-Exit 'restart'

Write-Host '[deploy] tunggu health'
$probe = $null
for ($i = 0; $i -lt 20; $i++) {
    Start-Sleep -Seconds 1
    $probe = ssh -o BatchMode=yes $Ssh "curl -fsS $health" 2>$null
    if ($LASTEXITCODE -eq 0 -and $probe -match '"ok"\s*:\s*true') {
        break
    }
    $probe = $null
}

if ($null -eq $probe) {
    ssh -o BatchMode=yes $Ssh 'sudo journalctl -u librasense.service -n 100 --no-pager'
    throw 'health gagal, lihat log di atas'
}

Write-Host "[deploy] sehat: $probe"
