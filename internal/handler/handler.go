package handler

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/ClaraMarjory/Highway7/internal/db"
	"github.com/ClaraMarjory/Highway7/internal/iptables"
	"github.com/ClaraMarjory/Highway7/internal/ss"
	remotessh "github.com/ClaraMarjory/Highway7/internal/ssh"
	"github.com/gin-gonic/gin"
)

var (
	sessions   = make(map[string]time.Time)
	sessionMu  sync.RWMutex
	sessionTTL = 24 * time.Hour
)

func RegisterRoutes(r *gin.Engine) {
	r.POST("/api/login", handleLogin)

	api := r.Group("/api", authMiddleware())
	{
		api.GET("/servers", listServers)
		api.POST("/servers", addServer)
		api.DELETE("/servers/:id", deleteServer)
		api.POST("/servers/:id/test", testServer)

		api.GET("/forwards", listForwards)
		api.POST("/forwards", addForward)
		api.DELETE("/forwards/:id", deleteForward)
		api.POST("/forwards/:id/toggle", toggleForward)

		api.GET("/ss", listSSNodes)
		api.POST("/ss/deploy", deploySSNode)
		api.DELETE("/ss/:id", deleteSSNode)

		api.GET("/status", systemStatus)
		api.GET("/iptables", showIPTables)
	}

	r.Static("/static", "./web/static")
	r.NoRoute(func(c *gin.Context) {
		c.File("./web/static/index.html")
	})
}

// ==================== Auth ====================

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

// ==================== Helper ====================

func getServerCreds(id string) (host, user, authType, authValue string, port int, err error) {
	err = db.DB.QueryRow(`SELECT host, port, user, auth_type, auth_value FROM servers WHERE id = ?`, id).
		Scan(&host, &port, &user, &authType, &authValue)
	return
}

// ==================== Servers ====================

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

	var fwdCount int
	db.DB.QueryRow(`SELECT COUNT(*) FROM forwards WHERE server_id = ?`, id).Scan(&fwdCount)
	if fwdCount > 0 {
		c.JSON(400, gin.H{"error": fmt.Sprintf("server has %d forward(s), delete them first", fwdCount)})
		return
	}

	var ssCount int
	db.DB.QueryRow(`SELECT COUNT(*) FROM ss_nodes WHERE server_id = ?`, id).Scan(&ssCount)
	if ssCount > 0 {
		c.JSON(400, gin.H{"error": fmt.Sprintf("server has %d SS node(s), delete them first", ssCount)})
		return
	}

	_, err := db.DB.Exec(`DELETE FROM servers WHERE id = ?`, id)
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}
	c.JSON(200, gin.H{"message": "deleted"})
}

func testServer(c *gin.Context) {
	id := c.Param("id")
	host, user, authType, authValue, port, err := getServerCreds(id)
	if err != nil {
		c.JSON(404, gin.H{"error": "server not found"})
		return
	}

	err = remotessh.TestConnection(host, port, user, authType, authValue)
	if err != nil {
		db.DB.Exec(`UPDATE servers SET status = 'offline', updated_at = CURRENT_TIMESTAMP WHERE id = ?`, id)
		c.JSON(200, gin.H{"status": "offline", "error": err.Error()})
		return
	}

	db.DB.Exec(`UPDATE servers SET status = 'online', updated_at = CURRENT_TIMESTAMP WHERE id = ?`, id)
	c.JSON(200, gin.H{"status": "online", "message": "SSH connection OK"})
}

// ==================== Forwards ====================

func listForwards(c *gin.Context) {
	rows, err := db.DB.Query(`SELECT f.id, f.name, f.server_id, COALESCE(s.name,''), f.listen_port, f.target_host, f.target_port, f.protocol, f.status
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
	c.JSON(200, gin.H{"id": id, "message": "forward added, use toggle to activate"})
}

func deleteForward(c *gin.Context) {
	id := c.Param("id")

	var listenPort, targetPort int
	var targetHost, proto, status string
	err := db.DB.QueryRow(`SELECT listen_port, target_host, target_port, protocol, status FROM forwards WHERE id = ?`, id).
		Scan(&listenPort, &targetHost, &targetPort, &proto, &status)
	if err != nil {
		c.JSON(404, gin.H{"error": "forward not found"})
		return
	}

	if status == "active" {
		if err := iptables.RemoveDNAT(listenPort, targetHost, targetPort, proto); err != nil {
			c.JSON(500, gin.H{"error": fmt.Sprintf("remove iptables rule failed: %v", err)})
			return
		}
		iptables.SaveRules()
	}

	_, err = db.DB.Exec(`DELETE FROM forwards WHERE id = ?`, id)
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}
	c.JSON(200, gin.H{"message": "deleted"})
}

func toggleForward(c *gin.Context) {
	id := c.Param("id")

	var listenPort, targetPort int
	var targetHost, proto, status string
	err := db.DB.QueryRow(`SELECT listen_port, target_host, target_port, protocol, status FROM forwards WHERE id = ?`, id).
		Scan(&listenPort, &targetHost, &targetPort, &proto, &status)
	if err != nil {
		c.JSON(404, gin.H{"error": "forward not found"})
		return
	}

	if status == "active" {
		if err := iptables.RemoveDNAT(listenPort, targetHost, targetPort, proto); err != nil {
			c.JSON(500, gin.H{"error": fmt.Sprintf("remove rule failed: %v", err)})
			return
		}
		db.DB.Exec(`UPDATE forwards SET status = 'inactive' WHERE id = ?`, id)
		iptables.SaveRules()
		c.JSON(200, gin.H{"status": "inactive", "message": "forward deactivated"})
	} else {
		iptables.EnsureForwarding()
		iptables.EnsureMasquerade()

		if err := iptables.AddDNAT(listenPort, targetHost, targetPort, proto); err != nil {
			c.JSON(500, gin.H{"error": fmt.Sprintf("add rule failed: %v", err)})
			return
		}
		db.DB.Exec(`UPDATE forwards SET status = 'active' WHERE id = ?`, id)
		iptables.SaveRules()
		c.JSON(200, gin.H{"status": "active", "message": "forward activated"})
	}
}

// ==================== SS Nodes ====================

func listSSNodes(c *gin.Context) {
	rows, err := db.DB.Query(`SELECT n.id, n.server_id, COALESCE(s.name,''), n.port, n.method, n.status
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

	sid := strconv.FormatInt(req.ServerID, 10)
	host, user, authType, authValue, sshPort, err := getServerCreds(sid)
	if err != nil {
		c.JSON(404, gin.H{"error": "server not found"})
		return
	}

	result, err := db.DB.Exec(
		`INSERT INTO ss_nodes (server_id, port, password, method, status) VALUES (?,?,?,'none','deploying')`,
		req.ServerID, req.Port, req.Password,
	)
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}
	nodeID, _ := result.LastInsertId()

	go func() {
		err := ss.Deploy(host, sshPort, user, authType, authValue, req.Port, req.Password, "none")
		if err != nil {
			db.DB.Exec(`UPDATE ss_nodes SET status = 'failed' WHERE id = ?`, nodeID)
			return
		}
		db.DB.Exec(`UPDATE ss_nodes SET status = 'active' WHERE id = ?`, nodeID)
	}()

	c.JSON(200, gin.H{"id": nodeID, "message": "deploying SS node (method=none)"})
}

func deleteSSNode(c *gin.Context) {
	id := c.Param("id")

	var serverID int64
	var status string
	err := db.DB.QueryRow(`SELECT server_id, status FROM ss_nodes WHERE id = ?`, id).Scan(&serverID, &status)
	if err != nil {
		c.JSON(404, gin.H{"error": "SS node not found"})
		return
	}

	if status == "active" {
		sid := strconv.FormatInt(serverID, 10)
		host, user, authType, authValue, sshPort, err := getServerCreds(sid)
		if err == nil {
			ss.Stop(host, sshPort, user, authType, authValue)
		}
	}

	_, err = db.DB.Exec(`DELETE FROM ss_nodes WHERE id = ?`, id)
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}
	c.JSON(200, gin.H{"message": "deleted"})
}

// ==================== System ====================

func systemStatus(c *gin.Context) {
	var serverCount, forwardCount, ssCount, activeForwards int
	db.DB.QueryRow(`SELECT COUNT(*) FROM servers`).Scan(&serverCount)
	db.DB.QueryRow(`SELECT COUNT(*) FROM forwards`).Scan(&forwardCount)
	db.DB.QueryRow(`SELECT COUNT(*) FROM forwards WHERE status = 'active'`).Scan(&activeForwards)
	db.DB.QueryRow(`SELECT COUNT(*) FROM ss_nodes`).Scan(&ssCount)

	c.JSON(200, gin.H{
		"version":         "0.1.0",
		"servers":         serverCount,
		"forwards":        forwardCount,
		"active_forwards": activeForwards,
		"ss_nodes":        ssCount,
	})
}

func showIPTables(c *gin.Context) {
	rules, err := iptables.ListNATRules()
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}
	c.JSON(200, gin.H{"rules": rules})
}
