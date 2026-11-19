#!/usr/bin/env bash
# ==============================================================================
# LibraSense - VPS service account provisioning
#
# Creates the dedicated service account that owns the LibraSense container,
# installs the SSH key, builds the directory tree, and writes scoped sudo
# rights. It does not start the service and does not touch other projects on
# the host.
#
# Idempotent: safe to run repeatedly. Existing users, keys, and directories
# are never recreated or truncated.
#
# Usage from the workstation, through the bootstrap admin account:
#   scp deploy/setup-user.sh vps-lama:/tmp/librasense-setup.sh
#   ssh vps-lama 'sudo bash /tmp/librasense-setup.sh; rm -f /tmp/librasense-setup.sh'
#
# Optional arguments:
#   --pubkey-file <path>      Public key to authorize (default: admin key)
#   --pubkey "<ssh-ed25519 ...>"  Inline public key
#   --admin-user <name>       Account whose authorized_keys is copied
#   --user <name>             Service account (default: librasense)
#   --data-uid <uid>          Numeric owner of the data directory (default: 10001)
#   --sudo-mode scoped|full   scoped (default) or full NOPASSWD
#   --dry-run                 Print actions without applying changes
#
# The container runs as uid 10001 so the SQLite files are not owned by root.
# Keep that uid in sync with deploy/Containerfile.
# ==============================================================================

set -euo pipefail

SERVICE_USER="librasense"
SERVICE_HOME=""
SERVICE_GROUP=""
DATA_UID="10001"
ADMIN_USER="${SUDO_USER:-}"
PUBKEY_FILE=""
PUBKEY_INLINE=""
SUDO_MODE="scoped"
DRY_RUN="0"

log()  { printf '[librasense-setup] %s\n' "$*"; }
warn() { printf '[librasense-setup] WARN: %s\n' "$*" >&2; }
fail() { printf '[librasense-setup] ERROR: %s\n' "$*" >&2; exit 1; }

usage() {
    awk 'NR==1 { next } /^#/ { sub(/^# ?/, ""); print; next } { exit }' "$0"
    exit 0
}

while [[ $# -gt 0 ]]; do
    case "$1" in
        --pubkey-file) PUBKEY_FILE="${2:-}"; shift 2 ;;
        --pubkey)      PUBKEY_INLINE="${2:-}"; shift 2 ;;
        --admin-user)  ADMIN_USER="${2:-}"; shift 2 ;;
        --user)        SERVICE_USER="${2:-}"; shift 2 ;;
        --data-uid)    DATA_UID="${2:-}"; shift 2 ;;
        --sudo-mode)   SUDO_MODE="${2:-}"; shift 2 ;;
        --dry-run)     DRY_RUN="1"; shift ;;
        -h|--help)     usage ;;
        *) fail "unknown argument: $1" ;;
    esac
done

[[ "$(id -u)" -eq 0 ]] || fail "must run as root (use sudo)"
[[ "$SERVICE_USER" =~ ^[a-z_][a-z0-9_-]{0,31}$ ]] || fail "invalid user name: $SERVICE_USER"
[[ "$DATA_UID" =~ ^[0-9]+$ ]] || fail "invalid data uid: $DATA_UID"
[[ "$SUDO_MODE" == "scoped" || "$SUDO_MODE" == "full" ]] || fail "--sudo-mode must be scoped or full"

command -v useradd >/dev/null 2>&1 || fail "useradd not found"
command -v systemctl >/dev/null 2>&1 || fail "systemctl not found"
command -v visudo >/dev/null 2>&1 || fail "visudo not found"

SERVICE_HOME="/home/${SERVICE_USER}"

run() {
    if [[ "$DRY_RUN" == "1" ]]; then
        printf '[librasense-setup] DRY: %s\n' "$*"
    else
        "$@"
    fi
}

# ------------------------------------------------------------------------------
# 1. Service account
# ------------------------------------------------------------------------------
if id -u "$SERVICE_USER" >/dev/null 2>&1; then
    log "user ${SERVICE_USER} already exists, leaving it untouched"
    SERVICE_GROUP="$(id -gn "$SERVICE_USER")"
else
    log "creating service account ${SERVICE_USER}"
    run useradd --create-home --home-dir "$SERVICE_HOME" --shell /bin/bash \
        --comment "LibraSense service account" "$SERVICE_USER"
    run passwd --lock "$SERVICE_USER" >/dev/null
    SERVICE_GROUP="$SERVICE_USER"
fi

[[ -n "$SERVICE_GROUP" ]] || SERVICE_GROUP="$SERVICE_USER"
run install -d -m 0750 -o "$SERVICE_USER" -g "$SERVICE_GROUP" "$SERVICE_HOME"

# ------------------------------------------------------------------------------
# 2. SSH access
# ------------------------------------------------------------------------------
SSH_DIR="${SERVICE_HOME}/.ssh"
AUTH_KEYS="${SSH_DIR}/authorized_keys"
RESOLVED_KEY=""

if [[ -n "$PUBKEY_INLINE" ]]; then
    RESOLVED_KEY="$PUBKEY_INLINE"
elif [[ -n "$PUBKEY_FILE" ]]; then
    [[ -r "$PUBKEY_FILE" ]] || fail "cannot read pubkey file: $PUBKEY_FILE"
    RESOLVED_KEY="$(tr -d '\r\n' < "$PUBKEY_FILE")"
elif [[ -n "$ADMIN_USER" && -r "/home/${ADMIN_USER}/.ssh/authorized_keys" ]]; then
    log "copying keys from /home/${ADMIN_USER}/.ssh/authorized_keys"
    RESOLVED_KEY="$(grep -Ev '^[[:space:]]*(#|$)' "/home/${ADMIN_USER}/.ssh/authorized_keys")"
elif [[ -r /root/.ssh/authorized_keys ]]; then
    log "copying keys from /root/.ssh/authorized_keys"
    RESOLVED_KEY="$(grep -Ev '^[[:space:]]*(#|$)' /root/.ssh/authorized_keys)"
fi

run install -d -m 0700 -o "$SERVICE_USER" -g "$SERVICE_GROUP" "$SSH_DIR"

if [[ -n "$RESOLVED_KEY" ]]; then
    if [[ "$DRY_RUN" == "1" ]]; then
        log "DRY: would append resolved public key to ${AUTH_KEYS}"
        log "DRY: would set ${AUTH_KEYS} to 0600 ${SERVICE_USER}:${SERVICE_GROUP}"
    else
        touch "$AUTH_KEYS"
        while IFS= read -r key; do
            [[ -z "$key" ]] && continue
            if ! grep -qF "$key" "$AUTH_KEYS"; then
                printf '%s\n' "$key" >> "$AUTH_KEYS"
                log "authorized key added"
            fi
        done <<< "$RESOLVED_KEY"
        chown "$SERVICE_USER:$SERVICE_GROUP" "$AUTH_KEYS"
        chmod 0600 "$AUTH_KEYS"
    fi
else
    warn "no public key resolved, SSH login for ${SERVICE_USER} will not work"
fi

# ------------------------------------------------------------------------------
# 3. Directory tree
# ------------------------------------------------------------------------------
log "directory tree under ${SERVICE_HOME}"
run install -d -m 0750 -o "$SERVICE_USER" -g "$SERVICE_GROUP" "${SERVICE_HOME}/bin"
run install -d -m 0750 -o "$SERVICE_USER" -g "$SERVICE_GROUP" "${SERVICE_HOME}/backups"
# Numeric owner on purpose: the container runs as this uid.
run install -d -m 0700 -o "$DATA_UID" -g "$DATA_UID" "${SERVICE_HOME}/data"

if [[ -f "${SERVICE_HOME}/.env" ]]; then
    log ".env exists, leaving it untouched"
    run chmod 0600 "${SERVICE_HOME}/.env"
    run chown "$SERVICE_USER:$SERVICE_GROUP" "${SERVICE_HOME}/.env"
fi

# ------------------------------------------------------------------------------
# 4. Sudo scope
# ------------------------------------------------------------------------------
SUDOERS_FILE="/etc/sudoers.d/90-librasense"
SUDOERS_TMP="$(mktemp)"

if [[ "$SUDO_MODE" == "full" ]]; then
    printf '%s ALL=(ALL) NOPASSWD: ALL\n' "$SERVICE_USER" > "$SUDOERS_TMP"
else
    # No Cmnd_Alias on purpose. This host loads /etc/sudoers.d twice
    # (@includedir and #includedir), so aliases in a drop-in are reported as
    # duplicates. Explicit command lists are idempotent and keep sudo quiet.
    cat > "$SUDOERS_TMP" <<EOF
${SERVICE_USER} ALL=(root) NOPASSWD: /usr/bin/podman, \\
    /usr/bin/systemctl start librasense.service, \\
    /usr/bin/systemctl stop librasense.service, \\
    /usr/bin/systemctl restart librasense.service, \\
    /usr/bin/systemctl status librasense.service, \\
    /usr/bin/systemctl is-active librasense.service, \\
    /usr/bin/systemctl enable librasense.service, \\
    /usr/bin/systemctl disable librasense.service, \\
    /usr/bin/systemctl daemon-reload, \\
    /usr/bin/journalctl -u librasense.service, \\
    /usr/bin/journalctl -u librasense.service -n 100 --no-pager, \\
    /usr/bin/install -m 0644 /tmp/librasense.service /etc/systemd/system/librasense.service
EOF
fi

if [[ "$DRY_RUN" == "1" ]]; then
    log "DRY: would install ${SUDOERS_FILE} with sudo mode ${SUDO_MODE}"
else
    visudo -cf "$SUDOERS_TMP" >/dev/null || { rm -f "$SUDOERS_TMP"; fail "generated sudoers file is invalid"; }
    install -m 0440 -o root -g root "$SUDOERS_TMP" "$SUDOERS_FILE"
    log "sudo scope installed at ${SUDOERS_FILE} (${SUDO_MODE})"
fi
rm -f "$SUDOERS_TMP"

# ------------------------------------------------------------------------------
# 5. Summary
# ------------------------------------------------------------------------------
log "done"
printf '\nNext steps:\n'
printf '  1. Create %s/.env from deploy/.env.example and set DEVICE_TOKEN, CF_ACCESS_TEAM_DOMAIN, CF_ACCESS_AUD.\n' "$SERVICE_HOME"
printf '  2. Build and push the image from the workstation: .\\deploy\\deploy.ps1\n'
printf '  3. Check the tunnel: cloudflared-claw.service must be active, then add both hostnames in the Zero Trust dashboard.\n'
