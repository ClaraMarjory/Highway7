#!/bin/bash
# Highway7 Uninstaller
# Usage: bash uninstall.sh

set -e

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

INSTALL_DIR="/opt/highway7"
SERVICE_NAME="highway7"

info()  { echo -e "${GREEN}[INFO]${NC} $1"; }
warn()  { echo -e "${YELLOW}[WARN]${NC} $1"; }

[ "$(id -u)" -ne 0 ] && { echo -e "${RED}[ERROR]${NC} Please run as root"; exit 1; }

# 避免卸载时当前目录被删导致getcwd报错
cd /root

echo ""
echo "  Highway7 Uninstaller"
echo "  ===================="
echo ""

# Stop service if running
if systemctl is-active --quiet ${SERVICE_NAME} 2>/dev/null; then
    info "Stopping service..."
    systemctl stop ${SERVICE_NAME}
fi

# Disable and remove service
if [ -f /etc/systemd/system/${SERVICE_NAME}.service ]; then
    info "Removing systemd service..."
    systemctl disable ${SERVICE_NAME} 2>/dev/null || true
    rm -f /etc/systemd/system/${SERVICE_NAME}.service
    systemctl daemon-reload
fi

# Kill any remaining process
pkill -f "${INSTALL_DIR}/highway" 2>/dev/null || true

# Ask about data
if [ -d "${INSTALL_DIR}/data" ]; then
    read -p "Delete database and data? (y/N): " DEL_DATA < /dev/tty
    if [ "$DEL_DATA" = "y" ] || [ "$DEL_DATA" = "Y" ]; then
        info "Removing all files including data..."
        rm -rf ${INSTALL_DIR}
    else
        info "Keeping data directory, removing everything else..."
        find ${INSTALL_DIR} -mindepth 1 -not -path "${INSTALL_DIR}/data*" -delete
    fi
else
    rm -rf ${INSTALL_DIR}
fi

echo ""
info "========================================="
info "  Highway7 uninstalled."
info "  iptables rules NOT touched."
info "  Run 'iptables -t nat -F' to flush if needed."
info "========================================="
