package httpapi

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"pxe-server/internal/db"
	"pxe-server/internal/logger"
)

// handleGetConfig 获取全局配置。
func (s *Server) handleGetConfig(c *gin.Context) {
	rows, err := db.GetAllConfigs()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "msg": err.Error()})
		return
	}
	cfg := map[string]string{}
	for _, r := range rows {
		cfg[r.ConfigKey] = r.ConfigValue
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": cfg})
}

// dhcpUpdateReq DHCP 配置更新请求（地址池由子网接口管理，此处不含单网段字段）。
type dhcpUpdateReq struct {
	Enabled      *bool   `json:"enabled"`
	ListenIP     *string `json:"listen_ip"`
	Interface    *string `json:"interface"`
	PXEIP        *string `json:"pxe_ip"`
	LeaseTime    *int    `json:"lease_time"`
	BootFileBIOS *string `json:"boot_file_bios"`
	BootFileX86  *string `json:"boot_file_x86"`
	BootFileARM  *string `json:"boot_file_arm"`
	IpxeScript   *string `json:"ipxe_script"`
}

// handleUpdateDHCP 更新 DHCP 配置并热重载。
func (s *Server) handleUpdateDHCP(c *gin.Context) {
	var req dhcpUpdateReq
	if !s.parseJSONBody(c, &req) {
		return
	}
	updates := map[string]string{}
	setStr(&updates, "dhcp_listen_ip", req.ListenIP)
	setStr(&updates, "dhcp_interface", req.Interface)
	setStr(&updates, "dhcp_pxe_ip", req.PXEIP)
	setStr(&updates, "dhcp_boot_file_bios", req.BootFileBIOS)
	setStr(&updates, "dhcp_boot_file_x86", req.BootFileX86)
	setStr(&updates, "dhcp_boot_file_arm", req.BootFileARM)
	setStr(&updates, "dhcp_ipxe_script", req.IpxeScript)
	if req.Enabled != nil {
		setStrVal(&updates, "dhcp_enabled", boolStr(*req.Enabled))
	}
	if req.LeaseTime != nil {
		setStrVal(&updates, "dhcp_lease_time", intStr(*req.LeaseTime))
	}
	// 递增配置版本号触发热加载
	bumpVersion(updates, "dhcp_config_version")

	for k, v := range updates {
		if err := db.SetConfig(k, v); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "msg": err.Error()})
			return
		}
	}
	if err := s.cfg.ReloadAll(); err != nil {
		logger.Error("配置重载失败: %v", err)
	}
	s.writeLog(c, "cfg_dhcp_update", "更新 DHCP 配置")
	c.JSON(http.StatusOK, gin.H{"code": 0, "msg": "DHCP 配置已更新并热重载"})
}

// tftpUpdateReq TFTP 配置更新请求。
type tftpUpdateReq struct {
	Enabled         *bool   `json:"enabled"`
	ListenIP        *string `json:"listen_ip"`
	RootDir         *string `json:"root_dir"`
	TransferTimeout *int    `json:"transfer_timeout"`
	MaxConnections  *int    `json:"max_connections"`
	AccessLog       *bool   `json:"access_log"`
}

// handleUpdateTFTP 更新 TFTP 配置并热重载。
func (s *Server) handleUpdateTFTP(c *gin.Context) {
	var req tftpUpdateReq
	if !s.parseJSONBody(c, &req) {
		return
	}
	updates := map[string]string{}
	setStr(&updates, "tftp_listen_ip", req.ListenIP)
	setStr(&updates, "tftp_root_dir", req.RootDir)
	if req.Enabled != nil {
		setStrVal(&updates, "tftp_enabled", boolStr(*req.Enabled))
	}
	if req.TransferTimeout != nil {
		setStrVal(&updates, "tftp_transfer_timeout", intStr(*req.TransferTimeout))
	}
	if req.MaxConnections != nil {
		setStrVal(&updates, "tftp_max_connections", intStr(*req.MaxConnections))
	}
	if req.AccessLog != nil {
		setStrVal(&updates, "tftp_access_log", boolStr(*req.AccessLog))
	}
	bumpVersion(updates, "tftp_config_version")

	for k, v := range updates {
		if err := db.SetConfig(k, v); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "msg": err.Error()})
			return
		}
	}
	if err := s.cfg.ReloadAll(); err != nil {
		logger.Error("配置重载失败: %v", err)
	}
	s.writeLog(c, "cfg_tftp_update", "更新 TFTP 配置")
	c.JSON(http.StatusOK, gin.H{"code": 0, "msg": "TFTP 配置已更新并热重载"})
}

// httpUpdateReq HTTP 配置更新请求。
type httpUpdateReq struct {
	ListenAddr    *string `json:"listen_addr"`
	WebRoot       *string `json:"web_root"`
	AdminUser     *string `json:"admin_user"`
	AdminPassword *string `json:"admin_password"`
}

// handleUpdateHTTP 更新 HTTP 配置。
func (s *Server) handleUpdateHTTP(c *gin.Context) {
	var req httpUpdateReq
	if !s.parseJSONBody(c, &req) {
		return
	}
	updates := map[string]string{}
	setStr(&updates, "http_listen_addr", req.ListenAddr)
	setStr(&updates, "http_web_root", req.WebRoot)
	setStr(&updates, "http_admin_user", req.AdminUser)
	if req.AdminPassword != nil && *req.AdminPassword != "" {
		setStr(&updates, "http_admin_password", req.AdminPassword)
	}
	bumpVersion(updates, "http_config_version")

	for k, v := range updates {
		if err := db.SetConfig(k, v); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "msg": err.Error()})
			return
		}
	}
	if err := s.cfg.ReloadAll(); err != nil {
		logger.Error("配置重载失败: %v", err)
	}
	s.writeLog(c, "cfg_http_update", "更新 HTTP 配置")
	c.JSON(http.StatusOK, gin.H{"code": 0, "msg": "HTTP 配置已更新"})
}

// 辅助函数
func setStr(m *map[string]string, k string, v *string) {
	if v != nil {
		(*m)[k] = *v
	}
}

func setStrVal(m *map[string]string, k string, v string) {
	(*m)[k] = v
}

func boolStr(b bool) string {
	if b {
		return "true"
	}
	return "false"
}

func intStr(i int) string {
	if i == 0 {
		return "0"
	}
	neg := i < 0
	if neg {
		i = -i
	}
	var d []byte
	for i > 0 {
		d = append([]byte{byte('0' + i%10)}, d...)
		i /= 10
	}
	if neg {
		return "-" + string(d)
	}
	return string(d)
}

func bumpVersion(m map[string]string, key string) {
	cur, err := db.GetConfigVersion(key)
	if err != nil {
		cur = 1
	}
	m[key] = intStr(int(cur + 1))
}
