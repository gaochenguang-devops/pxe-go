// Package db 负责 SQLite 初始化、数据表 CRUD、唯一性校验逻辑。
package db

import (
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	// 纯 Go SQLite 驱动，无 CGO 依赖，可静态编译，规避 glibc 版本问题。
	_ "modernc.org/sqlite"

	"pxe-server/internal/logger"
	"pxe-server/internal/model"
)

var (
	// DB 全局数据库句柄
	DB *sql.DB
)

// Init 初始化 SQLite 数据库并自动建表、写入默认配置。
func Init(dbPath string) error {
	if err := os.MkdirAll(filepath.Dir(dbPath), 0755); err != nil {
		return err
	}
	var err error
	// modernc.org/sqlite 的 DSN pragma 使用 _pragma= 语法（区别于 go-sqlite3）
	DB, err = sql.Open("sqlite", dbPath+"?_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)")
	if err != nil {
		return err
	}
	DB.SetMaxOpenConns(1) // SQLite 单写多读，限制并发写
	if err := createTables(); err != nil {
		return err
	}
	if err := seedDefaultConfig(); err != nil {
		return err
	}
	if err := migrateDHCPSubnet(); err != nil {
		return err
	}
	return nil
}

// Close 关闭数据库连接。
func Close() error {
	if DB != nil {
		return DB.Close()
	}
	return nil
}

func createTables() error {
	stmts := []string{
		// 全局配置表
		`CREATE TABLE IF NOT EXISTS sys_config (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			config_key TEXT UNIQUE NOT NULL,
			config_value TEXT NOT NULL DEFAULT '',
			config_desc TEXT NOT NULL DEFAULT '',
			update_time INTEGER NOT NULL DEFAULT 0
		)`,
		// 资源表
		`CREATE TABLE IF NOT EXISTS pxe_resource (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			name TEXT NOT NULL,
			res_type TEXT NOT NULL DEFAULT 'other',
			arch_type TEXT NOT NULL DEFAULT 'all',
			local_path TEXT NOT NULL DEFAULT '',
			size INTEGER NOT NULL DEFAULT 0,
			upload_time DATETIME NOT NULL
		)`,
		// 系统镜像表
		`CREATE TABLE IF NOT EXISTS os_image (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			name TEXT NOT NULL,
			x86_repo_path TEXT NOT NULL DEFAULT '',
			x86_iso_path TEXT NOT NULL DEFAULT '',
			arm_repo_path TEXT NOT NULL DEFAULT '',
			arm_iso_path TEXT NOT NULL DEFAULT '',
			active INTEGER NOT NULL DEFAULT 0
		)`,
		// KS 模板表
		`CREATE TABLE IF NOT EXISTS ks_template (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			name TEXT NOT NULL,
			os_type TEXT NOT NULL DEFAULT '',
			content TEXT NOT NULL DEFAULT '',
			root_password TEXT NOT NULL DEFAULT '',
			active INTEGER NOT NULL DEFAULT 0,
			is_default INTEGER NOT NULL DEFAULT 0,
			create_time DATETIME NOT NULL
		)`,
		// iPXE 脚本表（同一时间仅一个 active=1）
		`CREATE TABLE IF NOT EXISTS ipxe_script (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			name TEXT NOT NULL,
			content TEXT NOT NULL DEFAULT '',
			active INTEGER NOT NULL DEFAULT 0,
			is_default INTEGER NOT NULL DEFAULT 0,
			create_time DATETIME NOT NULL
		)`,
		// 主机管理表（ipmi_addr 唯一索引，仅基础属性）
		`CREATE TABLE IF NOT EXISTS host_info (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			mac_addr TEXT NOT NULL DEFAULT '',
			hostname TEXT NOT NULL DEFAULT '',
			ipmi_addr TEXT NOT NULL DEFAULT '',
			ipmi_user TEXT NOT NULL DEFAULT '',
			ipmi_pass TEXT NOT NULL DEFAULT '',
			install_status TEXT NOT NULL DEFAULT 'idle',
			create_time DATETIME NOT NULL
		)`,
		`CREATE UNIQUE INDEX IF NOT EXISTS idx_host_ipmi ON host_info(ipmi_addr)`,
		// 主机资源表（ipmi_addr 唯一索引，Bond 网络配置，用于 node-info.txt）
		`CREATE TABLE IF NOT EXISTS host_resource (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			ipmi_addr TEXT NOT NULL DEFAULT '',
			hostname TEXT NOT NULL DEFAULT '',
			bond0_ip TEXT NOT NULL DEFAULT '',
			bond0_mask TEXT NOT NULL DEFAULT '',
			bond0_gateway TEXT NOT NULL DEFAULT '',
			bond0_ipv6 TEXT NOT NULL DEFAULT '',
			bond0_ipv6mask TEXT NOT NULL DEFAULT '',
			bond0_ipv6gw TEXT NOT NULL DEFAULT '',
			bond2_ip TEXT NOT NULL DEFAULT '',
			bond2_mask TEXT NOT NULL DEFAULT '',
			bond2_gateway TEXT NOT NULL DEFAULT '',
			bond2_ipv6 TEXT NOT NULL DEFAULT '',
			bond2_ipv6mask TEXT NOT NULL DEFAULT '',
			bond2_ipv6gw TEXT NOT NULL DEFAULT '',
			bond1_ip TEXT NOT NULL DEFAULT '',
			bond1_mask TEXT NOT NULL DEFAULT '',
			bond1_gateway TEXT NOT NULL DEFAULT ''
		)`,
		`CREATE UNIQUE INDEX IF NOT EXISTS idx_host_res_ipmi ON host_resource(ipmi_addr)`,
	// DHCP 多网段配置表
	`CREATE TABLE IF NOT EXISTS dhcp_subnet (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		name TEXT NOT NULL DEFAULT '',
		ip_pool_start TEXT NOT NULL DEFAULT '',
		ip_pool_end TEXT NOT NULL DEFAULT '',
		subnet_mask TEXT NOT NULL DEFAULT '',
		gateway TEXT NOT NULL DEFAULT '',
		dns_servers TEXT NOT NULL DEFAULT '',
		enabled INTEGER NOT NULL DEFAULT 1,
		sort_order INTEGER NOT NULL DEFAULT 0
	)`,
		// 部署脚本表
		`CREATE TABLE IF NOT EXISTS deploy_script (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			name TEXT NOT NULL DEFAULT '',
			content TEXT NOT NULL DEFAULT '',
			active INTEGER NOT NULL DEFAULT 0,
			is_default INTEGER NOT NULL DEFAULT 0,
			create_time DATETIME NOT NULL
		)`,
		// 操作审计日志表
		`CREATE TABLE IF NOT EXISTS oper_log (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			operator TEXT NOT NULL DEFAULT '',
			op_type TEXT NOT NULL DEFAULT '',
			detail TEXT NOT NULL DEFAULT '',
			client_ip TEXT NOT NULL DEFAULT '',
			op_time DATETIME NOT NULL
		)`,
		// 装机完成上报记录表
		`CREATE TABLE IF NOT EXISTS install_record (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			hostname TEXT NOT NULL DEFAULT '',
			ipmi_addr TEXT NOT NULL DEFAULT '',
			mac TEXT NOT NULL DEFAULT '',
			ip TEXT NOT NULL DEFAULT '',
			client_ip TEXT NOT NULL DEFAULT '',
			arch TEXT NOT NULL DEFAULT '',
			interfaces TEXT NOT NULL DEFAULT '',
			lldp TEXT NOT NULL DEFAULT '',
			status TEXT NOT NULL DEFAULT 'success',
			report_time DATETIME NOT NULL
		)`,
	}
	for _, s := range stmts {
		if _, err := DB.Exec(s); err != nil {
			return err
		}
	}
	// 轻量迁移：为旧表补充新增列（若不存在）
	if err := ensureColumn("os_image", "x86_repo_path", "TEXT NOT NULL DEFAULT ''"); err != nil {
		return err
	}
	if err := ensureColumn("os_image", "x86_iso_path", "TEXT NOT NULL DEFAULT ''"); err != nil {
		return err
	}
	if err := ensureColumn("os_image", "arm_repo_path", "TEXT NOT NULL DEFAULT ''"); err != nil {
		return err
	}
	if err := ensureColumn("os_image", "arm_iso_path", "TEXT NOT NULL DEFAULT ''"); err != nil {
		return err
	}
	if err := ensureColumn("os_image", "active", "INTEGER NOT NULL DEFAULT 0"); err != nil {
		return err
	}
	if err := ensureColumn("ks_template", "active", "INTEGER NOT NULL DEFAULT 0"); err != nil {
		return err
	}
	if err := ensureColumn("ks_template", "is_default", "INTEGER NOT NULL DEFAULT 0"); err != nil {
		return err
	}
	if err := ensureColumn("ks_template", "root_password", "TEXT NOT NULL DEFAULT ''"); err != nil {
		return err
	}
	if err := ensureColumn("install_record", "client_ip", "TEXT NOT NULL DEFAULT ''"); err != nil {
		return err
	}
	if err := ensureColumn("ipxe_script", "is_default", "INTEGER NOT NULL DEFAULT 0"); err != nil {
		return err
	}
	if err := ensureColumn("deploy_script", "active", "INTEGER NOT NULL DEFAULT 0"); err != nil {
		return err
	}
	if err := ensureColumn("deploy_script", "is_default", "INTEGER NOT NULL DEFAULT 0"); err != nil {
		return err
	}
	// 迁移：将 host_info 中的 bond 列数据转移到 host_resource 表（若 host_resource 刚新建且 host_info 仍含旧 bond 列）
	if err := migrateHostResource(); err != nil {
		return err
	}
	return nil
}

// migrateHostResource 将旧 host_info 表中的 bond 网络数据迁移到新的 host_resource 表。
func migrateHostResource() error {
	// 检查 host_resource 是否有数据（首次迁移）
	var cnt int
	row := DB.QueryRow(`SELECT COUNT(1) FROM host_resource`)
	if err := row.Scan(&cnt); err != nil {
		return err
	}
	if cnt > 0 {
		return nil // 已有数据，跳过
	}
	// 检查 host_info 是否有 bond0_ip 列（旧表结构）
	cols, err := DB.Query(`SELECT 1 FROM pragma_table_info('host_info') WHERE name='bond0_ip'`)
	if err != nil {
		return err
	}
	defer cols.Close()
	if !cols.Next() {
		return nil // 无旧列，无需迁移
	}
	// 执行数据迁移：从 host_info 复制 bond 字段到 host_resource
	_, err = DB.Exec(`INSERT INTO host_resource(ipmi_addr, hostname, bond0_ip, bond0_mask, bond0_gateway, bond0_ipv6, bond0_ipv6mask, bond0_ipv6gw, bond2_ip, bond2_mask, bond2_gateway, bond2_ipv6, bond2_ipv6mask, bond2_ipv6gw, bond1_ip, bond1_mask, bond1_gateway) SELECT ipmi_addr, hostname, bond0_ip, bond0_mask, bond0_gateway, bond0_ipv6, bond0_ipv6mask, bond0_ipv6gw, bond2_ip, bond2_mask, bond2_gateway, bond2_ipv6, bond2_ipv6mask, bond2_ipv6gw, bond1_ip, bond1_mask, bond1_gateway FROM host_info WHERE ipmi_addr != '' OR hostname != ''`)
	return err
}

// ensureColumn 若表中不存在指定列，则执行 ALTER TABLE 添加。
func ensureColumn(table, column, ddl string) error {
	rows, err := DB.Query(`SELECT 1 FROM pragma_table_info(?) WHERE name=?`, table, column)
	if err != nil {
		return err
	}
	defer rows.Close()
	if rows.Next() {
		return nil // 列已存在
	}
	_, err = DB.Exec("ALTER TABLE " + table + " ADD COLUMN " + column + " " + ddl)
	return err
}

// seedDefaultConfig 写入默认配置（若不存在）。
func seedDefaultConfig() error {
	defaults := map[string]string{
		"dhcp_enabled":            "true",
		"dhcp_listen_ip":          "0.0.0.0",
		"dhcp_interface":          "",
		"dhcp_pxe_ip":             "192.168.10.10",
		"dhcp_lease_time":         "86400",
		"dhcp_boot_file_bios":     "undionly.kpxe",
		"dhcp_boot_file_x86":      "ipxe-x86_64.efi",
		"dhcp_boot_file_arm":      "ipxe-aarch64.efi",
		"dhcp_ipxe_script":        "autoexec.ipxe",
		"dhcp_config_version":     "1",
		"tftp_enabled":            "true",
		"tftp_listen_ip":          "0.0.0.0",
		"tftp_root_dir":           "assets/tftp_root",
		"tftp_transfer_timeout":   "5",
		"tftp_max_connections":    "128",
		"tftp_access_log":         "true",
		"tftp_config_version":     "1",
		"http_listen_addr":        "0.0.0.0:80",
		"http_web_root":           "assets/web_root",
		"http_admin_user":         "admin",
		"http_admin_password":     "admin123",
		"http_config_version":     "1",
	}
	for k, v := range defaults {
		if err := insertDefaultConfig(k, v); err != nil {
			return err
		}
	}
	return nil
}

func insertDefaultConfig(key, value string) error {
	var cnt int
	err := DB.QueryRow(`SELECT COUNT(1) FROM sys_config WHERE config_key=?`, key).Scan(&cnt)
	if err != nil {
		return err
	}
	if cnt > 0 {
		return nil
	}
	_, err = DB.Exec(`INSERT INTO sys_config(config_key, config_value, config_desc, update_time) VALUES(?,?,?,?)`,
		key, value, "", time.Now().Unix())
	return err
}

// migrateDHCPSubnet 将旧的单网段配置（dhcp_ip_pool_start 等）迁移为子网记录。
// 仅在 dhcp_subnet 表为空时执行；迁移完成后删除旧配置键。
// 旧数据库：用旧键的用户配置值创建子网；全新安装：无旧键时使用默认网段创建一条子网。
func migrateDHCPSubnet() error {
	var cnt int
	if err := DB.QueryRow(`SELECT COUNT(1) FROM dhcp_subnet`).Scan(&cnt); err != nil {
		return err
	}
	if cnt > 0 {
		// 已存在子网，仅清理残留旧键
		cleanOldDHCPKeys()
		return nil
	}

	// 读取旧单网段配置（可能不存在则用默认值）
	poolStart := configValueOr("dhcp_ip_pool_start", "192.168.10.100")
	poolEnd := configValueOr("dhcp_ip_pool_end", "192.168.10.200")
	mask := configValueOr("dhcp_subnet_mask", "255.255.255.0")
	gateway := configValueOr("dhcp_gateway", "192.168.10.1")
	dns := configValueOr("dhcp_dns_servers", "192.168.10.1")

	if poolStart == "" || poolEnd == "" || mask == "" {
		// 无有效旧配置，跳过迁移（用户需手动配置子网）
		return nil
	}

	_, err := DB.Exec(`INSERT INTO dhcp_subnet(name, ip_pool_start, ip_pool_end, subnet_mask, gateway, dns_servers, enabled, sort_order)
		VALUES(?,?,?,?,?,?,1,0)`,
		"默认子网", poolStart, poolEnd, mask, gateway, dns)
	if err != nil {
		return err
	}
	cleanOldDHCPKeys()
	return nil
}

// configValueOr 读取配置值，不存在或为空时返回默认值。
func configValueOr(key, def string) string {
	v, err := GetConfig(key)
	if err != nil || v == "" {
		return def
	}
	return v
}

// cleanOldDHCPKeys 删除旧的单网段配置键。
func cleanOldDHCPKeys() {
	for _, k := range []string{"dhcp_ip_pool_start", "dhcp_ip_pool_end", "dhcp_subnet_mask", "dhcp_gateway", "dhcp_dns_servers"} {
		if _, err := DB.Exec(`DELETE FROM sys_config WHERE config_key=?`, k); err != nil {
			logger.Warn("清理旧 DHCP 配置键失败 %s: %v", k, err)
		}
	}
}

// GetAllConfigs 返回全部配置键值。
func GetAllConfigs() ([]*model.SysConfig, error) {
	rows, err := DB.Query(`SELECT id, config_key, config_value, config_desc, update_time FROM sys_config`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*model.SysConfig
	for rows.Next() {
		c := &model.SysConfig{}
		if err := rows.Scan(&c.ID, &c.ConfigKey, &c.ConfigValue, &c.ConfigDesc, &c.UpdateTime); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, nil
}

// SetConfig 更新单个配置项。
func SetConfig(key, value string) error {
	_, err := DB.Exec(`INSERT INTO sys_config(config_key, config_value, config_desc, update_time) VALUES(?,?,?,?)
		ON CONFLICT(config_key) DO UPDATE SET config_value=excluded.config_value, update_time=excluded.update_time`,
		key, value, "", time.Now().Unix())
	return err
}

// GetConfig 获取单个配置项。
func GetConfig(key string) (string, error) {
	var v string
	err := DB.QueryRow(`SELECT config_value FROM sys_config WHERE config_key=?`, key).Scan(&v)
	if err != nil {
		return "", err
	}
	return v, nil
}

// GetConfigVersion 获取配置版本号（用于热加载检测）。
func GetConfigVersion(key string) (int64, error) {
	v, err := GetConfig(key)
	if err != nil {
		return 0, err
	}
	var n int64
	fmt.Sscanf(v, "%d", &n)
	return n, nil
}

// ErrDuplicateIPMI IPMI 地址重复错误。
var ErrDuplicateIPMI = errors.New("ipmi_addr already exists")

// ErrNotFound 记录不存在。
var ErrNotFound = errors.New("record not found")

// hostExistsByIPMI 检查 IPMI 地址是否已存在（数据库层唯一性校验）。
func hostExistsByIPMI(ipmiAddr string, excludeID int64) (bool, error) {
	var cnt int
	row := DB.QueryRow(`SELECT COUNT(1) FROM host_info WHERE ipmi_addr=? AND id<>?`, ipmiAddr, excludeID)
	if err := row.Scan(&cnt); err != nil {
		return false, err
	}
	return cnt > 0, nil
}

// hostExistsByMAC 检查 MAC 是否已存在。
func hostExistsByMAC(macAddr string, excludeID int64) (bool, error) {
	var cnt int
	row := DB.QueryRow(`SELECT COUNT(1) FROM host_info WHERE mac_addr=? AND id<>?`, macAddr, excludeID)
	if err := row.Scan(&cnt); err != nil {
		return false, err
	}
	return cnt > 0, nil
}

// CheckHostUnique 校验主机 IPMI/MAC 唯一性，供 handler 调用。
func CheckHostUnique(ipmiAddr, macAddr string, excludeID int64) error {
	if ipmiAddr != "" {
		exists, err := hostExistsByIPMI(ipmiAddr, excludeID)
		if err != nil {
			return err
		}
		if exists {
			return ErrDuplicateIPMI
		}
	}
	if macAddr != "" {
		exists, err := hostExistsByMAC(macAddr, excludeID)
		if err != nil {
			return err
		}
		if exists {
			return errors.New("mac_addr already exists")
		}
	}
	return nil
}
