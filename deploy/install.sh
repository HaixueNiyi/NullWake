#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"
INSTALL_BIN="/usr/local/bin/nullwake-agent"
CONFIG_DIR="/etc/nullwake"
CONFIG_FILE="${CONFIG_DIR}/config.toml"
SERVICE_DST="/etc/systemd/system/nullwake-agent.service"

if [[ "${EUID}" -ne 0 ]]; then
  echo "Please run as root (e.g. sudo ./deploy/install.sh)" >&2
  exit 1
fi

detect_distro() {
  if [[ -r /etc/os-release ]]; then
    # shellcheck disable=SC1091
    . /etc/os-release
    echo "${ID:-unknown}"
  else
    echo "unknown"
  fi
}

DISTRO="$(detect_distro)"
echo "Detected Linux distribution: ${DISTRO}"

case "${DISTRO}" in
  ubuntu|debian)
    echo "Checking dependencies (ca-certificates, dbus)..."
    if ! command -v go >/dev/null 2>&1; then
      echo "Go toolchain not found. Install Go or build agent binary first." >&2
    fi
    ;;
  arch|endeavouros|manjaro)
    echo "Arch-based distro detected."
    if ! command -v go >/dev/null 2>&1; then
      echo "Hint: install Go with 'pacman -S go' if building from source." >&2
    fi
    ;;
  *)
    echo "Generic Linux distro (${DISTRO}); proceeding with standard systemd installation."
    ;;
esac

# Build static single binary (CGO_ENABLED=0) when Go is installed.
if command -v go >/dev/null 2>&1; then
  echo "Building static nullwake-agent binary (CGO_ENABLED=0)..."
  (
    cd "${REPO_ROOT}/agent"
    CGO_ENABLED=0 go build -ldflags="-s -w" -o "${INSTALL_BIN}" ./cmd/agent
  )
  chmod 0755 "${INSTALL_BIN}"
else
  echo "Error: Go toolchain is required to build from source." >&2
  exit 1
fi

# Create dedicated unprivileged service account if absent.
if ! id -u nullwake >/dev/null 2>&1; then
  useradd --system --no-create-home --shell /usr/sbin/nologin nullwake
fi

# Add service user to video/render groups when present for DRM sysfs access.
for grp in video render; do
  if getent group "${grp}" >/dev/null 2>&1; then
    usermod -aG "${grp}" nullwake || true
  fi
done

mkdir -p "${CONFIG_DIR}" /var/lib/nullwake
chown -R nullwake:nullwake /var/lib/nullwake
chmod 0700 /var/lib/nullwake

if [[ ! -f "${CONFIG_FILE}" ]]; then
  cp "${REPO_ROOT}/agent/config.example.toml" "${CONFIG_FILE}"
  # Route push socket into RuntimeDirectory=nullwake so unprivileged User=nullwake owns it;
  # systemd ExecStartPre links /run/nullwake.sock -> /run/nullwake/nullwake.sock.
  sed -i 's|path = "/run/nullwake.sock"|path = "/run/nullwake/nullwake.sock"|' "${CONFIG_FILE}"
  chown root:nullwake "${CONFIG_FILE}"
  chmod 0640 "${CONFIG_FILE}"
  echo "Created ${CONFIG_FILE} from config.example.toml"
else
  echo "Keeping existing ${CONFIG_FILE}"
fi

cp "${SCRIPT_DIR}/nullwake-agent.service" "${SERVICE_DST}"
chmod 0644 "${SERVICE_DST}"

systemctl daemon-reload
systemctl enable nullwake-agent.service

cat <<'EOF'

NullWake Agent installed successfully!

Next steps:
1. Edit /etc/nullwake/config.toml (set hostname, optional auth_key, and [[services]]).
2. Authenticate Tailscale on first run:
   - Either set auth_key in /etc/nullwake/config.toml (or TS_AUTHKEY), OR
   - Run: sudo systemctl start nullwake-agent && journalctl -u nullwake-agent -f
     and open the printed Tailscale login URL in your browser.
3. Optional: test pushing an event to the in-memory ring buffer:
   echo '{"service_id":"backup","event":"Manual test event"}' | nc -U -N /run/nullwake.sock
EOF
