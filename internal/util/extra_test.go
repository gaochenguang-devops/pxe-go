package util

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestSafeJoinWithin(t *testing.T) {
	base := t.TempDir()
	// 正常子路径
	p, err := SafeJoinWithin(base, "sub/file.txt")
	if err != nil {
		t.Fatalf("SafeJoinWithin err: %v", err)
	}
	if !strings.HasSuffix(filepath.ToSlash(p), "sub/file.txt") {
		t.Errorf("unexpected path: %s", p)
	}
	// 目录穿越应报错
	if _, err := SafeJoinWithin(base, "../../etc/passwd"); err == nil {
		t.Error("expected traversal error")
	}
	// 绝对路径被当作相对路径拼接在 base 内，不应穿越（SafeJoin 保证不逃逸 base）
	pAbs, err := SafeJoinWithin(base, "/sub/abs.txt")
	if err != nil {
		t.Fatalf("SafeJoinWithin absolute err: %v", err)
	}
	if !strings.HasSuffix(filepath.ToSlash(pAbs), "sub/abs.txt") {
		t.Errorf("absolute path should be joined under base, got %s", pAbs)
	}
}

func TestJoinISOPath(t *testing.T) {
	cases := []struct {
		dir, name, want string
	}{
		{".", "file.txt", "file.txt"},
		{"", "file.txt", "file.txt"},
		{"dir", "file.txt", "dir/file.txt"},
		{"dir/sub", "f", "dir/sub/f"},
		{"dir/", "f", "dir/f"},
	}
	for _, c := range cases {
		if got := joinISOPath(c.dir, c.name); got != c.want {
			t.Errorf("joinISOPath(%q,%q) = %q, want %q", c.dir, c.name, got, c.want)
		}
	}
}

func TestBase64RoundTrip(t *testing.T) {
	in := []byte("hello pxe-server 中文")
	enc := b64Encode(in)
	if enc == "" {
		t.Fatal("b64Encode returned empty")
	}
	dec, err := b64Decode(enc)
	if err != nil {
		t.Fatalf("b64Decode err: %v", err)
	}
	if string(dec) != string(in) {
		t.Errorf("round trip mismatch: %q != %q", dec, in)
	}
}

func TestB64DecodeInvalid(t *testing.T) {
	// 非法的 base64 字符串应返回错误
	if _, err := b64Decode("!!!not-base64!!!"); err == nil {
		t.Error("expected decode error for invalid input")
	}
}

func TestEncryptPasswordEdgeCases(t *testing.T) {
	// 空明文返回空
	if got := EncryptPassword("", "key"); got != "" {
		t.Errorf("EncryptPassword empty = %q, want empty", got)
	}
	// 非空密钥可往返
	plain := "secret"
	key := "my-key"
	if got := DecryptPassword(EncryptPassword(plain, key), key); got != plain {
		t.Errorf("round trip = %q, want %q", got, plain)
	}
}

func TestDecryptPasswordInvalid(t *testing.T) {
	// 非法密文返回空
	if got := DecryptPassword("!!!invalid!!!", "key"); got != "" {
		t.Errorf("DecryptPassword invalid = %q, want empty", got)
	}
	// 空密文返回空
	if got := DecryptPassword("", "key"); got != "" {
		t.Errorf("DecryptPassword empty = %q, want empty", got)
	}
}

func TestNormalizeMACLowercase(t *testing.T) {
	// 大写与连字符混合 → 小写冒号分隔
	if got := NormalizeMAC("AA-BB-CC-DD-EE-FF"); got != "aa:bb:cc:dd:ee:ff" {
		t.Errorf("NormalizeMAC = %q", got)
	}
}
