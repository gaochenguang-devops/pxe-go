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
	pool := &IPPool{
		start: net.IPv4(192, 168, 1, 100).To4(),
		end:   net.IPv4(192, 168, 1, 102).To4(),
		lease: 3600,
	}
	s := &Server{
		leases: make(map[string]*lease),
	}

	// 池内地址应命中
	if !s.poolContains(net.IPv4(192, 168, 1, 100), pool) {
		t.Error("pool start should be contained")
	}
	if !s.poolContains(net.IPv4(192, 168, 1, 102), pool) {
		t.Error("pool end should be contained")
	}
	if s.poolContains(net.IPv4(192, 168, 1, 103), pool) {
		t.Error("out-of-range should not be contained")
	}
	if s.poolContains(net.IPv4(192, 168, 1, 99), pool) {
		t.Error("below start should not be contained")
	}

	// 分配第一个 IP
	ip1 := s.allocate("aa:bb:cc:dd:ee:01", pool)
	if !ip1.Equal(net.IPv4(192, 168, 1, 100)) {
		t.Errorf("first alloc = %v, want 192.168.1.100", ip1)
	}
	// 记录租约后，同一 MAC 复用同一 IP
	s.recordLease("aa:bb:cc:dd:ee:01", ip1, "h1", pool)
	ipAgain := s.allocate("aa:bb:cc:dd:ee:01", pool)
	if !ipAgain.Equal(ip1) {
		t.Errorf("same MAC should reuse IP, got %v", ipAgain)
	}
	// 不同 MAC 分配到下一个 IP
	ip2 := s.allocate("aa:bb:cc:dd:ee:02", pool)
	if !ip2.Equal(net.IPv4(192, 168, 1, 101)) {
		t.Errorf("second alloc = %v, want 192.168.1.101", ip2)
	}
}

func ip(n byte) net.IP {
	return net.IPv4(0, 0, 0, n).To4()
}

func TestMatchSubnetByPeerIP(t *testing.T) {
	// 子网池：10.1.0.0/16、192.168.2.0/24 与停用的 192.168.9.0/24
	s := &Server{
		subnets: []*IPPool{
			{network: net.IPv4(10, 1, 0, 0).To4(), prefix: 16, enabled: true},
			{network: net.IPv4(192, 168, 2, 0).To4(), prefix: 24, enabled: true},
			{network: net.IPv4(192, 168, 9, 0).To4(), prefix: 24, enabled: false}, // 停用不匹配
		},
	}

	m, _ := dhcpv4.New()
	dc := model.DHCPConfig{}
	// 来源 IP 落入第一子网（giaddr 为零值，用来源 IP）
	peer := &net.UDPAddr{IP: net.IPv4(10, 1, 5, 5).To4()}
	if got := s.matchSubnet(m, peer, dc); got != s.subnets[0] {
		t.Error("peer in 10.1.0.0/16 should match subnet[0]")
	}
	// 来源 IP 落入第二子网
	peer2 := &net.UDPAddr{IP: net.IPv4(192, 168, 2, 10).To4()}
	if got := s.matchSubnet(m, peer2, dc); got != s.subnets[1] {
		t.Error("peer in 192.168.2.0/24 should match subnet[1]")
	}
	// 来源 IP 落入停用子网 → 不匹配（nil）
	peer3 := &net.UDPAddr{IP: net.IPv4(192, 168, 9, 10).To4()}
	if got := s.matchSubnet(m, peer3, dc); got != nil {
		t.Error("disabled subnet should not match")
	}
	// 来源 IP 不匹配任何子网 → nil
	peer4 := &net.UDPAddr{IP: net.IPv4(8, 8, 8, 8).To4()}
	if got := s.matchSubnet(m, peer4, dc); got != nil {
		t.Error("unmatched peer should be nil")
	}
}

func TestMatchSubnetByGiaddr(t *testing.T) {
	s := &Server{
		subnets: []*IPPool{
			{network: net.IPv4(10, 2, 0, 0).To4(), prefix: 16, enabled: true},
			{network: net.IPv4(10, 3, 0, 0).To4(), prefix: 16, enabled: true},
		},
	}
	// giaddr 优先级高于来源 IP
	m, _ := dhcpv4.New()
	m.GatewayIPAddr = net.IPv4(10, 3, 0, 1).To4()
	peer := &net.UDPAddr{IP: net.IPv4(10, 2, 0, 1).To4()}
	if got := s.matchSubnet(m, peer, model.DHCPConfig{}); got != s.subnets[1] {
		t.Error("giaddr should take priority over peer IP")
	}
}

// TestMatchSubnet_SameSegmentDirect 同网段直连（无中继、广播、源 0.0.0.0）：
// 靠服务器监听 IP（ListenIP）匹配子网。
func TestMatchSubnet_SameSegmentDirect(t *testing.T) {
	s := &Server{
		subnets: []*IPPool{
			{network: net.IPv4(10, 122, 240, 0).To4(), prefix: 26, enabled: true},
			{network: net.IPv4(10, 122, 240, 128).To4(), prefix: 26, enabled: true},
		},
	}
	m, _ := dhcpv4.New()
	// giaddr 为空、peer 为广播(0.0.0.0)，仅靠 ListenIP 识别
	dc := model.DHCPConfig{ListenIP: "10.122.240.62"}
	peer := &net.UDPAddr{IP: net.IPv4zero.To4()}
	if got := s.matchSubnet(m, peer, dc); got != s.subnets[0] {
		t.Error("same-segment direct should match by ListenIP")
	}
	// 监听 IP 属于第二子网
	dc2 := model.DHCPConfig{ListenIP: "10.122.240.190"}
	if got := s.matchSubnet(m, peer, dc2); got != s.subnets[1] {
		t.Error("same-segment direct should match second subnet by ListenIP")
	}
	// 监听 IP 无匹配 → nil
	dc3 := model.DHCPConfig{ListenIP: "192.168.99.1"}
	if got := s.matchSubnet(m, peer, dc3); got != nil {
		t.Error("unmatched ListenIP should be nil")
	}
}

func TestNetworkOf(t *testing.T) {
	netIP, prefix := networkOf("192.168.1.100", "255.255.255.0")
	if prefix != 24 {
		t.Errorf("prefix = %d, want 24", prefix)
	}
	if !netIP.Equal(net.IPv4(192, 168, 1, 0).To4()) {
		t.Errorf("network = %v, want 192.168.1.0", netIP)
	}
	// 非法输入
	if n, p := networkOf("bad", "255.255.255.0"); n != nil || p != -1 {
		t.Error("invalid input should return nil,-1")
	}
}
