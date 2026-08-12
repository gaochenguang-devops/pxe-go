package db

import (
	"pxe-server/internal/model"
)

// ListDHCPSubnets 返回全部 DHCP 子网（按 sort_order 排序）。
func ListDHCPSubnets() ([]*model.DHCPSubnet, error) {
	rows, err := DB.Query(`SELECT id, name, ip_pool_start, ip_pool_end, subnet_mask, gateway, dns_servers, enabled
		FROM dhcp_subnet ORDER BY sort_order ASC, id ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*model.DHCPSubnet
	for rows.Next() {
		s := &model.DHCPSubnet{}
		var enabled int
		if err := rows.Scan(&s.ID, &s.Name, &s.IPPoolStart, &s.IPPoolEnd, &s.SubnetMask, &s.Gateway, &s.DNSServers, &enabled); err != nil {
			return nil, err
		}
		s.Enabled = enabled == 1
		out = append(out, s)
	}
	return out, rows.Err()
}

// GetDHCPSubnet 查询单个子网。
func GetDHCPSubnet(id int64) (*model.DHCPSubnet, error) {
	var s model.DHCPSubnet
	var enabled int
	err := DB.QueryRow(`SELECT id, name, ip_pool_start, ip_pool_end, subnet_mask, gateway, dns_servers, enabled
		FROM dhcp_subnet WHERE id=?`, id).
		Scan(&s.ID, &s.Name, &s.IPPoolStart, &s.IPPoolEnd, &s.SubnetMask, &s.Gateway, &s.DNSServers, &enabled)
	if err != nil {
		return nil, err
	}
	s.Enabled = enabled == 1
	return &s, nil
}

// CreateDHCPSubnet 新增子网。
func CreateDHCPSubnet(s *model.DHCPSubnet) (int64, error) {
	enabled := 0
	if s.Enabled {
		enabled = 1
	}
	res, err := DB.Exec(`INSERT INTO dhcp_subnet(name, ip_pool_start, ip_pool_end, subnet_mask, gateway, dns_servers, enabled, sort_order)
		VALUES(?,?,?,?,?,?,?,?)`,
		s.Name, s.IPPoolStart, s.IPPoolEnd, s.SubnetMask, s.Gateway, s.DNSServers, enabled, 0)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

// UpdateDHCPSubnet 更新子网。
func UpdateDHCPSubnet(id int64, s *model.DHCPSubnet) error {
	enabled := 0
	if s.Enabled {
		enabled = 1
	}
	_, err := DB.Exec(`UPDATE dhcp_subnet SET name=?, ip_pool_start=?, ip_pool_end=?, subnet_mask=?, gateway=?, dns_servers=?, enabled=? WHERE id=?`,
		s.Name, s.IPPoolStart, s.IPPoolEnd, s.SubnetMask, s.Gateway, s.DNSServers, enabled, id)
	return err
}

// DeleteDHCPSubnet 删除子网。
func DeleteDHCPSubnet(id int64) error {
	_, err := DB.Exec(`DELETE FROM dhcp_subnet WHERE id=?`, id)
	return err
}
