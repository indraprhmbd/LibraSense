Status: done
Tanggal: 19 Nov 2026
Pemilik: tim LibraSense
Terkait: 002-firmware-push-19-nov-2026.md

# Arsitektur Go dan panel Alpine

## Konteks

Versi Laravel dihapus saat folder proyek dirapikan. Direktori dibangun ulang dari nol pada 19 Nov 2026 dengan target yang sudah dipilih sebelumnya: perangkat mengirim data ke VPS, panel dilindungi Cloudflare Access, dan server harus ringan.

Pilihan awal TypeScript dan Svelte dibatalkan sebelum diuji karena dua alasan: runtime Node dan `node_modules` menambah beban VPS kecil, dan panel tidak butuh build step. Go menghasilkan satu binary statis, Alpine cukup satu file JS tanpa build.

## Keputusan

**API dan panel dalam satu binary Go.** Modul `librasense` memisahkan `internal/store`, `internal/settings`, `internal/decision`, `internal/httpapi`, dan `internal/config`. Panel statis di-embed lewat `web/embed.go`.

**SQLite memakai `modernc.org/sqlite`.** Driver murni Go, jadi `CGO_ENABLED=0` dan binary bisa jalan di distroless. Satu koneksi tulis (`SetMaxOpenConns(1)`) cukup untuk satu perangkat dan menghindari lock.

**Semua keputusan aktuator di server.** `internal/decision` menghitung panas, kebisingan, kipas, dan buzzer dari pengaturan tersimpan plus pembacaan terakhir. Perangkat hanya menerima `state` dan menyalakan pin. Tidak ada ambang di firmware.

**Relay mengalihkan kipas 12V.** Modul relay satu kanal dan kipas DC 12V adalah satu jalur keluaran. State server hanya punya `fan` dan `buzzer`, bukan relay terpisah. Field `relay` di payload `applied` hanya cermin `fan` untuk kompatibilitas.

**Autentikasi dua jalur.** `/device/*` memakai bearer token, dibandingkan sebagai hash SHA-256 dengan `subtle.ConstantTimeCompare`. `/api/*` memakai JWT Cloudflare Access, diverifikasi terhadap JWKS tim dengan `golang-jwt/jwt/v5`. Bila `CF_ACCESS_AUD` tidak diisi dan bypass mati, server menolak start. Ini gagal tertutup.

**Panel tanpa build step.** `web/index.html`, `web/app.js`, `web/app.css`, dan Alpine 3.14.8 yang di-vendor. Semua ditanam ke binary, tidak ada `npm` saat deploy.

**Hostname panel dan device dipisah.** Cloudflare Access dipasang hanya di hostname panel. Hostname device tidak boleh punya aplikasi Access, karena ESP32 tidak bisa melewati halaman login. Keduanya menuju service yang sama lewat cloudflared.

## Perubahan

Struktur:

```
cmd/librasense/main.go       startup, graceful shutdown
internal/config              parsing env, validasi, gagal tertutup
internal/store               SQLite, schema, query
internal/settings            tipe, default, merge, validasi
internal/decision            evaluasi aktuator dan status
internal/httpapi             route, auth, handler, test
web/                         panel Alpine dan embed
esp32/librasense_node/       firmware kirim data
deploy/                      Containerfile, unit systemd, script deploy
```

Alur data:

1. ESP32 `POST /device/readings` dengan sensor dan `applied` terakhir.
2. Server simpan pembacaan, hitung state dari pengaturan dan pembacaan.
3. Server simpan state ke tabel `actuators` dan balas `state` plus `commands`.
4. ESP32 menyalakan relay dan buzzer sesuai `state`, lalu lapor `applied` pada kiriman berikutnya.
5. Panel membaca `/api/overview` dan `/api/history` tiap 5 detik.

Endpoint:

| Method | Path | Auth | Fungsi |
|---|---|---|---|
| GET | `/health` | - | cek hidup |
| POST | `/device/readings` | bearer token | kirim sensor, terima state |
| POST | `/device/ack` | bearer token | konfirmasi perintah |
| GET | `/api/overview` | Access JWT | status, pengaturan, state |
| GET | `/api/history` | Access JWT | riwayat, maksimal 500 |
| PUT | `/api/settings` | Access JWT | validasi dan simpan pengaturan |
| POST | `/api/commands` | Access JWT | restart, reset, test1..3 |

Skema SQLite: `devices`, `readings`, `settings`, `commands`, `actuators`. `readings` menyimpan sensor sebagai JSON, bukan tabel ternormalisasi, karena jumlah sensor tetap dan panel hanya butuh satu baris utuh.

## Verifikasi

- `go test ./...`: 10 test lulus, mencakup token salah, payload rusak, histeresis kipas, perintah sekali kirim, urutan riwayat, validasi pengaturan, dan Access gagal tertutup.
- `go vet ./...`: bersih.
- `go build -o bin/librasense.exe ./cmd/librasense`: sukses, 16.556.640 byte.
- Smoke test dengan `ACCESS_DEV_BYPASS=true`: `/health` benar, panel 200 dan memuat Alpine, `/device/readings` tanpa token 401, kiriman suhu 34,5 menghasilkan `state.fan=true`, `state.buzzer=[true,false,false]`, `status.hot=[1]`, `PUT /api/settings` dengan `b1=90` mendapat 422.

## Terbuka

- Hanya satu perangkat. `DEVICE_ID` dan `DEVICE_TOKEN` masih env tunggal. Banyak node butuh tabel perangkat dan token per baris.
- Belum ada retensi data. Tabel `readings` tumbuh tanpa batas.
- Firmware belum dikompilasi karena `arduino-cli` tidak tersedia di mesin ini. Lihat dokumen 002.
- Belum ada rate limit di hostname device.

## Rujukan

- `internal/decision/decision.go`
- `internal/httpapi/device.go`
- `internal/httpapi/api.go`
- `internal/store/store.go`
- `web/app.js`
- `deploy/README.md`
