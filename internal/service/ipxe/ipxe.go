// Package ipxe 负责 autoexec.ipxe 脚本的动态拼装与渲染。
package ipxe

import (
	"strings"

	"pxe-server/internal/config"
	"pxe-server/internal/db"
)

// RenderAutoexecForImage 根据指定镜像名称渲染完整的 autoexec.ipxe。
// 按标准模板生成，仅替换占位镜像名（默认 euler2110）；
// PXE 服务 IP 保留 @@PXE_SERVER@@ 占位符，由客户端拉取时由 renderScriptWithPlaceholder 替换；
// 架构由 iPXE 客户端通过 cpuid / buildarch 运行时识别，无需传 arch/mac。
func RenderAutoexecForImage(cfg *config.Manager, imageID int64, name, arch string) string {
	// 取镜像名称（优先显式 name，其次按 imageID 查库，再次取默认生效镜像）
	if name == "" && imageID > 0 {
		if img, err := db.GetOSImage(imageID); err == nil {
			name = img.Name
		}
	}
	if name == "" {
		if img, err := db.GetActiveOSImage(); err == nil && img.Name != "" {
			name = img.Name
		}
	}
	if name == "" {
		name = "euler2110"
	}
	_ = arch // 架构由模板内 cpuid/buildarch 运行时识别，不使用 arch 参数

	var b strings.Builder
	b.WriteString("#!ipxe\n\n")
	b.WriteString("set boot_root http://@@PXE_SERVER@@\n")
	b.WriteString("set ks_root http://@@PXE_SERVER@@\n\n")
	b.WriteString(":start\n")
	b.WriteString("menu BC Linux for Euler 21.10\n")
	b.WriteString("item --gap --   -------------------------------------------------------------------------------\n")
	b.WriteString("item bclinux [1] Install BC Linux for Euler 21.10 (auto architecture)\n")
	b.WriteString("item reboot  [2] Reboot\n")
	b.WriteString("item exit    [3] Boot from local disk\n")
	b.WriteString("item --gap --   -------------------------------------------------------------------------------\n")
	b.WriteString("choose --default bclinux --timeout 3000 target || goto exit\n")
	b.WriteString("goto ${target}\n\n")
	b.WriteString(":bclinux\n")
	b.WriteString("dhcp\n")
	b.WriteString("cpuid --ext 29 && set arch x86_64 ||\n")
	b.WriteString("iseq ${buildarch} i386 && set arch x86_64 ||\n")
	b.WriteString("iseq ${buildarch} arm64 && set arch aarch64 ||\n")
	b.WriteString("isset ${arch}\n\n")
	b.WriteString("kernel ${boot_root}/repo/" + name + "/${arch}/images/pxeboot/vmlinuz initrd=initrd.img inst.repo=${boot_root}/repo/" + name + "/${arch} inst.stage2=${boot_root}/repo/" + name + "/${arch} network noipv6 ip=dhcp ksdevice=bootif BOOTIF=${net0/mac} ks=${ks_root}/ks.cfg console=tty0 console=ttyS0,115200n8 inst.text\n")
	b.WriteString("initrd ${boot_root}/repo/" + name + "/${arch}/images/pxeboot/initrd.img\n")
	b.WriteString("boot\n\n")
	b.WriteString(":reboot\n")
	b.WriteString("reboot\n\n")
	b.WriteString(":exit\n")
	b.WriteString("exit\n")
	return b.String()
}
