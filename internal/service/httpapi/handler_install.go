package httpapi

import (
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"

	"pxe-server/internal/db"
	"pxe-server/internal/logger"
	"pxe-server/internal/model"
)

// handleInstallComplete 接收装机完成上报（无需登录，客户端通过 wget 调用）。
// GET /install-complete?status=success&hostname=xxx&ipmi=xxx&mac=xxx&ip=xxx&arch=xxx&interfaces=xxx&lldp=xxx
func (s *Server) handleInstallComplete(c *gin.Context) {
	clientIP := c.ClientIP()
	record := &model.InstallRecord{
		Hostname:   c.Query("hostname"),
		IPMIAddr:   c.Query("ipmi"),
		MAC:        c.Query("mac"),
		IP:         c.Query("ip"),
		ClientIP:   clientIP,
		Arch:       c.Query("arch"),
		Interfaces: c.Query("interfaces"),
		LLDP:       c.Query("lldp"),
		Status:     c.DefaultQuery("status", "success"),
	}

	if _, err := db.CreateInstallRecord(record); err != nil {
		logger.Error("装机记录写入失败: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "msg": "记录失败"})
		return
	}

	logger.Info("装机完成上报: hostname=%s ip=%s mac=%s arch=%s client=%s", record.Hostname, record.IP, record.MAC, record.Arch, clientIP)
	c.JSON(http.StatusOK, gin.H{"code": 0, "msg": "OK"})
}

// handleListInstallRecords 查询装机完成记录（含全局统计）。
// GET /api/install-records?limit=50&offset=0
func (s *Server) handleListInstallRecords(c *gin.Context) {
	limit := int(parseInt64(c.DefaultQuery("limit", "50")))
	offset := int(parseInt64(c.DefaultQuery("offset", "0")))
	if limit <= 0 || limit > 200 {
		limit = 50
	}

	list, err := db.ListInstallRecords(limit, offset)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "msg": err.Error()})
		return
	}
	total, _ := db.CountInstallRecords()
	successCount, _ := db.CountInstallRecordsByStatus("success")
	failedCount, _ := db.CountInstallRecordsByStatus("failed")
	noLldpCount, _ := db.CountInstallRecordsNoLldp()

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"data":    list,
		"total":   total,
		"success": successCount,
		"failed":  failedCount,
		"noLldp":  noLldpCount,
	})

	s.writeLog(c, "install_list", fmt.Sprintf("查询装机记录（共 %d 条）", total))
}
