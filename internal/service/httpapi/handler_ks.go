package httpapi

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"pxe-server/internal/db"
	"pxe-server/internal/logger"
	"pxe-server/internal/model"
	"pxe-server/internal/service/ksrender"
	"pxe-server/internal/util"
)

// handleRenderActiveKS 渲染当前生效 KS 模板（占位符替换 + %pre/%post 动态注入）。
// 供 KS 模板页"查看渲染后 ks 文件"使用。
func (s *Server) handleRenderActiveKS(c *gin.Context) {
	mac := c.Query("mac")
	tplID := parseInt64(c.Query("tpl"))
	imageName := c.Query("image")
	content, err := ksrender.RenderKS(s.cfg, mac, tplID, imageName)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"code": 404, "msg": "渲染失败: " + err.Error()})
		return
	}
	c.Header("Content-Type", "text/plain")
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": content})
}

// handleListKSTemplates KS 模板列表。
func (s *Server) handleListKSTemplates(c *gin.Context) {
	list, err := db.ListKSTemplates()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "msg": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": list})
}

// handleCreateKSTemplate 新增 KS 模板。
func (s *Server) handleCreateKSTemplate(c *gin.Context) {
	var req model.KSTemplateReq
	if !s.parseJSONBody(c, &req) {
		return
	}
	// 仅记录模板名与"是否设置密码"布尔值，禁止明文记录密码
	logger.FromGin(c).With(
		"name", req.Name,
		"has_root_password", req.RootPassword != "",
	).Info("创建 KS 模板")
	if req.Name == "" {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": "模板名称不能为空"})
		return
	}
	if req.RootPassword == "" {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": "Root 密码不能为空"})
		return
	}
	if existing, err := db.GetKSTemplateByName(req.Name); err != nil {
		logger.FromGin(c).Warn("检查 KS 模板重名失败: %v", err)
	} else if existing != nil {
		c.JSON(http.StatusConflict, gin.H{"code": 409, "msg": "模板名称已存在"})
		return
	}
	tpl := &model.KSTemplate{Name: req.Name, OsType: req.OsType, Content: req.Content, RootPassword: req.RootPassword}
	id, err := db.CreateKSTemplate(tpl)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "msg": err.Error()})
		return
	}
	// 若当前无任何生效模板，自动将该模板设为生效并写盘
	if _, err := db.GetActiveKSTemplate(); err != nil {
		if err := s.activateKSTemplate(id); err != nil {
			logger.FromGin(c).Warn("自动激活 KS 模板失败 id=%d: %v", id, err)
		}
	}
	s.writeLog(c, "ks_create", "新增 KS 模板: "+req.Name)
	c.JSON(http.StatusOK, gin.H{"code": 0, "msg": "创建成功", "id": id})
}

// handleUpdateKSTemplate 更新 KS 模板。
func (s *Server) handleUpdateKSTemplate(c *gin.Context) {
	id := parseInt64(c.Param("id"))
	if id <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": "无效ID"})
		return
	}
	old, err := db.GetKSTemplate(id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "msg": err.Error()})
		return
	}
	if old != nil && old.IsDefault == 1 {
		c.JSON(http.StatusForbidden, gin.H{"code": 403, "msg": "默认模板不允许修改"})
		return
	}
	var req model.KSTemplateReq
	if !s.parseJSONBody(c, &req) {
		return
	}
	if existing, err := db.GetKSTemplateByName(req.Name); err != nil {
		logger.FromGin(c).Warn("检查 KS 模板重名失败: %v", err)
	} else if existing != nil && existing.ID != id {
		c.JSON(http.StatusConflict, gin.H{"code": 409, "msg": "模板名称已存在"})
		return
	}
	if req.RootPassword == "" {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": "Root 密码不能为空"})
		return
	}
	tpl := &model.KSTemplate{Name: req.Name, OsType: req.OsType, Content: req.Content, RootPassword: req.RootPassword}
	if err := db.UpdateKSTemplate(id, tpl); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "msg": err.Error()})
		return
	}
	// 仅当编辑的是当前生效模板时，自动同步到磁盘 web_root/ks.cfg
	if active, err := db.GetActiveKSTemplate(); err == nil && active.ID == id {
		if err := writeFileEnsureDir(s.diskKSPath(), []byte(req.Content)); err != nil {
			logger.Error("KS 模板同步磁盘失败: %v", err)
		}
	}
	s.writeLog(c, "ks_update", "更新 KS 模板: "+req.Name)
	c.JSON(http.StatusOK, gin.H{"code": 0, "msg": "更新成功"})
}

// handleSetActiveKSTemplate 设置 KS 模板生效（同一时间仅一个生效）。
func (s *Server) handleSetActiveKSTemplate(c *gin.Context) {
	id := parseInt64(c.Param("id"))
	if id <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": "无效ID"})
		return
	}
	if err := s.activateKSTemplate(id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "msg": err.Error()})
		return
	}
	s.writeLog(c, "ks_active", "设置 KS 模板生效")
	c.JSON(http.StatusOK, gin.H{"code": 0, "msg": "已设置生效"})
}

// ensureKSTemplateActive 确保至少有一个 KS 模板处于生效状态。
func (s *Server) ensureKSTemplateActive() {
	if active, err := db.GetActiveKSTemplate(); err != nil {
		logger.Warn("检查生效 KS 模板失败: %v", err)
	} else if active != nil {
		return
	}
	list, err := db.ListKSTemplates()
	if err != nil {
		logger.Warn("查询 KS 模板列表失败: %v", err)
		return
	}
	if len(list) == 0 {
		return
	}
	// 优先默认模板，否则第一个
	for _, t := range list {
		if t.IsDefault == 1 {
			if err := s.activateKSTemplate(t.ID); err != nil {
				logger.Warn("自动激活默认 KS 模板失败 id=%d: %v", t.ID, err)
			}
			return
		}
	}
	if err := s.activateKSTemplate(list[0].ID); err != nil {
		logger.Warn("自动激活 KS 模板失败 id=%d: %v", list[0].ID, err)
	}
}

// handleDeleteKSTemplate 删除 KS 模板。
func (s *Server) handleDeleteKSTemplate(c *gin.Context) {
	id := parseInt64(c.Param("id"))
	if id <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": "无效ID"})
		return
	}
	old, err := db.GetKSTemplate(id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "msg": err.Error()})
		return
	}
	if old != nil && old.IsDefault == 1 {
		c.JSON(http.StatusForbidden, gin.H{"code": 403, "msg": "默认模板不允许删除"})
		return
	}
	if err := db.DeleteKSTemplate(id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "msg": err.Error()})
		return
	}
	// 删除后自动保证至少有一个生效的模板
	s.ensureKSTemplateActive()
	s.writeLog(c, "ks_delete", "删除 KS 模板 ID="+int64Str(id))
	c.JSON(http.StatusOK, gin.H{"code": 0, "msg": "已删除"})
}

// activateKSTemplate 将指定模板设为生效并写盘 web_root/ks.cfg。
// 写盘前替换 @@PXE_SERVER@@ 和 @@ROOT_PASSWORD@@ 占位符。
func (s *Server) activateKSTemplate(id int64) error {
	t, err := db.SetActiveKSTemplate(id)
	if err != nil {
		return err
	}
	content := s.replaceKSTemplatePlaceholders(t)
	return writeFileEnsureDir(s.diskKSPath(), []byte(content))
}

// replaceKSTemplatePlaceholders 替换 KS 模板中的占位符。
func (s *Server) replaceKSTemplatePlaceholders(t *model.KSTemplate) string {
	content := s.replacePXEPlaceholder(t.Content)
	if t.RootPassword != "" {
		content, _ = util.ReplaceAllPlaceholder(content, "@@ROOT_PASSWORD@@", t.RootPassword)
	}
	return content
}
