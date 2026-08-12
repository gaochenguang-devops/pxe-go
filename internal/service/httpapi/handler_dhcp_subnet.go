package httpapi

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"pxe-server/internal/db"
	"pxe-server/internal/logger"
	"pxe-server/internal/model"
)

// subnetReq DHCP 子网请求体。
type subnetReq struct {
	Name        string `json:"name"`
	IPPoolStart string `json:"ip_pool_start"`
	IPPoolEnd   string `json:"ip_pool_end"`
	SubnetMask  string `json:"subnet_mask"`
	Gateway     string `json:"gateway"`
	DNSServers  string `json:"dns_servers"`
	Enabled     bool   `json:"enabled"`
}

// handleListDHCPSubnets 子网列表。
func (s *Server) handleListDHCPSubnets(c *gin.Context) {
	list, err := db.ListDHCPSubnets()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "msg": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": list})
}

// handleCreateDHCPSubnet 新增子网。
func (s *Server) handleCreateDHCPSubnet(c *gin.Context) {
	var req subnetReq
	if !s.parseJSONBody(c, &req) {
		return
	}
	if req.IPPoolStart == "" || req.IPPoolEnd == "" || req.SubnetMask == "" {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": "地址池起止与掩码不能为空"})
		return
	}
	sub := &model.DHCPSubnet{
		Name:        req.Name,
		IPPoolStart: req.IPPoolStart,
		IPPoolEnd:   req.IPPoolEnd,
		SubnetMask:  req.SubnetMask,
		Gateway:     req.Gateway,
		DNSServers:  req.DNSServers,
		Enabled:     req.Enabled,
	}
	id, err := db.CreateDHCPSubnet(sub)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "msg": err.Error()})
		return
	}
	s.bumpDHCPVersion()
	s.writeLog(c, "cfg_dhcp_subnet_add", "新增 DHCP 网段 "+req.Name)
	c.JSON(http.StatusOK, gin.H{"code": 0, "msg": "网段已新增", "id": id})
}

// handleUpdateDHCPSubnet 更新子网。
func (s *Server) handleUpdateDHCPSubnet(c *gin.Context) {
	id := parseInt64(c.Param("id"))
	if id <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": "无效ID"})
		return
	}
	var req subnetReq
	if !s.parseJSONBody(c, &req) {
		return
	}
	sub := &model.DHCPSubnet{
		ID:          id,
		Name:        req.Name,
		IPPoolStart: req.IPPoolStart,
		IPPoolEnd:   req.IPPoolEnd,
		SubnetMask:  req.SubnetMask,
		Gateway:     req.Gateway,
		DNSServers:  req.DNSServers,
		Enabled:     req.Enabled,
	}
	if err := db.UpdateDHCPSubnet(id, sub); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "msg": err.Error()})
		return
	}
	s.bumpDHCPVersion()
	s.writeLog(c, "cfg_dhcp_subnet_update", "更新 DHCP 网段 ID="+int64Str(id))
	c.JSON(http.StatusOK, gin.H{"code": 0, "msg": "网段已更新"})
}

// handleDeleteDHCPSubnet 删除子网。
func (s *Server) handleDeleteDHCPSubnet(c *gin.Context) {
	id := parseInt64(c.Param("id"))
	if id <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": "无效ID"})
		return
	}
	if err := db.DeleteDHCPSubnet(id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "msg": err.Error()})
		return
	}
	s.bumpDHCPVersion()
	s.writeLog(c, "cfg_dhcp_subnet_delete", "删除 DHCP 网段 ID="+int64Str(id))
	c.JSON(http.StatusOK, gin.H{"code": 0, "msg": "网段已删除"})
}

// bumpDHCPVersion 递增 DHCP 配置版本并重载，触发 DHCP 服务热重载。
func (s *Server) bumpDHCPVersion() {
	updates := map[string]string{}
	bumpVersion(updates, "dhcp_config_version")
	for k, v := range updates {
		if err := db.SetConfig(k, v); err != nil {
			logger.Warn("递增 DHCP 版本失败: %v", err)
		}
	}
	if err := s.cfg.ReloadAll(); err != nil {
		logger.Warn("配置重载失败: %v", err)
	}
}
