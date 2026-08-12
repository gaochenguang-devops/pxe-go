package httpapi

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"pxe-server/internal/db"
	"pxe-server/internal/model"
	"pxe-server/internal/service/ipmi"
	"pxe-server/internal/util"
)

// hostVO 主机视图对象（IPMI 密码脱敏）。
type hostVO struct {
	model.HostInfo
	IPMIPassMasked string `json:"ipmi_pass_masked"`
}

// handleListHosts 主机列表（支持分页参数 page / pageSize / search；无分页参数时返回全量）。
func (s *Server) handleListHosts(c *gin.Context) {
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

	var list []*model.HostInfo
	var err error

	if hasPage {
		offset := (page - 1) * pageSize
		list, err = db.ListHostsPage(pageSize, offset, search)
	} else {
		list, err = db.ListHosts()
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "msg": err.Error()})
		return
	}

	vos := make([]hostVO, 0, len(list))
	for _, h := range list {
		vo := hostVO{HostInfo: *h}
		if h.IPMIPass != "" {
			vo.IPMIPassMasked = "******"
			vo.IPMIPass = ""
		}
		vos = append(vos, vo)
	}

	if hasPage {
		total, err := db.CountHosts(search)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "msg": err.Error()})
			return
		}
		totalPages := (total + pageSize - 1) / pageSize
		c.JSON(http.StatusOK, gin.H{
			"code":       0,
			"data":       vos,
			"total":      total,
			"page":       page,
			"pageSize":   pageSize,
			"totalPages": totalPages,
		})
	} else {
		c.JSON(http.StatusOK, gin.H{"code": 0, "data": vos})
	}
}

// handleCreateHost 新增主机（数据库层+接口层双重校验 IPMI 唯一）。
func (s *Server) handleCreateHost(c *gin.Context) {
	var req model.HostReq
	if !s.parseJSONBody(c, &req) {
		return
	}
	if req.IPMIAddr != "" && !util.IsValidIPv4(req.IPMIAddr) {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": "IPMI 地址不合法"})
		return
	}
	// 接口层唯一性校验
	if err := db.CheckHostUnique(req.IPMIAddr, "", 0); err != nil {
		if err == db.ErrDuplicateIPMI {
			c.JSON(http.StatusConflict, gin.H{"code": 409, "msg": "该IPMI地址已绑定其他主机"})
			return
		}
		c.JSON(http.StatusConflict, gin.H{"code": 409, "msg": err.Error()})
		return
	}
	encPass := ipmi.EncryptPass(req.IPMIPass, s.cfg)
	host := &model.HostInfo{
		Hostname:      req.Hostname,
		IPMIAddr:      req.IPMIAddr,
		IPMIUser:      req.IPMIUser,
		IPMIPass:      encPass,
		InstallStatus: req.InstallStatus,
	}
	if host.InstallStatus == "" {
		host.InstallStatus = "idle"
	}
	id, err := db.CreateHost(host)
	if err != nil {
		if err == db.ErrDuplicateIPMI {
			c.JSON(http.StatusConflict, gin.H{"code": 409, "msg": "该IPMI地址已绑定其他主机"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "msg": err.Error()})
		return
	}
	s.writeLog(c, "host_create", "新增主机: "+host.Hostname+" IPMI="+host.IPMIAddr)
	c.JSON(http.StatusOK, gin.H{"code": 0, "msg": "新增成功", "id": id})
}

// handleUpdateHost 编辑主机。
func (s *Server) handleUpdateHost(c *gin.Context) {
	id := parseInt64(c.Param("id"))
	if id <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": "无效ID"})
		return
	}
	var req model.HostReq
	if !s.parseJSONBody(c, &req) {
		return
	}
	// 数据库层唯一性校验（排除自身）
	if err := db.CheckHostUnique(req.IPMIAddr, "", id); err != nil {
		if err == db.ErrDuplicateIPMI {
			c.JSON(http.StatusConflict, gin.H{"code": 409, "msg": "该IPMI地址已绑定其他主机"})
			return
		}
		c.JSON(http.StatusConflict, gin.H{"code": 409, "msg": err.Error()})
		return
	}
	host := &model.HostInfo{
		Hostname:      req.Hostname,
		IPMIAddr:      req.IPMIAddr,
		IPMIUser:      req.IPMIUser,
		IPMIPass:      req.IPMIPass,
		InstallStatus: req.InstallStatus,
	}
	// 若未传新密码则保留旧密码
	if host.IPMIPass != "" && host.IPMIPass != "******" {
		host.IPMIPass = ipmi.EncryptPass(host.IPMIPass, s.cfg)
	} else {
		old, err := db.GetHostByID(id)
		if err == nil {
			host.IPMIPass = old.IPMIPass
		}
	}
	if err := db.UpdateHost(id, host); err != nil {
		if err == db.ErrDuplicateIPMI {
			c.JSON(http.StatusConflict, gin.H{"code": 409, "msg": "该IPMI地址已绑定其他主机"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "msg": err.Error()})
		return
	}
	s.writeLog(c, "host_update", "编辑主机: "+host.Hostname)
	c.JSON(http.StatusOK, gin.H{"code": 0, "msg": "更新成功"})
}

// handleDeleteHost 删除主机。
func (s *Server) handleDeleteHost(c *gin.Context) {
	id := parseInt64(c.Param("id"))
	if id <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": "无效ID"})
		return
	}
	if err := db.DeleteHost(id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "msg": err.Error()})
		return
	}
	s.writeLog(c, "host_delete", "删除主机 ID="+int64Str(id))
	c.JSON(http.StatusOK, gin.H{"code": 0, "msg": "已删除"})
}

// ipmiPowerReq IPMI 电源操作请求。
type ipmiPowerReq struct {
	Action string `json:"action" binding:"required"` // on/off/cycle/status/reset
}

// handleIPMIPower IPMI 电源操作。
func (s *Server) handleIPMIPower(c *gin.Context) {
	id := parseInt64(c.Param("id"))
	if id <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": "无效ID"})
		return
	}
	var req ipmiPowerReq
	if !s.parseJSONBody(c, &req) {
		return
	}
	action := ipmi.NormalizeAction(req.Action)
	if action == "" {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": "不支持的电源操作"})
		return
	}
	host, err := db.GetHostByID(id)
	if err != nil || host.IPMIAddr == "" {
		c.JSON(http.StatusNotFound, gin.H{"code": 404, "msg": "主机不存在或未配置IPMI"})
		return
	}
	pass := ipmi.DecryptPass(host.IPMIPass, s.cfg)
	out, err := s.ipmiClt.RunPower(host.IPMIAddr, host.IPMIUser, pass, action)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "msg": "IPMI 操作失败: " + err.Error()})
		s.writeLog(c, "ipmi_power", "主机["+host.Hostname+"]电源操作["+action+"]失败: "+err.Error())
		return
	}
	s.writeLog(c, "ipmi_power", "主机["+host.Hostname+"]执行电源操作["+action+"]")
	c.JSON(http.StatusOK, gin.H{"code": 0, "msg": "操作已执行", "data": out})
}

// handleIPMIStatus 查询电源状态。
func (s *Server) handleIPMIStatus(c *gin.Context) {
	id := parseInt64(c.Param("id"))
	if id <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": "无效ID"})
		return
	}
	host, err := db.GetHostByID(id)
	if err != nil || host.IPMIAddr == "" {
		c.JSON(http.StatusNotFound, gin.H{"code": 404, "msg": "主机不存在或未配置IPMI"})
		return
	}
	pass := ipmi.DecryptPass(host.IPMIPass, s.cfg)
	out, err := s.ipmiClt.PowerStatus(host.IPMIAddr, host.IPMIUser, pass)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "msg": "查询失败: " + err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": gin.H{"status": out}})
}

// ipmiBootReq IPMI 引导设备设置请求。
type ipmiBootReq struct {
	Device string `json:"device"` // pxe/disk
}

// handleIPMIBootDevice 设置下次启动设备。
func (s *Server) handleIPMIBootDevice(c *gin.Context) {
	id := parseInt64(c.Param("id"))
	if id <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": "无效ID"})
		return
	}
	var req ipmiBootReq
	if !s.parseJSONBody(c, &req) {
		return
	}
	host, err := db.GetHostByID(id)
	if err != nil || host.IPMIAddr == "" {
		c.JSON(http.StatusNotFound, gin.H{"code": 404, "msg": "主机不存在或未配置IPMI"})
		return
	}
	pass := ipmi.DecryptPass(host.IPMIPass, s.cfg)
	var out string
	if req.Device == "pxe" {
		out, err = s.ipmiClt.SetBootDevicePXE(host.IPMIAddr, host.IPMIUser, pass)
	} else {
		out, err = s.ipmiClt.SetBootDeviceDisk(host.IPMIAddr, host.IPMIUser, pass)
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "msg": "设置失败: " + err.Error()})
		return
	}
	s.writeLog(c, "ipmi_boot", "主机["+host.Hostname+"]设置启动设备["+req.Device+"]")
	c.JSON(http.StatusOK, gin.H{"code": 0, "msg": "已设置", "data": out})
}
