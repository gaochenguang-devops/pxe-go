package ksrender

import (
	"strings"
	"testing"
)

func TestInjectPre(t *testing.T) {
	content := "%pre\nexisting line\n%end"
	script := "# injected"
	out := injectPre(content, script)
	if !strings.Contains(out, "# injected") {
		t.Error("pre script not injected")
	}
	// 应插入在 %pre 行之后
	if !strings.Contains(out, "%pre\n# injected\nexisting line") {
		t.Errorf("injection position wrong:\n%s", out)
	}
}

func TestInjectPost(t *testing.T) {
	content := "%post\nlog\n%end"
	script := "# post-injected"
	out := injectPost(content, script)
	if !strings.Contains(out, "# post-injected") {
		t.Error("post script not injected")
	}
}

// 验证注释行包含 %pre/%post 字样时不会干扰注入位置。
func TestInjectIgnoresCommentMention(t *testing.T) {
	content := "# %pre / %post 动态内容由服务端注入。\n" +
		"graphical\n" +
		"%pre\n" +
		"%end\n" +
		"%packages\n" +
		"@core\n" +
		"%end\n" +
		"%post\n" +
		"%end\n"
	preScript := "\n# ===== PRE SCRIPT =====\n"
	postScript := "\n# ===== POST SCRIPT =====\n"

	out := injectPre(content, preScript)
	out = injectPost(out, postScript)

	// 注释应保持在文件顶部，脚本不得插入到注释行附近
	if strings.Index(out, "PRE SCRIPT") < strings.Index(out, "%pre\n") {
		t.Errorf("pre script injected before the pre section:\n%s", out)
	}
	if strings.Index(out, "POST SCRIPT") < strings.Index(out, "%post\n") {
		t.Errorf("post script injected before the post section:\n%s", out)
	}
	// 注释行本身应保持原样
	if !strings.Contains(out, "# %pre / %post 动态内容由服务端注入。") {
		t.Error("comment line was modified")
	}
}

func TestBuildPreScriptHasLVM(t *testing.T) {
	s := buildPreScript("")
	if !strings.Contains(s, "volgroup bel") {
		t.Error("pre script missing LVM volgroup")
	}
	if !strings.Contains(s, "logvol / --fstype") {
		t.Error("pre script missing LVM logvol")
	}
	if !strings.Contains(s, "uname -m") {
		t.Error("pre script missing arch detection")
	}
	if !strings.Contains(s, "/tmp/partinfo") {
		t.Error("pre script missing partinfo generation")
	}
	if !strings.Contains(s, "/tmp/arch-repo") {
		t.Error("pre script missing arch-repo generation")
	}
}

func TestBuildPostScriptPullsScripts(t *testing.T) {
	s := buildPostScript("00:11:22:33:44:55")
	if !strings.Contains(s, "deploy.sh") || !strings.Contains(s, "lldp.sh") {
		t.Error("post script missing deploy scripts")
	}
	if !strings.Contains(s, "node-mac") {
		t.Error("post script missing node-mac")
	}
}
