package model

import "time"

type Server struct {
	ID        int64     `json:"id"`
	Name      string    `json:"name"`
	Host      string    `json:"host"`
	Port      int       `json:"port"`
	User      string    `json:"user"`
	AuthType  string    `json:"auth_type"`  // "key" or "password"
	AuthValue string    `json:"auth_value"` // private key path or password
	Role      string    `json:"role"`       // "landing" or "relay"
	Status    string    `json:"status"`     // "online","offline","unknown"
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type Forward struct {
	ID         int64     `json:"id"`
	Name       string    `json:"name"`
	ServerID   int64     `json:"server_id"`
	ListenPort int       `json:"listen_port"`
	TargetHost string    `json:"target_host"`
	TargetPort int       `json:"target_port"`
	Protocol   string    `json:"protocol"` // "tcp" or "tcp+udp"
	Status     string    `json:"status"`   // "active","inactive"
	BytesUp    int64     `json:"bytes_up"`
	BytesDown  int64     `json:"bytes_down"`
	CreatedAt  time.Time `json:"created_at"`
}

type SSNode struct {
	ID       int64  `json:"id"`
	ServerID int64  `json:"server_id"`
	Port     int    `json:"port"`
	Password string `json:"password"`
	Method   string `json:"method"` // "none" or "plain"
	Status   string `json:"status"`
}
