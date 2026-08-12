package ipxe

import (
	"strings"
	"testing"

	"pxe-server/internal/db"
)

func TestRenderAutoexecForImage_ExplicitName(t *testing.T) {
	out := RenderAutoexecForImage(nil, 0, "euler1", "x86_64")
	if !strings.Contains(out, "#!ipxe") {
		t.Error("missing #!ipxe header")
	}
	// 镜像安装源位于 /repo/{name} 下
	if !strings.Contains(out, "kernel ${boot_root}/repo/euler1/${arch}/images/pxeboot/vmlinuz") {
		t.Errorf("missing kernel line with /repo path:\n%s", out)
	}
	if !strings.Contains(out, "inst.repo=${boot_root}/repo/euler1/${arch}") {
		t.Error("missing inst.repo with /repo prefix")
	}
	if !strings.Contains(out, "inst.stage2=${boot_root}/repo/euler1/${arch}") {
		t.Error("missing inst.stage2 with /repo prefix")
	}
	if !strings.Contains(out, "initrd ${boot_root}/repo/euler1/${arch}/images/pxeboot/initrd.img") {
		t.Error("missing initrd line")
	}
	// 保留 PXE_SERVER 占位符
	if !strings.Contains(out, "set boot_root http://@@PXE_SERVER@@") {
		t.Error("missing PXE_SERVER placeholder")
	}
	// 架构由运行时识别，不应硬编码 x86_64
	if strings.Contains(out, "/euler1/x86_64/") {
		t.Error("arch should not be hardcoded")
	}
}

func TestRenderAutoexecForImage_DefaultName(t *testing.T) {
	// 初始化内存数据库（无任何镜像），验证 fallback 到默认 euler2110
	if err := db.Init(":memory:"); err != nil {
		t.Fatalf("db init failed: %v", err)
	}
	defer db.Close()

	out := RenderAutoexecForImage(nil, 0, "", "")
	if !strings.Contains(out, "kernel ${boot_root}/repo/euler2110/${arch}/images/pxeboot/vmlinuz") {
		t.Errorf("fallback name not used:\n%s", out)
	}
}

func TestRenderAutoexecForImage_BootMenuItems(t *testing.T) {
	out := RenderAutoexecForImage(nil, 0, "img", "")
	for _, item := range []string{"item bclinux", "item reboot", "item exit"} {
		if !strings.Contains(out, item) {
			t.Errorf("missing menu item: %s", item)
		}
	}
}
