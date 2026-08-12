package ksrender

import (
	"strings"
	"testing"
)

func TestNormRepoPath(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		{"euler1", "/euler1"},
		{"/euler1", "/euler1"},
		{"euler1/", "/euler1"},
		{"/euler1/", "/euler1"},
		{"", ""},
		{"/", ""},
	}
	for _, c := range cases {
		if got := normRepoPath(c.in); got != c.want {
			t.Errorf("normRepoPath(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestFindSectionStart(t *testing.T) {
	content := "# header\n%pre\nfoo\n%end\n"
	if idx := findSectionStart(content, "%pre"); idx != 9 {
		t.Errorf("findSectionStart %%%%pre = %d, want 9", idx)
	}
	if idx := findSectionStart(content, "%end"); idx != 18 {
		t.Errorf("findSectionStart %%%%end = %d, want 18", idx)
	}
	// 不存在返回 -1
	if idx := findSectionStart(content, "%post"); idx != -1 {
		t.Errorf("findSectionStart %%%%post = %d, want -1", idx)
	}
	// 出现在行中间（如注释）不应匹配
	if idx := findSectionStart("x%pre y\n", "%pre"); idx != -1 {
		t.Errorf("findSectionStart should only match whole-line marker, got %d", idx)
	}
}

func TestAfterLineEnd(t *testing.T) {
	// "abc\ndef" 中 idx=2 所在行行尾（换行后）为 4
	if got := afterLineEnd("abc\ndef", 0); got != 4 {
		t.Errorf("afterLineEnd(0) = %d, want 4", got)
	}
	// idx 之后无换行返回 -1
	if got := afterLineEnd("abc", 1); got != -1 {
		t.Errorf("afterLineEnd no newline = %d, want -1", got)
	}
}

func TestStripSection(t *testing.T) {
	content := "header\n%pre\nold pre\n%end\n%post\nold post\n%end\nfooter\n"
	out := stripSection(content, "%pre")
	out = stripSection(out, "%post")
	if strings.Contains(out, "old pre") {
		t.Error("pre section not stripped")
	}
	if strings.Contains(out, "old post") {
		t.Error("post section not stripped")
	}
	if !strings.Contains(out, "header") || !strings.Contains(out, "footer") {
		t.Error("header/footer should be preserved")
	}
}

func TestStripSectionMissingEnd(t *testing.T) {
	// 有 %pre 但无 %end：仅移除 %pre 行本身
	content := "header\n%pre\nstray\n"
	out := stripSection(content, "%pre")
	if strings.Contains(out, "%pre") {
		t.Errorf("pre marker should be removed:\n%s", out)
	}
	if !strings.Contains(out, "header") {
		t.Error("header should be preserved")
	}
}

func TestInjectPre_NoMarker(t *testing.T) {
	// 模板无 %pre 标记 → 追加完整段
	out := injectPre("graphical\n", "SCRIPT")
	if !strings.Contains(out, "%pre\nSCRIPT") {
		t.Error("pre section should be appended:\n" + out)
	}
	if !strings.Contains(out, "%end") {
		t.Error("pre section missing end marker")
	}
}

func TestInjectPost_NoMarker(t *testing.T) {
	out := injectPost("graphical\n", "SCRIPT")
	if !strings.Contains(out, "%post\nSCRIPT") {
		t.Error("post section should be appended:\n" + out)
	}
	if !strings.Contains(out, "%end") {
		t.Error("post section missing end marker")
	}
}

func TestLoadDiskKSTemplateNilCfg(t *testing.T) {
	if _, ok := loadDiskKSTemplate(nil); ok {
		t.Error("nil cfg should return ok=false")
	}
}

func TestBuildPreScript_RepoPrefix(t *testing.T) {
	// 显式 imageName → URL 带 /repo 前缀
	s := buildPreScript("euler1")
	if !strings.Contains(s, "url --url=http://@@PXE_SERVER@@/repo/euler1/${REPO_ARCH}/") {
		t.Errorf("pre script missing /repo URL prefix:\n%s", s)
	}
}

func TestBuildPostScript_EmptyMAC(t *testing.T) {
	// 空 MAC 时不注入 node-mac 行
	s := buildPostScript("")
	if strings.Contains(s, "node-mac") {
		t.Error("empty MAC should not produce node-mac line")
	}
}
