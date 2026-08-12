package util

import (
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	diskfs "github.com/diskfs/go-diskfs"
	"github.com/diskfs/go-diskfs/filesystem/iso9660"
)

// ExtractISO 将 ISO9660 镜像内容解压到目标目录。
// isoPath: ISO 文件路径；destDir: 解压目标目录。
// 使用 diskfs 库跨平台读取 ISO9660 文件系统。
func ExtractISO(isoPath, destDir string) error {
	if err := os.MkdirAll(destDir, 0o755); err != nil {
		return err
	}
	d, err := diskfs.Open(isoPath, diskfs.WithOpenMode(diskfs.ReadOnly), diskfs.WithSectorSize(2048))
	if err != nil {
		return fmt.Errorf("open iso: %w", err)
	}
	defer d.Close()
	// ISO9660 物理块大小为 2048，直接按 ISO9660 读取（绕过自动检测）
	fsys, err := iso9660.Read(d.Backend, d.Size, 0, 2048)
	if err != nil {
		return fmt.Errorf("read iso filesystem: %w", err)
	}
	// diskfs 的路径以 "." 表示根目录，子路径为相对格式（不以 / 开头）
	return extractISORecursive(fsys, ".", destDir)
}

// extractISORecursive 递归提取 ISO 文件系统中的目录与文件。
func extractISORecursive(fsys fs.FS, isoPath, destDir string) error {
	entries, err := fs.ReadDir(fsys, isoPath)
	if err != nil {
		return fmt.Errorf("readdir %s: %w", isoPath, err)
	}
	for _, e := range entries {
		name := e.Name()
		src := joinISOPath(isoPath, name)
		// 相对路径作为磁盘目标子路径
		rel := src
		if rel == "." {
			rel = ""
		}
		target := filepath.Join(destDir, filepath.FromSlash(rel))

		if e.IsDir() {
			if err := os.MkdirAll(target, 0o755); err != nil {
				return err
			}
			if err := extractISORecursive(fsys, src, destDir); err != nil {
				return err
			}
			continue
		}

		// 普通文件：读取并写出
		f, err := fsys.Open(src)
		if err != nil {
			continue
		}
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			f.Close()
			return err
		}
		out, err := os.Create(target)
		if err != nil {
			f.Close()
			return err
		}
		if _, err := io.Copy(out, f); err != nil {
			out.Close()
			f.Close()
			return err
		}
		out.Close()
		f.Close()
	}
	return nil
}

func joinISOPath(dir, name string) string {
	if dir == "." || dir == "" {
		return name
	}
	return strings.TrimSuffix(dir, "/") + "/" + name
}
