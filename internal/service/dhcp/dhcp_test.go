package dhcp

import (
	"net"
	"testing"

	"github.com/insomniacslk/dhcp/dhcpv4"

	"pxe-server/internal/model"
)

func TestMacString(t *testing.T) {
	cases := []struct {
		hw   net.HardwareAddr
		want string
	}{
		{net.HardwareAddr{0x00, 0x11, 0x22, 0x33, 0x44, 0x55}, "00:11:22:33:44:55"},
		{net.HardwareAddr{0xAA, 0xBB, 0xCC}, "aa:bb:cc"},
		{net.HardwareAddr{}, "unknown"},
		{nil, "unknown"},
	}
	for _, c := range cases {
		if got := macString(c.hw); got != c.want {
			t.Errorf("macString(%v) = %q, want %q", c.hw, got, c.want)
		}
	}
}

func TestClientArchToString(t *testing.T) {
	cases := []struct {
		arch uint16
		want string
	}{
		{ArchBIOS, "BIOS"},
		{ArchX86UEFI, "x86_64 UEFI"},
		{ArchX86UEFI2, "x86_64 UEFI"},
		{ArchArmUEFI, "aarch64 UEFI"},
		{99, "unknown"},
	}
	for _, c := range cases {
		if got := clientArchToString(c.arch); got != c.want {
			t.Errorf("clientArchToString(%d) = %q, want %q", c.arch, got, c.want)
		}
	}
}

func TestBootFile(t *testing.T) {
	s := &Server{}
	dc := model.DHCPConfig{
		BootFileBIOS: "undionly.kpxe",
		BootFileX86:  "ipxe-x86_64.efi",
		BootFileARM:  "ipxe-aarch64.efi",
	}
	cases := []struct {
		arch uint16
		want string
	}{
		{ArchBIOS, "undionly.kpxe"},
		{ArchX86UEFI, "ipxe-x86_64.efi"},
		{ArchX86UEFI2, "ipxe-x86_64.efi"},
		{ArchArmUEFI, "ipxe-aarch64.efi"},
		{5, "undionly.kpxe"},
	}
	for _, c := range cases {
		if got := s.bootFile(dc, c.arch); got != c.want {
			t.Errorf("bootFile(arch=%d) = %q, want %q", c.arch, got, c.want)
		}
	}
}

func TestIPLessEqual(t *testing.T) {
	cases := []struct {
		a, b net.IP
		want bool
	}{
		{ip(1), ip(1), true},
		{ip(1), ip(2), true},
		{ip(2), ip(1), false},
		{ip(0), ip(255), true},
	}
	for _, c := range cases {
		if got := ipLessEqual(c.a, c.b); got != c.want {
			t.Errorf("ipLessEqual(%v,%v) = %v, want %v", c.a, c.b, got, c.want)
		}
	}
}

func TestIncIP(t *testing.T) {
	a := ip(10)
	incIP(a)
	if !a.Equal(net.IPv4(0, 0, 0, 11)) {
		t.Errorf("incIP = %v, want 0.0.0.11", a)
	}
	// 进位：0.0.0.255 → 0.0.1.0
	b := net.IPv4(0, 0, 0, 255).To4()
	incIP(b)
	if !b.Equal(net.IPv4(0, 0, 1, 0)) {
		t.Errorf("incIP carry = %v, want 0.0.1.0", b)
	}
}

func TestCloneIP(t *testing.T) {
	if got := cloneIP(nil); got != nil {
		t.Error("cloneIP(nil) should be nil")
	}
	src := net.IPv4(10, 0, 0, 1).To4()
	dst := cloneIP(src)
	src[0] = 99
	if dst[0] != 10 {
		t.Error("cloneIP should not share underlying array")
	}
}

func TestClientArch(t *testing.T) {
	s := &Server{}
	// 无 Option 93 → BIOS
	m, _ := dhcpv4.New()
	if got := s.clientArch(m); got != ArchBIOS {
		t.Errorf("no option clientArch = %d, want %d", got, ArchBIOS)
	}
	// 手动设置 Option 93
	// Option 93 = aarch64(11)
	m2, _ := dhcpv4.New()
	m2.Options[uint8(optClientArch)] = []byte{0, 11}
	if got := s.clientArch(m2); got != ArchArmUEFI {
		t.Errorf("clientArch arm = %d, want %d", got, ArchArmUEFI)
	}
	// Option 93 = x86 uefi(7)
	m3, _ := dhcpv4.New()
	m3.Options[uint8(optClientArch)] = []byte{0, 7}
	if got := s.clientArch(m3); got != ArchX86UEFI {
		t.Errorf("clientArch x86 = %d, want %d", got, ArchX86UEFI)
	}
}

func TestAllocateAndPoolContains(t *testing.T) {
	s := &Server{
		ipPool: &IPPool{
			start: net.IPv4(192, 168, 1, 100).To4(),
			end:   net.IPv4(192, 168, 1, 102).To4(),
			lease: 3600,
		},
		leases: make(map[string]*lease),
	}

	// 池内地址应命中
	if !s.poolContains(net.IPv4(192, 168, 1, 100)) {
		t.Error("pool start should be contained")
	}
	if !s.poolContains(net.IPv4(192, 168, 1, 102)) {
		t.Error("pool end should be contained")
	}
	if s.poolContains(net.IPv4(192, 168, 1, 103)) {
		t.Error("out-of-range should not be contained")
	}
	if s.poolContains(net.IPv4(192, 168, 1, 99)) {
		t.Error("below start should not be contained")
	}

	// 分配第一个 IP
	ip1 := s.allocate("aa:bb:cc:dd:ee:01")
	if !ip1.Equal(net.IPv4(192, 168, 1, 100)) {
		t.Errorf("first alloc = %v, want 192.168.1.100", ip1)
	}
	// 记录租约后，同一 MAC 复用同一 IP
	s.recordLease("aa:bb:cc:dd:ee:01", ip1, "h1")
	ipAgain := s.allocate("aa:bb:cc:dd:ee:01")
	if !ipAgain.Equal(ip1) {
		t.Errorf("same MAC should reuse IP, got %v", ipAgain)
	}
	// 不同 MAC 分配到下一个 IP
	ip2 := s.allocate("aa:bb:cc:dd:ee:02")
	if !ip2.Equal(net.IPv4(192, 168, 1, 101)) {
		t.Errorf("second alloc = %v, want 192.168.1.101", ip2)
	}
}

func ip(n byte) net.IP {
	return net.IPv4(0, 0, 0, n).To4()
}
