package iptables

import (
	"fmt"
	"os/exec"
	"strings"

	remotessh "github.com/ClaraMarjory/Highway7/internal/ssh"
)

// Local iptables commands (for master/controller node)

// EnsureForwarding enables IP forwarding on the kernel
func EnsureForwarding() error {
	return exec.Command("sysctl", "-w", "net.ipv4.ip_forward=1").Run()
}

// AddDNAT adds a PREROUTING DNAT rule
func AddDNAT(listenPort int, targetHost string, targetPort int, proto string) error {
	// iptables -t nat -A PREROUTING -p tcp --dport 55555 -j DNAT --to 1.2.3.4:55555
	cmd := exec.Command("iptables", "-t", "nat", "-A", "PREROUTING",
		"-p", proto,
		"--dport", fmt.Sprintf("%d", listenPort),
		"-j", "DNAT",
		"--to-destination", fmt.Sprintf("%s:%d", targetHost, targetPort),
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("add DNAT: %s: %w", string(out), err)
	}
	return nil
}

// RemoveDNAT removes a PREROUTING DNAT rule
func RemoveDNAT(listenPort int, targetHost string, targetPort int, proto string) error {
	cmd := exec.Command("iptables", "-t", "nat", "-D", "PREROUTING",
		"-p", proto,
		"--dport", fmt.Sprintf("%d", listenPort),
		"-j", "DNAT",
		"--to-destination", fmt.Sprintf("%s:%d", targetHost, targetPort),
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("remove DNAT: %s: %w", string(out), err)
	}
	return nil
}

// EnsureMasquerade adds MASQUERADE if not present
func EnsureMasquerade() error {
	// Check if already exists
	out, _ := exec.Command("iptables", "-t", "nat", "-S", "POSTROUTING").CombinedOutput()
	if strings.Contains(string(out), "MASQUERADE") {
		return nil
	}
	cmd := exec.Command("iptables", "-t", "nat", "-A", "POSTROUTING", "-j", "MASQUERADE")
	outB, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("add MASQUERADE: %s: %w", string(outB), err)
	}
	return nil
}

// SaveRules persists iptables rules (requires iptables-persistent)
func SaveRules() error {
	cmd := exec.Command("bash", "-c", "iptables-save > /etc/iptables/rules.v4")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("save rules: %s: %w", string(out), err)
	}
	return nil
}

// ListNATRules returns current NAT table rules
func ListNATRules() (string, error) {
	out, err := exec.Command("iptables", "-t", "nat", "-L", "-n", "--line-numbers").CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("list rules: %s: %w", string(out), err)
	}
	return string(out), nil
}

// Remote iptables commands (for controlled nodes via SSH)

// RemoteAddDNAT adds DNAT on a remote server
func RemoteAddDNAT(host string, sshPort int, user, authType, authValue string,
	listenPort int, targetHost string, targetPort int, proto string) error {

	cmd := fmt.Sprintf("iptables -t nat -A PREROUTING -p %s --dport %d -j DNAT --to-destination %s:%d",
		proto, listenPort, targetHost, targetPort)

	_, err := remotessh.RunCommand(host, sshPort, user, authType, authValue, cmd)
	return err
}

// RemoteEnsureMasquerade ensures MASQUERADE on remote server
func RemoteEnsureMasquerade(host string, sshPort int, user, authType, authValue string) error {
	cmd := `iptables -t nat -S POSTROUTING | grep -q MASQUERADE || iptables -t nat -A POSTROUTING -j MASQUERADE`
	_, err := remotessh.RunCommand(host, sshPort, user, authType, authValue, cmd)
	return err
}

// RemoteSaveRules saves iptables on remote server
func RemoteSaveRules(host string, sshPort int, user, authType, authValue string) error {
	cmd := "iptables-save > /etc/iptables/rules.v4 2>/dev/null || netfilter-persistent save 2>/dev/null || true"
	_, err := remotessh.RunCommand(host, sshPort, user, authType, authValue, cmd)
	return err
}
