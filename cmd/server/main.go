// 入口：初始化组件、启动多服务 Goroutine、信号捕获优雅退出。
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"pxe-server/internal/config"
	"pxe-server/internal/db"
	"pxe-server/internal/logger"
	"pxe-server/internal/service/dhcp"
	"pxe-server/internal/service/httpapi"
	"pxe-server/internal/service/tftp"
)

// version 通过 -ldflags "-X main.version=<tag>" 注入，未指定时默认为 dev。
var version = "dev"

var (
	dbPath    = flag.String("db", "data/pxe-server.db", "SQLite 数据库文件路径")
	logPath   = flag.String("log", "logs/pxe-server.log", "日志文件路径（留空则仅控制台）")
	logLevel  = flag.String("level", "info", "日志级别: debug/info/warn/error")
	showVer   = flag.Bool("v", false, "显示版本号后退出")
)

func main() {
	flag.Parse()

	// -v 显示版本并退出
	if *showVer {
		fmt.Printf("pxe-server %s\n", version)
		os.Exit(0)
	}

	// 初始化日志
	lv := logger.LevelInfo
	switch *logLevel {
	case "debug":
		lv = logger.LevelDebug
	case "warn":
		lv = logger.LevelWarn
	case "error":
		lv = logger.LevelError
	}
	logger.Init(*logPath, 20, 10, lv)

	logger.Info("===== PXE 装机服务启动 (version: %s) =====", version)

	// 初始化数据库
	if err := db.Init(*dbPath); err != nil {
		logger.Error("数据库初始化失败: %v", err)
		os.Exit(1)
	}
	defer db.Close()
	logger.Info("SQLite 数据库初始化完成: %s", *dbPath)

	// 初始化配置管理器
	cfg, err := config.NewManager()
	if err != nil {
		logger.Error("配置加载失败: %v", err)
		os.Exit(1)
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
		logger.Error("DHCP 服务启动失败: %v", err)
		os.Exit(1)
	}

	tftpSrv := tftp.NewServer(cfg)
	if err := tftpSrv.Start(); err != nil {
		logger.Error("TFTP 服务启动失败: %v", err)
		os.Exit(1)
	}

	httpSrv := httpapi.NewServer(cfg)
	if err := httpSrv.Start(); err != nil {
		logger.Error("HTTP 服务启动失败: %v", err)
		os.Exit(1)
	}

	// 配置热加载轮询
	reloadTicker := time.NewTicker(5 * time.Second)
	defer reloadTicker.Stop()
	stopReload := make(chan struct{})
	go func() {
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
}
