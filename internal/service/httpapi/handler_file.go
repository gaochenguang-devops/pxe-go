package httpapi

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/gin-gonic/gin"

	"pxe-server/internal/logger"
)

// uploadDirName 固定上传目录名（相对 web_root，公开可访问）。
const uploadDirName = "uploads"

// uploadsDir 返回上传目录绝对路径（web_root/uploads），不存在则创建。
func (s *Server) uploadsDir() string {
	root := s.cfg.HTTP().WebRoot
	if root == "" {
		root = "assets/web_root"
	}
	dir := filepath.Join(root, uploadDirName)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		logger.Warn("创建上传目录失败: %v", err)
	}
	return dir
}

// fileItem 文件列表项。
type fileItem struct {
	Name    string `json:"name"`
	Size    int64  `json:"size"`
	ModTime string `json:"mod_time"`
	URL     string `json:"url"`
}

// uploadedResult 单个文件上传结果。
type uploadedResult struct {
	Name  string `json:"name"`
	URL   string `json:"url"`
	Error string `json:"error,omitempty"`
}

// handleUploadFile 支持多文件同时上传到 web_root/uploads，公开可访问。
func (s *Server) handleUploadFile(c *gin.Context) {
	form, err := c.MultipartForm()
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": "解析上传表单失败: " + err.Error()})
		return
	}
	files := form.File["files"]
	if len(files) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": "请选择要上传的文件"})
		return
	}

	dir := s.uploadsDir()
	var results []uploadedResult
	successCount := 0

	for _, fh := range files {
		name := filepath.Base(fh.Filename)
		if name == "" || name == "." || name == ".." {
			results = append(results, uploadedResult{Name: fh.Filename, Error: "文件名不合法"})
			continue
		}
		dst := filepath.Join(dir, name)
		if err := c.SaveUploadedFile(fh, dst); err != nil {
			logger.Warn("保存文件失败 %s: %v", name, err)
			results = append(results, uploadedResult{Name: name, Error: "保存失败: " + err.Error()})
			continue
		}
		url := "/" + uploadDirName + "/" + name
		results = append(results, uploadedResult{Name: name, URL: url})
		successCount++
	}

	msg := fmt.Sprintf("上传完成：成功 %d/%d", successCount, len(files))
	s.writeLog(c, "file_upload", fmt.Sprintf("批量上传文件 %d 个，成功 %d", len(files), successCount))
	c.JSON(http.StatusOK, gin.H{"code": 0, "msg": msg, "data": gin.H{"results": results, "success": successCount, "total": len(files)}})
}

// handleListUploadedFiles 列出上传目录内所有文件（不含子目录），并给出公开访问 URL。
func (s *Server) handleListUploadedFiles(c *gin.Context) {
	dir := s.uploadsDir()
	entries, err := os.ReadDir(dir)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 0, "data": []fileItem{}})
		return
	}
	items := make([]fileItem, 0, len(entries))
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		info, err := e.Info()
		if err != nil {
			logger.Warn("读取上传文件信息失败 %s: %v", e.Name(), err)
			continue
		}
		items = append(items, fileItem{
			Name:    e.Name(),
			Size:    info.Size(),
			ModTime: info.ModTime().Format(time.RFC3339),
			URL:     "/" + uploadDirName + "/" + e.Name(),
		})
	}
	sort.Slice(items, func(i, j int) bool { return items[i].Name < items[j].Name })
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": items})
}

// handleDeleteUploadedFile 删除上传目录中的指定文件。
func (s *Server) handleDeleteUploadedFile(c *gin.Context) {
	name := filepath.Base(c.Param("name"))
	if name == "" || name == "." || name == ".." {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": "文件名不合法"})
		return
	}
	path := filepath.Join(s.uploadsDir(), name)
	if err := os.Remove(path); err != nil {
		if os.IsNotExist(err) {
			c.JSON(http.StatusNotFound, gin.H{"code": 404, "msg": "文件不存在"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "msg": "删除失败: " + err.Error()})
		return
	}
	s.writeLog(c, "file_delete", "删除文件: "+name)
	c.JSON(http.StatusOK, gin.H{"code": 0, "msg": "已删除"})
}
