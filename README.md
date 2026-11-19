# LibraSense

Pemantauan suhu, kelembapan, dan kebisingan tiga rak perpustakaan. ESP32 membaca sensor dan menjalankan aktuator. Semua keputusan ambang, kipas, dan buzzer dihitung server Go.

## Isi

| Path | Keterangan |
|---|---|
| `cmd/librasense` | entrypoint server |
| `internal/config` | parsing env, validasi, gagal tertutup |
| `internal/store` | SQLite, schema, query |
| `internal/settings` | tipe pengaturan, default, validasi |
| `internal/decision` | evaluasi kipas, buzzer, status |
| `internal/httpapi` | route, auth, handler, test |
| `web` | panel Alpine, di-embed ke binary |
| `esp32/librasense_node` | firmware kirim data |
| `deploy` | Containerfile, unit systemd, script deploy |
| `docs` | catatan teknis dan journal |

## Menjalankan lokal

Butuh Go 1.26.

```
go build -o bin/librasense ./cmd/librasense
```

```
APP_ENV=development
PORT=8787
DATA_DIR=./data
DEVICE_ID=perpustakaan-1
DEVICE_TOKEN=token-lokal-panjang
ACCESS_DEV_BYPASS=true
```

Di PowerShell, set env lalu jalankan `.\bin\librasense.exe`. Buka `http://127.0.0.1:8787`.

`ACCESS_DEV_BYPASS=true` melewati verifikasi Cloudflare Access. Jangan dipakai di produksi, server menolak kombinasi itu dengan `APP_ENV=production`.

## Alur

1. ESP32 `POST /device/readings` dengan sensor dan state yang sedang diterapkan.
2. Server menyimpan pembacaan, menghitung state aktuator, menyimpan, lalu membalas `state` dan `commands`.
3. ESP32 menyalakan relay kipas dan buzzer sesuai balasan.
4. Panel membaca `/api/overview` dan `/api/history`.

Relay satu kanal mengalihkan kipas 12V, jadi state server hanya `fan`, bukan relay terpisah. Field `relay` pada payload perangkat hanya cermin `fan`.

## Endpoint

| Method | Path | Auth |
|---|---|---|
| GET | `/health` | - |
| POST | `/device/readings` | bearer device token |
| POST | `/device/ack` | bearer device token |
| GET | `/api/overview` | Cloudflare Access JWT |
| GET | `/api/history` | Cloudflare Access JWT |
| PUT | `/api/settings` | Cloudflare Access JWT |
| POST | `/api/commands` | Cloudflare Access JWT |

Perintah: `restart`, `reset`, `test1`, `test2`, `test3`.

## Test

```
go vet ./...
go test ./...
```

## Deploy

Target: VPS bersama dengan Podman 4.9.3 rootful dan cloudflared token mode.

```powershell
.\deploy\check-vps.ps1    # sekali, sebelum deploy pertama
.\deploy\build.ps1
.\deploy\deploy.ps1
```

Runbook lengkap: `.agents/skills/librasense-vps-runbook/SKILL.md`.
Ringkasan operator: `deploy/README.md`.

Port 8097 bind ke loopback, hanya lewat tunnel. Panel memakai Access, hostname
device tidak. Jangan mengganggu MedVault di port 8095 dan 8096.

## Firmware

Salin `esp32/librasense_node/secrets.example.h` menjadi `secrets.h`, isi WiFi, `LIBRASENSE_API_BASE`, `LIBRASENSE_DEVICE_ID`, `LIBRASENSE_DEVICE_TOKEN`. Flash dengan Arduino IDE, board ESP32, library DHT sensor library.

## Dokumentasi

- `docs/README-docs.md`: konvensi penulisan.
- `docs/001-go-architecture-19-nov-2026.md`: arsitektur dan verifikasi.
- `docs/002-firmware-push-19-nov-2026.md`: pin map dan kontrak perangkat.
