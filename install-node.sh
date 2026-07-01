#!/bin/bash
set -euo pipefail

# x-ui-node installer: sets up the 3x-ui pull-mode node agent as a systemd
# service. The agent dials out to the master panel, pulls its rendered Xray
# config, and reports traffic back — it never exposes an inbound API port.
#
# Non-interactive usage (the master panel generates this command with the
# node's guid and token filled in):
#   bash <(curl -Ls https://raw.githubusercontent.com/Wangin1996/3x-ui/main/install-node.sh) \
#     --master https://panel.example.com:2053 --guid <guid> --token <token>
#
# Flags (or the matching XUI_NODE_* env vars):
#   --master URL    master panel base URL           (XUI_NODE_MASTER_URL)
#   --guid GUID     this node's guid                 (XUI_NODE_GUID)
#   --token TOKEN   this node's per-node API token   (XUI_NODE_API_TOKEN)
#   --repo OWNER/R  release source repo              (default Wangin1996/3x-ui)
#   --release TAG   release tag to install           (default dev-latest)
#   --insecure      skip master TLS verification (self-signed test panels)

RED='\033[0;31m'; GREEN='\033[0;32m'; YELLOW='\033[0;33m'; PLAIN='\033[0m'
err() { echo -e "${RED}$*${PLAIN}" >&2; }
info() { echo -e "${YELLOW}$*${PLAIN}"; }
ok() { echo -e "${GREEN}$*${PLAIN}"; }

MASTER="${XUI_NODE_MASTER_URL:-}"
GUID="${XUI_NODE_GUID:-}"
TOKEN="${XUI_NODE_API_TOKEN:-}"
REPO="${XUI_NODE_REPO:-Wangin1996/3x-ui}"
RELEASE_TAG="${XUI_NODE_RELEASE:-dev-latest}"
INSECURE="${XUI_NODE_TLS_SKIP_VERIFY:-}"

while [[ $# -gt 0 ]]; do
  case "$1" in
    --master) MASTER="$2"; shift 2 ;;
    --guid) GUID="$2"; shift 2 ;;
    --token) TOKEN="$2"; shift 2 ;;
    --repo) REPO="$2"; shift 2 ;;
    --release) RELEASE_TAG="$2"; shift 2 ;;
    --insecure) INSECURE="true"; shift ;;
    *) err "unknown argument: $1"; exit 1 ;;
  esac
done

[[ $EUID -ne 0 ]] && { err "please run as root (sudo)"; exit 1; }
if [[ -z "$MASTER" || -z "$GUID" || -z "$TOKEN" ]]; then
  err "--master, --guid and --token are required"
  echo "usage: install-node.sh --master https://panel:2053 --guid <guid> --token <token> [--insecure]"
  exit 1
fi

arch=$(uname -m)
case "$arch" in
  x86_64|amd64) PLATFORM="amd64" ;;
  aarch64|arm64) PLATFORM="arm64" ;;
  armv7l) PLATFORM="armv7" ;;
  armv6l) PLATFORM="armv6" ;;
  armv5*) PLATFORM="armv5" ;;
  i386|i686) PLATFORM="386" ;;
  s390x) PLATFORM="s390x" ;;
  *) err "unsupported architecture: $arch"; exit 1 ;;
esac

INSTALL_DIR="/usr/local/x-ui"
URL="https://github.com/${REPO}/releases/download/${RELEASE_TAG}/x-ui-linux-${PLATFORM}.tar.gz"

info "Downloading ${URL}"
tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT
curl -fL -o "$tmp/x-ui.tar.gz" "$URL"
tar -xzf "$tmp/x-ui.tar.gz" -C "$tmp"
if [[ ! -x "$tmp/x-ui/x-ui-node" ]]; then
  err "x-ui-node not found in the release package (need a build that bundles the agent)"
  exit 1
fi

systemctl stop x-ui-node 2>/dev/null || true
mkdir -p "$INSTALL_DIR/bin"
cp "$tmp/x-ui/x-ui-node" "$INSTALL_DIR/x-ui-node"
chmod +x "$INSTALL_DIR/x-ui-node"
cp -r "$tmp/x-ui/bin/." "$INSTALL_DIR/bin/"
chmod +x "$INSTALL_DIR/bin/"xray-* 2>/dev/null || true

umask 077
{
  echo "XUI_NODE_MASTER_URL=${MASTER}"
  echo "XUI_NODE_GUID=${GUID}"
  echo "XUI_NODE_API_TOKEN=${TOKEN}"
  echo "XUI_BIN_FOLDER=${INSTALL_DIR}/bin"
  [[ -n "$INSECURE" ]] && echo "XUI_NODE_TLS_SKIP_VERIFY=true"
} > /etc/default/x-ui-node

cat > /etc/systemd/system/x-ui-node.service <<EOF
[Unit]
Description=x-ui-node (3x-ui pull-mode node agent)
After=network.target
Wants=network.target

[Service]
Type=simple
EnvironmentFile=/etc/default/x-ui-node
ExecStart=${INSTALL_DIR}/x-ui-node
Restart=on-failure
RestartSec=5
LimitNOFILE=1048576

[Install]
WantedBy=multi-user.target
EOF

systemctl daemon-reload
systemctl enable x-ui-node >/dev/null 2>&1 || true
systemctl restart x-ui-node
sleep 1

if systemctl is-active --quiet x-ui-node; then
  ok "x-ui-node installed and running."
else
  err "x-ui-node failed to start — check: journalctl -u x-ui-node -e"
fi
echo "  config:  /etc/default/x-ui-node"
echo "  logs:    journalctl -u x-ui-node -f"
