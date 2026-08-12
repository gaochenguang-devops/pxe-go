package httpapi

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"pxe-server/internal/db"
	"pxe-server/internal/logger"
	"pxe-server/internal/model"
	"pxe-server/internal/util"
)

// handleUploadImage 上传系统镜像 ISO（支持一次上传两个架构：x86_64 + aarch64）。
// 表单字段：name（镜像名，如 euler1）、x86_iso（x86_64 ISO，可选）、arm_iso（aarch64 ISO，可选）。
// 可一次上传两个 ISO，也可只传一个（后补第二个）。分别解压到 web_root/repo/{name}/{arch} 并落库。
func (s *Server) handleUploadImage(c *gin.Context) {
	mr, err := c.Request.MultipartReader()
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": "解析 multipart 失败: " + err.Error()})
		return
	}

	webRoot := s.cfg.HTTP().WebRoot
	if webRoot == "" {
		webRoot = "assets/web_root"
	}
	// 安装源统一放在 web_root/repo 下，与 uploads、deploy.sh 等运行时文件隔离
	repoRoot := filepath.Join(webRoot, "repo")
	isoDir := filepath.Join(repoRoot, "isos")
	if err := os.MkdirAll(isoDir, 0o755); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "msg": "创建目录失败: " + err.Error()})
		return
	}

	// 先收集 name 与两个文件 part 的临时文件路径（流式写入，避免二次复制）
	var name string
	tmpFiles := map[string]string{} // arch -> 临时文件路径
	for {
		part, err := mr.NextPart()
		if err == io.EOF {
			break
		}
		if err != nil {
			cleanupTmp(tmpFiles)
			c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": "读取上传数据失败: " + err.Error()})
			return
		}
		arch := ""
		switch part.FormName() {
		case "name":
			data, _ := io.ReadAll(io.LimitReader(part, 1024))
			name = strings.TrimSpace(string(data))
		case "x86_iso":
			arch = "x86_64"
		case "arm_iso":
			arch = "aarch64"
		}
		if arch != "" && part.FormName() != "name" {
			// 流式写入临时文件
			if tmpFiles[arch] == "" {
				tmp, err := os.CreateTemp(isoDir, ".upload-*.tmp")
				if err != nil {
					cleanupTmp(tmpFiles)
					c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "msg": "创建临时文件失败: " + err.Error()})
					return
				}
				tmpFiles[arch] = tmp.Name()
				if _, err := io.Copy(tmp, part); err != nil {
					tmp.Close()
					os.Remove(tmp.Name())
					cleanupTmp(tmpFiles)
					c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "msg": "写入 ISO 失败: " + err.Error()})
					return
				}
				tmp.Close()
			} else {
				io.Copy(io.Discard, part)
			}
		}
		part.Close()
	}

	// 校验
	if name == "" || !safeImageName(name) {
		cleanupTmp(tmpFiles)
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": "镜像名称不能为空或含非法字符"})
		return
	}
	if len(tmpFiles) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": "请至少选择一个架构的 ISO 文件"})
		return
	}

	// 分别处理各架构：保存 ISO、解压、更新对应架构字段
	var uploaded []string
	var imgID int64
	img, err := findImageByName(name)
	if err != nil || img.ID == 0 {
		img = &model.OSImage{Name: name}
		imgID, _ = db.CreateOSImage(img)
		img.ID = imgID
	} else {
		imgID = img.ID
	}
	for arch, tmpPath := range tmpFiles {
		isoPath := filepath.Join(isoDir, name+"-"+arch+".iso")
		if err := os.Rename(tmpPath, isoPath); err != nil {
			os.Remove(tmpPath)
			cleanupTmp(tmpFiles)
			c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "msg": "保存 ISO 失败: " + err.Error()})
			return
		}
		destDir := filepath.Join(repoRoot, name, arch)
		if err := os.RemoveAll(destDir); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "msg": "清理旧目录失败: " + err.Error()})
			return
		}
		if err := util.ExtractISO(isoPath, destDir); err != nil {
			logger.Error("解压 ISO 失败 %s/%s: %v", name, arch, err)
			c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "msg": "解压 ISO 失败(" + arch + "): " + err.Error()})
			return
		}
		// 解压完成后删除 ISO 文件，不保留
		os.Remove(isoPath)
		// 更新对应架构字段
		repoPath := fmt.Sprintf("/repo/%s/%s", name, arch)
		if arch == "aarch64" {
			img.ArmRepoPath = repoPath
			img.ArmIsoPath = "" // ISO 已删除，不再记录路径
		} else {
			img.X86RepoPath = repoPath
			img.X86IsoPath = ""
		}
		uploaded = append(uploaded, arch)
	}
	_ = db.UpdateOSImage(imgID, img)
	// 若当前无默认生效镜像，自动将该镜像设为默认安装镜像
	if _, err := db.GetActiveOSImage(); err != nil && imgID > 0 {
		_, _ = db.SetActiveOSImage(imgID)
	}

	s.writeLog(c, "image_upload", "上传并解压镜像: "+name+" 架构["+strings.Join(uploaded, ",")+"]")
	c.JSON(http.StatusOK, gin.H{"code": 0, "msg": "上传并解压成功: " + name + " (" + strings.Join(uploaded, ",") + ")"})
}

// cleanupTmp 清理已写入的临时文件。
func cleanupTmp(tmpFiles map[string]string) {
	for _, p := range tmpFiles {
		if p != "" {
			os.Remove(p)
		}
	}
}

// findImageByName 按名称查找镜像（一个名称一条记录）。
func findImageByName(name string) (*model.OSImage, error) {
	list, err := db.ListOSImages()
	if err != nil {
		return nil, err
	}
	for _, im := range list {
		if im.Name == name {
			return im, nil
		}
	}
	return &model.OSImage{}, nil
}

// handleSetActiveOSImage 设置默认生效镜像（同一时间仅一个生效）。
func (s *Server) handleSetActiveOSImage(c *gin.Context) {
	id := parseInt64(c.Param("id"))
	if id <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": "无效ID"})
		return
	}
	if _, err := db.SetActiveOSImage(id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "msg": err.Error()})
		return
	}
	s.writeLog(c, "image_active", "设置默认安装镜像")
	c.JSON(http.StatusOK, gin.H{"code": 0, "msg": "已设为默认安装镜像"})
}

// handleUploadBootFile 更新指定镜像+架构的 initrd.img 或 vmlinuz。
// POST /api/image/:id/boot-file
// 表单字段：arch（x86_64 / aarch64）、type（initrd / vmlinuz）、file（上传文件）。
func (s *Server) handleUploadBootFile(c *gin.Context) {
	id := parseInt64(c.Param("id"))
	if id <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": "无效ID"})
		return
	}

	img, err := db.GetOSImage(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"code": 404, "msg": "镜像不存在"})
		return
	}

	arch := strings.TrimSpace(c.PostForm("arch"))
	fileType := strings.TrimSpace(c.PostForm("type"))
	if arch != "x86_64" && arch != "aarch64" {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": "架构参数无效，必须为 x86_64 或 aarch64"})
		return
	}
	if fileType != "initrd" && fileType != "vmlinuz" {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": "文件类型无效，必须为 initrd 或 vmlinuz"})
		return
	}

	// 检查该架构是否已上传 ISO
	repoPath := ""
	if arch == "x86_64" {
		repoPath = img.X86RepoPath
	} else {
		repoPath = img.ArmRepoPath
	}
	if repoPath == "" {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": "该架构尚未上传 ISO，请先上传 ISO 镜像"})
		return
	}

	// 接收上传文件
	file, header, err := c.Request.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": "读取上传文件失败: " + err.Error()})
		return
	}
	defer file.Close()

	webRoot := s.cfg.HTTP().WebRoot
	if webRoot == "" {
		webRoot = "assets/web_root"
	}

	// 目标路径: {webRoot}/repo/{name}/{arch}/images/pxeboot/{initrd.img|vmlinuz}
	targetDir := filepath.Join(webRoot, "repo", img.Name, arch, "images", "pxeboot")
	targetFile := filepath.Join(targetDir, fileType+".img")
	if fileType == "vmlinuz" {
		targetFile = filepath.Join(targetDir, "vmlinuz")
	}

	// 确保目标目录存在
	if err := os.MkdirAll(targetDir, 0o755); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "msg": "创建目录失败: " + err.Error()})
		return
	}

	// 备份旧文件
	if _, err := os.Stat(targetFile); err == nil {
		backupName := targetFile + ".bak." + time.Now().Format("20060102_150405")
		if err := os.Rename(targetFile, backupName); err != nil {
			logger.Warn("备份旧文件失败 %s: %v", targetFile, err)
		} else {
			logger.Info("已备份 %s -> %s", targetFile, backupName)
		}
	}

	// 写入新文件
	dst, err := os.Create(targetFile)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "msg": "创建文件失败: " + err.Error()})
		return
	}
	defer dst.Close()

	written, err := io.Copy(dst, file)
	if err != nil {
		os.Remove(targetFile)
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "msg": "写入文件失败: " + err.Error()})
		return
	}

	// 记录操作日志
	s.writeLog(c, "boot_file_update",
		fmt.Sprintf("更新镜像 %s (%s) 的 %s 文件 (%.2f MB)", img.Name, arch, header.Filename, float64(written)/1024/1024))
	c.JSON(http.StatusOK, gin.H{"code": 0, "msg": fmt.Sprintf("%s (%s) 已更新", header.Filename, arch)})
}

// safeImageName 校验镜像名/架构仅含字母数字下划线。
func safeImageName(s string) bool {
	for _, r := range s {
		if !(r >= 'a' && r <= 'z') && !(r >= 'A' && r <= 'Z') && !(r >= '0' && r <= '9') && r != '_' && r != '-' {
			return false
		}
	}
	return true
}

// handleListImages 镜像列表。
func (s *Server) handleListImages(c *gin.Context) {
	list, err := db.ListOSImages()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "msg": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": list})
}

// handleDeleteImage 删除镜像。
func (s *Server) handleDeleteImage(c *gin.Context) {
	id := parseInt64(c.Param("id"))
	if id <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": "无效ID"})
		return
	}
	if err := db.DeleteOSImage(id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "msg": err.Error()})
		return
	}
	s.writeLog(c, "image_delete", "删除镜像 ID="+int64Str(id))
	c.JSON(http.StatusOK, gin.H{"code": 0, "msg": "已删除"})
}
