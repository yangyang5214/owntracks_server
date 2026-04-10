#!/usr/bin/env bash
set -euo pipefail

INSTALL_DIR="${INSTALL_DIR:-/opt/owntracks}"
SERVICE="${SERVICE:-owntracks-server.service}"
BINARY_NAME="${BINARY_NAME:-owntracks_server}"

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(git -C "$SCRIPT_DIR" rev-parse --show-toplevel)"

SUDO=()
if [[ "$(id -u)" -ne 0 ]]; then
  SUDO=(sudo)
fi

cd "$REPO_ROOT"

if command -v task >/dev/null 2>&1; then
  task build
else
  GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -ldflags="-s -w" -trimpath -o "$BINARY_NAME" ./cmd/server
fi

"${SUDO[@]}" mkdir -p "$INSTALL_DIR"

unit="/etc/systemd/system/$SERVICE"
if [[ -f "$unit" ]] || "${SUDO[@]}" test -f "$unit"; then
  "${SUDO[@]}" systemctl stop "$SERVICE" || true
fi

"${SUDO[@]}" install -m 755 "$BINARY_NAME" "$INSTALL_DIR/$BINARY_NAME"

if [[ -f "$unit" ]] || "${SUDO[@]}" test -f "$unit"; then
  "${SUDO[@]}" systemctl start "$SERVICE"
  echo "已安装 $INSTALL_DIR/$BINARY_NAME 并启动 $SERVICE"
else
  echo "已安装 $INSTALL_DIR/$BINARY_NAME（未检测到 $unit，未执行 systemctl）" >&2
fi
