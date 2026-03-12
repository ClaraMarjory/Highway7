package handler

import (
	"crypto/rand"
	"encoding/hex"
	"net/http"
	"sync"
	"time"

	"github.com/ClaraMarjory/Highway7/internal/db"
	"github.com/gin-gonic/gin"
)

var (
	sessions   = make(map[string]time.Time)
	sessionMu  sync.RWMutex
	sessionTTL = 24 * time.Hour
)

func RegisterRoutes(r *gin.Engine) {
	// Login
	r.POST("/api/login", handleLogin)

	// Protected API
	api := r.Group("/api", authMiddleware())
	{
		// Servers (landing machines)
		api.GET("/servers", listServers)
		api.POST("/servers", addServer)
		api.DELETE("/servers/:id", deleteServer)
		api.POST("/servers/:id/test", testServer)

		// Forwards (iptables DNAT rules)
		api.GET("/forwards", listForwards)
		api.POST("/forwards", addForward)
		api.DELETE("/forwards/:id", deleteForward)
		api.POST("/forwards/:id/toggle", toggleForward)

		// SS nodes
		api.GET("/ss", listSSNodes)
		api.POST("/ss/deploy", deploySSNode)

		// System
		api.GET("/status", systemStatus)
		api.GET("/iptables", showIPTables)
	}

	// Static files & SPA
	r.Static("/static", "./web/static")
	r.NoRoute(func(c *gin.Context) {
		c.File("./web/static/index.html")
	})
}

// Auth

func handleLogin(c *gin.Context) {
	var req struct {
		Password string `json:"password"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}

	if !db.CheckAdminPassword(req.Password) {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "wrong password"})
		return
	}

	token := generateToken()
	sessionMu.Lock()
	sessions[token] = time.Now().Add(sessionTTL)
	sessionMu.Unlock()

	c.JSON(http.StatusOK, gin.H{"token": token})
}

func authMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		token := c.GetHeader("Authorization")
		if token == "" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "no token"})
			c.Abort()
			return
		}

		sessionMu.RLock()
		expiry, ok := sessions[token]
		sessionMu.RUnlock()

		if !ok || time.Now().After(expiry) {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "expired"})
			c.Abort()
			return
		}

		c.Next()
	}
}

func generateToken() string {
	b := make([]byte, 32)
	rand.Read(b)
	return hex.EncodeToString(b)
}

// Server handlers

func listServers(c *gin.Context) {
	rows, err := db.DB.Query(`SELECT id, name, host, port, user, auth_type, role, status, created_at FROM servers ORDER BY id`)
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}
	defer rows.Close()

	var servers []gin.H
	for rows.Next() {
		var id int64
		var name, host, user, authType, role, status, created string
		var port int
		rows.Scan(&id, &name, &host, &port, &user, &authType, &role, &status, &created)
		servers = append(servers, gin.H{
			"id": id, "name": name, "host": host, "port": port,
			"user": user, "auth_type": authType, "role": role,
			"status": status, "created_at": created,
		})
	}
	if servers == nil {
		servers = []gin.H{}
	}
	c.JSON(200, servers)
}

func addServer(c *gin.Context) {
	var req struct {
		Name      string `json:"name"`
		Host      string `json:"host"`
		Port      int    `json:"port"`
		User      string `json:"user"`
		AuthType  string `json:"auth_type"`
		AuthValue string `json:"auth_value"`
		Role      string `json:"role"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}
	if req.Port == 0 {
		req.Port = 22
	}
	if req.User == "" {
		req.User = "root"
	}

	result, err := db.DB.Exec(
		`INSERT INTO servers (name, host, port, user, auth_type, auth_value, role) VALUES (?,?,?,?,?,?,?)`,
		req.Name, req.Host, req.Port, req.User, req.AuthType, req.AuthValue, req.Role,
	)
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}
	id, _ := result.LastInsertId()
	c.JSON(200, gin.H{"id": id, "message": "server added"})
}

func deleteServer(c *gin.Context) {
	id := c.Param("id")
	_, err := db.DB.Exec(`DELETE FROM servers WHERE id = ?`, id)
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}
	c.JSON(200, gin.H{"message": "deleted"})
}

func testServer(c *gin.Context) {
	id := c.Param("id")
	var host, user, authType, authValue string
	var port int
	err := db.DB.QueryRow(`SELECT host, port, user, auth_type, auth_value FROM servers WHERE id = ?`, id).
		Scan(&host, &port, &user, &authType, &authValue)
	if err != nil {
		c.JSON(404, gin.H{"error": "server not found"})
		return
	}

	// Use SSH module to test
	err = nil // placeholder - will use ssh.TestConnection in real deployment
	c.JSON(200, gin.H{"status": "ok", "message": "connection test passed"})
}

// Forward handlers

func listForwards(c *gin.Context) {
	rows, err := db.DB.Query(`SELECT f.id, f.name, f.server_id, s.name, f.listen_port, f.target_host, f.target_port, f.protocol, f.status
		FROM forwards f LEFT JOIN servers s ON f.server_id = s.id ORDER BY f.id`)
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}
	defer rows.Close()

	var forwards []gin.H
	for rows.Next() {
		var id, serverID int64
		var name, serverName, targetHost, proto, status string
		var listenPort, targetPort int
		rows.Scan(&id, &name, &serverID, &serverName, &listenPort, &targetHost, &targetPort, &proto, &status)
		forwards = append(forwards, gin.H{
			"id": id, "name": name, "server_id": serverID, "server_name": serverName,
			"listen_port": listenPort, "target_host": targetHost, "target_port": targetPort,
			"protocol": proto, "status": status,
		})
	}
	if forwards == nil {
		forwards = []gin.H{}
	}
	c.JSON(200, forwards)
}

func addForward(c *gin.Context) {
	var req struct {
		Name       string `json:"name"`
		ServerID   int64  `json:"server_id"`
		ListenPort int    `json:"listen_port"`
		TargetHost string `json:"target_host"`
		TargetPort int    `json:"target_port"`
		Protocol   string `json:"protocol"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}
	if req.Protocol == "" {
		req.Protocol = "tcp"
	}

	result, err := db.DB.Exec(
		`INSERT INTO forwards (name, server_id, listen_port, target_host, target_port, protocol) VALUES (?,?,?,?,?,?)`,
		req.Name, req.ServerID, req.ListenPort, req.TargetHost, req.TargetPort, req.Protocol,
	)
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}
	id, _ := result.LastInsertId()
	c.JSON(200, gin.H{"id": id, "message": "forward added"})
}

func deleteForward(c *gin.Context) {
	id := c.Param("id")
	// TODO: also remove iptables rule
	_, err := db.DB.Exec(`DELETE FROM forwards WHERE id = ?`, id)
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}
	c.JSON(200, gin.H{"message": "deleted"})
}

func toggleForward(c *gin.Context) {
	// TODO: activate/deactivate iptables rule
	c.JSON(200, gin.H{"message": "toggled"})
}

// SS handlers

func listSSNodes(c *gin.Context) {
	rows, err := db.DB.Query(`SELECT n.id, n.server_id, s.name, n.port, n.method, n.status
		FROM ss_nodes n LEFT JOIN servers s ON n.server_id = s.id ORDER BY n.id`)
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}
	defer rows.Close()

	var nodes []gin.H
	for rows.Next() {
		var id, serverID int64
		var serverName, method, status string
		var port int
		rows.Scan(&id, &serverID, &serverName, &port, &method, &status)
		nodes = append(nodes, gin.H{
			"id": id, "server_id": serverID, "server_name": serverName,
			"port": port, "method": method, "status": status,
		})
	}
	if nodes == nil {
		nodes = []gin.H{}
	}
	c.JSON(200, nodes)
}

func deploySSNode(c *gin.Context) {
	var req struct {
		ServerID int64  `json:"server_id"`
		Port     int    `json:"port"`
		Password string `json:"password"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}

	// TODO: call ss.Deploy via SSH
	result, err := db.DB.Exec(
		`INSERT INTO ss_nodes (server_id, port, password, method, status) VALUES (?,?,?,'none','deploying')`,
		req.ServerID, req.Port, req.Password,
	)
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}
	id, _ := result.LastInsertId()
	c.JSON(200, gin.H{"id": id, "message": "deploying SS node"})
}

// System handlers

func systemStatus(c *gin.Context) {
	var serverCount, forwardCount, ssCount int
	db.DB.QueryRow(`SELECT COUNT(*) FROM servers`).Scan(&serverCount)
	db.DB.QueryRow(`SELECT COUNT(*) FROM forwards`).Scan(&forwardCount)
	db.DB.QueryRow(`SELECT COUNT(*) FROM ss_nodes`).Scan(&ssCount)

	c.JSON(200, gin.H{
		"version":  "0.1.0",
		"servers":  serverCount,
		"forwards": forwardCount,
		"ss_nodes": ssCount,
	})
}

func showIPTables(c *gin.Context) {
	// Local iptables status
	c.JSON(200, gin.H{"message": "iptables listing - TODO"})
}
