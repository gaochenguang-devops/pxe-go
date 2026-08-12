package httpapi

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"pxe-server/internal/db"
	"pxe-server/internal/logger"
	"pxe-server/internal/model"
)

// ipxeScriptReq iPXE 脚本保存请求。
type ipxeScriptReq struct {
	Name    string `json:"name"`
	Content string `json:"content"`
}

// handleListIPxeScripts iPXE 脚本列表。
func (s *Server) handleListIPxeScripts(c *gin.Context) {
	list, err := db.ListIPxeScripts()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "msg": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": list})
}

// handleCreateIPxeScript 新增 iPXE 脚本（不自动激活）。
func (s *Server) handleCreateIPxeScript(c *gin.Context) {
	var req ipxeScriptReq
	if !s.parseJSONBody(c, &req) {
		return
	}
	req.Name = strings.TrimSpace(req.Name)
	if req.Name == "" {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": "脚本名称不能为空"})
		return
	}
	if req.Content == "" {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": "脚本内容不能为空"})
		return
	}
	scr := &model.IPxeScript{Name: req.Name, Content: req.Content}
	id, err := db.CreateIPxeScript(scr)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "msg": err.Error()})
		return
	}
	// 若当前无任何生效脚本，则自动设为生效并写盘
	if _, err := db.GetActiveIPxeScript(); err != nil {
		if err := s.activateIPxeScript(id); err != nil {
			logger.FromGin(c).Warn("自动激活 iPXE 脚本失败 id=%d: %v", id, err)
		}
	}
	s.writeLog(c, "ipxe_create", "新增 iPXE 脚本: "+req.Name)
	c.JSON(http.StatusOK, gin.H{"code": 0, "msg": "创建成功", "id": id})
}

// handleUpdateIPxeScript 更新 iPXE 脚本。
func (s *Server) handleUpdateIPxeScript(c *gin.Context) {
	id := parseInt64(c.Param("id"))
	if id <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": "无效 ID"})
		return
	}
	old, err := db.GetIPxeScript(id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "msg": err.Error()})
		return
	}
	if old != nil && old.IsDefault == 1 {
		c.JSON(http.StatusForbidden, gin.H{"code": 403, "msg": "默认脚本不允许修改"})
		return
	}
	var req ipxeScriptReq
	if !s.parseJSONBody(c, &req) {
		return
	}
	req.Name = strings.TrimSpace(req.Name)
	if req.Name == "" {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": "脚本名称不能为空"})
		return
	}
	if err := db.UpdateIPxeScript(id, req.Name, req.Content); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "msg": err.Error()})
		return
	}
	// 若该脚本是当前生效的，同步更新磁盘 autoexec.ipxe
	if active, err := db.GetActiveIPxeScript(); err == nil && active.ID == id {
		s.writeIPxeToDisk(req.Content)
	}
	s.writeLog(c, "ipxe_update", "更新 iPXE 脚本: "+req.Name)
	c.JSON(http.StatusOK, gin.H{"code": 0, "msg": "更新成功"})
}

// handleDeleteIPxeScript 删除 iPXE 脚本。
func (s *Server) handleDeleteIPxeScript(c *gin.Context) {
	id := parseInt64(c.Param("id"))
	if id <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": "无效 ID"})
		return
	}
	scr, err := db.GetIPxeScript(id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "msg": err.Error()})
		return
	}
	if scr != nil && scr.IsDefault == 1 {
		c.JSON(http.StatusForbidden, gin.H{"code": 403, "msg": "默认脚本不允许删除"})
		return
	}
	if err := db.DeleteIPxeScript(id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "msg": err.Error()})
		return
	}
	// 删除后自动保证至少有一个生效的脚本
	s.ensureIPxeScriptActive()
	s.writeLog(c, "ipxe_delete", "删除 iPXE 脚本: "+scrName(scr))
	c.JSON(http.StatusOK, gin.H{"code": 0, "msg": "删除成功"})
}

// handleSetActiveIPxeScript 设置 iPXE 脚本生效（同一时间仅一个生效）。
func (s *Server) handleSetActiveIPxeScript(c *gin.Context) {
	id := parseInt64(c.Param("id"))
	if id <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": "无效 ID"})
		return
	}
	if err := s.activateIPxeScript(id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "msg": err.Error()})
		return
	}
	s.writeLog(c, "ipxe_active", "设置 iPXE 脚本生效")
	c.JSON(http.StatusOK, gin.H{"code": 0, "msg": "已设置生效"})
}

// ensureIPxeScriptActive 确保至少有一个 iPXE 脚本处于生效状态。
func (s *Server) ensureIPxeScriptActive() {
	if active, err := db.GetActiveIPxeScript(); err != nil {
		logger.Warn("检查生效 iPXE 脚本失败: %v", err)
	} else if active != nil {
		return
	}
	list, err := db.ListIPxeScripts()
	if err != nil {
		logger.Warn("查询 iPXE 脚本列表失败: %v", err)
		return
	}
	if len(list) == 0 {
		return
	}
	for _, scr := range list {
		if scr.IsDefault == 1 {
			if err := s.activateIPxeScript(scr.ID); err != nil {
				logger.Warn("自动激活默认 iPXE 脚本失败 id=%d: %v", scr.ID, err)
			}
			return
		}
	}
	if err := s.activateIPxeScript(list[0].ID); err != nil {
		logger.Warn("自动激活 iPXE 脚本失败 id=%d: %v", list[0].ID, err)
	}
}

// activateIPxeScript 将脚本设为生效并写盘 autoexec.ipxe。
func (s *Server) activateIPxeScript(id int64) error {
	scr, err := db.SetActiveIPxeScript(id)
	if err != nil {
		return err
	}
	return s.writeIPxeToDisk(scr.Content)
}

// writeIPxeToDisk 将 iPXE 脚本内容写入 tftp_root/autoexec.ipxe（供 TFTP 下发）。
// 由于 TFTP 传输无法做占位符替换，写盘前将 @@PXE_SERVER@@ 替换为当前 PXE 服务 IP。
func (s *Server) writeIPxeToDisk(content string) error {
	content = s.replacePXEPlaceholder(content)
	if err := writeFileEnsureDir(s.autoexecIPxePath(), []byte(content)); err != nil {
		logger.Error("写入 autoexec.ipxe 失败: %v", err)
		return err
	}
	logger.Info("已更新 autoexec.ipxe")
	return nil
}

func scrName(s *model.IPxeScript) string {
	if s == nil {
		return ""
	}
	return s.Name
}
