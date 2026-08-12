package db

import (
	"time"

	"pxe-server/internal/model"
)

// CreateDeployScript 新增部署脚本。
func CreateDeployScript(s *model.DeployScript) (int64, error) {
	if s.CreateTime.IsZero() {
		s.CreateTime = time.Now()
	}
	res, err := DB.Exec(`INSERT INTO deploy_script(name, content, active, is_default, create_time) VALUES(?,?,?,?,?)`,
		s.Name, s.Content, s.Active, s.IsDefault, s.CreateTime)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

// UpdateDeployScript 更新部署脚本（不改变 active/is_default）。
func UpdateDeployScript(id int64, name, content string) error {
	_, err := DB.Exec(`UPDATE deploy_script SET name=?, content=? WHERE id=?`, name, content, id)
	return err
}

// DeleteDeployScript 删除部署脚本（默认模板不可删除，由调用方检查）。
func DeleteDeployScript(id int64) error {
	_, err := DB.Exec(`DELETE FROM deploy_script WHERE id=?`, id)
	return err
}

// GetDeployScript 查询单个部署脚本。
func GetDeployScript(id int64) (*model.DeployScript, error) {
	row := DB.QueryRow(`SELECT id, name, content, active, is_default, create_time FROM deploy_script WHERE id=?`, id)
	s := &model.DeployScript{}
	if err := row.Scan(&s.ID, &s.Name, &s.Content, &s.Active, &s.IsDefault, &s.CreateTime); err != nil {
		return nil, err
	}
	return s, nil
}

// ListDeployScripts 查询全部部署脚本。
func ListDeployScripts() ([]*model.DeployScript, error) {
	rows, err := DB.Query(`SELECT id, name, content, active, is_default, create_time FROM deploy_script ORDER BY id DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*model.DeployScript
	for rows.Next() {
		s := &model.DeployScript{}
		if err := rows.Scan(&s.ID, &s.Name, &s.Content, &s.Active, &s.IsDefault, &s.CreateTime); err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, nil
}

// GetActiveDeployScript 获取当前生效的部署脚本。
func GetActiveDeployScript() (*model.DeployScript, error) {
	row := DB.QueryRow(`SELECT id, name, content, active, is_default, create_time FROM deploy_script WHERE active=1 LIMIT 1`)
	s := &model.DeployScript{}
	if err := row.Scan(&s.ID, &s.Name, &s.Content, &s.Active, &s.IsDefault, &s.CreateTime); err != nil {
		return nil, err
	}
	return s, nil
}

// SetActiveDeployScript 设置指定脚本生效（其他全部置为 0）。
func SetActiveDeployScript(id int64) error {
	if _, err := DB.Exec(`UPDATE deploy_script SET active=0`); err != nil {
		return err
	}
	_, err := DB.Exec(`UPDATE deploy_script SET active=1 WHERE id=?`, id)
	return err
}
