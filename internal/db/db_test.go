package db

import (
	"testing"

	"pxe-server/internal/model"
)

// 初始化内存数据库，供各测试使用。
func setupTestDB(t *testing.T) {
	t.Helper()
	if err := Init(":memory:"); err != nil {
		t.Fatalf("Init memory db failed: %v", err)
	}
	t.Cleanup(func() { _ = Close() })
}

func TestConfigGetSet(t *testing.T) {
	setupTestDB(t)
	// 默认配置应存在
	rows, err := GetAllConfigs()
	if err != nil {
		t.Fatalf("GetAllConfigs err: %v", err)
	}
	if len(rows) == 0 {
		t.Error("default config should not be empty")
	}
	// 设置自定义配置
	if err := SetConfig("test_key", "test_val"); err != nil {
		t.Fatalf("SetConfig err: %v", err)
	}
	rows, err = GetAllConfigs()
	if err != nil {
		t.Fatalf("GetAllConfigs after set err: %v", err)
	}
	found := false
	for _, r := range rows {
		if r.ConfigKey == "test_key" && r.ConfigValue == "test_val" {
			found = true
		}
	}
	if !found {
		t.Error("custom config not found after SetConfig")
	}
}

func TestOSImageCRUD(t *testing.T) {
	setupTestDB(t)

	// 创建
	img := &model.OSImage{
		Name:        "euler1",
		X86RepoPath: "/repo/euler1/x86_64",
		ArmRepoPath: "/repo/euler1/aarch64",
	}
	id, err := CreateOSImage(img)
	if err != nil {
		t.Fatalf("CreateOSImage err: %v", err)
	}
	if id <= 0 {
		t.Fatal("CreateOSImage returned invalid id")
	}

	// 查询列表
	list, err := ListOSImages()
	if err != nil {
		t.Fatalf("ListOSImages err: %v", err)
	}
	if len(list) != 1 || list[0].Name != "euler1" {
		t.Errorf("unexpected list: %+v", list)
	}

	// 按 ID 查询
	got, err := GetOSImage(id)
	if err != nil {
		t.Fatalf("GetOSImage err: %v", err)
	}
	if got.X86RepoPath != "/repo/euler1/x86_64" {
		t.Errorf("got repo path = %q", got.X86RepoPath)
	}

	// 设为默认
	if _, err := SetActiveOSImage(id); err != nil {
		t.Fatalf("SetActiveOSImage err: %v", err)
	}
	active, err := GetActiveOSImage()
	if err != nil {
		t.Fatalf("GetActiveOSImage err: %v", err)
	}
	if active.ID != id || active.Active != 1 {
		t.Errorf("active image wrong: %+v", active)
	}

	// 更新
	got.X86RepoPath = "/repo/euler1/x86_64/updated"
	if err := UpdateOSImage(id, got); err != nil {
		t.Fatalf("UpdateOSImage err: %v", err)
	}
	got2, _ := GetOSImage(id)
	if got2.X86RepoPath != "/repo/euler1/x86_64/updated" {
		t.Error("update did not persist")
	}

	// 删除
	if err := DeleteOSImage(id); err != nil {
		t.Fatalf("DeleteOSImage err: %v", err)
	}
	list, _ = ListOSImages()
	if len(list) != 0 {
		t.Error("image should be deleted")
	}
}

func TestHostCRUD(t *testing.T) {
	setupTestDB(t)

	h := &model.HostInfo{
		Hostname:      "node1",
		IPMIAddr:      "10.0.0.1",
		IPMIUser:      "admin",
		IPMIPass:      "secret",
		InstallStatus: "pending",
	}
	id, err := CreateHost(h)
	if err != nil {
		t.Fatalf("CreateHost err: %v", err)
	}
	if id <= 0 {
		t.Fatal("invalid host id")
	}

	// 重复 IPMI 应被拒绝
	dup := &model.HostInfo{IPMIAddr: "10.0.0.1"}
	if _, err := CreateHost(dup); err == nil {
		t.Error("duplicate IPMI should be rejected")
	}

	// 查询
	got, err := GetHostByID(id)
	if err != nil {
		t.Fatalf("GetHostByID err: %v", err)
	}
	if got.Hostname != "node1" {
		t.Errorf("got hostname = %q", got.Hostname)
	}

	// 列表
	list, err := ListHosts()
	if err != nil {
		t.Fatalf("ListHosts err: %v", err)
	}
	if len(list) != 1 {
		t.Errorf("host list len = %d, want 1", len(list))
	}

	// 更新
	got.Hostname = "node1-updated"
	if err := UpdateHost(id, got); err != nil {
		t.Fatalf("UpdateHost err: %v", err)
	}
	got2, _ := GetHostByID(id)
	if got2.Hostname != "node1-updated" {
		t.Error("host update did not persist")
	}

	// 删除
	if err := DeleteHost(id); err != nil {
		t.Fatalf("DeleteHost err: %v", err)
	}
	list, _ = ListHosts()
	if len(list) != 0 {
		t.Error("host should be deleted")
	}
}

func TestKSTemplateCRUD(t *testing.T) {
	setupTestDB(t)

	tpl := &model.KSTemplate{
		Name:    "default",
		Content: "graphical\n",
		Active:  1,
	}
	id, err := CreateKSTemplate(tpl)
	if err != nil {
		t.Fatalf("CreateKSTemplate err: %v", err)
	}

	// 查询生效模板
	active, err := GetActiveKSTemplate()
	if err != nil {
		t.Fatalf("GetActiveKSTemplate err: %v", err)
	}
	if active.ID != id {
		t.Errorf("active template id = %d, want %d", active.ID, id)
	}

	// 列表
	list, err := ListKSTemplates()
	if err != nil {
		t.Fatalf("ListKSTemplates err: %v", err)
	}
	if len(list) < 1 {
		t.Error("ks template list should not be empty")
	}

	// 删除
	if err := DeleteKSTemplate(id); err != nil {
		t.Fatalf("DeleteKSTemplate err: %v", err)
	}
}

func TestDHCPSubnetCRUD(t *testing.T) {
	setupTestDB(t)
	// 清空迁移自动创建的默认子网，独立测试 CRUD
	if _, err := DB.Exec(`DELETE FROM dhcp_subnet`); err != nil {
		t.Fatalf("clear subnets: %v", err)
	}

	// 新增
	sub := &model.DHCPSubnet{
		Name:        "办公子网",
		IPPoolStart: "192.168.1.100",
		IPPoolEnd:   "192.168.1.200",
		SubnetMask:  "255.255.255.0",
		Gateway:     "192.168.1.1",
		DNSServers:  "192.168.1.1",
		Enabled:     true,
	}
	id, err := CreateDHCPSubnet(sub)
	if err != nil {
		t.Fatalf("CreateDHCPSubnet err: %v", err)
	}
	if id <= 0 {
		t.Fatal("invalid subnet id")
	}

	// 查询列表
	list, err := ListDHCPSubnets()
	if err != nil {
		t.Fatalf("ListDHCPSubnets err: %v", err)
	}
	if len(list) != 1 || list[0].Name != "办公子网" || !list[0].Enabled {
		t.Errorf("unexpected list: %+v", list)
	}

	// 按 ID 查询
	got, err := GetDHCPSubnet(id)
	if err != nil {
		t.Fatalf("GetDHCPSubnet err: %v", err)
	}
	if got.IPPoolStart != "192.168.1.100" {
		t.Errorf("got pool start = %q", got.IPPoolStart)
	}

	// 更新
	got.Name = "研发子网"
	got.Enabled = false
	if err := UpdateDHCPSubnet(id, got); err != nil {
		t.Fatalf("UpdateDHCPSubnet err: %v", err)
	}
	got2, _ := GetDHCPSubnet(id)
	if got2.Name != "研发子网" || got2.Enabled {
		t.Error("update did not persist")
	}

	// 删除
	if err := DeleteDHCPSubnet(id); err != nil {
		t.Fatalf("DeleteDHCPSubnet err: %v", err)
	}
	list, _ = ListDHCPSubnets()
	if len(list) != 0 {
		t.Error("subnet should be deleted")
	}
}

func TestIPxeScriptCRUD(t *testing.T) {
	setupTestDB(t)

	// 创建生效脚本
	s := &model.IPxeScript{
		Name:    "default",
		Content: "#!ipxe\n",
		Active:  1,
	}
	id, err := CreateIPxeScript(s)
	if err != nil {
		t.Fatalf("CreateIPxeScript err: %v", err)
	}

	// 查询生效脚本
	active, err := GetActiveIPxeScript()
	if err != nil {
		t.Fatalf("GetActiveIPxeScript err: %v", err)
	}
	if active.ID != id {
		t.Errorf("active script id = %d, want %d", active.ID, id)
	}

	// 切换生效脚本
	s2 := &model.IPxeScript{Name: "second", Content: "#!ipxe\n", Active: 0}
	id2, err := CreateIPxeScript(s2)
	if err != nil {
		t.Fatalf("CreateIPxeScript second err: %v", err)
	}
	if _, err := SetActiveIPxeScript(id2); err != nil {
		t.Fatalf("SetActiveIPxeScript err: %v", err)
	}
	active2, _ := GetActiveIPxeScript()
	if active2.ID != id2 {
		t.Errorf("active should switch to script %d, got %d", id2, active2.ID)
	}

	// 删除
	if err := DeleteIPxeScript(id); err != nil {
		t.Fatalf("DeleteIPxeScript err: %v", err)
	}
	list, _ := ListIPxeScripts()
	if len(list) != 1 {
		t.Errorf("ipxe script list len = %d, want 1", len(list))
	}
}

// TestMigrateDHCPSubnet 验证旧单网段配置自动迁移为子网记录。
func TestMigrateDHCPSubnet(t *testing.T) {
	setupTestDB(t)
	// 清空已迁移的子网与旧配置键，模拟旧数据库状态
	if _, err := DB.Exec(`DELETE FROM dhcp_subnet`); err != nil {
		t.Fatalf("clear subnets: %v", err)
	}
	for _, k := range []string{"dhcp_ip_pool_start", "dhcp_ip_pool_end", "dhcp_subnet_mask", "dhcp_gateway", "dhcp_dns_servers"} {
		_, _ = DB.Exec(`DELETE FROM sys_config WHERE config_key=?`, k)
	}

	// 写入旧的单网段配置键
	if err := SetConfig("dhcp_ip_pool_start", "10.0.0.100"); err != nil {
		t.Fatal(err)
	}
	if err := SetConfig("dhcp_ip_pool_end", "10.0.0.200"); err != nil {
		t.Fatal(err)
	}
	if err := SetConfig("dhcp_subnet_mask", "255.255.255.0"); err != nil {
		t.Fatal(err)
	}
	if err := SetConfig("dhcp_gateway", "10.0.0.1"); err != nil {
		t.Fatal(err)
	}
	if err := SetConfig("dhcp_dns_servers", "10.0.0.1"); err != nil {
		t.Fatal(err)
	}

	if err := migrateDHCPSubnet(); err != nil {
		t.Fatalf("migrateDHCPSubnet err: %v", err)
	}

	// 应生成一条子网，值来自旧配置键
	list, err := ListDHCPSubnets()
	if err != nil {
		t.Fatalf("ListDHCPSubnets err: %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("expected 1 subnet after migrate, got %d", len(list))
	}
	if list[0].IPPoolStart != "10.0.0.100" || list[0].Gateway != "10.0.0.1" {
		t.Errorf("migrated subnet wrong: %+v", list[0])
	}

	// 旧配置键应被清理
	for _, k := range []string{"dhcp_ip_pool_start", "dhcp_ip_pool_end", "dhcp_subnet_mask", "dhcp_gateway", "dhcp_dns_servers"} {
		if v, _ := GetConfig(k); v != "" {
			t.Errorf("old config key %q should be removed, got %q", k, v)
		}
	}

	// 幂等：再次迁移不再新增
	if err := migrateDHCPSubnet(); err != nil {
		t.Fatalf("second migrate err: %v", err)
	}
	list, _ = ListDHCPSubnets()
	if len(list) != 1 {
		t.Errorf("second migrate should not duplicate, got %d", len(list))
	}
}
