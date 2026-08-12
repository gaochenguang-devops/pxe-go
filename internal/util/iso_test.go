package util

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/diskfs/go-diskfs/backend/file"
	"github.com/diskfs/go-diskfs/disk"
	"github.com/diskfs/go-diskfs/filesystem"
	"github.com/diskfs/go-diskfs/filesystem/iso9660"
)

// createTestISO 用 go-diskfs 动态生成一个最小的 ISO9660 镜像（内容仅含 hello.txt），
// 返回临时 ISO 文件路径。自包含、可移植，不依赖任何本地固定路径或外部 fixture。
func createTestISO(t *testing.T) string {
	t.Helper()
	isoPath := filepath.Join(t.TempDir(), "test.iso")

	// 创建一个空的原始文件并预留足够空间（64 个 2048 扇区）
	raw, err := os.Create(isoPath)
	if err != nil {
		t.Fatalf("create iso file: %v", err)
	}
	if err := raw.Truncate(64 * 2048); err != nil {
		t.Fatalf("truncate iso file: %v", err)
	}
	be := file.New(raw, false)
	// ISO9660 要求 block size 为 2048 的倍数，手动构造 Disk 指定 LogicalBlocksize
	dd := &disk.Disk{
		Backend:           be,
		Size:              64 * 2048,
		LogicalBlocksize:  2048,
		PhysicalBlocksize: 2048,
	}
	fsys, err := dd.CreateFilesystem(disk.FilesystemSpec{
		Partition: 0,
		FSType:    filesystem.TypeISO9660,
	})
	if err != nil {
		t.Fatalf("create iso filesystem: %v", err)
	}

	content := []byte("hello world!")
	f, err := fsys.OpenFile("/hello.txt", os.O_CREATE|os.O_WRONLY)
	if err != nil {
		t.Fatalf("open hello.txt: %v", err)
	}
	if _, err := f.Write(content); err != nil {
		t.Fatalf("write hello.txt: %v", err)
	}
	if err := f.Close(); err != nil {
		t.Fatalf("close hello.txt: %v", err)
	}
	// ISO9660 需调用 Finalize 把临时目录内容真正写入磁盘
	if iso, ok := fsys.(*iso9660.FileSystem); ok {
		if err := iso.Finalize(iso9660.FinalizeOptions{}); err != nil {
			t.Fatalf("finalize iso: %v", err)
		}
	}
	if err := raw.Close(); err != nil {
		t.Fatalf("close iso file: %v", err)
	}
	return isoPath
}

func TestExtractISO(t *testing.T) {
	isoPath := createTestISO(t)

	dest := t.TempDir()
	if err := ExtractISO(isoPath, dest); err != nil {
		t.Fatalf("ExtractISO err: %v", err)
	}

	// 验证解压出了 hello.txt 且内容一致
	data, err := os.ReadFile(filepath.Join(dest, "hello.txt"))
	if err != nil {
		t.Fatalf("read extracted hello.txt: %v", err)
	}
	if string(data) != "hello world!" {
		t.Errorf("extracted content = %q, want %q", data, "hello world!")
	}
}

func TestExtractISO_InvalidPath(t *testing.T) {
	// 不存在的 ISO 应返回错误，而非 panic
	if err := ExtractISO(filepath.Join(t.TempDir(), "missing.iso"), t.TempDir()); err == nil {
		t.Error("expected error for missing iso")
	}
}
