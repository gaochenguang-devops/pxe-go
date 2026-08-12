package db

import (
	"time"

	"pxe-server/internal/model"
)

// CreateIPxeScript 新增 iPXE 脚本。
func CreateIPxeScript(s *model.IPxeScript) (int64, error) {
	if s.CreateTime.IsZero() {
		s.CreateTime = time.Now()
	}
	res, err := DB.Exec(`INSERT INTO ipxe_script(name, content, active, is_default, create_time) VALUES(?,?,?,?,?)`,
		s.Name, s.Content, s.Active, s.IsDefault, s.CreateTime)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

// UpdateIPxeScript 更新 iPXE 脚本（不改变 active/is_default）。
func UpdateIPxeScript(id int64, name, content string) error {
	_, err := DB.Exec(`UPDATE ipxe_script SET name=?, content=? WHERE id=?`, name, content, id)
	return err
}

// DeleteIPxeScript 删除 iPXE 脚本（默认模板不可删除，由调用方检查）。
func DeleteIPxeScript(id int64) error {
	_, err := DB.Exec(`DELETE FROM ipxe_script WHERE id=?`, id)
	return err
}

// GetIPxeScript 查询单个 iPXE 脚本。
func GetIPxeScript(id int64) (*model.IPxeScript, error) {
	row := DB.QueryRow(`SELECT id, name, content, active, is_default, create_time FROM ipxe_script WHERE id=?`, id)
	s := &model.IPxeScript{}
	if err := row.Scan(&s.ID, &s.Name, &s.Content, &s.Active, &s.IsDefault, &s.CreateTime); err != nil {
		return nil, err
	}
	return s, nil
}

// ListIPxeScripts 查询全部 iPXE 脚本。
func ListIPxeScripts() ([]*model.IPxeScript, error) {
	rows, err := DB.Query(`SELECT id, name, content, active, is_default, create_time FROM ipxe_script ORDER BY id DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*model.IPxeScript
	for rows.Next() {
		s := &model.IPxeScript{}
		if err := rows.Scan(&s.ID, &s.Name, &s.Content, &s.Active, &s.IsDefault, &s.CreateTime); err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, nil
}

// GetActiveIPxeScript 获取当前生效的 iPXE 脚本。
func GetActiveIPxeScript() (*model.IPxeScript, error) {
	row := DB.QueryRow(`SELECT id, name, content, active, is_default, create_time FROM ipxe_script WHERE active=1 LIMIT 1`)
	s := &model.IPxeScript{}
	if err := row.Scan(&s.ID, &s.Name, &s.Content, &s.Active, &s.IsDefault, &s.CreateTime); err != nil {
		return nil, err
	}
	return s, nil
}

// SetActiveIPxeScript 将指定脚本设为生效，并将其他脚本置为不生效。
// 返回设为生效的脚本。
func SetActiveIPxeScript(id int64) (*model.IPxeScript, error) {
	// 先把所有脚本置为不生效
	if _, err := DB.Exec(`UPDATE ipxe_script SET active=0`); err != nil {
		return nil, err
	}
	// 再激活目标脚本
	if _, err := DB.Exec(`UPDATE ipxe_script SET active=1 WHERE id=?`, id); err != nil {
		return nil, err
	}
	return GetIPxeScript(id)
}

// CountIPxeScripts 统计 iPXE 脚本数量。
func CountIPxeScripts() (int, error) {
	var n int
	err := DB.QueryRow(`SELECT COUNT(1) FROM ipxe_script`).Scan(&n)
	return n, err
}
