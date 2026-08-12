package httpapi

import (
	"bufio"
	"net/http"
	"os"
	"strings"

	"github.com/gin-gonic/gin"

	"pxe-server/internal/db"
	"pxe-server/internal/logger"
	"pxe-server/internal/middleware"
	"pxe-server/internal/model"
)

// writeLog 记录操作审计日志。
func (s *Server) writeLog(c *gin.Context, opType, detail string) {
	operator := c.GetHeader("X-Operator")
	if operator == "" {
		operator = "admin"
	}
	op := &model.OperLog{
		Operator: operator,
		OpType:   opType,
		Detail:   detail,
		ClientIP: middleware.ClientIP(c),
	}
	if _, err := db.CreateOperLog(op); err != nil {
		// 审计日志失败不影响主流程
	}
}

// handleListOperLogs 操作日志列表（支持 module / opType / search 参数）。
func (s *Server) handleListOperLogs(c *gin.Context) {
	limit := parseInt64(c.DefaultQuery("limit", "100"))
	offset := parseInt64(c.DefaultQuery("offset", "0"))
	module := c.Query("module")
	opType := c.Query("opType")
	search := strings.TrimSpace(c.Query("search"))
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	var list []*model.OperLog
	var err error
	list, err = db.ListOperLogsFiltered(module, opType, search, int(limit), int(offset))
	if err != nil {
		logger.Error("查询操作日志失败: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "msg": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": list})
}

// handleTailLogFile 读取日志文件尾部 N 行（/api/logfile?lines=100&filter=DHCP）。
func (s *Server) handleTailLogFile(c *gin.Context) {
	lines := int(parseInt64(c.DefaultQuery("lines", "100")))
	filter := strings.TrimSpace(c.Query("filter"))
	if lines <= 0 || lines > 500 {
		lines = 100
	}
	logPath := "logs/pxe-server.log"
	if _, err := os.Stat(logPath); os.IsNotExist(err) {
		c.JSON(http.StatusOK, gin.H{"code": 0, "data": []string{}})
		return
	}

	file, err := os.Open(logPath)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "msg": "无法打开日志文件"})
		return
	}
	defer file.Close()

	// 读取所有行并取尾部
	var allLines []string
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		if filter == "" || strings.Contains(line, filter) {
			allLines = append(allLines, line)
		}
	}

	start := len(allLines) - lines
	if start < 0 {
		start = 0
	}
	result := allLines[start:]
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": result})
}
