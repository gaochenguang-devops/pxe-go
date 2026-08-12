// Package model 定义数据库实体、配置结构体与请求入参。
package model

import "time"

// ============ 数据库实体 ============

// SysConfig DHCP/TFTP/HTTP 全局配置，配置热加载数据源。
type SysConfig struct {
	ID          int64  `json:"id"`
	ConfigKey   string `json:"config_key"`
	ConfigValue string `json:"config_value"`
	ConfigDesc  string `json:"config_desc"`
	UpdateTime  int64  `json:"update_time"`
}

// PXEResource 上传导入资源表。
type PXEResource struct {
	ID         int64     `json:"id"`
	Name       string    `json:"name"`
	ResType    string    `json:"res_type"`    // firmware/kernel/initrd/script/ks_template/repo/other
	ArchType   string    `json:"arch_type"`   // bios/x86_uefi/aarch64_uefi/all
	LocalPath  string    `json:"local_path"`  // 存储路径（TFTP 根目录或 HTTP 目录）
	Size       int64     `json:"size"`        // 文件大小（字节）
	UploadTime time.Time `json:"upload_time"` // 上传时间
}

// OSImage 系统镜像表（一个镜像名称对应两个架构，同一时间仅一个 active=1 为默认安装镜像）。
type OSImage struct {
	ID          int64  `json:"id"`
	Name        string `json:"name"` // 镜像名称，如 euler1
	X86RepoPath string `json:"x86_repo_path"`
	X86IsoPath  string `json:"x86_iso_path"`
	ArmRepoPath string `json:"arm_repo_path"`
	ArmIsoPath  string `json:"arm_iso_path"`
	Active      int    `json:"active"` // 1=默认生效，0=未生效
}

// KSTemplate KS 无人值守模板表（同一时间仅一个 active=1）。
type KSTemplate struct {
	ID           int64     `json:"id"`
	Name         string    `json:"name"`
	OsType       string    `json:"os_type"`
	Content      string    `json:"content"`       // 原始带占位符 ks 文本
	RootPassword string    `json:"root_password"` // root 密码，替换 @@ROOT_PASSWORD@@ 占位符
	Active       int       `json:"active"`        // 1=生效，0=未生效；全局仅一个为 1
	IsDefault    int       `json:"is_default"`    // 1=内置默认模板（不可删除/修改）
	CreateTime   time.Time `json:"create_time"`
}

// IPxeScript iPXE 脚本表（同一时间仅一个 active=1）。
type IPxeScript struct {
	ID         int64     `json:"id"`
	Name       string    `json:"name"`
	Content    string    `json:"content"`
	Active     int       `json:"active"`    // 1=生效，0=未生效；全局仅一个为 1
	IsDefault  int       `json:"is_default"` // 1=内置默认模板（不可删除/修改）
	CreateTime time.Time `json:"create_time"`
}

// HostInfo 主机管理表（ipmi_addr 唯一索引）。仅包含主机基础属性，Bond 网络资源见 HostResource 表。
type HostInfo struct {
	ID            int64     `json:"id"`
	Hostname      string    `json:"hostname"`
	IPMIAddr      string    `json:"ipmi_addr"` // UNIQUE
	IPMIUser      string    `json:"ipmi_user"`
	IPMIPass      string    `json:"ipmi_pass"` // 加密存储
	InstallStatus string    `json:"install_status"`
	CreateTime    time.Time `json:"create_time"`
}

// HostResource 主机资源表（ipmi_addr 唯一索引）。存储 Bond 网络配置，用于生成 node-info.txt。
type HostResource struct {
	ID       int64  `json:"id"`
	IPMIAddr string `json:"ipmi_addr"` // UNIQUE，关联 HostInfo
	Hostname string `json:"hostname"`
	// Bond0（业务网络）
	Bond0IP       string `json:"bond0_ip"`
	Bond0Mask     string `json:"bond0_mask"`
	Bond0Gateway  string `json:"bond0_gateway"`
	Bond0IPv6     string `json:"bond0_ipv6"`
	Bond0IPv6Mask string `json:"bond0_ipv6mask"`
	Bond0IPv6Gw   string `json:"bond0_ipv6gw"`
	// Bond2（存储网络）
	Bond2IP       string `json:"bond2_ip"`
	Bond2Mask     string `json:"bond2_mask"`
	Bond2Gateway  string `json:"bond2_gateway"`
	Bond2IPv6     string `json:"bond2_ipv6"`
	Bond2IPv6Mask string `json:"bond2_ipv6mask"`
	Bond2IPv6Gw   string `json:"bond2_ipv6gw"`
	// Bond1（服务网络）
	Bond1IP      string `json:"bond1_ip"`
	Bond1Mask    string `json:"bond1_mask"`
	Bond1Gateway string `json:"bond1_gateway"`
}

// DeployScript 部署脚本表（单条，持久化到 web_root/deploy.sh）。
type DeployScript struct {
	ID         int64     `json:"id"`
	Name       string    `json:"name"`
	Content    string    `json:"content"`
	Active     int       `json:"active"`     // 1=生效，0=未生效；全局仅一个为 1
	IsDefault  int       `json:"is_default"` // 1=内置默认模板（不可删除/修改）
	CreateTime time.Time `json:"create_time"`
}

// OperLog 操作审计日志表。
type OperLog struct {
	ID       int64     `json:"id"`
	Operator string    `json:"operator"`
	OpType   string    `json:"op_type"`
	Detail   string    `json:"detail"`
	ClientIP string    `json:"client_ip"`
	OpTime   time.Time `json:"op_time"`
}

// InstallRecord 装机完成上报记录。
type InstallRecord struct {
	ID         int64     `json:"id"`
	Hostname   string    `json:"hostname"`
	IPMIAddr   string    `json:"ipmi_addr"`
	MAC        string    `json:"mac"`
	IP         string    `json:"ip"`
	ClientIP   string    `json:"client_ip"`
	Arch       string    `json:"arch"`
	Interfaces string    `json:"interfaces"`
	LLDP       string    `json:"lldp"`
	Status     string    `json:"status"`
	ReportTime time.Time `json:"report_time"`
}

// ============ 配置结构体 ============

// DHCPConfig DHCP 全局配置。
type DHCPConfig struct {
	Enabled      bool   `json:"enabled"`
	ListenIP     string `json:"listen_ip"`     // 监听 IP（UDP 67）
	Interface    string `json:"interface"`     // 绑定网卡
	PXEIP        string `json:"pxe_ip"`        // 服务本机 PXE_IP
	IPPoolStart  string `json:"ip_pool_start"` // 地址池起始
	IPPoolEnd    string `json:"ip_pool_end"`   // 地址池结束
	SubnetMask   string `json:"subnet_mask"`   // 子网掩码
	Gateway      string `json:"gateway"`       // 网关
	DNSServers   string `json:"dns_servers"`   // DNS，逗号分隔
	LeaseTime    int    `json:"lease_time"`    // 租期（秒）
	BootFileBIOS string `json:"boot_file_bios"` // BIOS 引导文件
	BootFileX86  string `json:"boot_file_x86"`  // x86_64 UEFI 引导文件
	BootFileARM  string `json:"boot_file_arm"`  // aarch64 UEFI 引导文件
	IpxeScript  string `json:"ipxe_script"`   // iPXE 脚本（识别 Option 175 后下发）
	ConfigVersion int64  `json:"config_version"` // 配置版本，用于热加载检测
}

// TFTPConfig TFTP 全局配置。
type TFTPConfig struct {
	Enabled        bool   `json:"enabled"`
	ListenIP       string `json:"listen_ip"`   // UDP 69 监听 IP
	RootDir        string `json:"root_dir"`    // TFTP 根目录
	TransferTimeout int   `json:"transfer_timeout"` // UDP 传输超时（秒）
	MaxConnections  int   `json:"max_connections"` // 最大并发连接
	AccessLog       bool   `json:"access_log"`     // 访问日志开关
	ConfigVersion  int64  `json:"config_version"`
}

// HTTPConfig HTTP 全局配置。
type HTTPConfig struct {
	ListenAddr    string `json:"listen_addr"`    // HTTP 监听地址，如 0.0.0.0:80
	WebRoot       string `json:"web_root"`       // HTTP 资源根目录
	AdminUser     string `json:"admin_user"`     // 后台管理员账号
	AdminPassword string `json:"admin_password"` // 后台管理员密码
	ConfigVersion int64  `json:"config_version"`
}

// ============ 请求入参 ============

// LoginReq 登录请求。
type LoginReq struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
}

// HostReq 主机新增/编辑请求（仅基础属性，Bond 网络资源见 HostResourceReq）。
type HostReq struct {
	Hostname      string `json:"hostname"`
	IPMIAddr      string `json:"ipmi_addr"`
	IPMIUser      string `json:"ipmi_user"`
	IPMIPass      string `json:"ipmi_pass"`
	InstallStatus string `json:"install_status"`
}

// HostResourceReq 主机资源新增/编辑请求（Bond 网络配置）。
type HostResourceReq struct {
	IPMIAddr      string `json:"ipmi_addr"`
	Hostname      string `json:"hostname"`
	Bond0IP       string `json:"bond0_ip"`
	Bond0Mask     string `json:"bond0_mask"`
	Bond0Gateway  string `json:"bond0_gateway"`
	Bond0IPv6     string `json:"bond0_ipv6"`
	Bond0IPv6Mask string `json:"bond0_ipv6mask"`
	Bond0IPv6Gw   string `json:"bond0_ipv6gw"`
	Bond2IP       string `json:"bond2_ip"`
	Bond2Mask     string `json:"bond2_mask"`
	Bond2Gateway  string `json:"bond2_gateway"`
	Bond2IPv6     string `json:"bond2_ipv6"`
	Bond2IPv6Mask string `json:"bond2_ipv6mask"`
	Bond2IPv6Gw   string `json:"bond2_ipv6gw"`
	Bond1IP       string `json:"bond1_ip"`
	Bond1Mask     string `json:"bond1_mask"`
	Bond1Gateway  string `json:"bond1_gateway"`
}

// KSTemplateReq KS 模板请求。
type KSTemplateReq struct {
	Name         string `json:"name" binding:"required"`
	OsType       string `json:"os_type"`
	Content      string `json:"content"`
	RootPassword string `json:"root_password"`
}
