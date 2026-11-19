Status: done
Tanggal: 19 Nov 2026
Pemilik: tim LibraSense
Terkait: 001-go-architecture-19-nov-2026.md, 002-firmware-push-19-nov-2026.md

# Target deploy VPS bersama

## Konteks

Repo sebelumnya hanya punya draf deploy: quadlet rootless, cloudflared dengan
file config, dan asumsi host kosong. Kenyataannya LibraSense akan menumpang di
VPS yang sudah menjalankan MedVault dengan Podman 4.9.3 rootful, port 8095 dan
8096, serta satu tunnel Cloudflare token mode bernama
`cloudflared-claw.service`. Draf lama tidak bisa dipakai apa adanya dan berisiko
mengganggu layanan yang sudah jalan.

## Keputusan

**Akun layanan terpisah `librasense`.** Mengikuti pola MedVault: akun bootstrap
`liteclaw` hanya untuk provisioning, akun layanan memiliki container, env, data,
dan unit. Sudo dibatasi ke podman, systemctl unit LibraSense, journalctl unit
LibraSense, dan satu perintah install unit.

**Podman rootful, bukan quadlet rootless.** Host hanya punya socket rootful dan
MedVault memakai pola yang sama. Unit systemd menjalankan `podman run` di
foreground dengan `Type=simple`, sehingga systemd melacak proses container dan
bisa restart sendiri. Container lama dihapus lewat `ExecStartPre`, bukan lewat
flag restart container.

**Port 8097, bind ke 127.0.0.1.** Bersebelahan dengan MedVault, hanya bisa
dicapai lewat tunnel. Port diverifikasi bebas pada 19 Nov 2026. Bila suatu saat
terpakai, unit, hostname, dan runbook harus diubah bersamaan, bukan diam-diam
pindah port.

**Binary dibangun di workstation, bukan di VPS.** Host tidak punya Go dan tidak
akan dipasangi. `deploy/build.ps1` cross compile `linux/amd64` dengan
`CGO_ENABLED=0`. Transfer memakai `podman save` dan `podman load`. Image lama
disimpan sebagai tag `:prev` untuk rollback.

**Fallback build image di host.** Workstation tidak punya Podman machine, WSL,
maupun Docker, sehingga `podman build` lokal tidak bisa dijalankan tanpa
unduhan besar. `deploy/build-remote.ps1` karena itu mengirim binary hasil cross
compile dan `Containerfile` ke `/tmp/librasense-build`, lalu menjalankan
`podman build` di host. Yang berjalan di host hanya perakitan image, bukan
`go build`. Deploy pertama 19 Nov 2026 memakai jalur ini.

**Container berjalan sebagai uid 10001.** Berbeda dari MedVault yang berjalan
sebagai root di dalam container. Direktori `data/` dimiliki uid tersebut dan
tidak boleh diubah ke `librasense`, karena container tidak bisa menulis.
Konsekuensinya backup butuh akses bootstrap admin.

**Runtime Alpine dengan CA certificate.** Server harus mengambil JWKS Cloudflare
Access lewat HTTPS. Image tanpa CA akan gagal memverifikasi JWT dan seluruh
panel akan 401.

**Tunnel tetap satu, ingress di dashboard.** Tunnel token mode tidak menyimpan
config di disk. Dua hostname ditambahkan di Zero Trust: panel dengan aplikasi
Access baru, device tanpa Access. Aplikasi Access MedVault tidak boleh dipakai
ulang, karena `CF_ACCESS_AUD` harus menunjuk aplikasi panel LibraSense.

**Env tidak pernah ditimpa otomatis.** Deploy script hanya menulis `.env` bila
dipanggil dengan `-EnvFile` secara eksplisit. Ini mencegah token perangkat dan
AUD hilang saat redeploy.

**Sudo tanpa Cmnd_Alias.** Host memuat `/etc/sudoers.d` dua kali, lewat
`@includedir` dan `#includedir`. Alias di drop-in selalu dilaporkan sebagai
duplikat. `setup-user.sh` karena itu menulis daftar perintah eksplisit, bukan
`Cmnd_Alias`. Ditemukan saat provisioning 19 Nov 2026.

## Perubahan

Tambahan:

- `.agents/skills/librasense-vps-runbook/SKILL.md` dan
  `references/vps-access.md`: runbook operasional.
- `deploy/setup-user.sh`: provisioning akun, direktori, SSH, dan sudo scope.
- `deploy/build.ps1`: cross compile dan build image.
- `deploy/build-remote.ps1`: cross compile lalu rakit image di host, dipakai
  saat workstation tidak punya runtime container.
- `deploy/deploy.ps1`: tag rollback, transfer image, install unit, restart,
  tunggu health. Flag `-Remote` melewati transfer karena image sudah di host.
- `deploy/check-vps.sh` dan `deploy/check-vps.ps1`: preflight read-only.
- `deploy/smoke.ps1`: cek panel dan auth perangkat lewat loopback.
- `deploy/systemd/librasense.service`: unit kanonik.
- `deploy/cloudflared/README.md`: langkah dashboard.

Dihapus:

- `deploy/systemd/librasense.container`: quadlet rootless tidak sesuai host.
- `deploy/cloudflared/config.yml`: tunnel berjalan token mode.

Diubah:

- `deploy/Containerfile`: dari multi stage Go menjadi image Alpine yang menyalin
  binary hasil cross compile, user 10001, CA certificate.
- `deploy/README.md`: ringkasan operator.
- `.env.example`: port 8097 dan data di `/data`.

## Verifikasi

- `go build ./cmd/librasense`, `go vet ./...`, `go test ./...`: lulus di
  workstation.
- `node --check web/app.js`: lulus.
- `deploy/build.ps1` sampai tahap cross compile: binary `linux/amd64` 10,62 MB
  di `dist/librasense`.
- `podman build` tidak bisa dijalankan di workstation ini: tidak ada Podman
  machine, WSL, maupun Docker. Image dirakit di host lewat `-Remote` dan
  berhasil pada percobaan pertama.
- Semua script PowerShell lolos parse, `setup-user.sh` dan `check-vps.sh` lolos
  `bash -n`.
- Preflight VPS dijalankan 19 Nov 2026 sebagai `ubuntu`: 8097 bebas, tunnel
  `cloudflared-claw.service` aktif, `librasense` belum ada, RAM tersedia 1384 MB,
  disk bebas 25 G, sqlite3 tidak terpasang.
- Provisioning dijalankan 19 Nov 2026 lewat `liteclaw`: akun `librasense`
  (uid 1005), home `/home/librasense` mode 0750, `data/` milik 10001 mode 0700,
  kunci SSH tersalin, sudo scope aktif tanpa prompt.
- Dua bug script ditemukan dan diperbaiki saat provisioning: `visudo` memeriksa
  path tujuan alih-alih file temp, dan `Cmnd_Alias` bentrok karena host memuat
  `/etc/sudoers.d` dua kali. Alias SSH distandarkan ke `vps-librasense`.
- Deploy pertama 19 Nov 2026: unit `librasense.service` active dan enabled,
  container `librasense` jalan sebagai uid 10001, port 8097 listening,
  `/health` mengembalikan `{"ok":true}`.
- Env produksi ditulis di host, mode 0600, `DEVICE_TOKEN` 32 byte acak,
  `CF_ACCESS_TEAM_DOMAIN=https://<team>.cloudflareaccess.com`,
  `CF_ACCESS_AUD=<CF_ACCESS_AUD>`.
- Edge: `https://iot.<domain>/health` 200,
  `/api/state` tanpa JWT 401, `/device/readings` tanpa token 401,
  `https://librasense.<domain>/` 302 ke login Access.
- Kiriman perangkat tiruan dengan token asli diterima: server menyimpan reading
  dan mengembalikan `state` (`fan` true, `buzzer` rak 1 true) untuk suhu 31,4 C.
- `deploy/smoke.ps1` lewat `ssh -N -L 18097:127.0.0.1:8097 vps-librasense`
  lulus enam cek: health, panel HTML, `app.js`, `app.css`, Alpine vendored, dan
  401 pada `/device/readings`. Jalur loopback sengaja melewati Access.

## Terbuka

- Belum ada retensi data dan belum ada jadwal backup otomatis.
- Firmware tetap belum dikompilasi, sehingga perangkat asli belum mengirim data.
- `TLS_INSECURE` di firmware masih aktif, perlu CA pinning sebelum dipakai di
  luar prototipe.

## Rujukan

- `.agents/skills/librasense-vps-runbook/SKILL.md`
- `deploy/README.md`
- `deploy/setup-user.sh`
- `deploy/systemd/librasense.service`
- `deploy/Containerfile`
