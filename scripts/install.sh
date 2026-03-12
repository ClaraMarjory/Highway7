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

# Check root
[ "$(id -u)" -ne 0 ] && error "Please run as root"

# Check OS
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
        $PM install -y $cmd &>/dev/null
    fi
done

# Enable IP forwarding
info "Enabling IP forwarding..."
sysctl -w net.ipv4.ip_forward=1 &>/dev/null
grep -q "net.ipv4.ip_forward=1" /etc/sysctl.conf || echo "net.ipv4.ip_forward=1" >> /etc/sysctl.conf

# Install iptables-persistent
if [ "$PM" = "apt-get" ]; then
    if ! dpkg -l | grep -q iptables-persistent; then
        info "Installing iptables-persistent..."
        echo iptables-persistent iptables-persistent/autosave_v4 boolean true | debconf-set-selections
        echo iptables-persistent iptables-persistent/autosave_v6 boolean true | debconf-set-selections
        apt-get install -y iptables-persistent &>/dev/null
    fi
fi

# Ensure MASQUERADE
iptables -t nat -S POSTROUTING 2>/dev/null | grep -q MASQUERADE || {
    info "Adding MASQUERADE rule..."
    iptables -t nat -A POSTROUTING -j MASQUERADE
}

# Download binary
info "Downloading Highway7..."
mkdir -p $INSTALL_DIR/web/static $INSTALL_DIR/data

LATEST=$(curl -sL "https://api.github.com/repos/${REPO}/releases/latest" | grep tag_name | head -1 | cut -d'"' -f4)
[ -z "$LATEST" ] && error "Cannot get latest version"

DL_URL="https://github.com/${REPO}/releases/download/${LATEST}/highway-linux-amd64"
curl -sL "$DL_URL" -o $INSTALL_DIR/highway || error "Download failed"
chmod +x $INSTALL_DIR/highway

# Download web UI
curl -sL "https://raw.githubusercontent.com/${REPO}/main/web/static/index.html" -o $INSTALL_DIR/web/static/index.html

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
systemctl enable ${SERVICE_NAME}

# Set password
echo ""
read -p "Set admin password: " -s ADMIN_PASS
echo ""
[ -z "$ADMIN_PASS" ] && error "Password cannot be empty"
cd $INSTALL_DIR && ./highway -pass "$ADMIN_PASS" 2>/dev/null || true

# Start
systemctl start ${SERVICE_NAME}
sleep 1

if systemctl is-active --quiet ${SERVICE_NAME}; then
    IP=$(curl -s4 ip.sb 2>/dev/null || hostname -I | awk '{print $1}')
    echo ""
    info "========================================="
    info "  Highway7 installed successfully!"
    info "  Panel: http://${IP}:8888"
    info "  Service: systemctl status highway7"
    info "========================================="
else
    error "Service failed to start. Check: journalctl -u highway7"
fi
