package httpapi

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"pxe-server/internal/middleware"
	"pxe-server/internal/model"
)

// handleLogin 登录。
func (s *Server) handleLogin(c *gin.Context) {
	var req model.LoginReq
	if !s.parseJSONBody(c, &req) {
		return
	}
	hc := s.cfg.HTTP()
	if req.Username == hc.AdminUser && req.Password == hc.AdminPassword {
		token := middleware.IssueToken()
		c.Header("X-Operator", req.Username)
		s.writeLog(c, "login", "管理员登录成功")
		c.JSON(http.StatusOK, gin.H{"code": 0, "msg": "登录成功", "data": gin.H{"token": token}})
		return
	}
	c.JSON(http.StatusUnauthorized, gin.H{"code": 401, "msg": "用户名或密码错误"})
}

// handleLogout 登出。
func (s *Server) handleLogout(c *gin.Context) {
	token := c.GetHeader("Authorization")
	if token != "" {
		middleware.InvalidateToken(token)
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "msg": "已退出登录"})
}

// changePwdReq 修改密码请求。
type changePwdReq struct {
	OldPassword string `json:"old_password"`
	NewPassword string `json:"new_password"`
}

// handleChangePassword 修改管理员密码。
func (s *Server) handleChangePassword(c *gin.Context) {
	var req changePwdReq
	if !s.parseJSONBody(c, &req) {
		return
	}
	if req.OldPassword == "" || req.NewPassword == "" {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": "旧密码和新密码不能为空"})
		return
	}
	hc := s.cfg.HTTP()
	if req.OldPassword != hc.AdminPassword {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": "旧密码不正确"})
		return
	}
	if err := s.cfg.UpdateAndReload("http_admin_password", req.NewPassword); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "msg": "保存密码失败: " + err.Error()})
		return
	}
	s.writeLog(c, "change_password", "管理员修改密码")
	c.JSON(http.StatusOK, gin.H{"code": 0, "msg": "密码已修改，请重新登录"})
}
