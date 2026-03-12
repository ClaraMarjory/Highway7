#!/bin/bash
# Highway7 Installer
# Usage: bash <(curl -sL https://raw.githubusercontent.com/ClaraMarjory/Highway7/main/scripts/install.sh)

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
echo "  Highway7 Installer"
echo "  ==================="
echo ""

# Check root
[ "$(id -u)" -ne 0 ] && error "Please run as root"
cd /root

# Check OS and package manager
if [ -f /etc/debian_version ]; then
    PM="apt-get"
elif [ -f /etc/redhat-release ]; then
    PM="yum"
else
    error "Unsupported OS. Debian/Ubuntu/CentOS only."
fi

# Install dependencies
info "Checking dependencies..."
for cmd in curl wget iptables; do
    if ! command -v $cmd &>/dev/null; then
        info "Installing $cmd..."
        $PM install -y -qq $cmd &>/dev/null
    fi
done

# Enable IP forwarding
info "Enabling IP forwarding..."
sysctl -w net.ipv4.ip_forward=1 &>/dev/null
grep -q "net.ipv4.ip_forward=1" /etc/sysctl.conf 2>/dev/null || echo "net.ipv4.ip_forward=1" >> /etc/sysctl.conf

# Install iptables-persistent (Debian/Ubuntu)
if [ "$PM" = "apt-get" ]; then
    if ! dpkg -l 2>/dev/null | grep -q iptables-persistent; then
        info "Installing iptables-persistent..."
        echo iptables-persistent iptables-persistent/autosave_v4 boolean true | debconf-set-selections 2>/dev/null
        echo iptables-persistent iptables-persistent/autosave_v6 boolean true | debconf-set-selections 2>/dev/null
        apt-get install -y -qq iptables-persistent &>/dev/null
    fi
fi

# Ensure MASQUERADE
iptables -t nat -S POSTROUTING 2>/dev/null | grep -q MASQUERADE || {
    info "Adding MASQUERADE rule..."
    iptables -t nat -A POSTROUTING -j MASQUERADE
}

# Stop old service if exists
if systemctl is-active --quiet ${SERVICE_NAME} 2>/dev/null; then
    info "Stopping existing Highway7..."
    systemctl stop ${SERVICE_NAME}
fi
pkill -f "${INSTALL_DIR}/highway" 2>/dev/null || true

# Get latest version
info "Getting latest release..."
LATEST=$(curl -sL "https://api.github.com/repos/${REPO}/releases/latest" 2>/dev/null | grep tag_name | head -1 | cut -d'"' -f4)
[ -z "$LATEST" ] && error "Cannot get latest version. Check network."
info "Latest version: $LATEST"

# Create directories
mkdir -p ${INSTALL_DIR}/web/static ${INSTALL_DIR}/data

# Download binary
info "Downloading highway binary..."
DL_URL="https://github.com/${REPO}/releases/download/${LATEST}/highway-linux-amd64"
curl -sL "$DL_URL" -o ${INSTALL_DIR}/highway || error "Download binary failed"
chmod +x ${INSTALL_DIR}/highway

# Download frontend
info "Downloading frontend..."
curl -sL "https://raw.githubusercontent.com/${REPO}/main/web/static/index.html" -o ${INSTALL_DIR}/web/static/index.html || error "Download frontend failed"

# Download uninstall script
curl -sL "https://raw.githubusercontent.com/${REPO}/main/scripts/uninstall.sh" -o ${INSTALL_DIR}/uninstall.sh 2>/dev/null
chmod +x ${INSTALL_DIR}/uninstall.sh 2>/dev/null

# Verify binary
${INSTALL_DIR}/highway -version 2>/dev/null || error "Binary verification failed"
info "Binary OK: $(${INSTALL_DIR}/highway -version 2>&1)"

# Create systemd service
info "Creating systemd service..."
cat > /etc/systemd/system/${SERVICE_NAME}.service << EOF
[Unit]
Description=Highway7 - AI Infrastructure Panel
After=network.target

[Service]
Type=simple
WorkingDirectory=${INSTALL_DIR}
ExecStart=${INSTALL_DIR}/highway -port 8888
Restart=on-failure
RestartSec=5
LimitNOFILE=65536

[Install]
WantedBy=multi-user.target
EOF

systemctl daemon-reload
systemctl enable ${SERVICE_NAME} &>/dev/null

# Set password (only if no existing database)
if [ ! -f "${INSTALL_DIR}/data/highway.db" ]; then
    echo ""
    echo -e "${YELLOW}设置管理密码（明文显示，请确认输入正确）${NC}"
    read -p "输入密码: " ADMIN_PASS < /dev/tty
    read -p "再输一次: " ADMIN_PASS2 < /dev/tty
    [ -z "$ADMIN_PASS" ] && error "密码不能为空"
    [ "$ADMIN_PASS" != "$ADMIN_PASS2" ] && error "两次密码不一致"
    cd ${INSTALL_DIR} && ./highway -pass "$ADMIN_PASS" 2>/dev/null || true
    info "密码已设置: $ADMIN_PASS"
else
    info "Existing database found, keeping current password."
fi

# Start service
systemctl start ${SERVICE_NAME}
sleep 1

if systemctl is-active --quiet ${SERVICE_NAME}; then
    IP=$(curl -s4 ip.sb 2>/dev/null || hostname -I | awk '{print $1}')
    echo ""
    info "========================================="
    info "  Highway7 ${LATEST} installed!"
    info "  Panel:     http://${IP}:8888"
    info "  Service:   systemctl status highway7"
    info "  Uninstall: bash ${INSTALL_DIR}/uninstall.sh"
    info "========================================="
else
    error "Service failed to start. Check: journalctl -u highway7"
fi
