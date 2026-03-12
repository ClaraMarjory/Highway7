#!/bin/bash
# Highway7 Updater
# Usage: bash <(curl -sL https://raw.githubusercontent.com/ClaraMarjory/Highway7/main/scripts/update.sh)

set -e

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

INSTALL_DIR="/opt/highway7"
SERVICE_NAME="highway7"
REPO="ClaraMarjory/Highway7"

info()  { echo -e "${GREEN}[INFO]${NC} $1"; }
warn()  { echo -e "${YELLOW}[WARN]${NC} $1"; }
error() { echo -e "${RED}[ERROR]${NC} $1"; exit 1; }

echo ""
echo "  Highway7 Updater"
echo "  ================="
echo ""

[ "$(id -u)" -ne 0 ] && error "Please run as root"
cd /root
[ ! -d "${INSTALL_DIR}" ] && error "Highway7 not installed. Run install.sh first."

# Current version
CUR_VER=$(${INSTALL_DIR}/highway -version 2>&1 | grep -o 'v[0-9.]*' || echo "unknown")
info "Current version: $CUR_VER"

# Get latest
LATEST=$(curl -sL "https://api.github.com/repos/${REPO}/releases/latest" 2>/dev/null | grep tag_name | head -1 | cut -d'"' -f4)
[ -z "$LATEST" ] && error "Cannot get latest version. Check network."
info "Latest version:  $LATEST"

if [ "$CUR_VER" = "$LATEST" ]; then
    info "Already up to date."
    exit 0
fi

# Stop service
info "Stopping service..."
systemctl stop ${SERVICE_NAME} 2>/dev/null || pkill -f "${INSTALL_DIR}/highway" 2>/dev/null || true
sleep 1

# Backup current binary
cp ${INSTALL_DIR}/highway ${INSTALL_DIR}/highway.bak 2>/dev/null || true

# Download new binary
info "Downloading ${LATEST}..."
DL_URL="https://github.com/${REPO}/releases/download/${LATEST}/highway-linux-amd64"
curl -sL "$DL_URL" -o ${INSTALL_DIR}/highway.new || error "Download failed"
chmod +x ${INSTALL_DIR}/highway.new

# Verify new binary
${INSTALL_DIR}/highway.new -version 2>/dev/null || {
    warn "New binary verification failed, rolling back..."
    mv ${INSTALL_DIR}/highway.bak ${INSTALL_DIR}/highway
    systemctl start ${SERVICE_NAME}
    error "Update failed, rolled back to ${CUR_VER}"
}

# Replace binary
mv ${INSTALL_DIR}/highway.new ${INSTALL_DIR}/highway
rm -f ${INSTALL_DIR}/highway.bak

# Update frontend
info "Updating frontend..."
curl -sL "https://raw.githubusercontent.com/${REPO}/main/web/static/index.html" -o ${INSTALL_DIR}/web/static/index.html || warn "Frontend update failed, keeping old version"

# Update uninstall script
curl -sL "https://raw.githubusercontent.com/${REPO}/main/scripts/uninstall.sh" -o ${INSTALL_DIR}/uninstall.sh 2>/dev/null
chmod +x ${INSTALL_DIR}/uninstall.sh 2>/dev/null

# Restart
info "Starting service..."
systemctl start ${SERVICE_NAME}
sleep 1

if systemctl is-active --quiet ${SERVICE_NAME}; then
    NEW_VER=$(${INSTALL_DIR}/highway -version 2>&1 | grep -o 'v[0-9.]*' || echo "$LATEST")
    echo ""
    info "========================================="
    info "  Updated: ${CUR_VER} => ${NEW_VER}"
    info "  Data preserved, password unchanged."
    info "========================================="
else
    error "Service failed to start. Check: journalctl -u highway7"
fi
