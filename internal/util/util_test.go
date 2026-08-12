package util

import (
	"archive/zip"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestIsValidIPv4(t *testing.T) {
	cases := []struct {
		in   string
		want bool
	}{
		{"192.168.1.1", true},
		{"0.0.0.0", true},
		{"255.255.255.255", true},
		{"256.1.1.1", false},
		{"abc", false},
		{"", false},
	}
	for _, c := range cases {
		if got := IsValidIPv4(c.in); got != c.want {
			t.Errorf("IsValidIPv4(%q) = %v, want %v", c.in, got, c.want)
		}
	}
}

func TestIsValidMAC(t *testing.T) {
	cases := []struct {
		in   string
		want bool
	}{
		{"00:11:22:33:44:55", true},
		{"00-11-22-33-44-55", true},
		{"AA:BB:CC:DD:EE:FF", true},
		{"00:11:22:33:44", false},
		{"GG:11:22:33:44:55", false},
		{"", false},
	}
	for _, c := range cases {
		if got := IsValidMAC(c.in); got != c.want {
			t.Errorf("IsValidMAC(%q) = %v, want %v", c.in, got, c.want)
		}
	}
}

func TestNormalizeMAC(t *testing.T) {
	if got := NormalizeMAC("00-11-22-33-44-55"); got != "00:11:22:33:44:55" {
		t.Errorf("NormalizeMAC = %q", got)
	}
}

func TestSafeJoinTraversal(t *testing.T) {
	// 使用真实临时目录作为 base，保证跨平台行为一致（避免 Windows 风格硬编码路径）
	base := t.TempDir()
	// 目录穿越应被拒绝
	if _, err := SafeJoin(base, "../../etc/passwd"); err == nil {
		t.Error("expected traversal error, got nil")
	}
	// 正常路径应成功
	p, err := SafeJoin(base, "autoexec.ipxe")
	if err != nil {
		t.Fatalf("SafeJoin err: %v", err)
	}
	if !strings.HasSuffix(filepath.ToSlash(p), "autoexec.ipxe") {
		t.Errorf("unexpected path: %s", p)
	}
}

func TestEncryptDecryptPassword(t *testing.T) {
	key := "test-key"
	plain := "MySecret123"
	cipher := EncryptPassword(plain, key)
	if cipher == plain {
		t.Error("cipher should differ from plain")
	}
	if got := DecryptPassword(cipher, key); got != plain {
		t.Errorf("Decrypt = %q, want %q", got, plain)
	}
}

func TestReplaceAllPlaceholder(t *testing.T) {
	text := "a @@PXE_SERVER@@ b @@PXE_SERVER@@"
	out, n := ReplaceAllPlaceholder(text, "@@PXE_SERVER@@", "1.2.3.4")
	if n != 2 {
		t.Errorf("replace count = %d, want 2", n)
	}
	if out != "a 1.2.3.4 b 1.2.3.4" {
		t.Errorf("out = %q", out)
	}
}

func TestUnzipToDir(t *testing.T) {
	dir := t.TempDir()
	zipPath := filepath.Join(dir, "test.zip")
	// 创建一个 zip
	f, _ := os.Create(zipPath)
	w := zip.NewWriter(f)
	ww, _ := w.Create("dir/a.txt")
	ww.Write([]byte("hello"))
	w.Close()
	f.Close()

	dest := filepath.Join(dir, "out")
	files, err := UnzipToDir(zipPath, dest)
	if err != nil {
		t.Fatalf("UnzipToDir err: %v", err)
	}
	if len(files) != 1 {
		t.Errorf("files = %d, want 1", len(files))
	}
	data, err := os.ReadFile(filepath.Join(dest, "dir", "a.txt"))
	if err != nil || string(data) != "hello" {
		t.Errorf("unexpected content: %s err=%v", data, err)
	}
}

func TestUnzipToDirTraversal(t *testing.T) {
	dir := t.TempDir()
	zipPath := filepath.Join(dir, "evil.zip")
	f, _ := os.Create(zipPath)
	w := zip.NewWriter(f)
	// 恶意路径：../ 穿越
	ww, _ := w.Create("../../evil.txt")
	ww.Write([]byte("x"))
	w.Close()
	f.Close()

	dest := filepath.Join(dir, "out")
	if _, err := UnzipToDir(zipPath, dest); err == nil {
		t.Error("expected traversal rejection")
	}
}
