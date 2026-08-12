package httpapi

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"pxe-server/internal/model"
)

func TestParseInt64(t *testing.T) {
	cases := []struct {
		in   string
		want int64
	}{
		{"0", 0},
		{"123", 123},
		{"007", 7},
		{"-5", 0},   // 非数字返回 0
		{"12a", 0},  // 含非数字返回 0
		{"", 0},     // 空返回 0
		{"9999999999", 9999999999},
	}
	for _, c := range cases {
		if got := parseInt64(c.in); got != c.want {
			t.Errorf("parseInt64(%q) = %d, want %d", c.in, got, c.want)
		}
	}
}

func TestInt64Str(t *testing.T) {
	cases := []struct {
		in   int64
		want string
	}{
		{0, "0"},
		{123, "123"},
		{-456, "-456"},
		{1000000, "1000000"},
	}
	for _, c := range cases {
		if got := int64Str(c.in); got != c.want {
			t.Errorf("int64Str(%d) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestIntStr(t *testing.T) {
	cases := []struct {
		in   int
		want string
	}{
		{0, "0"},
		{7, "7"},
		{-99, "-99"},
		{1024, "1024"},
	}
	for _, c := range cases {
		if got := intStr(c.in); got != c.want {
			t.Errorf("intStr(%d) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestBoolStr(t *testing.T) {
	if boolStr(true) != "true" || boolStr(false) != "false" {
		t.Error("boolStr wrong")
	}
}

func TestSetStr(t *testing.T) {
	m := map[string]string{}
	v := "val"
	setStr(&m, "k", &v)
	if m["k"] != "val" {
		t.Error("setStr with non-nil should set")
	}
	// nil 指针不应修改
	setStr(&m, "k", nil)
	if m["k"] != "val" {
		t.Error("setStr with nil should not change")
	}
	setStrVal(&m, "k2", "v2")
	if m["k2"] != "v2" {
		t.Error("setStrVal should set")
	}
}

func TestScrName(t *testing.T) {
	if scrName(nil) != "" {
		t.Error("scrName(nil) should be empty")
	}
	if scrName(&model.IPxeScript{Name: "boot"}) != "boot" {
		t.Error("scrName should return name")
	}
}

func TestSafeImageName(t *testing.T) {
	cases := []struct {
		in   string
		want bool
	}{
		{"euler1", true},
		{"centos-7", true},
		{"a_b", true},
		{"a b", false},  // 空格不允许
		{"a/b", false},  // 斜杠不允许
		{"", true},      // 空串无非法字符，返回 true（调用方已提前拦截空名）
		{"a..b", false}, // 连续点不允许
		{"../etc", false},
	}
	for _, c := range cases {
		if got := safeImageName(c.in); got != c.want {
			t.Errorf("safeImageName(%q) = %v, want %v", c.in, got, c.want)
		}
	}
}

func TestNodeInfoValuesAndContent(t *testing.T) {
	r := &model.HostResource{
		IPMIAddr:   "10.0.0.1",
		Hostname:   "node1",
		Bond0IP:    "192.168.1.2",
		Bond0Mask:  "255.255.255.0",
		Bond0IPv6:  "fe80::1",
		Bond0IPv6Mask: "64",
	}
	vals := nodeInfoValues(r)
	if vals[0] != "10.0.0.1" || vals[1] != "node1" || vals[2] != "192.168.1.2" {
		t.Errorf("nodeInfoValues wrong: %v", vals)
	}
	// 长度与列数一致
	if len(vals) != len(nodeInfoColumns) {
		t.Errorf("vals len = %d, want %d", len(vals), len(nodeInfoColumns))
	}
	// 空列表 → 空内容
	if buildNodeInfoContent(nil) != "" {
		t.Error("empty content expected")
	}
	// 单条记录 → 单行空格分隔，且与 nodeInfoValues 顺序一致
	content := buildNodeInfoContent([]*model.HostResource{r})
	want := strings.Join(nodeInfoValues(r), " ") + "\n"
	if content != want {
		t.Errorf("content = %q, want %q", content, want)
	}
}

func TestGetRow(t *testing.T) {
	row := []string{"a", "b"}
	if get(row, 0) != "a" || get(row, 1) != "b" {
		t.Error("get in-range wrong")
	}
	if get(row, 5) != "" {
		t.Error("get out-of-range should be empty")
	}
	if get(nil, 0) != "" {
		t.Error("get nil row should be empty")
	}
}

func TestCleanIDs(t *testing.T) {
	in := []int64{1, 2, 0, 2, -3, 5}
	got := cleanIDs(in)
	want := []int64{1, 2, 5}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("cleanIDs = %v, want %v", got, want)
	}
	// 空输入
	if len(cleanIDs(nil)) != 0 {
		t.Error("cleanIDs(nil) should be empty")
	}
}

func TestToSet(t *testing.T) {
	got := toSet([]int64{1, 2, 1})
	if !got[1] || !got[2] || len(got) != 2 {
		t.Errorf("toSet wrong: %v", got)
	}
	if len(toSet(nil)) != 0 {
		t.Error("toSet(nil) should be empty")
	}
}

func TestCleanupTmp(t *testing.T) {
	dir := t.TempDir()
	p1 := filepath.Join(dir, "f1")
	p2 := filepath.Join(dir, "f2")
	_ = os.WriteFile(p1, []byte("x"), 0644)
	_ = os.WriteFile(p2, []byte("x"), 0644)
	cleanupTmp(map[string]string{"a": p1, "b": p2, "c": ""})
	if _, err := os.Stat(p1); !os.IsNotExist(err) {
		t.Error("f1 should be removed")
	}
	if _, err := os.Stat(p2); !os.IsNotExist(err) {
		t.Error("f2 should be removed")
	}
}
