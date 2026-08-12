package httpapi

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/xuri/excelize/v2"

	"pxe-server/internal/db"
	"pxe-server/internal/logger"
	"pxe-server/internal/model"
	"pxe-server/internal/service/ipmi"
)

// batchIDsReq 批量操作请求体（通用）。
type batchIDsReq struct {
	IDs []int64 `json:"ids"`
}

// batchIPMIReq 批量电源操作请求体。
type batchIPMIReq struct {
	IDs    []int64 `json:"ids"`
	Action string  `json:"action"`
}

// handleBatchDeleteHost 批量删除主机。
func (s *Server) handleBatchDeleteHost(c *gin.Context) {
	var req batchIDsReq
	if !s.parseJSONBody(c, &req) {
		return
	}
	ids := cleanIDs(req.IDs)
	if len(ids) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": "请选择要删除的主机"})
		return
	}
	logger.Info("批量删除主机请求: ids=%v", ids)
	if err := db.DeleteHostBatch(ids); err != nil {
		logger.Error("批量删除主机失败: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "msg": "批量删除失败: " + err.Error()})
		return
	}
	logger.Info("批量删除主机成功: %d 台", len(ids))
	s.writeLog(c, "host_batch_delete", fmt.Sprintf("批量删除主机 %d 台", len(ids)))
	c.JSON(http.StatusOK, gin.H{"code": 0, "msg": fmt.Sprintf("已删除 %d 台主机", len(ids))})
}

// handleBatchIPMI 批量 IPMI 电源操作（on/off/cycle/reset）。逐台执行，返回每台结果。
func (s *Server) handleBatchIPMI(c *gin.Context) {
	var req batchIPMIReq
	if !s.parseJSONBody(c, &req) {
		return
	}
	ids := cleanIDs(req.IDs)
	if len(ids) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": "请选择要操作的主机"})
		return
	}
	action := ipmi.NormalizeAction(req.Action)
	if action == "" {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": "不支持的电源操作"})
		return
	}
	ok, fail := 0, 0
	var failed []string
	for _, id := range ids {
		host, err := db.GetHostByID(id)
		if err != nil || host.IPMIAddr == "" {
			fail++
			failed = append(failed, fmt.Sprintf("%d(未配置IPMI)", id))
			continue
		}
		pass := ipmi.DecryptPass(host.IPMIPass, s.cfg)
		if _, err := s.ipmiClt.RunPower(host.IPMIAddr, host.IPMIUser, pass, action); err != nil {
			fail++
			failed = append(failed, host.Hostname+"("+err.Error()+")")
			continue
		}
		ok++
		s.writeLog(c, "ipmi_power", "批量电源["+action+"]主机["+host.Hostname+"]")
	}
	msg := fmt.Sprintf("批量%s：成功 %d，失败 %d", action, ok, fail)
	if len(failed) > 0 {
		msg += "；失败项：" + strings.Join(failed, ", ")
	}
	s.writeLog(c, "host_batch_ipmi", msg)
	c.JSON(http.StatusOK, gin.H{"code": 0, "msg": msg, "ok": ok, "fail": fail})
}

// handleBatchExportHostExcel 批量导出选中主机的管理 Excel（列同 hostExcelColumns）。
func (s *Server) handleBatchExportHostExcel(c *gin.Context) {
	var req batchIDsReq
	if !s.parseJSONBody(c, &req) {
		return
	}
	ids := cleanIDs(req.IDs)
	hosts, err := db.ListHosts()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "msg": err.Error()})
		return
	}
	idSet := toSet(ids)
	var rows [][]string
	for _, h := range hosts {
		if len(ids) > 0 && !idSet[h.ID] {
			continue
		}
		rows = append(rows, []string{h.Hostname, h.IPMIAddr, h.IPMIUser, ""})
	}
	s.writeLog(c, "host_batch_export", fmt.Sprintf("批量导出主机 Excel %d 条", len(rows)))
	_ = writeXlsx(c, "hosts.xlsx", hostExcelColumns, rows)
}

// handleBatchExportNodeInfoExcel 批量导出选中主机的资源 Excel（列同 nodeInfoColumns），并重新生成 node-info.txt。
func (s *Server) handleBatchExportNodeInfoExcel(c *gin.Context) {
	var req batchIDsReq
	if !s.parseJSONBody(c, &req) {
		return
	}
	ids := cleanIDs(req.IDs)
	resources, err := db.ListHostResources()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "msg": err.Error()})
		return
	}
	idSet := toSet(ids)
	var selected []*model.HostResource
	for _, r := range resources {
		if len(ids) > 0 && !idSet[r.ID] {
			continue
		}
		selected = append(selected, r)
	}
	f := excelize.NewFile()
	sheet := "主机资源"
	f.SetSheetName("Sheet1", sheet)
	for i, col := range nodeInfoColumns {
		cell, _ := excelize.CoordinatesToCellName(i+1, 1)
		f.SetCellValue(sheet, cell, col)
	}
	for r, res := range selected {
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
	if err := writeFileEnsureDir(s.nodeInfoPath(), []byte(buildNodeInfoContent(selected))); err != nil {
		logger.FromGin(c).Warn("重新生成 node-info.txt 失败: %v", err)
	}
	s.writeLog(c, "host_batch_export", fmt.Sprintf("批量导出主机资源 Excel %d 条", len(selected)))
	c.Header("Content-Disposition", "attachment; filename=host-resource.xlsx")
	c.Data(http.StatusOK, "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet", buf.Bytes())
}

// cleanIDs 去除 ids 中非正数项并去重。
func cleanIDs(ids []int64) []int64 {
	seen := map[int64]bool{}
	var out []int64
	for _, id := range ids {
		if id > 0 && !seen[id] {
			seen[id] = true
			out = append(out, id)
		}
	}
	return out
}

// toSet 将 id 列表转为集合。
func toSet(ids []int64) map[int64]bool {
	m := map[int64]bool{}
	for _, id := range ids {
		m[id] = true
	}
	return m
}
