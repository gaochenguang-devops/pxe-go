// Package config 负责从 sys_config 表加载全局配置并提供热加载能力。
package config

import (
	"strconv"
	"strings"
	"sync"

	"pxe-server/internal/db"
	"pxe-server/internal/model"
)

// Manager 配置管理器，缓存当前配置并提供热加载。
type Manager struct {
	mu      sync.RWMutex
	dhcp    model.DHCPConfig
	tftp    model.TFTPConfig
	http    model.HTTPConfig
	seedKey string // 用于密码加密的固定密钥
}

// NewManager 从数据库加载配置创建管理器。
func NewManager() (*Manager, error) {
	m := &Manager{seedKey: "pxe-server-seed-key-2026"}
	if err := m.ReloadAll(); err != nil {
		return nil, err
	}
	return m, nil
}

// SeedKey 返回加密密钥。
func (m *Manager) SeedKey() string { return m.seedKey }

// ReloadAll 从数据库重新加载所有配置。
func (m *Manager) ReloadAll() error {
	rows, err := db.GetAllConfigs()
	if err != nil {
		return err
	}
	cfg := make(map[string]string)
	for _, r := range rows {
		cfg[r.ConfigKey] = r.ConfigValue
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	// DHCP
	m.dhcp.Enabled = cfgBool(cfg, "dhcp_enabled", true)
	m.dhcp.ListenIP = cfgStr(cfg, "dhcp_listen_ip", "0.0.0.0")
	m.dhcp.Interface = cfgStr(cfg, "dhcp_interface", "")
	m.dhcp.PXEIP = cfgStr(cfg, "dhcp_pxe_ip", "192.168.10.10")
	m.dhcp.LeaseTime = cfgInt(cfg, "dhcp_lease_time", 86400)
	m.dhcp.BootFileBIOS = cfgStr(cfg, "dhcp_boot_file_bios", "undionly.kpxe")
	m.dhcp.BootFileX86 = cfgStr(cfg, "dhcp_boot_file_x86", "ipxe-x86_64.efi")
	m.dhcp.BootFileARM = cfgStr(cfg, "dhcp_boot_file_arm", "ipxe-aarch64.efi")
	m.dhcp.IpxeScript = cfgStr(cfg, "dhcp_ipxe_script", "autoexec.ipxe")
	m.dhcp.ConfigVersion = cfgInt64(cfg, "dhcp_config_version", 1)
	// 加载多网段配置（dhcp_subnet 表）
	if subs, err := db.ListDHCPSubnets(); err == nil {
		m.dhcp.Subnets = make([]model.DHCPSubnet, 0, len(subs))
		for _, p := range subs {
			m.dhcp.Subnets = append(m.dhcp.Subnets, *p)
		}
	} else {
		m.dhcp.Subnets = nil
	}

	// TFTP
	m.tftp.Enabled = cfgBool(cfg, "tftp_enabled", true)
	m.tftp.ListenIP = cfgStr(cfg, "tftp_listen_ip", "0.0.0.0")
	m.tftp.RootDir = cfgStr(cfg, "tftp_root_dir", "assets/tftp_root")
	m.tftp.TransferTimeout = cfgInt(cfg, "tftp_transfer_timeout", 5)
	m.tftp.MaxConnections = cfgInt(cfg, "tftp_max_connections", 128)
	m.tftp.AccessLog = cfgBool(cfg, "tftp_access_log", true)
	m.tftp.ConfigVersion = cfgInt64(cfg, "tftp_config_version", 1)

	// HTTP
	m.http.ListenAddr = cfgStr(cfg, "http_listen_addr", "0.0.0.0:80")
	m.http.WebRoot = cfgStr(cfg, "http_web_root", "assets/web_root")
	m.http.AdminUser = cfgStr(cfg, "http_admin_user", "admin")
	m.http.AdminPassword = cfgStr(cfg, "http_admin_password", "admin123")
	m.http.ConfigVersion = cfgInt64(cfg, "http_config_version", 1)

	return nil
}

// UpdateAndReload 更新配置并触发热加载。若 configVersion 变化则返回 true。
func (m *Manager) UpdateAndReload(key, value string) error {
	if err := db.SetConfig(key, value); err != nil {
		return err
	}
	return m.ReloadAll()
}

// DHCP 返回 DHCP 配置副本。
func (m *Manager) DHCP() model.DHCPConfig {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.dhcp
}

// TFTP 返回 TFTP 配置副本。
func (m *Manager) TFTP() model.TFTPConfig {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.tftp
}

// HTTP 返回 HTTP 配置副本。
func (m *Manager) HTTP() model.HTTPConfig {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.http
}

// HTTPRaw 返回 HTTP 配置原始（含未加密密码）。
func (m *Manager) HTTPRaw() model.HTTPConfig {
	return m.HTTP()
}

// 辅助转换函数
func cfgStr(cfg map[string]string, key, def string) string {
	if v, ok := cfg[key]; ok && v != "" {
		return v
	}
	return def
}

func cfgBool(cfg map[string]string, key string, def bool) bool {
	if v, ok := cfg[key]; ok {
		if b, err := strconv.ParseBool(v); err == nil {
			return b
		}
	}
	return def
}

func cfgInt(cfg map[string]string, key string, def int) int {
	if v, ok := cfg[key]; ok {
		if i, err := strconv.Atoi(v); err == nil {
			return i
		}
	}
	return def
}

func cfgInt64(cfg map[string]string, key string, def int64) int64 {
	if v, ok := cfg[key]; ok {
		if i, err := strconv.ParseInt(v, 10, 64); err == nil {
			return i
		}
	}
	return def
}

// SplitDNS 解析逗号分隔的 DNS 服务器列表。
func SplitDNS(s string) []string {
	var out []string
	for _, d := range strings.Split(s, ",") {
		d = strings.TrimSpace(d)
		if d != "" {
			out = append(out, d)
		}
	}
	return out
}
