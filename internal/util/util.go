// Package util 提供项目内通用的工具函数：IP/MAC 校验、路径安全校验、压缩包解压、字符串替换等。
package util

import (
	"archive/zip"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// IsValidIPv4 校验 IPv4 地址合法性。
func IsValidIPv4(ip string) bool {
	parsed := net.ParseIP(ip)
	return parsed != nil && parsed.To4() != nil
}

// IsValidMAC 校验 MAC 地址合法性，支持 00:11:22:33:44:55 或 00-11-22-33-44-55 格式。
func IsValidMAC(mac string) bool {
	re := regexp.MustCompile(`^([0-9A-Fa-f]{2}[:-]){5}[0-9A-Fa-f]{2}$`)
	return re.MatchString(mac)
}

// NormalizeMAC 将 MAC 统一转为小写且使用 ':' 分隔的标准格式。
func NormalizeMAC(mac string) string {
	mac = strings.ToLower(strings.TrimSpace(mac))
	mac = strings.ReplaceAll(mac, "-", ":")
	return mac
}

// SafeJoin 在 base 目录内安全拼接子路径，防止目录穿越。
// 返回最终路径；若 target 试图越过 base 目录则返回错误。
func SafeJoin(base, target string) (string, error) {
	baseAbs, err := filepath.Abs(base)
	if err != nil {
		return "", err
	}
	// 清理 target，去除 ../ 等相对路径成分
	clean := filepath.Clean(target)
	joined := filepath.Join(baseAbs, clean)
	joinedAbs, err := filepath.Abs(joined)
	if err != nil {
		return "", err
	}
	rel, err := filepath.Rel(baseAbs, joinedAbs)
	if err != nil {
		return "", err
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", errors.New("path traversal detected")
	}
	return joinedAbs, nil
}

// SafeJoinWithin 确保最终路径位于 base 目录之内（用于读取场景）。
func SafeJoinWithin(base, target string) (string, error) {
	return SafeJoin(base, target)
}

// ReplaceAllPlaceholder 将文本内所有占位符 old 替换为 new，并返回替换次数。
func ReplaceAllPlaceholder(text, old, new string) (string, int) {
	count := strings.Count(text, old)
	return strings.ReplaceAll(text, old, new), count
}

// UnzipToDir 将 zip 压缩包解压到 dest 目录，并做路径穿越防护。
// 返回解压出的文件相对路径列表。
func UnzipToDir(zipPath, dest string) ([]string, error) {
	r, err := zip.OpenReader(zipPath)
	if err != nil {
		return nil, err
	}
	defer r.Close()

	if err := os.MkdirAll(dest, 0755); err != nil {
		return nil, err
	}

	var files []string
	for _, f := range r.File {
		// 防 Zip Slip：拒绝绝对路径与穿越路径
		if strings.Contains(f.Name, "..") || filepath.IsAbs(f.Name) {
			return nil, fmt.Errorf("unsafe zip entry: %s", f.Name)
		}
		targetPath, err := SafeJoin(dest, f.Name)
		if err != nil {
			return nil, fmt.Errorf("unsafe zip entry: %s", f.Name)
		}
		if f.FileInfo().IsDir() {
			if err := os.MkdirAll(targetPath, 0755); err != nil {
				return nil, err
			}
			continue
		}
		if err := os.MkdirAll(filepath.Dir(targetPath), 0755); err != nil {
			return nil, err
		}
		src, err := f.Open()
		if err != nil {
			return nil, err
		}
		dst, err := os.Create(targetPath)
		if err != nil {
			src.Close()
			return nil, err
		}
		if _, err := io.Copy(dst, src); err != nil {
			src.Close()
			dst.Close()
			return nil, err
		}
		src.Close()
		dst.Close()
		files = append(files, f.Name)
	}
	return files, nil
}

// FileSize 获取文件大小（字节）。
func FileSize(path string) (int64, error) {
	info, err := os.Stat(path)
	if err != nil {
		return 0, err
	}
	return info.Size(), nil
}

// EncryptPassword 简单加密（XOR + base64），避免数据库明文裸存。
func EncryptPassword(plain string, key string) string {
	if plain == "" {
		return ""
	}
	b := []byte(plain)
	kb := []byte(key)
	for i := range b {
		b[i] ^= kb[i%len(kb)]
	}
	return b64Encode(b)
}

// DecryptPassword 解密 EncryptPassword 加密后的密文。
func DecryptPassword(cipher string, key string) string {
	if cipher == "" {
		return ""
	}
	b, err := b64Decode(cipher)
	if err != nil {
		return ""
	}
	kb := []byte(key)
	for i := range b {
		b[i] ^= kb[i%len(kb)]
	}
	return string(b)
}
