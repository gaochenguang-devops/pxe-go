package httpapi

import (
	"net/http"
	"path/filepath"
	"time"

	"github.com/gin-gonic/gin"

	"pxe-server/internal/db"
	"pxe-server/internal/model"
)

// deployPath 返回 web_root/deploy.sh 路径。
func (s *Server) deployPath() string {
	root := s.cfg.HTTP().WebRoot
	if root == "" {
		root = "assets/web_root"
	}
	return filepath.Join(root, "deploy.sh")
}

// handleListDeployScripts 列出所有部署脚本。
func (s *Server) handleListDeployScripts(c *gin.Context) {
	list, err := db.ListDeployScripts()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "msg": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": list})
}

// handleCreateDeployScript 新增部署脚本。
func (s *Server) handleCreateDeployScript(c *gin.Context) {
	var req struct {
		Name    string `json:"name"`
		Content string `json:"content"`
	}
	if !s.parseJSONBody(c, &req) {
		return
	}
	src := &model.DeployScript{
		Name:       req.Name,
		Content:    req.Content,
		Active:     0,
		CreateTime: time.Now(),
	}
	id, err := db.CreateDeployScript(src)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "msg": err.Error()})
		return
	}
	s.writeLog(c, "deploy_create", "创建部署脚本: "+req.Name)
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": id, "msg": "创建成功"})
}

// handleUpdateDeployScript 更新部署脚本。
func (s *Server) handleUpdateDeployScript(c *gin.Context) {
	id := parseInt64(c.Param("id"))
	if id <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": "无效 ID"})
		return
	}
	old, _ := db.GetDeployScript(id)
	if old != nil && old.IsDefault == 1 {
		c.JSON(http.StatusForbidden, gin.H{"code": 403, "msg": "默认脚本不允许修改"})
		return
	}
	var req struct {
		Name    string `json:"name"`
		Content string `json:"content"`
	}
	if !s.parseJSONBody(c, &req) {
		return
	}
	if err := db.UpdateDeployScript(id, req.Name, req.Content); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "msg": err.Error()})
		return
	}
	s.writeLog(c, "deploy_update", "更新部署脚本: "+req.Name)
	c.JSON(http.StatusOK, gin.H{"code": 0, "msg": "更新成功"})
}

// handleDeleteDeployScript 删除部署脚本。
func (s *Server) handleDeleteDeployScript(c *gin.Context) {
	id := parseInt64(c.Param("id"))
	if id <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": "无效 ID"})
		return
	}
	old, _ := db.GetDeployScript(id)
	if old != nil && old.IsDefault == 1 {
		c.JSON(http.StatusForbidden, gin.H{"code": 403, "msg": "默认脚本不允许删除"})
		return
	}
	if err := db.DeleteDeployScript(id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "msg": err.Error()})
		return
	}
	// 删除后自动保证至少有一个生效的脚本
	s.ensureDeployScriptActive()
	s.writeLog(c, "deploy_delete", "删除部署脚本")
	c.JSON(http.StatusOK, gin.H{"code": 0, "msg": "已删除"})
}

// handleSetActiveDeployScript 设置部署脚本生效。
func (s *Server) handleSetActiveDeployScript(c *gin.Context) {
	id := parseInt64(c.Param("id"))
	if id <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": "无效 ID"})
		return
	}
	if err := db.SetActiveDeployScript(id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "msg": err.Error()})
		return
	}
	// 将生效脚本持久化到 web_root/deploy.sh
	scr, _ := db.GetDeployScript(id)
	if scr != nil {
		_ = writeFileEnsureDir(s.deployPath(), []byte(scr.Content))
	}
	s.writeLog(c, "deploy_active", "设置生效部署脚本")
	c.JSON(http.StatusOK, gin.H{"code": 0, "msg": "已生效并持久化到 deploy.sh"})
}

// ensureDeployScriptActive 确保至少有一个部署脚本处于生效状态。
// 如果没有生效的脚本，自动将默认脚本（或第一个可用脚本）设为生效。
func (s *Server) ensureDeployScriptActive() {
	// 检查当前是否有生效的脚本
	if active, _ := db.GetActiveDeployScript(); active != nil {
		return // 已有生效的，无需处理
	}
	// 没有生效的，优先用默认脚本
	list, _ := db.ListDeployScripts()
	if len(list) == 0 {
		return
	}
	// 优先找默认脚本
	for _, scr := range list {
		if scr.IsDefault == 1 {
			_ = db.SetActiveDeployScript(scr.ID)
			return
		}
	}
	// 没有默认的，用第一个
	_ = db.SetActiveDeployScript(list[0].ID)
}

// handleGetDeployScriptContent 读取脚本详情（用于编辑预览）。
func (s *Server) handleGetDeployScriptContent(c *gin.Context) {
	id := parseInt64(c.Param("id"))
	if id <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": "无效 ID"})
		return
	}
	scr, err := db.GetDeployScript(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"code": 404, "msg": "脚本不存在"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": gin.H{
		"name":    scr.Name,
		"content": scr.Content,
	}})
}
