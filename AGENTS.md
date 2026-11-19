# Panduan agent

Proyek ini punya satu pohon: `D:\KULIAH\SEM 5\IoT\LibraSense\LibraSense`. Tidak ada repo git, tidak ada build frontend.

## Aturan yang tidak boleh dilanggar

- Mikrokontroler hanya membaca sensor, mengirim data, dan menjalankan perintah. Tidak ada ambang, histeresis, atau keputusan aktuator di firmware. Semua logika di `internal/decision`.
- Panel hanya Alpine.js, HTML, CSS, dan JS statis. Jangan menambah npm, Vite, Svelte, React, atau build step.
- Server satu binary Go dengan `//go:embed` untuk panel. Jangan tambah runtime eksternal.
- SQLite memakai `modernc.org/sqlite` supaya `CGO_ENABLED=0`.
- Pengaturan dimiliki server. Perangkat tidak menyimpan atau mengirim ambang.
- Relay satu kanal mengalihkan kipas 12V. Jangan menghidupkan kembali state relay terpisah di pengaturan atau panel.
- VPS dipakai bersama MedVault. Jangan menyentuh port 8095 dan 8096, unit `medvault-pod.service`, atau hostname tunnel MedVault. Jangan me-restart `cloudflared-claw.service` tanpa alasan.
- Deploy hanya lewat `deploy/` dan runbook `.agents/skills/librasense-vps-runbook/SKILL.md`. Jangan menjalankan `go build` di VPS.
- `/device/*` memakai bearer token. `/api/*` memakai Cloudflare Access JWT. Jangan melemahkan salah satunya.
- WiFi hanya diubah lewat portal AP perangkat, bukan dari panel, supaya tidak terkunci.
- Jangan mengubah `dump/knowledge/`. Itu basis pengetahuan laporan pengguna.

## Konvensi

- Prosa dan dokumen: bahasa Indonesia, tanpa emoji, tanpa em dash dan en dash, ringkas, jangan mengarang fakta.
- Kode dan nama file: bahasa Inggris, identifier pendek yang sudah dipakai (`b1`, `hys`, `iv`, `fm`, `bm`, `al`, `na`).
- JSON pengaturan memakai kunci pendek yang sama di server, panel, dan test.
- Setiap perubahan perilaku harus masuk `docs/` dengan nomor baru dan entri `docs/journal.json`.
- Perubahan harus lolos `gofmt -w .`, `go vet ./...`, `go test ./...`, dan `go build ./cmd/librasense`.
- Deploy saja: `deploy/build.ps1` lalu `deploy/deploy.ps1`. Jangan menambah langkah manual di luar runbook.

## Peta cepat

| Kebutuhan | File |
|---|---|
| tambah endpoint | `internal/httpapi/server.go`, lalu handler di `api.go` atau `device.go` |
| ubah ambang atau aktuator | `internal/decision/decision.go`, `internal/settings/settings.go` |
| ubah schema | `internal/store/store.go` |
| ubah panel | `web/index.html`, `web/app.js`, `web/app.css` |
| ubah firmware | `esp32/librasense_node/librasense_node.ino` |
| deploy | `deploy/`, `.agents/skills/librasense-vps-runbook/SKILL.md` |

## Jebakan

- `web/embed.go` gagal build bila `index.html`, `app.css`, `app.js`, atau `vendor/alpine.min.js` hilang.
- `ACCESS_DEV_BYPASS=true` ditolak saat `APP_ENV=production`.
- Store membuka satu koneksi tulis. Jangan tambah goroutine penulis paralel tanpa mengubahnya.
- `data/` berisi SQLite dan diabaikan git.
