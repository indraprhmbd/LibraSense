param(
    [string]$Base = 'http://127.0.0.1:18097'
)

# The base URL must reach the container from the workstation, normally through
#   ssh -N -L 18097:127.0.0.1:8097 vps-librasense
# The loopback path bypasses Cloudflare Access on purpose, so every check below
# is about the service itself, not about the Access policy at the edge.

$ErrorActionPreference = 'Stop'
$failed = 0

function Check {
    param(
        [string]$Name,
        [scriptblock]$Probe
    )
    try {
        $result = & $Probe
        if ($result) {
            Write-Host "[ok]   $Name"
        } else {
            Write-Host "[gagal] $Name"
            $script:failed++
        }
    } catch {
        Write-Host "[gagal] $Name : $($_.Exception.Message)"
        $script:failed++
    }
}

Check 'health' {
    $health = Invoke-RestMethod -Uri "$Base/health" -TimeoutSec 5
    $health.ok -eq $true
}

Check 'panel HTML' {
    $page = Invoke-WebRequest -Uri "$Base/" -UseBasicParsing -TimeoutSec 5
    $page.StatusCode -eq 200 -and $page.Content -match 'vendor/alpine.min.js'
}

Check 'app.js' {
    (Invoke-WebRequest -Uri "$Base/app.js" -UseBasicParsing -TimeoutSec 5).StatusCode -eq 200
}

Check 'app.css' {
    (Invoke-WebRequest -Uri "$Base/app.css" -UseBasicParsing -TimeoutSec 5).StatusCode -eq 200
}

Check 'alpine vendored' {
    (Invoke-WebRequest -Uri "$Base/vendor/alpine.min.js" -UseBasicParsing -TimeoutSec 5).StatusCode -eq 200
}

Check 'device tanpa token 401' {
    $status = 0
    try {
        Invoke-WebRequest -Uri "$Base/device/readings" -Method Post -ContentType 'application/json' -Body '{}' -UseBasicParsing -TimeoutSec 5 | Out-Null
        $status = 200
    } catch {
        $response = $_.Exception.Response
        if ($null -ne $response -and $null -ne $response.StatusCode) {
            $status = [int]$response.StatusCode
        }
    }
    $status -eq 401
}

if ($failed -gt 0) {
    Write-Host "[smoke] $failed cek gagal"
    exit 1
}

Write-Host '[smoke] semua cek lulus'
