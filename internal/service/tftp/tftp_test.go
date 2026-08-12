package tftp

import (
	"io"
	"net"
	"testing"
)

func TestIsIPxeScript(t *testing.T) {
	cases := []struct {
		name string
		want bool
	}{
		{"autoexec.ipxe", true},
		{"boot.ipxe", true},
		{"menu.ipxe.cfg", true},
		{"AUTOEXEC.IPXE", true}, // 大小写不敏感
		{"vmlinuz", false},
		{"initrd.img", false},
		{"autoexec.txt", false},
		{"", false},
	}
	for _, c := range cases {
		if got := isIPxeScript(c.name); got != c.want {
			t.Errorf("isIPxeScript(%q) = %v, want %v", c.name, got, c.want)
		}
	}
}

// mockReaderFrom 实现 io.ReaderFrom + RemoteAddr，用于测试 getRemoteAddr。
type mockReaderFrom struct {
	addr net.Addr
}

func (m *mockReaderFrom) ReadFrom(r io.Reader) (int64, error) {
	return 0, nil
}

func (m *mockReaderFrom) RemoteAddr() net.Addr { return m.addr }

type fakeAddr string

func (f fakeAddr) Network() string { return "udp" }
func (f fakeAddr) String() string  { return string(f) }

func TestGetRemoteAddr(t *testing.T) {
	// 实现了 RemoteAddr → 提取 host
	m := &mockReaderFrom{addr: fakeAddr("192.168.1.5:2000")}
	if got := getRemoteAddr(m); got != "192.168.1.5" {
		t.Errorf("getRemoteAddr = %q, want 192.168.1.5", got)
	}
	// 未实现 RemoteAddr → 返回 "?"
	if got := getRemoteAddr(nil); got != "?" {
		t.Errorf("getRemoteAddr(nil) = %q, want ?", got)
	}
}
