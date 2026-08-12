package httpapi

import (
	"net/http"
	"os"
	"path/filepath"

	"github.com/gin-gonic/gin"

	"pxe-server/internal/service/ipxe"
)

// handleRenderIPxeByImage 根据系统镜像渲染 autoexec.ipxe（供 iPXE 脚本页选择镜像后生成）。
// 参数：image_id（镜像ID，可选）、name/arch（镜像名/架构，可选）。
func (s *Server) handleRenderIPxeByImage(c *gin.Context) {
	imageID := parseInt64(c.Query("image_id"))
	name := c.Query("name")
	arch := c.Query("arch")
	content := ipxe.RenderAutoexecForImage(s.cfg, imageID, name, arch)
	c.Header("Content-Type", "text/plain")
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": content})
}

// autoexecIPxePath 返回 autoexec.ipxe 磁盘路径。
func (s *Server) autoexecIPxePath() string {
	root := s.cfg.TFTP().RootDir
	if root == "" {
		root = "assets/tftp_root"
	}
	return filepath.Join(root, "autoexec.ipxe")
}

// diskKSPath 返回磁盘 ks.cfg 路径。
func (s *Server) diskKSPath() string {
	root := s.cfg.HTTP().WebRoot
	if root == "" {
		root = "assets/web_root"
	}
	return filepath.Join(root, "ks.cfg")
}

// writeFileEnsureDir 确保父目录存在后写入文件。
func writeFileEnsureDir(path string, data []byte) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}
