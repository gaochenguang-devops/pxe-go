// Package tftp 实现 TFTP 文件服务。
// 底层基于成熟的 github.com/pin/tftp 库（RFC 1350 + RFC 2347/2348/2349，
// 自动处理 blksize/tsize/OACK 等 option 协商），本层负责：
//   - 配置热加载（根目录、监听地址、并发限制）
//   - 路径安全校验（防止目录穿越）
//   - 访问日志与优雅停止
package tftp

import (
	"bytes"
	"io"
	"net"
	"os"
	"strings"
	"time"

	"github.com/pin/tftp"

	"pxe-server/internal/config"
	"pxe-server/internal/logger"
	"pxe-server/internal/model"
	"pxe-server/internal/util"
)

// Server TFTP 服务器（基于 pin/tftp 库）。
type Server struct {
	cfg         *config.Manager
	conn        *net.UDPConn
	tftpSrv     *tftp.Server
	stopCh      chan struct{}
	lastVersion int64
	rootDir     string // 当前生效的根目录
	timeout     time.Duration
}

// NewServer 创建 TFTP 服务器。
func NewServer(cfg *config.Manager) *Server {
	return &Server{
		cfg:    cfg,
		stopCh: make(chan struct{}),
	}
}

// Start 启动 TFTP 服务。
func (s *Server) Start() error {
	tc := s.cfg.TFTP()
	if !tc.Enabled {
		logger.Info("TFTP 服务已禁用，跳过启动")
		return nil
	}
	s.applyConfig(tc)
	if err := s.startConn(tc); err != nil {
		return err
	}
	logger.Info("TFTP 服务已启动，根目录 %s，超时 %s", s.rootDir, s.timeout)
	return nil
}

// Stop 停止 TFTP 服务。
func (s *Server) Stop() {
	s.stopConn()
	logger.Info("TFTP 服务已停止")
}

// startConn 启动底层连接与服务 goroutine。
func (s *Server) startConn(tc model.TFTPConfig) error {
	s.stopCh = make(chan struct{})
	s.tftpSrv = tftp.NewServer(s.readHandler, nil)
	s.tftpSrv.SetTimeout(s.timeout)
	s.tftpSrv.SetRetries(3)
	addr := &net.UDPAddr{IP: net.ParseIP(tc.ListenIP).To4(), Port: 69}
	if addr.IP == nil {
		addr.IP = net.IPv4zero
	}
	conn, err := net.ListenUDP("udp4", addr)
	if err != nil {
		return err
	}
	s.conn = conn
	go func() {
		defer logger.Recover("tftp-serve")
		s.tftpSrv.Serve(conn)
	}()
	return nil
}

// stopConn 停止底层连接。
func (s *Server) stopConn() {
	if s.tftpSrv != nil {
		s.tftpSrv.Shutdown()
		s.tftpSrv = nil
	}
	if s.conn != nil {
		_ = s.conn.Close()
		s.conn = nil
	}
}

// Reload 配置热重载（含启停控制）。
func (s *Server) Reload() {
	tc := s.cfg.TFTP()
	if tc.ConfigVersion == s.lastVersion {
		return
	}
	s.lastVersion = tc.ConfigVersion
	running := s.conn != nil
	if tc.Enabled && !running {
		s.applyConfig(tc)
		if err := s.startConn(tc); err != nil {
			logger.Error("TFTP 重载启动失败: %v", err)
		} else {
			logger.Info("TFTP 已重新启动，根目录 %s", s.rootDir)
		}
	} else if !tc.Enabled && running {
		s.stopConn()
		logger.Info("TFTP 已停用")
	} else if tc.Enabled && running {
		s.applyConfig(tc)
		logger.Info("TFTP 配置热重载完成，根目录 %s", s.rootDir)
	}
}

func (s *Server) applyConfig(tc model.TFTPConfig) {
	s.rootDir = tc.RootDir
	s.timeout = time.Duration(tc.TransferTimeout) * time.Second
	if s.timeout <= 0 {
		s.timeout = 5 * time.Second
	}
}

// readHandler 处理 TFTP 读请求（GET）。
func (s *Server) readHandler(filename string, rf io.ReaderFrom) error {
	start := time.Now()
	root := s.rootDir
	if root == "" {
		root = "assets/tftp_root"
	}

	// 尝试从 rf 获取客户端地址
	clientIP := getRemoteAddr(rf)

	path, err := util.SafeJoinWithin(root, filename)
	if err != nil {
		logger.Warn("TFTP 拒绝非法路径 %q (client=%s): %v", filename, clientIP, err)
		return os.ErrNotExist
	}

	data, err := os.ReadFile(path)
	if err != nil {
		logger.Debug("TFTP 文件不存在: %s (client=%s)", path, clientIP)
		return err
	}

	if isIPxeScript(filename) && bytes.Contains(data, []byte("@@PXE_SERVER@@")) {
		pxeIP := strings.TrimSpace(s.cfg.DHCP().PXEIP)
		if pxeIP != "" {
			data = bytes.ReplaceAll(data, []byte("@@PXE_SERVER@@"), []byte(pxeIP))
		}
	}

	if _, err := rf.ReadFrom(bytes.NewReader(data)); err != nil {
		if isClientTerminate(err) {
			logger.Info("TFTP 请求: %s (client=%s) OK 耗时%s", filename, clientIP, time.Since(start).Round(time.Millisecond))
			return nil
		}
		logger.Warn("TFTP 传输失败 %s (client=%s): %v", filename, clientIP, err)
		return err
	}
	logger.Info("TFTP 请求: %s (client=%s) OK 耗时%s", filename, clientIP, time.Since(start).Round(time.Millisecond))
	return nil
}

// getRemoteAddr 从 io.ReaderFrom 尝试提取客户端 IP。
// pin/tftp 库内部使用 net.UDPConn 作为 ReadFrom 的 writer，可通过类型断言获取远程地址。
func getRemoteAddr(rf io.ReaderFrom) string {
	type remoteAddr interface{ RemoteAddr() net.Addr }
	if ra, ok := rf.(remoteAddr); ok {
		addr := ra.RemoteAddr()
		if addr != nil {
			host, _, _ := net.SplitHostPort(addr.String())
			return host
		}
	}
	return "?"
}

// isClientTerminate 判断是否为客户端主动终止传输的 TFTP ERROR（code=8 Terminate transfer）。
func isClientTerminate(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return strings.Contains(msg, "code=8") || strings.Contains(msg, "Terminate transfer")
}

// isIPxeScript 判断文件名是否为 iPXE 脚本（需要做占位符替换）。
func isIPxeScript(name string) bool {
	n := strings.ToLower(name)
	return strings.HasSuffix(n, ".ipxe") || strings.HasSuffix(n, ".ipxe.cfg") || n == "autoexec.ipxe"
}
