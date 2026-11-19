# Cloudflare Tunnel untuk LibraSense

Host memakai satu tunnel token mode yang sudah berjalan sebagai unit
`cloudflared-claw.service`. File config tidak disimpan di disk, jadi ingress
diatur di dashboard Cloudflare Zero Trust, bukan di repo ini.

## Yang harus dibuat

Dua public hostname, keduanya menuju service yang sama:

| Hostname | Service | Cloudflare Access |
|---|---|---|
| `librasense.<domain>` | `http://127.0.0.1:8097` | aplikasi Access khusus panel |
| `iot.<domain>` | `http://127.0.0.1:8097` | tidak ada |

Keduanya sudah aktif sejak 19 Nov 2026. Tim: `<team>.cloudflareaccess.com`.
AUD aplikasi panel: `<CF_ACCESS_AUD>`.

Jangan pasang Access di hostname device. ESP32 tidak bisa melewati halaman
login. Endpoint `/device/*` tetap dilindungi bearer token perangkat.

Jangan memakai ulang aplikasi Access milik MedVault. Buat aplikasi baru supaya
`CF_ACCESS_AUD` LibraSense terpisah dan policy bisa berbeda.

## Langkah

Sudah dijalankan 19 Nov 2026.

1. Zero Trust, Networks, Tunnels, pilih tunnel yang aktif.
2. Public Hostname, tambah hostname panel, type HTTP, URL `127.0.0.1:8097`.
3. Public Hostname, tambah hostname device, type HTTP, URL `127.0.0.1:8097`.
4. Access, Applications, buat aplikasi self hosted untuk hostname panel saja.
5. Salin Application Audience tag ke `CF_ACCESS_AUD` di `/home/librasense/.env`.
6. Isi `CF_ACCESS_TEAM_DOMAIN` dengan `https://<tim>.cloudflareaccess.com`.

## Verifikasi

```
curl -s https://iot.<domain>/health
curl -sI https://librasense.<domain>/
```

`/health` harus menjawab JSON tanpa login. Panel harus mengembalikan redirect
ke halaman login Access bila belum punya sesi.

## Catatan

- Tunnel yang sama dipakai MedVault. Jangan mengubah hostname atau service
  MedVault, dan jangan me-restart tunnel tanpa alasan.
- Bila hostname device mendapat halaman login Access, berarti aplikasi Access
  salah dipasang. Hapus aplikasi untuk hostname itu.
