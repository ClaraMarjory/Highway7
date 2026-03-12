package ssh

import (
	"fmt"
	"net"
	"os"
	"strings"
	"time"

	"golang.org/x/crypto/ssh"
)

// RunCommand executes a command on a remote server via SSH
func RunCommand(host string, port int, user, authType, authValue, command string) (string, error) {
	config := &ssh.ClientConfig{
		User:            user,
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout:         10 * time.Second,
	}

	switch authType {
	case "key":
		key, err := parseKey(authValue)
		if err != nil {
			return "", fmt.Errorf("parse key: %w", err)
		}
		config.Auth = []ssh.AuthMethod{ssh.PublicKeys(key)}
	case "password":
		config.Auth = []ssh.AuthMethod{ssh.Password(authValue)}
	default:
		return "", fmt.Errorf("unsupported auth type: %s", authType)
	}

	addr := fmt.Sprintf("%s:%d", host, port)
	client, err := ssh.Dial("tcp", addr, config)
	if err != nil {
		return "", fmt.Errorf("ssh dial %s: %w", addr, err)
	}
	defer client.Close()

	session, err := client.NewSession()
	if err != nil {
		return "", fmt.Errorf("new session: %w", err)
	}
	defer session.Close()

	output, err := session.CombinedOutput(command)
	if err != nil {
		return string(output), fmt.Errorf("exec '%s': %w\noutput: %s", command, err, string(output))
	}

	return strings.TrimSpace(string(output)), nil
}

// TestConnection checks if SSH connection works
func TestConnection(host string, port int, user, authType, authValue string) error {
	_, err := RunCommand(host, port, user, authType, authValue, "echo ok")
	return err
}

// CheckPort tests if a remote port is reachable
func CheckPort(host string, port int, timeout time.Duration) bool {
	addr := fmt.Sprintf("%s:%d", host, port)
	conn, err := net.DialTimeout("tcp", addr, timeout)
	if err != nil {
		return false
	}
	conn.Close()
	return true
}

func parseKey(keyPath string) (ssh.Signer, error) {
	keyData, err := os.ReadFile(keyPath)
	if err != nil {
		return nil, fmt.Errorf("read key %s: %w", keyPath, err)
	}
	signer, err := ssh.ParsePrivateKey(keyData)
	if err != nil {
		return nil, fmt.Errorf("parse key: %w", err)
	}
	return signer, nil
}
