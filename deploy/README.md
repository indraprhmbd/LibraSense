# Deploy LibraSense

Target: VPS bersama, Podman 4.9.3 rootful, cloudflared token mode. Runbook
lengkap ada di `.agents/skills/librasense-vps-runbook/SKILL.md`.

Alur singkat:

```powershell
.\deploy\check-vps.ps1    # sekali, sebelum deploy pertama
.\deploy\build.ps1
.\deploy\deploy.ps1
.\deploy\smoke.ps1        # lewat ssh -N -L 18097:127.0.0.1:8097 vps-librasense
```

Bila workstation tidak punya runtime container (tidak ada Podman machine, WSL,
atau Docker), pakai jalur build di host:

```powershell
.\deploy\deploy.ps1 -Remote
```

Binary tetap di-cross compile di workstation. Host hanya merakit image dari
binary itu lewat `podman build`, tidak menjalankan `go build`. Jalur ini
menghemat beberapa GB unduhan Podman machine dan sudah dipakai untuk deploy
pertama 19 Nov 2026.

## Sekali saja

Buat akun layanan dan direktori:

```powershell
scp deploy/setup-user.sh vps-lama:/tmp/librasense-setup.sh
ssh vps-lama 'sudo bash /tmp/librasense-setup.sh; rm -f /tmp/librasense-setup.sh'
```

Buat env di host, jangan menimpa env yang sudah ada:

```powershell
scp .env.example vps-lama:/tmp/librasense.env
ssh vps-lama 'sudo install -m 0600 -o librasense -g librasense /tmp/librasense.env /home/librasense/.env; rm -f /tmp/librasense.env'
```

Isi `DEVICE_TOKEN` minimal 16 karakter, `CF_ACCESS_TEAM_DOMAIN`, dan
`CF_ACCESS_AUD` untuk aplikasi Access panel saja.

## Cloudflare

Tambahkan dua public hostname di dashboard tunnel, keduanya ke
`http://127.0.0.1:8097`:

- `panel.<domain>` dengan Access.
- `device.<domain>` tanpa Access.

Detail di `deploy/cloudflared/README.md`. Jangan mengubah hostname MedVault.

## Operasi

| Tugas | Perintah |
|---|---|
| status | `ssh vps-librasense 'systemctl is-active librasense.service'` |
| log | `ssh vps-librasense 'sudo journalctl -u librasense.service -n 100 --no-pager'` |
| restart | `ssh vps-librasense 'sudo systemctl restart librasense.service'` |
| rollback | `ssh vps-librasense 'sudo podman tag localhost/librasense:prev localhost/librasense:latest && sudo systemctl restart librasense.service'` |
| health | `ssh vps-librasense 'curl -fsS http://127.0.0.1:8097/health'` |

Backup dan restore ada di runbook bagian 12, dijalankan dari alias `vps-lama`.
Hentikan unit sebelum menyalin database.

## Aturan

- Jangan menjalankan `go build` di host. Binary dibuat di workstation.
- `-Remote` hanya merakit image di host dari binary hasil cross compile.
- Jangan memakai port 8095 atau 8096, itu MedVault.
- Jangan me-restart `cloudflared-claw.service` tanpa alasan.
- Jangan mengubah `data/` menjadi milik `librasense`. Container berjalan
  sebagai uid 10001.
