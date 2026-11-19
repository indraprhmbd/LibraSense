---
name: librasense-vps-runbook
description: >-
  Deployment and operations runbook for LibraSense Go on the user's shared VPS.
  Use whenever the user asks to deploy, redeploy, roll back, inspect, restart,
  back up, or troubleshoot librasense.service, the librasense container, the
  Cloudflare tunnel hostnames, or /home/librasense on that host.
---

# LibraSense VPS Runbook

This skill covers only the LibraSense deployment. The same host also runs
MedVault on ports 8095 and 8096 and shares one Cloudflare tunnel. Do not touch
those.

Host facts marked verified come from the MedVault runbook, checked against the
host on 12 November 2026. LibraSense specific values marked pending must be
confirmed with `deploy/check-vps.ps1` before the first deploy. If a live check
differs, trust the live check and update this file.

---

## 1. Access

| Role | Account | Purpose |
| :--- | :--- | :--- |
| Bootstrap admin | `liteclaw` | Key based SSH, full sudo. Provisioning and unit install. |
| Bootstrap console | `ubuntu` | Full sudo. Used for the 19 Nov 2026 preflight. |
| Service account | `librasense` | Owns the container, image, env, data, and systemd unit. uid 1005, scoped sudo. |

`librasense` was provisioned on 19 November 2026. The account exists, the key is
installed, the directory tree is in place, and the sudo scope is active.
`vps-librasense` is the workstation SSH alias for it.

| Parameter | Value |
| :--- | :--- |
| SSH alias for deploys | `vps-librasense` |
| SSH alias for bootstrap | `vps-lama` (liteclaw) |
| Service account home | `/home/librasense` |
| Service account uid and gid | `1005` |
| Identity file | `~/.ssh/id_ed25519` with `IdentitiesOnly yes` |
| Service account shell | `/bin/bash`, password locked |
| Container uid | `10001` |

Workstation `~/.ssh/config`:

```text
Host vps-lama
    HostName <VPS_HOST>
    User liteclaw
    IdentityFile ~/.ssh/id_ed25519
    IdentitiesOnly yes

Host vps-librasense
    HostName <VPS_HOST>
    User librasense
    IdentityFile ~/.ssh/id_ed25519
    IdentitiesOnly yes
```

Use `vps-librasense` for deploys. Use `vps-lama` only for provisioning and
backups. Never pass `-i` on the command line.

### 1.1 Provisioning the service account

`deploy/setup-user.sh` is idempotent. It creates the user, copies the SSH key,
builds the directory tree, and writes `/etc/sudoers.d/90-librasense`. It does
not start the service and does not install the unit.

```powershell
scp deploy/setup-user.sh vps-lama:/tmp/librasense-setup.sh
ssh vps-lama 'sudo bash /tmp/librasense-setup.sh; rm -f /tmp/librasense-setup.sh'
```

Preview:

```powershell
scp deploy/setup-user.sh vps-lama:/tmp/librasense-setup.sh
ssh vps-lama 'sudo bash /tmp/librasense-setup.sh --dry-run; rm -f /tmp/librasense-setup.sh'
```

`scp` then execute. Do not pipe scripts through `ssh "bash -s"` from
PowerShell, heredocs get corrupted.

---

## 2. Host Baseline

| Parameter | Value | State |
| :--- | :--- | :--- |
| OS | Ubuntu 24.04.5 LTS | verified 19 Nov 2026 |
| Kernel | `6.8.0-149-generic` | verified 19 Nov 2026 |
| CPU | 2 vCPU | verified |
| RAM | 1967 MB total, 1384 MB available at check | verified 19 Nov 2026 |
| Swap | 1987 MB at `/swap.img`, about 2.5 MB used | verified 19 Nov 2026 |
| Disk `/` | 40 G total, 14 G used, 25 G free | verified 19 Nov 2026 |
| systemd | 255 | verified |
| Podman | 4.9.3, rootful, `podman.socket` active | verified |
| SELinux | not enabled, `:Z` is a no-op | verified |
| sqlite3 on host | absent, backups use stop and tar | verified 19 Nov 2026 |
| `librasense` user | provisioned, uid and gid 1005 | verified 19 Nov 2026 |
| Sudo scope | active, no password prompt for podman and the unit | verified 19 Nov 2026 |
| Go toolchain for `librasense` | none | by design |
| LibraSense port | 8097, listening on `127.0.0.1` | live since 19 Nov 2026 |
| Panel hostname | `librasense.<domain>` | Access application active |
| Device hostname | `iot.<domain>` | no Access application |
| Access team domain | `<team>.cloudflareaccess.com` | verified 19 Nov 2026 |
| Access AUD, panel app | `<CF_ACCESS_AUD>` | in `/home/librasense/.env` |
| Container image | `localhost/librasense:latest`, built on host 19 Nov 2026 | running |
| Unit `librasense.service` | active, enabled, restarts on failure | verified 19 Nov 2026 |

Cross compile on Windows. Never install Go or Node on the host.

---

## 3. Podman 4.9.3 Constraints

Proven failure modes on this host. Follow them exactly.

1. `podman ps` rejects every argument. `podman ps -a --format ...` fails with
   `Error: podman ps takes no arguments`. Use `sudo podman ps -a`.
2. Rootful podman requires `sudo`. The service account has scoped sudo for
   `/usr/bin/podman`, so `sudo podman ...` never prompts.
3. Port publication happens at container creation. The unit publishes
   `127.0.0.1:8097:8097` on every start and removes the previous container
   first, so publish conflicts surface as a unit failure, not a silent change.
4. Restart policy is owned by systemd, not by `--restart` on the container.
5. Do not add a pod for a single container. The unit runs `podman run` in the
   foreground so systemd tracks the process directly.

---

## 4. Port Allocation

| Port | Bind | Purpose |
| :--- | :--- | :--- |
| `8097` | `127.0.0.1` | LibraSense container, HTTP and panel |
| `8095`, `8096` | `127.0.0.1`, pod internal | MedVault, do not disturb |

Confirm before first deploy:

```bash
ss -ltn | grep -E ':(8097)\b' || echo "8097 free"
```

If 8097 is taken, stop. Do not silently pick another port: the unit file, the
Cloudflare public hostnames, and this runbook all assume 8097. Change all of
them together.

---

## 5. Filesystem Layout

```text
/home/librasense/
|-- .ssh/authorized_keys      mode 0600
|-- .env                      mode 0600, token and Access config
|-- bin/                      mode 0750, reserved for host side tools
|-- backups/                  mode 0750
`-- data/                     mode 0700, owner 10001:10001
    |-- librasense.db
    |-- librasense.db-wal
    `-- librasense.db-shm
```

`data/` is bind mounted to `/data` with `:Z`. The container runs as uid 10001,
so the host directory must stay owned by 10001. Do not `chown` it to
`librasense`.

### 5.1 Sudo scope

`/etc/sudoers.d/90-librasense` grants:

- `/usr/bin/podman`
- `systemctl start|stop|restart|status|is-active|enable|disable|daemon-reload`
  limited to `librasense.service`
- `journalctl -u librasense.service`, with and without `-n 100 --no-pager`
- one exact `install` of `/tmp/librasense.service`

Rootful podman is root equivalent by design. Treat this as damage limiting,
not a security boundary.

The host loads `/etc/sudoers.d` twice, once with `@includedir` and once with
`#includedir`. Do not define `Cmnd_Alias` in a drop-in on this host: sudo will
report every alias as already defined. Write explicit command lists instead,
which is what `deploy/setup-user.sh` does.

---

## 6. Build and Transfer

### 6.1 Image

Built on the workstation. The host does not compile Go.

```powershell
.\deploy\build.ps1
```

It cross compiles `linux/amd64` with `CGO_ENABLED=0`, then builds
`localhost/librasense:latest` from `deploy/Containerfile`. The runtime is
Alpine with CA certificates, because the server fetches Cloudflare Access JWKS
over HTTPS.

Never run `go build` on the host.

If the workstation has no container runtime (no Podman machine and no WSL or
Docker), build the image on the host from the cross compiled binary:

```powershell
.\deploy\deploy.ps1 -Remote
```

That runs `deploy/build-remote.ps1`: the binary is still compiled here, only
`podman build` runs on the host in `/tmp/librasense-build`. This was the path
used for the first deploy on 19 Nov 2026.

### 6.2 Deploy

```powershell
.\deploy\deploy.ps1
```

The script tags the current image as `:prev`, transfers the new tar, loads it,
installs the unit, restarts, and waits for `/health`.

With `-Remote` the image already exists on the host, so the tag, save, and load
steps are skipped and only the unit install and restart remain.

First deploy needs an env file. It is never generated or overwritten by the
script:

```powershell
scp .env.example vps-lama:/tmp/librasense.env
ssh vps-lama 'sudo install -m 0600 -o librasense -g librasense /tmp/librasense.env /home/librasense/.env; rm -f /tmp/librasense.env'
.\deploy\deploy.ps1
```

Fill `DEVICE_TOKEN`, `CF_ACCESS_TEAM_DOMAIN`, and `CF_ACCESS_AUD` first.
`DEVICE_TOKEN` must be at least 16 characters and must match
`LIBRASENSE_DEVICE_TOKEN` in the firmware.

To upload a corrected env later, `-EnvFile` is the only way the deploy script
touches `.env`.

---

## 7. systemd Unit

Installed at `/etc/systemd/system/librasense.service`. Canonical copy is
`deploy/systemd/librasense.service`.

| Property | Value |
| :--- | :--- |
| Type | `simple`, podman runs in the foreground |
| ExecStart | `podman run` with `--memory=128m --memory-swap=160m` |
| ExecStartPre | `podman rm -f librasense`, failure ignored |
| ExecStop | `podman stop -t 10 librasense` |
| Restart | `always`, 10 s delay |
| Publish | `127.0.0.1:8097:8097` |
| Volume | `/home/librasense/data:/data:Z` |
| Env | `--env-file /home/librasense/.env` |

Commands:

```bash
sudo systemctl daemon-reload
sudo systemctl enable librasense.service
sudo systemctl restart librasense.service
systemctl is-active librasense.service
sudo journalctl -u librasense.service -n 100 --no-pager
```

---

## 8. Cloudflare Tunnel and Access

One token managed tunnel already runs as `cloudflared-claw.service`. Ingress
lives in the Zero Trust dashboard, not on disk.

| Hostname | Service | Access |
| :--- | :--- | :--- |
| `panel.<domain>` | `http://127.0.0.1:8097` | new Access application |
| `device.<domain>` | `http://127.0.0.1:8097` | none |

Rules:

- Never add an Access application to the device hostname. The ESP32 cannot pass
  a login page.
- Never reuse the MedVault Access application. `CF_ACCESS_AUD` must identify the
  LibraSense panel application.
- `/device/*` is protected by the device bearer token. `/api/*` is protected by
  the Access JWT.
- The tunnel is shared. Do not restart it or edit MedVault hostnames.

Detailed steps are in `deploy/cloudflared/README.md`.

---

## 9. Verification

### 9.1 Manual host checks

```bash
systemctl is-active librasense.service
sudo systemctl status librasense.service --no-pager
sudo podman ps -a
sudo podman stats --no-stream
curl -fsS http://127.0.0.1:8097/health
free -m
df -h /
```

Expected: unit active, container up, `/health` returns `{"ok":true}`, container
RSS under 128 MB.

### 9.2 Smoke from the workstation

```powershell
ssh -N -L 18097:127.0.0.1:8097 vps-librasense
```

Second terminal:

```powershell
.\deploy\smoke.ps1
```

Covers health, panel HTML, `app.js`, `app.css`, vendored Alpine, and the 401 on
`/device/readings` without a token. The loopback path bypasses Access on
purpose, so it proves the service, not the edge policy.

### 9.3 Edge checks

```bash
curl -fsS https://device.<domain>/health
curl -sI https://panel.<domain>/ | head -n 5
```

Panel must redirect to the Access login when there is no session. Device must
answer `/health` directly.

Known good on 19 Nov 2026:

| Check | Result |
| :--- | :--- |
| `https://iot.<domain>/health` | `200` |
| `https://iot.<domain>/api/state` without JWT | `401` |
| `https://iot.<domain>/device/readings` without token | `401` |
| `https://iot.<domain>/device/readings` with device token | `200`, decision engine returns `state` |
| `https://librasense.<domain>/` without session | `302` to `<team>.cloudflareaccess.com` |

The device hostname must never gain an Access application. The panel hostname
must never lose one.

### 9.4 Device check

The firmware is healthy when `applied.fan` in the panel feedback equals the
server decision within one poll interval, and when a `test1` command pulses the
buzzer once.

---

## 10. Redeploy

```powershell
.\deploy\check-vps.ps1   # only needed when host facts are in doubt
.\deploy\deploy.ps1
.\deploy\smoke.ps1       # through the tunnel from section 9.2
```

`deploy.ps1` keeps the previous image as `localhost/librasense:prev`.

---

## 11. Rollback

```bash
sudo podman tag localhost/librasense:prev localhost/librasense:latest
sudo systemctl restart librasense.service
curl -fsS http://127.0.0.1:8097/health
```

Rollback swaps the image only. The SQLite schema is forward compatible for the
current single device setup. If a release changed the schema and must be undone,
restore the database from a backup taken before that release.

---

## 12. Backup and Restore

Run this section from the bootstrap alias `vps-lama`, not from `librasense`.
The scoped sudo for `librasense` does not include `tar` or `chown`.

Stop first. WAL files must not move while the server writes.

```bash
sudo systemctl stop librasense.service
stamp="$(date +%F-%H%M)"
sudo tar -C /home/librasense/data -czf "/tmp/librasense-${stamp}.tar.gz" .
sudo mv "/tmp/librasense-${stamp}.tar.gz" /home/librasense/backups/
sudo chown librasense:librasense "/home/librasense/backups/librasense-${stamp}.tar.gz"
sudo systemctl start librasense.service
```

Restore:

```bash
sudo systemctl stop librasense.service
sudo tar -C /home/librasense/data -xzf /home/librasense/backups/<file>.tar.gz
sudo chown -R 10001:10001 /home/librasense/data
sudo systemctl start librasense.service
```

Copy backups off the host. A backup that lives only on the same disk is not a
backup.

---

## 13. Troubleshooting

| Symptom | Likely cause | Action |
| :--- | :--- | :--- |
| SSH ke `vps-librasense` ditolak | Belum provisioning, key tidak tersalin, atau `AllowUsers`/`AllowGroups` di sshd | Cek `ssh -v`, jalankan ulang `setup-user.sh`, periksa `/etc/ssh/sshd_config.d/` |
| Unit restart loop, `image not known` | Image never loaded or wrong tag | `sudo podman images`, rerun `deploy.ps1` |
| Unit fails with `address already in use` | 8097 taken | `ss -ltnp`, resolve owner, do not change the port silently |
| Container exits at start | Missing or invalid env | `sudo journalctl -u librasense -n 100 --no-pager`, check `DEVICE_TOKEN` length and Access vars |
| `konfigurasi: CF_ACCESS... wajib diisi` | Env incomplete in production | Fill both vars, `ACCESS_DEV_BYPASS=false` |
| Panel returns 401 for a logged in user | AUD mismatch or JWKS unreachable | Compare `CF_ACCESS_AUD` with the application, check outbound HTTPS |
| Device gets an Access login page | Access application covers the device hostname | Remove the application from that hostname |
| Device 401 | Token mismatch | Compare `DEVICE_TOKEN` with firmware `LIBRASENSE_DEVICE_TOKEN` |
| Panel loads but stays offline | No device ingest in 30 s | Check firmware WiFi, API base, and time sync |
| Data directory write error | Host `data/` not owned by 10001 | `sudo chown -R 10001:10001 /home/librasense/data` |
| Memory cap hit | History query or leak | `sudo podman stats --no-stream`, inspect `journalctl`, raise cap only within budget |
| `podman ps -a --format` fails | Podman 4.9.3 argument handling | Drop the flag, see section 3 |
| Unit active but container gone after reboot | Unit was never enabled | `systemctl is-enabled librasense.service` must say `enabled` |

---

## 14. Capacity Budget

| Resource | Total | Available at check | LibraSense cap |
| :--- | :--- | :--- | :--- |
| RAM | 1967 MB | 1384 MB | 128 MB |
| Swap | 1987 MB | 1985 MB | 160 MB combined cap |
| Disk `/` | 40 G | 25 G | image about 20 MB, data grows slowly |

Rules:

- Keep the container under 128 MB. Measure with `sudo podman stats --no-stream`.
- Swap is a safety net, not capacity.
- `readings` has no retention yet. Check `du -sh /home/librasense/data` monthly
  and prune only on explicit request.
- Re-check `free -m` and `df -h /` before adding anything else to the host.

---

## 15. Reference

- `references/vps-access.md`: access baseline, port map, verification commands.
- `deploy/README.md`: short operator summary.
- `deploy/setup-user.sh`: provisioning script.
- `deploy/build.ps1`, `deploy/deploy.ps1`, `deploy/smoke.ps1`,
  `deploy/check-vps.ps1`: workstation automation.
- `docs/003-deploy-vps-19-nov-2026.md`: decisions and open items.
