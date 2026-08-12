// 服务启动命令定义：解析全局参数、初始化组件、启动多服务并优雅退出。
package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/spf13/cobra"

	"pxe-server/internal/config"
	"pxe-server/internal/db"
	"pxe-server/internal/logger"
	"pxe-server/internal/service/dhcp"
	"pxe-server/internal/service/httpapi"
	"pxe-server/internal/service/tftp"
)

// 全局命令行参数（通过 Cobra PersistentFlags 注入）。
var (
	dbPath   string
	logPath  string
	logLevel string
)

// newRootCmd 构建 pxe-server 根命令。
// 不传子命令时直接启动服务；-v/--version 显示版本；支持 help 子命令。
func newRootCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "pxe-server",
		Short: "PXE 网络装机服务",
		Long: `PXE 网络装机服务：集成 DHCP、TFTP、HTTP 服务，支持 Kickstart 自动安装、
镜像仓库扫描同步、主机资产与 IPMI 运维、多网段 DHCP 配置等。

用法示例:
  pxe-server                         使用默认参数启动服务
  pxe-server --db data/test.db       指定 SQLite 数据库文件
  pxe-server -v                     显示版本号后退出
  pxe-server help                   查看帮助`,
		Version:      version,
		SilenceUsage: true, // 启动失败时只输出错误，不打印完整用法
		RunE:         runServer,
	}
	// 与旧版 flag.Bool("v") 行为一致：pxe-server -v 输出版本号
	cmd.SetVersionTemplate(`pxe-server {{.Version}}
`)

	pf := cmd.PersistentFlags()
	pf.StringVar(&dbPath, "db", "data/pxe-server.db", "SQLite 数据库文件路径")
	pf.StringVar(&logPath, "log", "logs/pxe-server.log", "日志文件路径（留空则仅控制台）")
	pf.StringVar(&logLevel, "level", "info", "日志级别: debug/info/warn/error")

	return cmd
}

// normalizeArgs 将单横线长参数（-db/-log/-level，旧版 flag 风格）归一化为双横线，
// 兼容 Makefile run 目标与历史命令行；-v/-h 等真实 shorthand 不受影响。
func normalizeArgs(args []string) []string {
	knownLong := map[string]bool{"db": true, "log": true, "level": true}
	out := make([]string, 0, len(args))
	for _, a := range args {
		name := strings.TrimPrefix(a, "-")
		if strings.HasPrefix(a, "--") || len(name) <= 1 || !strings.HasPrefix(a, "-") {
			out = append(out, a)
			continue
		}
		key := name
		if i := strings.IndexByte(name, '='); i > 0 {
			key = name[:i]
		}
		if knownLong[key] {
			out = append(out, "--"+name)
			continue
		}
		out = append(out, a)
	}
	return out
}

// runServer 启动全部服务并阻塞等待退出信号，收到信号后优雅关闭。
func runServer(cmd *cobra.Command, args []string) error {
	// 初始化日志
	lv := logger.LevelInfo
	switch logLevel {
	case "debug":
		lv = logger.LevelDebug
	case "warn":
		lv = logger.LevelWarn
	case "error":
		lv = logger.LevelError
	}
	if err := logger.Init(logPath, 20, 10, lv); err != nil {
		// 日志文件不可用时降级为仅控制台输出，不影响服务启动
		fmt.Fprintf(os.Stderr, "初始化日志文件失败: %v，已降级为仅控制台输出\n", err)
	}

	logger.Info("===== PXE 装机服务启动 (version: %s) =====", version)

	// 初始化数据库
	if err := db.Init(dbPath); err != nil {
		return fmt.Errorf("数据库初始化失败: %w", err)
	}
	defer db.Close()
	logger.Info("SQLite 数据库初始化完成: %s", dbPath)

	// 初始化配置管理器
	cfg, err := config.NewManager()
	if err != nil {
		return fmt.Errorf("配置加载失败: %w", err)
	}
	logger.Info("配置加载完成")

	// 确保默认数据（首次启动导入内置 KS 模板、iPXE 脚本与部署脚本）
	if err := db.EnsureDefaultKSTemplate(); err != nil {
		logger.Warn("默认 KS 模板初始化跳过: %v", err)
	}
	if err := db.EnsureDefaultIPxeScript(); err != nil {
		logger.Warn("默认 iPXE 脚本初始化跳过: %v", err)
	}
	if err := db.EnsureDefaultDeployScript(); err != nil {
		logger.Warn("默认部署脚本初始化跳过: %v", err)
	}

	// 扫描 web_root/repo 下的镜像安装源目录，与数据库自动同步（repo_path 统一为 /repo/{镜像名}/{架构}）
	syncImageDirectories(cfg.HTTP().WebRoot)

	// 启动三大服务
	dhcpSrv := dhcp.NewServer(cfg)
	if err := dhcpSrv.Start(); err != nil {
		return fmt.Errorf("DHCP 服务启动失败: %w", err)
	}

	tftpSrv := tftp.NewServer(cfg)
	if err := tftpSrv.Start(); err != nil {
		return fmt.Errorf("TFTP 服务启动失败: %w", err)
	}

	httpSrv := httpapi.NewServer(cfg)
	if err := httpSrv.Start(); err != nil {
		return fmt.Errorf("HTTP 服务启动失败: %w", err)
	}

	// 配置热加载轮询
	reloadTicker := time.NewTicker(5 * time.Second)
	defer reloadTicker.Stop()
	stopReload := make(chan struct{})
	go func() {
		defer logger.Recover("config-reload")
		for {
			select {
			case <-reloadTicker.C:
				dhcpSrv.Reload()
				tftpSrv.Reload()
			case <-stopReload:
				return
			}
		}
	}()

	// 等待退出信号，优雅关闭
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	sigReceived := <-sig
	logger.Info("收到退出信号 %v，开始优雅关闭...", sigReceived)

	close(stopReload)
	dhcpSrv.Stop()
	tftpSrv.Stop()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	httpSrv.Stop(ctx)

	logger.Info("===== PXE 装机服务已退出 =====")
	return nil
}
