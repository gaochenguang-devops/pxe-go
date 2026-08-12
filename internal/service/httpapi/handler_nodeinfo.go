package httpapi

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/xuri/excelize/v2"

	"pxe-server/internal/db"
	"pxe-server/internal/logger"
	"pxe-server/internal/model"
)

// nodeInfoColumns node-info.txt 与 Excel 的列定义。
var nodeInfoColumns = []string{
	"IPMI", "主机名",
	"bond0_ip", "bond0_mask", "bond0_gateway", "bond0_ipv6", "bond0_ipv6mask", "bond0_ipv6gw",
	"bond2_ip", "bond2_mask", "bond2_gateway", "bond2_ipv6", "bond2_ipv6mask", "bond2_ipv6gw",
	"bond1_ip", "bond1_mask", "bond1_gateway",
}

// nodeInfoValues 提取主机资源各字段值（顺序与列一致）。
func nodeInfoValues(r *model.HostResource) []string {
	return []string{
		r.IPMIAddr, r.Hostname,
		r.Bond0IP, r.Bond0Mask, r.Bond0Gateway, r.Bond0IPv6, r.Bond0IPv6Mask, r.Bond0IPv6Gw,
		r.Bond2IP, r.Bond2Mask, r.Bond2Gateway, r.Bond2IPv6, r.Bond2IPv6Mask, r.Bond2IPv6Gw,
		r.Bond1IP, r.Bond1Mask, r.Bond1Gateway,
	}
}

// nodeInfoPath 返回 web_root/node-info.txt 路径。
func (s *Server) nodeInfoPath() string {
	root := s.cfg.HTTP().WebRoot
	if root == "" {
		root = "assets/web_root"
	}
	return filepath.Join(root, "node-info.txt")
}

// buildNodeInfoContent 从主机资源列表生成 node-info.txt 内容（空格分隔）。
func buildNodeInfoContent(resources []*model.HostResource) string {
	var b strings.Builder
	for _, r := range resources {
		b.WriteString(strings.Join(nodeInfoValues(r), " "))
		b.WriteString("\n")
	}
	return b.String()
}

// handleGetNodeInfoTemplate 下载主机资源 Excel 模版（含表头与一行示例）。
func (s *Server) handleGetNodeInfoTemplate(c *gin.Context) {
	example := []string{"192.168.10.20", "node-01",
		"192.168.10.21", "255.255.255.0", "192.168.10.1", "fe80::1", "64", "fe80::1",
		"", "", "", "", "", "",
		"", "", ""}
	s.writeLog(c, "nodeinfo_template", "下载主机资源 Excel 模版")
	_ = writeXlsx(c, "node-resource-template.xlsx", nodeInfoColumns, [][]string{example})
}

// handleExportNodeInfoTxt 导出 node-info.txt（同时写入 web_root）。
func (s *Server) handleExportNodeInfoTxt(c *gin.Context) {
	resources, err := db.ListHostResources()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "msg": err.Error()})
		return
	}
	content := buildNodeInfoContent(resources)
	if err := writeFileEnsureDir(s.nodeInfoPath(), []byte(content)); err != nil {
		logger.Error("写入 node-info.txt 失败: %v", err)
	}
	s.writeLog(c, "nodeinfo_export", "导出 node-info.txt")
	c.Header("Content-Disposition", "attachment; filename=node-info.txt")
	c.Data(http.StatusOK, "text/plain; charset=utf-8", []byte(content))
}

// handleExportNodeInfoExcel 导出主机资源为 Excel。
func (s *Server) handleExportNodeInfoExcel(c *gin.Context) {
	resources, err := db.ListHostResources()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "msg": err.Error()})
		return
	}
	f := excelize.NewFile()
	sheet := "主机资源"
	f.SetSheetName("Sheet1", sheet)
	for i, col := range nodeInfoColumns {
		cell, _ := excelize.CoordinatesToCellName(i+1, 1)
		f.SetCellValue(sheet, cell, col)
	}
	for r, res := range resources {
		vals := nodeInfoValues(res)
		for ci, v := range vals {
			cell, _ := excelize.CoordinatesToCellName(ci+1, r+2)
			f.SetCellValue(sheet, cell, v)
		}
	}
	buf, err := f.WriteToBuffer()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "msg": "生成 Excel 失败: " + err.Error()})
		return
	}
	s.writeLog(c, "nodeinfo_excel", "导出主机资源 Excel")
	c.Header("Content-Disposition", "attachment; filename=hosts.xlsx")
	c.Data(http.StatusOK, "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet", buf.Bytes())
}

// handleImportNodeInfoExcel 从 Excel 导入主机资源并重新生成 node-info.txt。
func (s *Server) handleImportNodeInfoExcel(c *gin.Context) {
	file, err := c.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": "未获取到文件: " + err.Error()})
		return
	}
	src, err := file.Open()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "msg": "打开文件失败: " + err.Error()})
		return
	}
	defer src.Close()
	f, err := excelize.OpenReader(src)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": "解析 Excel 失败: " + err.Error()})
		return
	}
	defer f.Close()
	sheet := f.GetSheetName(0)
	rows, err := f.GetRows(sheet)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": "读取 Excel 失败: " + err.Error()})
		return
	}
	imported := 0
	skipped := 0
	for i := 1; i < len(rows); i++ {
		row := rows[i]
		if len(row) < 2 {
			continue
		}
		ipmi := strings.TrimSpace(row[0])
		hostname := strings.TrimSpace(row[1])
		if ipmi == "" || hostname == "" {
			skipped++
			continue
		}
		r := &model.HostResource{
			IPMIAddr:     ipmi,
			Hostname:     hostname,
			Bond0IP:      get(row, 2), Bond0Mask: get(row, 3), Bond0Gateway: get(row, 4),
			Bond0IPv6:    get(row, 5), Bond0IPv6Mask: get(row, 6), Bond0IPv6Gw: get(row, 7),
			Bond2IP:      get(row, 8), Bond2Mask: get(row, 9), Bond2Gateway: get(row, 10),
			Bond2IPv6:    get(row, 11), Bond2IPv6Mask: get(row, 12), Bond2IPv6Gw: get(row, 13),
			Bond1IP:      get(row, 14), Bond1Mask: get(row, 15), Bond1Gateway: get(row, 16),
		}
		if err := db.UpsertHostResource(r); err != nil {
			logger.Warn("导入主机资源失败 %s: %v", ipmi, err)
			skipped++
			continue
		}
		imported++
	}
	resources, _ := db.ListHostResources()
	content := buildNodeInfoContent(resources)
	_ = writeFileEnsureDir(s.nodeInfoPath(), []byte(content))
	s.writeLog(c, "nodeinfo_import", fmt.Sprintf("导入主机资源 %d 条，跳过 %d 条", imported, skipped))
	c.JSON(http.StatusOK, gin.H{"code": 0, "msg": fmt.Sprintf("导入成功 %d 条，跳过 %d 条，已重新生成 node-info.txt", imported, skipped)})
}

func get(row []string, i int) string {
	if i < len(row) {
		return strings.TrimSpace(row[i])
	}
	return ""
}

// handleGetNodeInfo 读取当前 node-info.txt 内容。
func (s *Server) handleGetNodeInfo(c *gin.Context) {
	data, err := os.ReadFile(s.nodeInfoPath())
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 0, "data": ""})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": string(data)})
}

// handleListHostResources 列出主机资源（支持分页参数 page / pageSize；无分页参数时返回全量）。
func (s *Server) handleListHostResources(c *gin.Context) {
	hasPage := c.Query("page") != "" || c.Query("pageSize") != ""
	search := c.Query("search")
	page := 1
	pageSize := 20
	if v := c.Query("page"); v != "" {
		if p, err := strconv.Atoi(v); err == nil && p > 0 {
			page = p
		}
	}
	if v := c.Query("pageSize"); v != "" {
		if ps, err := strconv.Atoi(v); err == nil && ps > 0 && ps <= 200 {
			pageSize = ps
		}
	}

	var list []*model.HostResource
	var err error

	if hasPage {
		offset := (page - 1) * pageSize
		list, err = db.ListHostResourcesPage(pageSize, offset, search)
	} else {
		list, err = db.ListHostResources()
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "msg": err.Error()})
		return
	}

	if hasPage {
		total, err := db.CountHostResources(search)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "msg": err.Error()})
			return
		}
		totalPages := (total + pageSize - 1) / pageSize
		c.JSON(http.StatusOK, gin.H{
			"code":       0,
			"data":       list,
			"total":      total,
			"page":       page,
			"pageSize":   pageSize,
			"totalPages": totalPages,
		})
	} else {
		c.JSON(http.StatusOK, gin.H{"code": 0, "data": list})
	}
}

// handleBatchDeleteHostResource 批量删除主机资源并重新生成 node-info.txt。
func (s *Server) handleBatchDeleteHostResource(c *gin.Context) {
	var req struct {
		IDs []int64 `json:"ids"`
	}
	if !s.parseJSONBody(c, &req) {
		return
	}
	if len(req.IDs) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": "请选择要删除的主机资源"})
		return
	}
	logger.Info("批量删除主机资源请求: ids=%v", req.IDs)
	if err := db.DeleteHostResourceBatch(req.IDs); err != nil {
		logger.Error("批量删除主机资源失败: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "msg": "批量删除失败: " + err.Error()})
		return
	}
	logger.Info("批量删除主机资源成功: %d 条", len(req.IDs))
	// 重新生成 node-info.txt
	resources, _ := db.ListHostResources()
	content := buildNodeInfoContent(resources)
	_ = writeFileEnsureDir(s.nodeInfoPath(), []byte(content))

	s.writeLog(c, "res_batch_delete", fmt.Sprintf("批量删除主机资源 %d 条", len(req.IDs)))
	c.JSON(http.StatusOK, gin.H{"code": 0, "msg": fmt.Sprintf("已删除 %d 条主机资源，已重新生成 node-info.txt", len(req.IDs))})
}
