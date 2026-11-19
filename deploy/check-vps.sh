#!/usr/bin/env bash
# Read-only VPS preflight for LibraSense.
# Run as the bootstrap admin account. No sudo, no writes.
# Usage: bash check-vps.sh

echo "== identitas =="
id
uname -a
. /etc/os-release 2>/dev/null && echo "os=${PRETTY_NAME:-unknown}"

echo
echo "== kapasitas =="
free -m
swapon --show || true
df -h /

echo
echo "== tooling =="
command -v podman >/dev/null && podman --version || echo "podman absen"
systemctl --version | head -n 1
command -v curl >/dev/null && echo "curl ada" || echo "curl absen"
command -v sqlite3 >/dev/null && echo "sqlite3 ada" || echo "sqlite3 absen"

echo
echo "== unit terkait =="
for unit in cloudflared-claw.service librasense.service medvault-pod.service; do
    state="$(systemctl is-active "$unit" 2>/dev/null || true)"
    enabled="$(systemctl is-enabled "$unit" 2>/dev/null || true)"
    printf '%-28s active=%-10s enabled=%s\n' "$unit" "${state:-unknown}" "${enabled:-unknown}"
done

echo
echo "== akun dan direktori =="
getent passwd librasense || echo "user librasense belum ada"
getent group librasense || echo "grup librasense belum ada"
ls -ld /home/librasense /home/librasense/bin /home/librasense/data /home/librasense/backups 2>/dev/null || true
if [ -f /home/librasense/.env ]; then
    stat -c 'env mode=%a owner=%U group=%G' /home/librasense/.env
else
    echo "env belum ada"
fi
if [ -f /etc/systemd/system/librasense.service ]; then
    echo "unit file ada"
else
    echo "unit file belum ada"
fi

echo
echo "== port 8088 sampai 8100 =="
ss -ltn | awk 'NR==1 || /:(8088|8089|809[0-9]|8100)[[:space:]]/'
if ss -ltn | grep -qE ':8097[[:space:]]'; then
    echo "8097 DIPAKAI, pilih port lain dan perbarui deploy/systemd/librasense.service"
else
    echo "8097 bebas"
fi

echo
echo "== listener loopback =="
ss -ltn | grep '127.0.0.1' || true

echo
echo "== selesai =="
