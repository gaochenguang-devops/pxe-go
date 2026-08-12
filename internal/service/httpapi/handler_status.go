package httpapi

import (
	"net"
	"net/http"
	"os"
	"time"

	"github.com/gin-gonic/gin"

	"pxe-server/internal/db"
	"pxe-server/internal/logger"
)

type serviceStatus struct {
	Name   string `json:"name"`
	Port   string `json:"port"`
	Status string `json:"status"` // running / stopped
	Detail string `json:"detail"`
}

// handleServiceStatus 检测服务运行状态。
// GET /api/status
func (s *Server) handleServiceStatus(c *gin.Context) {
	cfg := s.cfg
	statuses := []serviceStatus{
		// DHCP 检测（默认端口 67）
		checkService("DHCP 服务", cfg.DHCP().ListenIP+":67", cfg.DHCP().Enabled),
		// TFTP 检测（默认端口 69）
		checkService("TFTP 服务", cfg.TFTP().ListenIP+":69", cfg.TFTP().Enabled),
		// HTTP 检测（自己本身一定在运行）
		{Name: "HTTP 服务", Port: cfg.HTTP().ListenAddr, Status: "running", Detail: "管理后台运行中"},
		// SQLite 检测
		checkSQLite(),
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": statuses})
}

func checkService(name, addr string, enabled bool) serviceStatus {
	status := serviceStatus{Name: name, Port: addr}
	if !enabled {
		status.Status = "stopped"
		status.Detail = "配置未启用"
		return status
	}
	conn, err := net.DialTimeout("udp", addr, 2*time.Second)
	if err != nil {
		status.Status = "stopped"
		status.Detail = "端口无法连接"
		logger.Warn("服务检测失败 %s (%s): %v", name, addr, err)
	} else {
		conn.Close()
		status.Status = "running"
		status.Detail = "正常响应"
	}
	return status
}

func checkSQLite() serviceStatus {
	status := serviceStatus{Name: "SQLite", Port: "本地存储"}
	// 检查数据库文件是否存在
	dbPath := "data/pxe-server.db"
	if _, err := os.Stat(dbPath); os.IsNotExist(err) {
		status.Status = "stopped"
		status.Detail = "数据库文件不存在"
		return status
	}
	// 尝试执行查询
	_, err := db.CountHosts("")
	if err != nil {
		status.Status = "stopped"
		status.Detail = "数据库查询失败"
		return status
	}
	status.Status = "running"
	status.Detail = "读写正常"
	return status
}
