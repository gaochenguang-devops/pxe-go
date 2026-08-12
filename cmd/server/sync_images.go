package main

import (
	"os"
	"path/filepath"
	"strings"

	"pxe-server/internal/db"
	"pxe-server/internal/logger"
	"pxe-server/internal/model"
)

// syncImageDirectories 扫描 web_root 下的镜像目录，与数据库自动同步。
// 目录结构: web_root/{镜像名}/{x86_64|aarch64}/
// 如果 {架构} 目录存在且包含 repodata/ 子目录，则认为是有效的 YUM repo。
// 如果 {架构} 目录存在且包含 .iso 文件，则记录 ISO 路径。
func syncImageDirectories(webRoot string) {
	if webRoot == "" {
		webRoot = "assets/web_root"
	}

	entries, err := os.ReadDir(webRoot)
	if err != nil {
		logger.Warn("镜像目录扫描失败 (web_root=%s): %v", webRoot, err)
		return
	}

	logger.Info("开始扫描镜像目录: %s", webRoot)
	synced := 0

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		imgName := entry.Name()
		// 跳过系统目录
		if imgName == "uploads" || strings.HasPrefix(imgName, ".") || imgName == "ui" {
			continue
		}

		imgDir := filepath.Join(webRoot, imgName)
		img := &model.OSImage{
			Name: imgName,
		}

		// 扫描 x86_64
		x86Dir := filepath.Join(imgDir, "x86_64")
		if info, err := os.Stat(x86Dir); err == nil && info.IsDir() {
			img.X86RepoPath = "/" + imgName + "/x86_64"
		}

		// 扫描 aarch64
		armDir := filepath.Join(imgDir, "aarch64")
		if info, err := os.Stat(armDir); err == nil && info.IsDir() {
			img.ArmRepoPath = "/" + imgName + "/aarch64"
		}

		// 至少有一个架构目录才入库
		if img.X86RepoPath == "" && img.ArmRepoPath == "" {
			continue
		}

		if err := db.UpsertOSImage(img); err != nil {
			logger.Warn("同步镜像 %s 到数据库失败: %v", imgName, err)
		} else {
			synced++
			logger.Info("镜像已同步: %s (x86=%s arm=%s)", imgName, img.X86RepoPath, img.ArmRepoPath)
		}
	}

	logger.Info("镜像目录扫描完成，共同步 %d 个镜像", synced)
}


