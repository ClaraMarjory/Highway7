package ss

import (
	"encoding/json"
	"fmt"

	remotessh "github.com/ClaraMarjory/Highway7/internal/ssh"
)

// SSConfig represents a shadowsocks-rust config
type SSConfig struct {
	Server     string `json:"server"`
	ServerPort int    `json:"server_port"`
	Password   string `json:"password"`
	Method     string `json:"method"`
	Mode       string `json:"mode"`
}

// Deploy installs and starts ss-rust on a remote landing server
func Deploy(host string, sshPort int, user, authType, authValue string,
	ssPort int, password, method string) error {

	if method == "" {
		method = "none"
	}

	// Step 1: Install ss-rust if not present
	installCmd := `
which ssserver >/dev/null 2>&1 || {
  apt-get update -qq && apt-get install -y -qq wget >/dev/null 2>&1
  SS_VER=$(wget -qO- https://api.github.com/repos/shadowsocks/shadowsocks-rust/releases/latest | grep tag_name | cut -d'"' -f4)
  [ -z "$SS_VER" ] && SS_VER="v1.21.2"
  wget -qO /tmp/ss.tar.xz "https://github.com/shadowsocks/shadowsocks-rust/releases/download/${SS_VER}/shadowsocks-${SS_VER}.x86_64-unknown-linux-gnu.tar.xz"
  cd /tmp && tar xf ss.tar.xz && mv ssserver sslocal ssurl /usr/local/bin/ 2>/dev/null
  chmod +x /usr/local/bin/ss* 2>/dev/null
}
echo "ss_installed"
`
	out, err := remotessh.RunCommand(host, sshPort, user, authType, authValue, installCmd)
	if err != nil {
		return fmt.Errorf("install ss-rust: %w (output: %s)", err, out)
	}

	// Step 2: Write config
	cfg := SSConfig{
		Server:     "0.0.0.0",
		ServerPort: ssPort,
		Password:   password,
		Method:     method,
		Mode:       "tcp_and_udp",
	}
	cfgJSON, _ := json.MarshalIndent(cfg, "", "  ")

	writeCmd := fmt.Sprintf(`mkdir -p /etc/shadowsocks-rust && cat > /etc/shadowsocks-rust/config.json << 'SSCFG'
%s
SSCFG
echo "config_written"`, string(cfgJSON))

	out, err = remotessh.RunCommand(host, sshPort, user, authType, authValue, writeCmd)
	if err != nil {
		return fmt.Errorf("write config: %w (output: %s)", err, out)
	}

	// Step 3: Create systemd service and start
	serviceCmd := `cat > /etc/systemd/system/shadowsocks-rust.service << 'EOF'
[Unit]
Description=Shadowsocks-Rust Server
After=network.target

[Service]
Type=simple
ExecStart=/usr/local/bin/ssserver -c /etc/shadowsocks-rust/config.json
Restart=on-failure
RestartSec=5
LimitNOFILE=65536

[Install]
WantedBy=multi-user.target
EOF
systemctl daemon-reload
systemctl enable shadowsocks-rust
systemctl restart shadowsocks-rust
sleep 1
systemctl is-active shadowsocks-rust
`
	out, err = remotessh.RunCommand(host, sshPort, user, authType, authValue, serviceCmd)
	if err != nil {
		return fmt.Errorf("start service: %w (output: %s)", err, out)
	}

	return nil
}

// Status checks ss-rust service status on remote server
func Status(host string, sshPort int, user, authType, authValue string) (string, error) {
	return remotessh.RunCommand(host, sshPort, user, authType, authValue,
		"systemctl is-active shadowsocks-rust 2>/dev/null && echo RUNNING || echo STOPPED")
}

// Stop stops ss-rust on remote server
func Stop(host string, sshPort int, user, authType, authValue string) error {
	_, err := remotessh.RunCommand(host, sshPort, user, authType, authValue,
		"systemctl stop shadowsocks-rust")
	return err
}
