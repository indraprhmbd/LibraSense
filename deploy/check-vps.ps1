param(
    [string]$Bootstrap = 'vps-lama'
)

$ErrorActionPreference = 'Stop'
$script = Join-Path $PSScriptRoot 'check-vps.sh'
$remote = '/tmp/librasense-check.sh'

if (-not (Test-Path $script)) { throw "script tidak ada: $script" }

scp $script "${Bootstrap}:$remote"
if ($LASTEXITCODE -ne 0) { throw 'scp check-vps gagal' }

ssh -o BatchMode=yes $Bootstrap "bash $remote"
$code = $LASTEXITCODE

ssh -o BatchMode=yes $Bootstrap "rm -f $remote" | Out-Null

if ($code -ne 0) { throw "preflight gagal dengan kode $code" }
