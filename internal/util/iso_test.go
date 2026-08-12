package util

import (
	"os"
	"path/filepath"
	"testing"
)

func TestExtractISO(t *testing.T) {
	// 使用 diskfs 官方测试 ISO fixture（有效 ISO9660）
	isoPath := filepath.Join("C:/Users/morning/go/pkg/mod/github.com/diskfs/go-diskfs@v1.9.4/filesystem/iso9660/testdata/9660.iso")
	if _, err := os.Stat(isoPath); err != nil {
		t.Skipf("iso fixture not found: %v", err)
	}
	dest := t.TempDir()
	if err := ExtractISO(isoPath, dest); err != nil {
		t.Fatalf("ExtractISO err: %v", err)
	}
	// 验证解压出了文件（9660.iso 含 /FOO 等）
	entries, err := os.ReadDir(dest)
	if err != nil {
		t.Fatalf("read dest: %v", err)
	}
	t.Logf("解压出的根目录项: %d 个", len(entries))
	for _, e := range entries {
		t.Logf("  - %s", e.Name())
	}
	if len(entries) == 0 {
		t.Error("解压结果为空")
	}
}
