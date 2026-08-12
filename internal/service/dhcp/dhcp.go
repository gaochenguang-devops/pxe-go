// Package dhcp 实现标准 DHCP 服务。
// 底层基于成熟的 github.com/insomniacslk/dhcp 库进行报文解析/构造，
// 本层负责：
//   - 地址池分配与租约管理
//   - 架构识别（Option 93）与引导文件下发（Option 66/67）
//   - iPXE 二次请求识别（Option 175）下发 autoexec.ipxe
//   - 配置热加载与优雅停止
package dhcp

import (
	"fmt"
	"net"
	"strings"
	"sync"
	"time"

	"github.com/insomniacslk/dhcp/dhcpv4"
	"github.com/insomniacslk/dhcp/dhcpv4/server4"

	"pxe-server/internal/config"
	"pxe-server/internal/logger"
	"pxe-server/internal/model"
)

// 架构类型（Option 93 client-arch）。
const (
	ArchBIOS     uint16 = 0
	ArchX86UEFI  uint16 = 7
	ArchX86UEFI2 uint16 = 9
	ArchArmUEFI  uint16 = 11
)

// 自定义 OptionCode（Option 93/175 库中无标准常量）。
type customOptionCode uint8

func (c customOptionCode) Code() uint8    { return uint8(c) }
func (c customOptionCode) String() string { return fmt.Sprintf("custom(%d)", uint8(c)) }

const (
	optClientArch customOptionCode = 93  // 客户端系统架构
	optIpxeVendor customOptionCode = 175 // iPXE 厂商标识
)

// lease 租约记录。
type lease struct {
	IP       net.IP
	MAC      string
	Hostname string
	ExpireAt time.Time
}

// Server DHCP 服务器。
type Server struct {
	cfg         *config.Manager
	conn        net.PacketConn
	srv         *server4.Server
	leases      map[string]*lease // key: MAC
	mu          sync.Mutex
	ipPool      *IPPool
	lastVersion int64
}

// IPPool 地址池管理。
type IPPool struct {
	start   net.IP
	end     net.IP
	mask    net.IP
	gateway net.IP
	lease   int
}

// NewServer 创建 DHCP 服务器。
func NewServer(cfg *config.Manager) *Server {
	return &Server{
		cfg:    cfg,
		leases: make(map[string]*lease),
	}
}

// Start 启动 DHCP 服务监听。
func (s *Server) Start() error {
	dc := s.cfg.DHCP()
	if !dc.Enabled {
		logger.Info("DHCP 服务已禁用，跳过启动")
		return nil
	}
	s.buildPool(dc)
	if err := s.startConn(dc); err != nil {
		return err
	}
	logger.Info("DHCP 服务已启动，地址池 %s-%s", s.ipPool.start, s.ipPool.end)
	return nil
}

// Stop 停止 DHCP 服务。
func (s *Server) Stop() {
	s.stopConn()
	logger.Info("DHCP 服务已停止")
}

// startConn 启动底层连接与服务 goroutine（已持有锁或无需锁）。
func (s *Server) startConn(dc model.DHCPConfig) error {
	addr := &net.UDPAddr{IP: net.ParseIP(dc.ListenIP).To4(), Port: 67}
	if addr.IP == nil {
		addr.IP = net.IPv4zero
	}
	conn, err := net.ListenUDP("udp4", addr)
	if err != nil {
		return err
	}
	if err := setBroadcastOn(conn); err != nil {
		logger.Warn("DHCP 启用广播失败: %v", err)
	}
	s.conn = conn
	srv, err := server4.NewServer("", addr, s.handler, server4.WithConn(conn))
	if err != nil {
		conn.Close()
		return err
	}
	s.srv = srv
	go func() { _ = srv.Serve() }()
	return nil
}

// stopConn 停止底层连接与 server4。
func (s *Server) stopConn() {
	if s.srv != nil {
		_ = s.srv.Close()
		s.srv = nil
	}
	if s.conn != nil {
		_ = s.conn.Close()
		s.conn = nil
	}
}

// Reload 配置热重载（含启停控制）。
func (s *Server) Reload() {
	dc := s.cfg.DHCP()
	if dc.ConfigVersion == s.lastVersion {
		return
	}
	s.lastVersion = dc.ConfigVersion
	// 根据 Enabled 状态启动/停止
	s.mu.Lock()
	running := s.conn != nil
	s.mu.Unlock()
	if dc.Enabled && !running {
		s.buildPool(dc)
		if err := s.startConn(dc); err != nil {
			logger.Error("DHCP 重载启动失败: %v", err)
		} else {
			logger.Info("DHCP 已重新启动，地址池 %s-%s", s.ipPool.start, s.ipPool.end)
		}
	} else if !dc.Enabled && running {
		s.stopConn()
		logger.Info("DHCP 已停用")
	} else if dc.Enabled && running {
		s.buildPool(dc)
		logger.Info("DHCP 配置热重载完成，地址池 %s-%s", s.ipPool.start, s.ipPool.end)
	}
}

// handler 处理每个 DHCPv4 报文。
func (s *Server) handler(conn net.PacketConn, peer net.Addr, m *dhcpv4.DHCPv4) {
	dc := s.cfg.DHCP()
	macStr := macString(m.ClientHWAddr)
	arch := s.clientArch(m)

	switch m.MessageType() {
	case dhcpv4.MessageTypeDiscover:
		logger.Debug("DHCP DISCOVER: MAC=%s arch=%s from %s", macStr, clientArchToString(arch), peer.String())
		ip := s.allocate(macStr)
		if ip == nil {
			logger.Warn("DHCP 地址池耗尽，无法为 %s 分配地址", macStr)
			return
		}
		reply := s.buildReply(m, dhcpv4.MessageTypeOffer, ip, arch, dc)
		writeReply(conn, peer, reply)

	case dhcpv4.MessageTypeRequest:
		s.handleRequest(conn, peer, m, arch, dc)

	case dhcpv4.MessageTypeRelease:
		s.release(macStr)
		logger.Debug("DHCP RELEASE: MAC=%s", macStr)

	case dhcpv4.MessageTypeDecline:
		s.release(macStr)
		logger.Debug("DHCP DECLINE: MAC=%s", macStr)
	}
}

// handleRequest 处理 REQUEST，返回 ACK/NACK。
func (s *Server) handleRequest(conn net.PacketConn, peer net.Addr, m *dhcpv4.DHCPv4, arch uint16, dc model.DHCPConfig) {
	macStr := macString(m.ClientHWAddr)
	requested := m.Options.Get(dhcpv4.OptionRequestedIPAddress)
	serverID := m.Options.Get(dhcpv4.OptionServerIdentifier)

	// 若指定了 server-id 且不是本服务器，忽略
	if len(serverID) == 4 {
		pxe := net.ParseIP(dc.PXEIP).To4()
		if pxe != nil && !net.IP(serverID).Equal(pxe) {
			return
		}
	}

	var ip net.IP
	if len(requested) == 4 {
		ip = net.IP(requested).To4()
		if !s.poolContains(ip) {
			writeReply(conn, peer, s.buildNak(m, dc))
			return
		}
	} else {
		ip = s.allocate(macStr)
		if ip == nil {
			writeReply(conn, peer, s.buildNak(m, dc))
			return
		}
	}
	s.recordLease(macStr, ip, m.HostName())
	logger.Debug("DHCP REQUEST: MAC=%s 分配 %s", macStr, ip.String())
	reply := s.buildReply(m, dhcpv4.MessageTypeAck, ip, arch, dc)
	writeReply(conn, peer, reply)
}

// clientArch 解析 Option 93 返回架构编号。
func (s *Server) clientArch(m *dhcpv4.DHCPv4) uint16 {
	v := m.Options.Get(optClientArch)
	if len(v) >= 2 {
		return uint16(v[0])<<8 | uint16(v[1])
	}
	return ArchBIOS
}

// clientArchToString 架构名称。
func clientArchToString(arch uint16) string {
	switch arch {
	case ArchBIOS:
		return "BIOS"
	case ArchX86UEFI, ArchX86UEFI2:
		return "x86_64 UEFI"
	case ArchArmUEFI:
		return "aarch64 UEFI"
	default:
		return "unknown"
	}
}

// bootFile 根据架构返回引导文件名。
func (s *Server) bootFile(dc model.DHCPConfig, arch uint16) string {
	switch arch {
	case ArchX86UEFI, ArchX86UEFI2:
		return dc.BootFileX86
	case ArchArmUEFI:
		return dc.BootFileARM
	default:
		return dc.BootFileBIOS
	}
}

// buildReply 构造 OFFER/ACK 应答。
func (s *Server) buildReply(req *dhcpv4.DHCPv4, msgType dhcpv4.MessageType, yiAddr net.IP, arch uint16, dc model.DHCPConfig) *dhcpv4.DHCPv4 {
	pxeIP := net.ParseIP(dc.PXEIP).To4()
	if pxeIP == nil {
		pxeIP = net.IPv4zero
	}

	reply, err := dhcpv4.NewReplyFromRequest(req,
		dhcpv4.WithReply(req),
		dhcpv4.WithMessageType(msgType),
	)
	if err != nil {
		logger.Error("构造 DHCP 应答失败: %v", err)
		return nil
	}
	reply.YourIPAddr = yiAddr.To4()
	reply.ServerIPAddr = pxeIP

	reply.UpdateOption(dhcpv4.OptServerIdentifier(pxeIP))
	reply.UpdateOption(dhcpv4.OptIPAddressLeaseTime(time.Duration(dc.LeaseTime) * time.Second))
	if s.ipPool.mask != nil {
		reply.UpdateOption(dhcpv4.OptSubnetMask(net.IPMask(s.ipPool.mask.To4())))
	}
	if s.ipPool.gateway != nil {
		reply.UpdateOption(dhcpv4.OptRouter(s.ipPool.gateway.To4()))
	}
	if dc.DNSServers != "" {
		var dns []net.IP
		for _, d := range strings.Split(dc.DNSServers, ",") {
			if ip := net.ParseIP(strings.TrimSpace(d)).To4(); ip != nil {
				dns = append(dns, ip)
			}
		}
		if len(dns) > 0 {
			reply.UpdateOption(dhcpv4.OptDNS(dns...))
		}
	}
	// Option 66 TFTP 服务器
	reply.UpdateOption(dhcpv4.OptTFTPServerName(pxeIP.String()))
	// Option 67 引导文件
	bootFile := s.bootFile(dc, arch)
	if req.Options.Has(optIpxeVendor) {
		bootFile = dc.IpxeScript // iPXE 二次请求下发 autoexec.ipxe
	}
	reply.UpdateOption(dhcpv4.OptBootFileName(bootFile))

	return reply
}

// buildNak 构造 NAK 应答。
func (s *Server) buildNak(req *dhcpv4.DHCPv4, dc model.DHCPConfig) *dhcpv4.DHCPv4 {
	pxeIP := net.ParseIP(dc.PXEIP).To4()
	if pxeIP == nil {
		pxeIP = net.IPv4zero
	}
	reply, err := dhcpv4.NewReplyFromRequest(req,
		dhcpv4.WithReply(req),
		dhcpv4.WithMessageType(dhcpv4.MessageTypeNak),
	)
	if err != nil {
		return nil
	}
	reply.UpdateOption(dhcpv4.OptServerIdentifier(pxeIP))
	return reply
}

// writeReply 发送 DHCP 应答（peer 已由 server4 处理：0.0.0.0 来源转广播）。
func writeReply(conn net.PacketConn, peer net.Addr, reply *dhcpv4.DHCPv4) {
	if reply == nil {
		return
	}
	if _, err := conn.WriteTo(reply.ToBytes(), peer); err != nil {
		logger.Error("DHCP 发送失败: %v", err)
	}
}

// buildPool 初始化地址池。
func (s *Server) buildPool(dc model.DHCPConfig) {
	s.ipPool = &IPPool{
		start:   net.ParseIP(dc.IPPoolStart).To4(),
		end:     net.ParseIP(dc.IPPoolEnd).To4(),
		mask:    net.ParseIP(dc.SubnetMask).To4(),
		gateway: net.ParseIP(dc.Gateway).To4(),
		lease:   dc.LeaseTime,
	}
	if s.ipPool.lease <= 0 {
		s.ipPool.lease = 86400
	}
}

// allocate 分配一个空闲 IP。
func (s *Server) allocate(mac string) net.IP {
	s.mu.Lock()
	defer s.mu.Unlock()
	if l, ok := s.leases[mac]; ok && time.Now().Before(l.ExpireAt) {
		return l.IP
	}
	for ip := cloneIP(s.ipPool.start); ipLessEqual(ip, s.ipPool.end); incIP(ip) {
		if s.isFreeLocked(ip) {
			return cloneIP(ip)
		}
	}
	return nil
}

func (s *Server) isFreeLocked(ip net.IP) bool {
	for _, l := range s.leases {
		if l.IP.Equal(ip) && time.Now().Before(l.ExpireAt) {
			return false
		}
	}
	return true
}

func (s *Server) recordLease(mac string, ip net.IP, hostname string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.leases[mac] = &lease{
		IP:       cloneIP(ip),
		MAC:      mac,
		Hostname: hostname,
		ExpireAt: time.Now().Add(time.Duration(s.ipPool.lease) * time.Second),
	}
}

func (s *Server) release(mac string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.leases, mac)
}

func (s *Server) poolContains(ip net.IP) bool {
	ip = ip.To4()
	return ipLessEqual(s.ipPool.start, ip) && ipLessEqual(ip, s.ipPool.end)
}

// macString 格式化 MAC。
func macString(hw net.HardwareAddr) string {
	if len(hw) == 0 {
		return "unknown"
	}
	parts := make([]string, len(hw))
	for i, b := range hw {
		parts[i] = fmt.Sprintf("%02x", b)
	}
	return strings.Join(parts, ":")
}

func cloneIP(ip net.IP) net.IP {
	if ip == nil {
		return nil
	}
	out := make(net.IP, len(ip))
	copy(out, ip)
	return out
}

func ipLessEqual(a, b net.IP) bool {
	for i := range 4 {
		if a[i] < b[i] {
			return true
		}
		if a[i] > b[i] {
			return false
		}
	}
	return true
}

func incIP(ip net.IP) {
	for i := len(ip) - 1; i >= 0; i-- {
		ip[i]++
		if ip[i] != 0 {
			break
		}
	}
}
