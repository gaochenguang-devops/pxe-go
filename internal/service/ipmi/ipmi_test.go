package ipmi

import (
	"reflect"
	"testing"
)

func TestBaseArgs(t *testing.T) {
	c := &Client{}
	got := c.baseArgs("10.0.0.1", "admin", "secret")
	want := []string{"-I", "lanplus", "-H", "10.0.0.1", "-U", "admin", "-P", "secret"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("baseArgs = %v, want %v", got, want)
	}
}

func TestNormalizeAction(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		{"on", "on"},
		{"OFF", "off"},
		{"Cycle", "cycle"},
		{"status", "status"},
		{"reset", "reset"},
		{"reboot", ""},      // 非法
		{"", ""},            // 空
		{"random", ""},      // 非法
		{" ON ", ""},        // 带空格不匹配
	}
	for _, c := range cases {
		if got := NormalizeAction(c.in); got != c.want {
			t.Errorf("NormalizeAction(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestErrIPMI(t *testing.T) {
	e := errIPMI("boom")
	if e.Error() != "boom" {
		t.Errorf("errIPMI.Error = %q, want boom", e.Error())
	}
	if errTimeout.Error() != "ipmitool timeout" {
		t.Errorf("errTimeout = %q", errTimeout.Error())
	}
}
