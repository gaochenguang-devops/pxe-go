package httpapi

import (
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/xuri/excelize/v2"

	"pxe-server/internal/db"
	"pxe-server/internal/logger"
	"pxe-server/internal/model"
	"pxe-server/internal/service/ipmi"
)

// hostExcelColumns 主机管理 Excel 列定义（不含 bond 网络与绑定镜像，绑定镜像在主机资源/编辑中配置）。
var hostExcelColumns = []string{"主机名", "IPMI地址", "IPMI用户", "IPMI密码"}

// writeXlsx 生成 xlsx 并回写响应（下载）。
func writeXlsx(c *gin.Context, filename string, cols []string, rows [][]string) error {
	f := excelize.NewFile()
	sheet := "Sheet1"
	for i, col := range cols {
		cell, _ := excelize.CoordinatesToCellName(i+1, 1)
		f.SetCellValue(sheet, cell, col)
	}
	for r, row := range rows {
		for ci, v := range row {
			cell, _ := excelize.CoordinatesToCellName(ci+1, r+2)
			f.SetCellValue(sheet, cell, v)
		}
	}
	buf, err := f.WriteToBuffer()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "msg": "生成 Excel 失败: " + err.Error()})
		return err
	}
	c.Header("Content-Disposition", "attachment; filename="+filename)
	c.Data(http.StatusOK, "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet", buf.Bytes())
	return nil
}

// handleGetHostExcelTemplate 下载主机管理 Excel 模版（含表头与一行示例）。
func (s *Server) handleGetHostExcelTemplate(c *gin.Context) {
	s.writeLog(c, "host_excel_template", "下载主机管理 Excel 模版")
	example := []string{"node-01", "192.168.10.20", "admin", "password"}
	_ = writeXlsx(c, "host-template.xlsx", hostExcelColumns, [][]string{example})
}

// hostExcelRow 解析后的主机管理导入行。
type hostExcelRow struct {
	Hostname string
	IPMIAddr string
	IPMIUser string
	IPMIPass string
}

// handleImportHostExcel 从 Excel 导入主机（按 IPMI upsert）。
// 列顺序与 hostExcelColumns 一致。
func (s *Server) handleImportHostExcel(c *gin.Context) {
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
	rows, err := f.GetRows(f.GetSheetName(0))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": "读取 Excel 失败: " + err.Error()})
		return
	}
	imported, skipped := 0, 0
	for i := 1; i < len(rows); i++ { // 跳过表头
		row := rows[i]
		ipmiAddr := get(row, 1)
		hostname := get(row, 0)
		if ipmiAddr == "" && hostname == "" {
			continue
		}
		r := &hostExcelRow{
			Hostname: hostname,
			IPMIAddr: ipmiAddr,
			IPMIUser: get(row, 2),
			IPMIPass: get(row, 3),
		}
		h := &model.HostInfo{
			Hostname:    r.Hostname,
			IPMIAddr:    r.IPMIAddr,
			IPMIUser:    r.IPMIUser,
			IPMIPass:    ipmi.EncryptPass(r.IPMIPass, s.cfg),
			InstallStatus: "idle",
		}
		if err := upsertHostBasic(h); err != nil {
			logger.Warn("导入主机 %s 失败: %v", r.IPMIAddr, err)
			skipped++
			continue
		}
		imported++
	}
	msg := fmt.Sprintf("导入成功 %d 条，跳过 %d 条", imported, skipped)
	s.writeLog(c, "host_excel_import", msg)
	c.JSON(http.StatusOK, gin.H{"code": 0, "msg": msg})
}

// upsertHostBasic 按 IPMI 查找主机，存在则更新基础属性、否则新增（不涉及 bond 资源）。
func upsertHostBasic(h *model.HostInfo) error {
	list, err := db.ListHosts()
	if err != nil {
		return err
	}
	for _, ex := range list {
		if ex.IPMIAddr == h.IPMIAddr {
			h.IPMIPass = ex.IPMIPass // 保留旧密码（除非显式传入新密码）
			return db.UpdateHost(ex.ID, h)
		}
	}
	_, err = db.CreateHost(h)
	return err
}

// handleExportHostExcel 导出主机管理 Excel。
func (s *Server) handleExportHostExcel(c *gin.Context) {
	hosts, err := db.ListHosts()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "msg": err.Error()})
		return
	}
	var rows [][]string
	for _, h := range hosts {
		rows = append(rows, []string{
			h.Hostname, h.IPMIAddr, h.IPMIUser, "",
		})
	}
	s.writeLog(c, "host_excel_export", "导出主机管理 Excel")
	_ = writeXlsx(c, "hosts.xlsx", hostExcelColumns, rows)
}
