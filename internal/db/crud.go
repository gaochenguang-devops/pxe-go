package db

import (
	"database/sql"
	"strings"
	"time"

	"pxe-server/internal/model"
)

// ============ Host 主机管理 ============

const hostCols = `id, hostname, ipmi_addr, ipmi_user, ipmi_pass, install_status, create_time`

func scanHost(row interface{ Scan(...any) error }) (*model.HostInfo, error) {
	h := &model.HostInfo{}
	err := row.Scan(&h.ID, &h.Hostname, &h.IPMIAddr, &h.IPMIUser, &h.IPMIPass, &h.InstallStatus, &h.CreateTime)
	return h, err
}

func CreateHost(h *model.HostInfo) (int64, error) {
	if err := CheckHostUnique(h.IPMIAddr, "", 0); err != nil {
		return 0, err
	}
	h.CreateTime = time.Now()
	res, err := DB.Exec(`INSERT INTO host_info(hostname, ipmi_addr, ipmi_user, ipmi_pass, install_status, create_time) VALUES(?,?,?,?,?,?)`,
		h.Hostname, h.IPMIAddr, h.IPMIUser, h.IPMIPass, h.InstallStatus, h.CreateTime)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

func UpdateHost(id int64, h *model.HostInfo) error {
	if err := CheckHostUnique(h.IPMIAddr, "", id); err != nil {
		return err
	}
	_, err := DB.Exec(`UPDATE host_info SET hostname=?, ipmi_addr=?, ipmi_user=?, ipmi_pass=?, install_status=? WHERE id=?`,
		h.Hostname, h.IPMIAddr, h.IPMIUser, h.IPMIPass, h.InstallStatus, id)
	return err
}

func DeleteHost(id int64) error {
	_, err := DB.Exec(`DELETE FROM host_info WHERE id=?`, id)
	return err
}

func DeleteHostBatch(ids []int64) error {
	if len(ids) == 0 {
		return nil
	}
	placeholders := strings.TrimSuffix(strings.Repeat("?,", len(ids)), ",")
	args := make([]any, 0, len(ids))
	for _, id := range ids {
		args = append(args, id)
	}
	_, err := DB.Exec(`DELETE FROM host_info WHERE id IN (`+placeholders+`)`, args...)
	return err
}

func GetHostByID(id int64) (*model.HostInfo, error) {
	return scanHost(DB.QueryRow(`SELECT `+hostCols+` FROM host_info WHERE id=?`, id))
}

func ListHosts() ([]*model.HostInfo, error) {
	rows, err := DB.Query(`SELECT ` + hostCols + ` FROM host_info ORDER BY id ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*model.HostInfo
	for rows.Next() {
		h := &model.HostInfo{}
		if err := rows.Scan(&h.ID, &h.Hostname, &h.IPMIAddr, &h.IPMIUser, &h.IPMIPass, &h.InstallStatus, &h.CreateTime); err != nil {
			return nil, err
		}
		out = append(out, h)
	}
	return out, nil
}

// ListHostsPage 分页查询主机列表（支持搜索）。
func ListHostsPage(limit, offset int, search string) ([]*model.HostInfo, error) {
	var rows *sql.Rows
	var err error
	if search != "" {
		like := "%" + search + "%"
		rows, err = DB.Query(`SELECT `+hostCols+` FROM host_info WHERE hostname LIKE ? OR ipmi_addr LIKE ? ORDER BY id ASC LIMIT ? OFFSET ?`,
			like, like, limit, offset)
	} else {
		rows, err = DB.Query(`SELECT `+hostCols+` FROM host_info ORDER BY id ASC LIMIT ? OFFSET ?`, limit, offset)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*model.HostInfo
	for rows.Next() {
		h := &model.HostInfo{}
		if err := rows.Scan(&h.ID, &h.Hostname, &h.IPMIAddr, &h.IPMIUser, &h.IPMIPass, &h.InstallStatus, &h.CreateTime); err != nil {
			return nil, err
		}
		out = append(out, h)
	}
	return out, nil
}

// CountHosts 统计主机总数（支持搜索过滤）。
func CountHosts(search string) (int, error) {
	var n int
	var err error
	if search != "" {
		like := "%" + search + "%"
		err = DB.QueryRow(`SELECT COUNT(*) FROM host_info WHERE hostname LIKE ? OR ipmi_addr LIKE ?`, like, like).Scan(&n)
	} else {
		err = DB.QueryRow(`SELECT COUNT(*) FROM host_info`).Scan(&n)
	}
	if err != nil {
		return 0, err
	}
	return n, nil
}

// ============ HostResource 主机资源（Bond 网络） ============

const resCols = `id, ipmi_addr, hostname, bond0_ip, bond0_mask, bond0_gateway, bond0_ipv6, bond0_ipv6mask, bond0_ipv6gw, bond2_ip, bond2_mask, bond2_gateway, bond2_ipv6, bond2_ipv6mask, bond2_ipv6gw, bond1_ip, bond1_mask, bond1_gateway`

func scanResource(row interface{ Scan(...any) error }) (*model.HostResource, error) {
	r := &model.HostResource{}
	err := row.Scan(&r.ID, &r.IPMIAddr, &r.Hostname,
		&r.Bond0IP, &r.Bond0Mask, &r.Bond0Gateway, &r.Bond0IPv6, &r.Bond0IPv6Mask, &r.Bond0IPv6Gw,
		&r.Bond2IP, &r.Bond2Mask, &r.Bond2Gateway, &r.Bond2IPv6, &r.Bond2IPv6Mask, &r.Bond2IPv6Gw,
		&r.Bond1IP, &r.Bond1Mask, &r.Bond1Gateway)
	return r, err
}

func UpsertHostResource(r *model.HostResource) error {
	existing, err := GetHostResourceByIPMI(r.IPMIAddr)
	if err == nil && existing != nil {
		_, err = DB.Exec(`UPDATE host_resource SET hostname=?, bond0_ip=?, bond0_mask=?, bond0_gateway=?, bond0_ipv6=?, bond0_ipv6mask=?, bond0_ipv6gw=?, bond2_ip=?, bond2_mask=?, bond2_gateway=?, bond2_ipv6=?, bond2_ipv6mask=?, bond2_ipv6gw=?, bond1_ip=?, bond1_mask=?, bond1_gateway=? WHERE ipmi_addr=?`,
			r.Hostname, r.Bond0IP, r.Bond0Mask, r.Bond0Gateway, r.Bond0IPv6, r.Bond0IPv6Mask, r.Bond0IPv6Gw,
			r.Bond2IP, r.Bond2Mask, r.Bond2Gateway, r.Bond2IPv6, r.Bond2IPv6Mask, r.Bond2IPv6Gw,
			r.Bond1IP, r.Bond1Mask, r.Bond1Gateway, r.IPMIAddr)
		return err
	}
	_, err = DB.Exec(`INSERT INTO host_resource(ipmi_addr, hostname, bond0_ip, bond0_mask, bond0_gateway, bond0_ipv6, bond0_ipv6mask, bond0_ipv6gw, bond2_ip, bond2_mask, bond2_gateway, bond2_ipv6, bond2_ipv6mask, bond2_ipv6gw, bond1_ip, bond1_mask, bond1_gateway) VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`,
		r.IPMIAddr, r.Hostname, r.Bond0IP, r.Bond0Mask, r.Bond0Gateway, r.Bond0IPv6, r.Bond0IPv6Mask, r.Bond0IPv6Gw,
		r.Bond2IP, r.Bond2Mask, r.Bond2Gateway, r.Bond2IPv6, r.Bond2IPv6Mask, r.Bond2IPv6Gw,
		r.Bond1IP, r.Bond1Mask, r.Bond1Gateway)
	return err
}

func GetHostResourceByIPMI(ipmi string) (*model.HostResource, error) {
	return scanResource(DB.QueryRow(`SELECT `+resCols+` FROM host_resource WHERE ipmi_addr=?`, ipmi))
}

func ListHostResources() ([]*model.HostResource, error) {
	rows, err := DB.Query(`SELECT ` + resCols + ` FROM host_resource ORDER BY id ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*model.HostResource
	for rows.Next() {
		r := &model.HostResource{}
		if err := rows.Scan(&r.ID, &r.IPMIAddr, &r.Hostname,
			&r.Bond0IP, &r.Bond0Mask, &r.Bond0Gateway, &r.Bond0IPv6, &r.Bond0IPv6Mask, &r.Bond0IPv6Gw,
			&r.Bond2IP, &r.Bond2Mask, &r.Bond2Gateway, &r.Bond2IPv6, &r.Bond2IPv6Mask, &r.Bond2IPv6Gw,
			&r.Bond1IP, &r.Bond1Mask, &r.Bond1Gateway); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, nil
}

// ListHostResourcesPage 分页查询主机资源（支持搜索）。
func ListHostResourcesPage(limit, offset int, search string) ([]*model.HostResource, error) {
	var rows *sql.Rows
	var err error
	if search != "" {
		like := "%" + search + "%"
		rows, err = DB.Query(`SELECT `+resCols+` FROM host_resource WHERE hostname LIKE ? OR ipmi_addr LIKE ? ORDER BY id ASC LIMIT ? OFFSET ?`,
			like, like, limit, offset)
	} else {
		rows, err = DB.Query(`SELECT `+resCols+` FROM host_resource ORDER BY id ASC LIMIT ? OFFSET ?`, limit, offset)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*model.HostResource
	for rows.Next() {
		r := &model.HostResource{}
		if err := rows.Scan(&r.ID, &r.IPMIAddr, &r.Hostname,
			&r.Bond0IP, &r.Bond0Mask, &r.Bond0Gateway, &r.Bond0IPv6, &r.Bond0IPv6Mask, &r.Bond0IPv6Gw,
			&r.Bond2IP, &r.Bond2Mask, &r.Bond2Gateway, &r.Bond2IPv6, &r.Bond2IPv6Mask, &r.Bond2IPv6Gw,
			&r.Bond1IP, &r.Bond1Mask, &r.Bond1Gateway); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, nil
}

// CountHostResources 统计主机资源总数（支持搜索过滤）。
func CountHostResources(search string) (int, error) {
	var n int
	var err error
	if search != "" {
		like := "%" + search + "%"
		err = DB.QueryRow(`SELECT COUNT(*) FROM host_resource WHERE hostname LIKE ? OR ipmi_addr LIKE ?`, like, like).Scan(&n)
	} else {
		err = DB.QueryRow(`SELECT COUNT(*) FROM host_resource`).Scan(&n)
	}
	if err != nil {
		return 0, err
	}
	return n, nil
}

func DeleteHostResourceByIPMI(ipmi string) error {
	_, err := DB.Exec(`DELETE FROM host_resource WHERE ipmi_addr=?`, ipmi)
	return err
}

func DeleteHostResourceBatch(ids []int64) error {
	if len(ids) == 0 {
		return nil
	}
	placeholders := strings.TrimSuffix(strings.Repeat("?,", len(ids)), ",")
	args := make([]any, 0, len(ids))
	for _, id := range ids {
		args = append(args, id)
	}
	_, err := DB.Exec(`DELETE FROM host_resource WHERE id IN (`+placeholders+`)`, args...)
	return err
}

// ============ PXE Resource 资源 ============

// ListResources 查询资源列表。
func ListResources() ([]*model.PXEResource, error) {
	rows, err := DB.Query(`SELECT id, name, res_type, arch_type, local_path, size, upload_time FROM pxe_resource ORDER BY id DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*model.PXEResource
	for rows.Next() {
		r := &model.PXEResource{}
		if err := rows.Scan(&r.ID, &r.Name, &r.ResType, &r.ArchType, &r.LocalPath, &r.Size, &r.UploadTime); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, nil
}

// ============ KS Template ============

// CreateKSTemplate 新增 KS 模板。
func CreateKSTemplate(t *model.KSTemplate) (int64, error) {
	t.CreateTime = time.Now()
	res, err := DB.Exec(`INSERT INTO ks_template(name, os_type, content, root_password, active, is_default, create_time) VALUES(?,?,?,?,?,?,?)`,
		t.Name, t.OsType, t.Content, t.RootPassword, t.Active, t.IsDefault, t.CreateTime)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

// UpdateKSTemplate 更新 KS 模板（不改变 active/is_default）。
func UpdateKSTemplate(id int64, t *model.KSTemplate) error {
	_, err := DB.Exec(`UPDATE ks_template SET name=?, os_type=?, content=?, root_password=? WHERE id=?`,
		t.Name, t.OsType, t.Content, t.RootPassword, id)
	return err
}

// DeleteKSTemplate 删除 KS 模板（默认模板不可删除，由调用方检查）。
func DeleteKSTemplate(id int64) error {
	_, err := DB.Exec(`DELETE FROM ks_template WHERE id=?`, id)
	return err
}

// GetKSTemplateByName 按名称查询 KS 模板（用于重名校验）。
func GetKSTemplateByName(name string) (*model.KSTemplate, error) {
	row := DB.QueryRow(`SELECT id, name, os_type, content, root_password, active, is_default, create_time FROM ks_template WHERE name=?`, name)
	t := &model.KSTemplate{}
	if err := row.Scan(&t.ID, &t.Name, &t.OsType, &t.Content, &t.RootPassword, &t.Active, &t.IsDefault, &t.CreateTime); err != nil {
		return nil, err
	}
	return t, nil
}

// GetKSTemplate 查询 KS 模板。
func GetKSTemplate(id int64) (*model.KSTemplate, error) {
	row := DB.QueryRow(`SELECT id, name, os_type, content, root_password, active, is_default, create_time FROM ks_template WHERE id=?`, id)
	t := &model.KSTemplate{}
	if err := row.Scan(&t.ID, &t.Name, &t.OsType, &t.Content, &t.RootPassword, &t.Active, &t.IsDefault, &t.CreateTime); err != nil {
		return nil, err
	}
	return t, nil
}

// ListKSTemplates 查询 KS 模板列表。
func ListKSTemplates() ([]*model.KSTemplate, error) {
	rows, err := DB.Query(`SELECT id, name, os_type, content, root_password, active, is_default, create_time FROM ks_template ORDER BY id DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*model.KSTemplate
	for rows.Next() {
		t := &model.KSTemplate{}
		if err := rows.Scan(&t.ID, &t.Name, &t.OsType, &t.Content, &t.RootPassword, &t.Active, &t.IsDefault, &t.CreateTime); err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, nil
}

// SetActiveKSTemplate 将指定模板设为生效，其他置为不生效。返回设为生效的模板。
func SetActiveKSTemplate(id int64) (*model.KSTemplate, error) {
	if _, err := DB.Exec(`UPDATE ks_template SET active=0`); err != nil {
		return nil, err
	}
	if _, err := DB.Exec(`UPDATE ks_template SET active=1 WHERE id=?`, id); err != nil {
		return nil, err
	}
	return GetKSTemplate(id)
}

// GetActiveKSTemplate 获取当前生效的 KS 模板。
func GetActiveKSTemplate() (*model.KSTemplate, error) {
	row := DB.QueryRow(`SELECT id, name, os_type, content, root_password, active, is_default, create_time FROM ks_template WHERE active=1 LIMIT 1`)
	t := &model.KSTemplate{}
	if err := row.Scan(&t.ID, &t.Name, &t.OsType, &t.Content, &t.RootPassword, &t.Active, &t.IsDefault, &t.CreateTime); err != nil {
		return nil, err
	}
	return t, nil
}

// ============ OS Image ============

// CreateOSImage 新增系统镜像（一个名称一条记录，含双架构路径）。
func CreateOSImage(img *model.OSImage) (int64, error) {
	res, err := DB.Exec(`INSERT INTO os_image(name, x86_repo_path, x86_iso_path, arm_repo_path, arm_iso_path, active) VALUES(?,?,?,?,?,?)`,
		img.Name, img.X86RepoPath, img.X86IsoPath, img.ArmRepoPath, img.ArmIsoPath, img.Active)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

// UpdateOSImage 更新系统镜像。
func UpdateOSImage(id int64, img *model.OSImage) error {
	_, err := DB.Exec(`UPDATE os_image SET name=?, x86_repo_path=?, x86_iso_path=?, arm_repo_path=?, arm_iso_path=? WHERE id=?`,
		img.Name, img.X86RepoPath, img.X86IsoPath, img.ArmRepoPath, img.ArmIsoPath, id)
	return err
}

// DeleteOSImage 删除系统镜像。
func DeleteOSImage(id int64) error {
	_, err := DB.Exec(`DELETE FROM os_image WHERE id=?`, id)
	return err
}

// GetOSImage 查询系统镜像。
func GetOSImage(id int64) (*model.OSImage, error) {
	row := DB.QueryRow(`SELECT id, name, x86_repo_path, x86_iso_path, arm_repo_path, arm_iso_path, active FROM os_image WHERE id=?`, id)
	img := &model.OSImage{}
	if err := row.Scan(&img.ID, &img.Name, &img.X86RepoPath, &img.X86IsoPath, &img.ArmRepoPath, &img.ArmIsoPath, &img.Active); err != nil {
		return nil, err
	}
	return img, nil
}

// ListOSImages 查询系统镜像列表。
func ListOSImages() ([]*model.OSImage, error) {
	rows, err := DB.Query(`SELECT id, name, x86_repo_path, x86_iso_path, arm_repo_path, arm_iso_path, active FROM os_image ORDER BY id DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*model.OSImage
	for rows.Next() {
		img := &model.OSImage{}
		if err := rows.Scan(&img.ID, &img.Name, &img.X86RepoPath, &img.X86IsoPath, &img.ArmRepoPath, &img.ArmIsoPath, &img.Active); err != nil {
			return nil, err
		}
		out = append(out, img)
	}
	return out, nil
}

// SetActiveOSImage 将指定镜像设为默认生效，其他置为不生效。
func SetActiveOSImage(id int64) (*model.OSImage, error) {
	if _, err := DB.Exec(`UPDATE os_image SET active=0`); err != nil {
		return nil, err
	}
	if _, err := DB.Exec(`UPDATE os_image SET active=1 WHERE id=?`, id); err != nil {
		return nil, err
	}
	return GetOSImage(id)
}

// GetActiveOSImage 获取当前默认生效的系统镜像。
func GetActiveOSImage() (*model.OSImage, error) {
	row := DB.QueryRow(`SELECT id, name, x86_repo_path, x86_iso_path, arm_repo_path, arm_iso_path, active FROM os_image WHERE active=1 LIMIT 1`)
	img := &model.OSImage{}
	if err := row.Scan(&img.ID, &img.Name, &img.X86RepoPath, &img.X86IsoPath, &img.ArmRepoPath, &img.ArmIsoPath, &img.Active); err != nil {
		return nil, err
	}
	return img, nil
}

// GetOSImageByName 按名称查询系统镜像。
func GetOSImageByName(name string) (*model.OSImage, error) {
	row := DB.QueryRow(`SELECT id, name, x86_repo_path, x86_iso_path, arm_repo_path, arm_iso_path, active FROM os_image WHERE name=?`, name)
	img := &model.OSImage{}
	if err := row.Scan(&img.ID, &img.Name, &img.X86RepoPath, &img.X86IsoPath, &img.ArmRepoPath, &img.ArmIsoPath, &img.Active); err != nil {
		return nil, err
	}
	return img, nil
}

// UpsertOSImage 按名称存在则更新，不存在则插入。
func UpsertOSImage(img *model.OSImage) error {
	existing, err := GetOSImageByName(img.Name)
	if err != nil {
		// 不存在，插入
		_, err = CreateOSImage(img)
		return err
	}
	// 存在，保留 active 状态，更新路径信息
	img.Active = existing.Active
	img.ID = existing.ID
	return UpdateOSImage(existing.ID, img)
}

// ============ Oper Log 操作日志 ============

// CreateOperLog 写入操作审计日志。
func CreateOperLog(op *model.OperLog) (int64, error) {
	op.OpTime = time.Now()
	res, err := DB.Exec(`INSERT INTO oper_log(operator, op_type, detail, client_ip, op_time) VALUES(?,?,?,?,?)`,
		op.Operator, op.OpType, op.Detail, op.ClientIP, op.OpTime)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

// ============ Install Record 装机完成上报 ============

// CreateInstallRecord 写入装机完成上报记录。
func CreateInstallRecord(r *model.InstallRecord) (int64, error) {
	r.ReportTime = time.Now()
	res, err := DB.Exec(`INSERT INTO install_record(hostname, ipmi_addr, mac, ip, client_ip, arch, interfaces, lldp, status, report_time) VALUES(?,?,?,?,?,?,?,?,?,?)`,
		r.Hostname, r.IPMIAddr, r.MAC, r.IP, r.ClientIP, r.Arch, r.Interfaces, r.LLDP, r.Status, r.ReportTime)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

// ListInstallRecords 分页查询装机记录。
func ListInstallRecords(limit, offset int) ([]*model.InstallRecord, error) {
	if limit <= 0 {
		limit = 50
	}
	rows, err := DB.Query(`SELECT id, hostname, ipmi_addr, mac, ip, client_ip, arch, interfaces, lldp, status, report_time FROM install_record ORDER BY id DESC LIMIT ? OFFSET ?`, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*model.InstallRecord
	for rows.Next() {
		r := &model.InstallRecord{}
		if err := rows.Scan(&r.ID, &r.Hostname, &r.IPMIAddr, &r.MAC, &r.IP, &r.ClientIP, &r.Arch, &r.Interfaces, &r.LLDP, &r.Status, &r.ReportTime); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, nil
}

// CountInstallRecords 统计装机记录总数。
func CountInstallRecords() (int, error) {
	var n int
	if err := DB.QueryRow(`SELECT COUNT(*) FROM install_record`).Scan(&n); err != nil {
		return 0, err
	}
	return n, nil
}

// CountInstallRecordsByStatus 按状态统计。
func CountInstallRecordsByStatus(status string) (int, error) {
	var n int
	if err := DB.QueryRow("SELECT COUNT(*) FROM install_record WHERE status=?", status).Scan(&n); err != nil {
		return 0, err
	}
	return n, nil
}

// CountInstallRecordsNoLldp 统计 LLDP 未识别的记录。
func CountInstallRecordsNoLldp() (int, error) {
	var n int
	if err := DB.QueryRow("SELECT COUNT(*) FROM install_record WHERE lldp='' OR lldp='-' OR lldp='unknown' OR lldp LIKE '%' || char(37) || 's' || char(37) || '%'").Scan(&n); err != nil {
		return 0, err
	}
	return n, nil
}

// ListOperLogs 分页查询操作日志。
func ListOperLogs(limit, offset int) ([]*model.OperLog, error) {
	if limit <= 0 {
		limit = 100
	}
	rows, err := DB.Query(`SELECT id, operator, op_type, detail, client_ip, op_time FROM oper_log ORDER BY id DESC LIMIT ? OFFSET ?`, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*model.OperLog
	for rows.Next() {
		op := &model.OperLog{}
		if err := rows.Scan(&op.ID, &op.Operator, &op.OpType, &op.Detail, &op.ClientIP, &op.OpTime); err != nil {
			return nil, err
		}
		out = append(out, op)
	}
	return out, nil
}

// ListOperLogsByModule 按模块前缀查询操作日志（如 dhcp→cfg_dhcp_save, tftp→cfg_tftp_save）。
func ListOperLogsByModule(module string, limit, offset int) ([]*model.OperLog, error) {
	if limit <= 0 {
		limit = 100
	}
	prefix := "cfg_" + module + "%"
	rows, err := DB.Query(`SELECT id, operator, op_type, detail, client_ip, op_time FROM oper_log WHERE op_type LIKE ? ORDER BY id DESC LIMIT ? OFFSET ?`, prefix, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*model.OperLog
	for rows.Next() {
		op := &model.OperLog{}
		if err := rows.Scan(&op.ID, &op.Operator, &op.OpType, &op.Detail, &op.ClientIP, &op.OpTime); err != nil {
			return nil, err
		}
		out = append(out, op)
	}
	return out, nil
}

// ListOperLogsFiltered 多条件查询操作日志。
func ListOperLogsFiltered(module, opType, search string, limit, offset int) ([]*model.OperLog, error) {
	if limit <= 0 {
		limit = 100
	}
	where := "1=1"
	args := make([]any, 0)

	if module != "" {
		where += " AND op_type LIKE ?"
		args = append(args, "cfg_"+module+"%")
	}
	if opType != "" {
		where += " AND op_type LIKE ?"
		args = append(args, "%"+opType+"%")
	}
	if search != "" {
		where += " AND (operator LIKE ? OR detail LIKE ? OR op_type LIKE ?)"
		kw := "%" + search + "%"
		args = append(args, kw, kw, kw)
	}

	query := `SELECT id, operator, op_type, detail, client_ip, op_time FROM oper_log WHERE ` + where + ` ORDER BY id DESC LIMIT ? OFFSET ?`
	args = append(args, limit, offset)

	rows, err := DB.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*model.OperLog
	for rows.Next() {
		op := &model.OperLog{}
		if err := rows.Scan(&op.ID, &op.Operator, &op.OpType, &op.Detail, &op.ClientIP, &op.OpTime); err != nil {
			return nil, err
		}
		out = append(out, op)
	}
	return out, nil
}
