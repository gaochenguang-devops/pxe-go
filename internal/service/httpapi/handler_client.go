package httpapi

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"pxe-server/internal/db"
	"pxe-server/internal/util"
)

// replacePXEPlaceholder 将内容中的 @@PXE_SERVER@@ 占位符替换为当前 PXE 服务 IP。
func (s *Server) replacePXEPlaceholder(content string) string {
	pxeIP := s.cfg.DHCP().PXEIP
	if pxeIP == "" {
		return content
	}
	replaced, _ := util.ReplaceAllPlaceholder(content, "@@PXE_SERVER@@", pxeIP)
	return replaced
}

// handleKickstartGeneric 从数据库生效模板渲染 ks.cfg，替换 @@PXE_SERVER@@ 和 @@ROOT_PASSWORD@@。
func (s *Server) handleKickstartGeneric(c *gin.Context) {
	tpl, err := db.GetActiveKSTemplate()
	if err != nil {
		c.String(http.StatusNotFound, "ks.cfg not found")
		return
	}
	content := s.replaceKSTemplatePlaceholders(tpl)
	c.Header("Content-Type", "text/plain")
	c.String(http.StatusOK, content)
}
