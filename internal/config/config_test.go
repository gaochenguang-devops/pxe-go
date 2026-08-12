package config

import (
	"reflect"
	"testing"
)

func TestCfgStr(t *testing.T) {
	cfg := map[string]string{"k1": "v1", "k2": ""}
	cases := []struct {
		key, def, want string
	}{
		{"k1", "default", "v1"},
		{"k2", "default", "default"}, // 空值回退默认
		{"missing", "default", "default"},
		{"missing", "", ""},
	}
	for _, c := range cases {
		if got := cfgStr(cfg, c.key, c.def); got != c.want {
			t.Errorf("cfgStr(%q) = %q, want %q", c.key, got, c.want)
		}
	}
}

func TestCfgBool(t *testing.T) {
	cfg := map[string]string{
		"t1": "true",
		"t2": "false",
		"t3": "1",
		"t4": "invalid",
	}
	cases := []struct {
		key  string
		def  bool
		want bool
	}{
		{"t1", false, true},
		{"t2", true, false},
		{"t3", false, true},
		{"t4", true, true}, // 非法值回退默认
		{"missing", true, true},
		{"missing", false, false},
	}
	for _, c := range cases {
		if got := cfgBool(cfg, c.key, c.def); got != c.want {
			t.Errorf("cfgBool(%q) = %v, want %v", c.key, got, c.want)
		}
	}
}

func TestCfgInt(t *testing.T) {
	cfg := map[string]string{
		"i1": "42",
		"i2": "abc",
		"i3": "-7",
	}
	cases := []struct {
		key  string
		def  int
		want int
	}{
		{"i1", 0, 42},
		{"i2", 10, 10}, // 非法回退默认
		{"i3", 0, -7},
		{"missing", 5, 5},
	}
	for _, c := range cases {
		if got := cfgInt(cfg, c.key, c.def); got != c.want {
			t.Errorf("cfgInt(%q) = %d, want %d", c.key, got, c.want)
		}
	}
}

func TestCfgInt64(t *testing.T) {
	cfg := map[string]string{
		"i64": "9007199254740992",
		"bad": "x",
	}
	cases := []struct {
		key  string
		def  int64
		want int64
	}{
		{"i64", 0, 9007199254740992},
		{"bad", 7, 7},
		{"missing", 3, 3},
	}
	for _, c := range cases {
		if got := cfgInt64(cfg, c.key, c.def); got != c.want {
			t.Errorf("cfgInt64(%q) = %d, want %d", c.key, got, c.want)
		}
	}
}

func TestSplitDNS(t *testing.T) {
	cases := []struct {
		in   string
		want []string
	}{
		{"1.1.1.1,8.8.8.8", []string{"1.1.1.1", "8.8.8.8"}},
		{" 1.1.1.1 , 8.8.8.8 ", []string{"1.1.1.1", "8.8.8.8"}},
		{"1.1.1.1", []string{"1.1.1.1"}},
		{"", nil},
		{",,", nil},
		{" ,  ", nil},
	}
	for _, c := range cases {
		if got := SplitDNS(c.in); !reflect.DeepEqual(got, c.want) {
			t.Errorf("SplitDNS(%q) = %v, want %v", c.in, got, c.want)
		}
	}
}

func TestManagerSeedKey(t *testing.T) {
	m := &Manager{seedKey: "pxe-server-seed-key-2026"}
	if m.SeedKey() != "pxe-server-seed-key-2026" {
		t.Errorf("SeedKey = %q", m.SeedKey())
	}
}

func TestManagerGettersEmpty(t *testing.T) {
	// 未加载配置时，getter 返回零值且不 panic
	m := &Manager{}
	if m.DHCP().Enabled != false {
		t.Error("DHCP default Enabled should be false")
	}
	if m.TFTP().RootDir != "" {
		t.Error("TFTP default RootDir should be empty")
	}
	if m.HTTP().WebRoot != "" {
		t.Error("HTTP default WebRoot should be empty")
	}
	// HTTPRaw 与 HTTP 一致
	if m.HTTPRaw().AdminUser != m.HTTP().AdminUser {
		t.Error("HTTPRaw should match HTTP")
	}
}
