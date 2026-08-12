// Package httpapi 负责 Gin 实例初始化、路由注册。
package httpapi

import (
	"context"
	"encoding/json"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"pxe-server/internal/config"
	"pxe-server/internal/db"
	"pxe-server/internal/logger"
	"pxe-server/internal/middleware"
	"pxe-server/internal/service/ipmi"
	"pxe-server/internal/util"
	"pxe-server/web"
)

// Server HTTP API 服务。
type Server struct {
	cfg     *config.Manager
	engine  *gin.Engine
	httpSrv *http.Server
	ipmiClt *ipmi.Client
}

// NewServer 创建 HTTP 服务。
func NewServer(cfg *config.Manager) *Server {
	gin.SetMode(gin.ReleaseMode)
	return &Server{
		cfg:     cfg,
		engine:  gin.New(),
		ipmiClt: ipmi.NewClient(cfg),
	}
}

// registerRoutes 注册全部路由。
func (s *Server) registerRoutes() {
	r := s.engine
	r.Use(middleware.CORS(), middleware.RequestLogger(), gin.Recovery())

	// 登录接口（无需鉴权）
	r.POST("/api/login", s.handleLogin)
	r.POST("/api/logout", s.handleLogout)
	r.PUT("/api/password", s.handleChangePassword)

	// 客户端 PXE/KS 接口（无需登录，但需限流）
	pxeGroup := r.Group("/", middleware.RateLimit(300, time.Minute))
	pxeGroup.GET("/install-complete", s.handleInstallComplete)

	// 管理后台（需鉴权）
	api := r.Group("/api", middleware.AuthRequired(), middleware.RateLimit(1200, time.Minute))
	{
		// 配置
		api.GET("/config", s.handleGetConfig)
		api.PUT("/config/dhcp", s.handleUpdateDHCP)
		api.PUT("/config/tftp", s.handleUpdateTFTP)
		api.PUT("/config/http", s.handleUpdateHTTP)

		// 资源管理
		api.GET("/resource", s.handleListResources)

		// 主机管理 Excel（模版下载/导入/导出）
		api.GET("/host/excel/template", s.handleGetHostExcelTemplate)
		api.POST("/host/excel/import", s.handleImportHostExcel)
		api.GET("/host/excel/export", s.handleExportHostExcel)

		// 主机批量管理（删除/IPMI电源/导出选中）
		api.POST("/host/batch/delete", s.handleBatchDeleteHost)
		api.POST("/host/batch/ipmi", s.handleBatchIPMI)
		api.POST("/host/batch/export", s.handleBatchExportHostExcel)

		// 主机资源 / node-info（独立 Excel 模版，bond 网络字段）
		api.GET("/host-resource/list", s.handleListHostResources)
		api.GET("/host-resource/template", s.handleGetNodeInfoTemplate)
		api.POST("/host-resource/import", s.handleImportNodeInfoExcel)
		api.GET("/host-resource/excel/export", s.handleExportNodeInfoExcel)
		api.GET("/host-resource/node-info/export", s.handleExportNodeInfoTxt)
		api.GET("/host-resource/node-info", s.handleGetNodeInfo)
		api.POST("/host-resource/batch/export", s.handleBatchExportNodeInfoExcel)
		api.POST("/host-resource/batch/delete", s.handleBatchDeleteHostResource)

		// 部署脚本
		api.GET("/deploy/script", s.handleListDeployScripts)
		api.POST("/deploy/script", s.handleCreateDeployScript)
		api.PUT("/deploy/script/:id", s.handleUpdateDeployScript)
		api.DELETE("/deploy/script/:id", s.handleDeleteDeployScript)
		api.POST("/deploy/script/:id/active", s.handleSetActiveDeployScript)
		api.GET("/deploy/script/:id/content", s.handleGetDeployScriptContent)

		// 主机资产管理
		api.GET("/host", s.handleListHosts)
		api.POST("/host", s.handleCreateHost)
		api.PUT("/host/:id", s.handleUpdateHost)
		api.DELETE("/host/:id", s.handleDeleteHost)

		// IPMI 运维
		api.POST("/host/:id/ipmi/power", s.handleIPMIPower)
		api.GET("/host/:id/ipmi/status", s.handleIPMIStatus)
		api.POST("/host/:id/ipmi/boot", s.handleIPMIBootDevice)

		// KS 模板
		api.GET("/ks/template", s.handleListKSTemplates)
		api.POST("/ks/template", s.handleCreateKSTemplate)
		api.PUT("/ks/template/:id", s.handleUpdateKSTemplate)
		api.DELETE("/ks/template/:id", s.handleDeleteKSTemplate)
		api.POST("/ks/template/:id/active", s.handleSetActiveKSTemplate)
		api.GET("/ks/template/render", s.handleRenderActiveKS)

		// iPXE 脚本管理（列表/CRUD/激活，同一时间仅一个生效）
		api.GET("/ipxe/script", s.handleListIPxeScripts)
		api.POST("/ipxe/script", s.handleCreateIPxeScript)
		api.PUT("/ipxe/script/:id", s.handleUpdateIPxeScript)
		api.DELETE("/ipxe/script/:id", s.handleDeleteIPxeScript)
		api.POST("/ipxe/script/:id/active", s.handleSetActiveIPxeScript)
		// 按系统镜像渲染 autoexec.ipxe（供 iPXE 脚本页选择镜像生成）
		api.GET("/ipxe/script/render", s.handleRenderIPxeByImage)

		// 仪表盘 / 服务状态
		api.GET("/status", s.handleServiceStatus)

		// 系统镜像
		api.GET("/image", s.handleListImages)
		api.POST("/image/upload", s.handleUploadImage)
		api.DELETE("/image/:id", s.handleDeleteImage)
		api.POST("/image/:id/active", s.handleSetActiveOSImage)
		api.POST("/image/:id/boot-file", s.handleUploadBootFile)

		// 操作审计日志
		api.GET("/operlog", s.handleListOperLogs)
		api.GET("/logfile", s.handleTailLogFile)

		// 装机完成上报记录
		api.GET("/install-records", s.handleListInstallRecords)

		// 文件管理（上传到 web_root/uploads，公开可访问）
		api.POST("/file/upload", s.handleUploadFile)
		api.GET("/file/list", s.handleListUploadedFiles)
		api.DELETE("/file/:name", s.handleDeleteUploadedFile)
	}

	// 静态资源路由：Web 管理后台页面
	s.registerWebUI()

	// 静态资源：系统安装源、部署脚本、占位符替换
	s.registerStaticResource()
}

// registerWebUI 注册后台静态页面。
// 优先使用磁盘 web/ui（便于本地覆盖/调试）；磁盘不存在时回退到嵌入二进制的 UI，
// 支持单文件分发（仅随二进制即可运行管理后台）。
func (s *Server) registerWebUI() {
	// 磁盘 Web UI 目录（相对项目根）
	uiDir := filepath.Join("web", "ui")
	if _, err := os.Stat(filepath.Join(uiDir, "index.html")); err == nil {
		s.registerDiskUI(uiDir)
		return
	}
	// 磁盘不存在 → 使用嵌入二进制的 UI
	if err := s.registerEmbeddedUI(); err != nil {
		logger.Warn("加载嵌入式 Web UI 失败: %v", err)
	}
}

// registerDiskUI 从磁盘目录提供管理后台页面。
func (s *Server) registerDiskUI(uiDir string) {
	logger.Info("Web UI 使用磁盘目录: %s", uiDir)
	s.engine.Static("/ui", uiDir)
	s.engine.GET("/", func(c *gin.Context) {
		index := filepath.Join(uiDir, "index.html")
		if _, err := os.Stat(index); err == nil {
			c.File(index)
			return
		}
		c.String(http.StatusOK, "pxe-server is running")
	})
}

// registerEmbeddedUI 从嵌入二进制的资源提供管理后台页面。
func (s *Server) registerEmbeddedUI() error {
	sub, err := fs.Sub(web.UI, "ui")
	if err != nil {
		return err
	}
	logger.Info("Web UI 使用嵌入资源")
	fileServer := http.FileServer(http.FS(sub))
	// /ui/*filepath 提供嵌入资源（css/js 子目录等）。
	// http.FileServer 从 sub FS root（已设为 ui）查找，故需去掉 URL 中的 "/ui" 前缀。
	s.engine.GET("/ui/*filepath", func(c *gin.Context) {
		fp := strings.TrimPrefix(c.Param("filepath"), "/")
		c.Request.URL.Path = "/" + fp
		fileServer.ServeHTTP(c.Writer, c.Request)
	})
	// 根路径和 /index.html 直接返回内容，避免 http.FileServer 的 301 重定向
	s.engine.GET("/", func(c *gin.Context) {
		data, err := fs.ReadFile(sub, "index.html")
		if err != nil {
			c.String(http.StatusNotFound, "index.html not found")
			return
		}
		c.Data(http.StatusOK, "text/html; charset=utf-8", data)
	})
	s.engine.GET("/index.html", func(c *gin.Context) {
		data, err := fs.ReadFile(sub, "index.html")
		if err != nil {
			c.String(http.StatusNotFound, "index.html not found")
			return
		}
		c.Data(http.StatusOK, "text/html; charset=utf-8", data)
	})
	return nil
}

// registerStaticResource 注册安装源与部署脚本静态路由，支持占位符替换。
func (s *Server) registerStaticResource() {
	httpRoot := s.cfg.HTTP().WebRoot
	if httpRoot == "" {
		httpRoot = "assets/web_root"
	}
	// 部署脚本类（含占位符替换，PXEIP 在请求时动态获取）
	s.engine.GET("/deploy.sh", s.renderScriptWithPlaceholder(httpRoot, "deploy.sh"))
	s.engine.GET("/lldp.sh", s.renderScriptWithPlaceholder(httpRoot, "lldp.sh"))
	s.engine.GET("/node-info.txt", s.renderScriptWithPlaceholder(httpRoot, "node-info.txt"))
	// /ks.cfg 走动态渲染：不仅替换占位符，还注入 %pre/%post 动态脚本。
	// （注意：需在 /ks/:mac/ks.cfg 之后注册，避免通配冲突；客户端无论请求 /ks.cfg 还是 /ks/{mac}/ks.cfg 都能得到完整 ks。）
	s.engine.GET("/ks.cfg", s.handleKickstartGeneric)

	// 安装源目录（repo/{镜像名}/{x86_64|aarch64} 等）
	s.engine.Static("/repo", filepath.Join(httpRoot, "repo"))
	// 通用静态文件服务（防穿越由 util 校验，这里仅服务文件）
	s.engine.GET("/files/*filepath", s.serveStaticFile(httpRoot))
	// 上传目录：web_root/uploads 下的文件公开可访问（无需登录），供所有人下载
	s.engine.GET("/uploads/*filepath", s.serveStaticFile(filepath.Join(httpRoot, uploadDirName)))

	// 根路径兜底静态文件服务：web_root 下的任意文件均可通过 HTTP 拉取
	// （如 /euler2110/...、/vmlinuz、/initrd 等），路径穿越由 util 校验。
	// 仅当文件在 web_root 下真实存在时返回，否则回落到默认 404。
	s.engine.NoRoute(s.serveRootStatic(httpRoot))
}

// serveRootStatic 提供 web_root 根路径下的通用静态文件兜底服务。
func (s *Server) serveRootStatic(root string) gin.HandlerFunc {
	return func(c *gin.Context) {
		// 跳过 API 前缀，避免干扰已定义但报错的路由
		if strings.HasPrefix(c.Request.URL.Path, "/api/") {
			c.String(http.StatusNotFound, "not found")
			return
		}
		rel := strings.TrimPrefix(c.Request.URL.Path, "/")
		if rel == "" {
			c.String(http.StatusNotFound, "not found")
			return
		}
		abs, err := util.SafeJoinWithin(root, rel)
		if err != nil {
			c.String(http.StatusForbidden, "forbidden")
			return
		}
		info, err := os.Stat(abs)
		if err != nil || info.IsDir() {
			c.String(http.StatusNotFound, "not found")
			return
		}
		c.File(abs)
	}
}

// renderScriptWithPlaceholder 读取脚本文件并替换占位符（@@PXE_SERVER@@ / @@PXE_IMAGE_NAME@@）。
func (s *Server) renderScriptWithPlaceholder(root, name string) gin.HandlerFunc {
	return func(c *gin.Context) {
		abs, err := util.SafeJoinWithin(root, name)
		if err != nil {
			c.String(http.StatusForbidden, "forbidden")
			return
		}
		data, err := os.ReadFile(abs)
		if err != nil {
			c.String(http.StatusNotFound, "not found")
			return
		}
		text := string(data)
		pxeIP := s.cfg.DHCP().PXEIP
		if pxeIP != "" {
			text, _ = util.ReplaceAllPlaceholder(text, "@@PXE_SERVER@@", pxeIP)
		}
		// 替换当前生效的镜像名称
		if img, err := db.GetActiveOSImage(); err == nil && img != nil {
			text, _ = util.ReplaceAllPlaceholder(text, "@@PXE_IMAGE_NAME@@", img.Name)
		}
		c.Data(http.StatusOK, "text/plain", []byte(text))
	}
}

// serveStaticFile 提供通用静态文件（带路径穿越防护）。
func (s *Server) serveStaticFile(root string) gin.HandlerFunc {
	return func(c *gin.Context) {
		rel := strings.TrimPrefix(c.Param("filepath"), "/")
		abs, err := util.SafeJoinWithin(root, rel)
		if err != nil {
			c.String(http.StatusForbidden, "forbidden")
			return
		}
		info, err := os.Stat(abs)
		if err != nil || info.IsDir() {
			c.String(http.StatusNotFound, "not found")
			return
		}
		c.File(abs)
	}
}

// Start 启动 HTTP 服务。
func (s *Server) Start() error {
	s.registerRoutes()
	addr := s.cfg.HTTP().ListenAddr
	s.httpSrv = &http.Server{
		Addr:    addr,
		Handler: s.engine,
	}
	logger.Info("HTTP 服务已启动，监听 %s", addr)
	go func() {
		if err := s.httpSrv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error("HTTP 服务错误: %v", err)
		}
	}()
	return nil
}

// Stop 优雅关闭 HTTP 服务。
func (s *Server) Stop(ctx context.Context) {
	if s.httpSrv != nil {
		s.httpSrv.Shutdown(ctx)
		logger.Info("HTTP 服务已停止")
	}
}

// parseJSONBody 解析 JSON 请求体。
func (s *Server) parseJSONBody(c *gin.Context, v any) bool {
	dec := json.NewDecoder(c.Request.Body)
	if err := dec.Decode(v); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": "请求体解析失败: " + err.Error()})
		return false
	}
	return true
}
