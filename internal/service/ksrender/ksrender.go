// Package ksrender 负责 KS 模板占位符替换、%pre 架构识别、磁盘筛选与 LVM 分区动态生成、%post 部署脚本注入。
package ksrender

import (
	"os"
	"path/filepath"
	"strings"

	"pxe-server/internal/config"
	"pxe-server/internal/db"
	"pxe-server/internal/model"
)

// RenderKSFromDB 从数据库模板渲染 ks.cfg（不使用磁盘缓存，用于 /ks.cfg 通用访问）。
func RenderKSFromDB(cfg *config.Manager, mac string, tplID int64, imageName string) (string, error) {
	return renderKS(cfg, mac, tplID, imageName, false)
}

// RenderKS 渲染指定主机的专属 ks.cfg。
// mac: 客户端 MAC；tplID: 指定 KS 模板 ID（0 表示自动匹配）；
// imageName: 指定软件源镜像名称（空则用默认生效镜像）。
func RenderKS(cfg *config.Manager, mac string, tplID int64, imageName string) (string, error) {
	return renderKS(cfg, mac, tplID, imageName, true)
}

func renderKS(cfg *config.Manager, mac string, tplID int64, imageName string, useDisk bool) (string, error) {
	// 获取模板：useDisk=true 时优先磁盘 ks.cfg；useDisk=false 时只用数据库
	var tpl *model.KSTemplate
	if useDisk {
		if diskContent, ok := loadDiskKSTemplate(cfg); ok {
			tpl = &model.KSTemplate{Content: diskContent}
		}
	}
	if tpl == nil && tplID > 0 {
		if t, err := db.GetKSTemplate(tplID); err == nil {
			tpl = t
		}
	}
	if tpl == nil {
		// 取当前生效的 KS 模板（active=1）
		if t, err := db.GetActiveKSTemplate(); err == nil {
			tpl = t
		}
	}
	if tpl == nil {
		// 兜底取默认模板
		list, _ := db.ListKSTemplates()
		if len(list) > 0 {
			tpl = list[0]
		}
	}
	if tpl == nil {
		return "", errNoTemplate
	}

	content := tpl.Content
	// 1. 保留 @@PXE_SERVER@@ 占位符，在客户端请求时由 renderScriptWithPlaceholder 动态替换

	// 2. 先移除模板中已有的 %pre ... %end 和 %post ... %end 段，避免重复注入
	content = stripSection(content, "%pre")
	content = stripSection(content, "%post")

	// 3. %pre 阶段动态内容（架构识别、磁盘筛选、软件源、LVM 分区）
	preScript := buildPreScript(cfg, imageName)
	content = injectPre(content, preScript)

	// 4. %post 阶段注入部署脚本拉取
	postScript := buildPostScript(cfg, mac)
	content = injectPost(content, postScript)

	return content, nil
}

// buildPreScript 生成 %pre 阶段脚本。
// 参考 pxe/install-pxe-common.sh 的 Kickstart %pre 逻辑：
// 识别架构 → 生成 /tmp/arch-repo（软件源）→ 筛选系统盘 → 生成 /tmp/partinfo（LVM 分区方案）。
// 模板通过 %include /tmp/arch-repo 与 %include /tmp/partinfo 引入。
func buildPreScript(cfg *config.Manager, imageName string) string {
	// 软件源 URL 前缀（如 /euler1）：优先用手动选择的镜像名称，否则用默认生效镜像
	repoPrefix := ""
	if imageName != "" {
		repoPrefix = normRepoPath(imageName)
	} else {
		repoPrefix = resolveInstallRepoPrefix()
	}

	var b strings.Builder
	b.WriteString("\n# ===== 动态生成 %pre 脚本 =====\n")
	b.WriteString("#!/bin/bash\n")
	b.WriteString("set -x\n")
	b.WriteString("exec >/tmp/ks-pre.log 2>&1\n")

	// 架构识别
	b.WriteString("ARCH=$(uname -m)\n")
	b.WriteString("if [ \"${ARCH}\" = \"aarch64\" ]; then\n")
	b.WriteString("    REPO_ARCH=aarch64\n")
	b.WriteString("else\n")
	b.WriteString("    REPO_ARCH=x86_64\n")
	b.WriteString("fi\n")

	// 生成软件源 /tmp/arch-repo（URL 使用 @@PXE_SERVER@@ 占位符，客户端拉取时由服务端替换）
	b.WriteString("cat > /tmp/arch-repo << REPO_EOF\n")
	if repoPrefix != "" {
		b.WriteString("url --url=http://@@PXE_SERVER@@" + repoPrefix + "/${REPO_ARCH}/\n")
	} else {
		b.WriteString("url --url=http://@@PXE_SERVER@@/euler2110/${REPO_ARCH}/\n")
	}
	b.WriteString("REPO_EOF\n")

	// 筛选系统盘：参考生产脚本按 /proc/partitions 磁盘容量范围匹配（4xx GB 系统盘）
	b.WriteString("disk=$(awk '$3 >= 461373440 && $3 <= 482344960 && $4 ~ /sd/ {print $4; exit}' /proc/partitions)\n")
	b.WriteString("if [ -z \"${disk}\" ]; then\n")
	b.WriteString("    echo \"No system disk matched the configured size range\" >&2\n")
	b.WriteString("    exit 1\n")
	b.WriteString("fi\n")

	// 生成分区方案 /tmp/partinfo（对齐生产 LVM bel 卷组方案）
	b.WriteString("cat > /tmp/partinfo << PART_EOF\n")
	b.WriteString("bootloader --append=\" crashkernel=auto\" --location=mbr --boot-drive=${disk}\n")
	b.WriteString("clearpart --all --initlabel --drives=${disk}\n")
	b.WriteString("part pv.817 --fstype=\"lvmpv\" --ondisk=${disk} --size=455836\n")
	b.WriteString("part /boot --fstype=\"xfs\" --ondisk=${disk} --size=500\n")
	b.WriteString("part /boot/efi --fstype=\"efi\" --ondisk=${disk} --size=500 --fsoptions=\"umask=0077,shortname=winnt\"\n")
	b.WriteString("volgroup bel --pesize=4096 pv.817\n")
	b.WriteString("logvol swap --fstype=\"swap\" --size=65536 --name=swap --vgname=bel\n")
	b.WriteString("logvol / --fstype=\"xfs\" --size=102400 --name=root --vgname=bel\n")
	b.WriteString("logvol /home --fstype=\"xfs\" --size=51200 --name=home --vgname=bel\n")
	b.WriteString("logvol /var --fstype=\"xfs\" --percent=100 --name=var --vgname=bel\n")
	b.WriteString("PART_EOF\n")

	b.WriteString("echo '===== %pre 完成 ====='\n")
	return b.String()
}

// buildPostScript 生成 %post 阶段脚本：使用 curl 拉取并执行远端运维部署脚本。
func buildPostScript(cfg *config.Manager, mac string) string {
	var b strings.Builder
	b.WriteString("\n# ===== 动态生成 %post 脚本 =====\n")
	b.WriteString("mkdir -p /root\n")
	b.WriteString("curl -sS http://@@PXE_SERVER@@/deploy.sh -o /root/deploy.sh\n")
	b.WriteString("curl -sS http://@@PXE_SERVER@@/node-info.txt -o /root/node-info.txt\n")
	b.WriteString("curl -sS http://@@PXE_SERVER@@/lldp.sh -o /root/lldp.sh\n")
	b.WriteString("chmod +x /root/deploy.sh /root/lldp.sh\n")
	if mac != "" {
		b.WriteString("echo \"node-mac: " + mac + "\" >> /root/node-info.txt\n")
	}
	b.WriteString("bash /root/deploy.sh\n")
	b.WriteString("echo '===== %post 完成 ====='\n")
	return b.String()
}

// injectPre 将脚本注入到独立的 %pre ... %end 段中。若模板无 %pre 标记则追加完整段。
func injectPre(content, script string) string {
	idx := findSectionStart(content, "%pre")
	if idx < 0 {
		return strings.TrimRight(content, "\n") + "\n\n%pre\n" + script + "%end\n"
	}
	insertAt := afterLineEnd(content, idx)
	if insertAt < 0 {
		return content + "\n" + script
	}
	return content[:insertAt] + script + "\n" + content[insertAt:]
}

// injectPost 将脚本注入到独立的 %post ... %end 段中。若模板无 %post 标记则追加完整段。
func injectPost(content, script string) string {
	idx := findSectionStart(content, "%post")
	if idx < 0 {
		return strings.TrimRight(content, "\n") + "\n\n%post\n" + script + "%end\n"
	}
	insertAt := afterLineEnd(content, idx)
	if insertAt < 0 {
		return content + "\n" + script
	}
	return content[:insertAt] + script + "\n" + content[insertAt:]
}

// findSectionStart 定位独立的段标记（如 %pre / %post），该标记必须独占一行。
// 返回标记起始下标；未找到返回 -1。
func findSectionStart(content, marker string) int {
	lines := strings.Split(content, "\n")
	pos := 0
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == marker {
			// 定位到该行开头（pos 为该行内容起点，需回退到实际行首）
			return pos + strings.Index(line, marker)
		}
		pos += len(line) + 1 // +1 补偿 "\n"
	}
	return -1
}

// afterLineEnd 返回 content 中从 idx 起当前行行尾（换行符之后）的位置。
// 若 idx 之后无换行，返回 -1。
func afterLineEnd(content string, idx int) int {
	rest := content[idx:]
	nl := strings.Index(rest, "\n")
	if nl < 0 {
		return -1
	}
	return idx + nl + 1
}

// stripSection 移除模板中从 marker（如 %pre / %post）到 %end 的完整段。
func stripSection(content, marker string) string {
	for {
		start := findSectionStart(content, marker)
		if start < 0 {
			break
		}
		endMarker := "%end"
		endIdx := findSectionStart(content[start:], endMarker)
		if endIdx < 0 {
			// 有 %pre 但没有 %end，只移除 %pre 行本身
			nl := afterLineEnd(content, start)
			if nl < 0 {
				content = content[:start]
			} else {
				content = content[:start] + content[nl:]
			}
			break
		}
		endIdx += start // 转为绝对位置
		nl := afterLineEnd(content, endIdx)
		if nl < 0 {
			content = content[:start]
		} else {
			content = content[:start] + content[nl:]
		}
	}
	return content
}

var errNoTemplate = errNew3("no ks template available")

type errString3 string

func errNew3(s string) error       { return errString3(s) }
func (e errString3) Error() string { return string(e) }

// resolveInstallRepoPrefix 解析用于 %pre 软件源 URL 的镜像名称前缀（如 /euler1）。
// 取当前默认生效镜像的名称；无生效镜像时取首个镜像；均无则返回空。
func resolveInstallRepoPrefix() string {
	// 数据库未初始化时返回空（避免测试/早期调用 panic）
	if db.DB == nil {
		return ""
	}
	if img, err := db.GetActiveOSImage(); err == nil && img.Name != "" {
		return normRepoPath(img.Name)
	}
	list, err := db.ListOSImages()
	if err != nil || len(list) == 0 {
		return ""
	}
	if list[0].Name != "" {
		return normRepoPath(list[0].Name)
	}
	return ""
}

// normRepoPath 规范化仓库路径：确保以 / 开头、去掉结尾多余斜杠。
func normRepoPath(p string) string {
	p = strings.TrimRight(p, "/")
	if p == "" {
		return ""
	}
	if !strings.HasPrefix(p, "/") {
		p = "/" + p
	}
	return p
}

// loadDiskKSTemplate 尝试从 web_root/ks.cfg 读取最新模板内容。
// 优先于数据库，保证 assets/web_root/ks.cfg 源码修改后无需重新导入即可生效。
// 读取失败（不存在等）时返回 ok=false，由调用方回退到数据库。
func loadDiskKSTemplate(cfg *config.Manager) (string, bool) {
	if cfg == nil {
		return "", false
	}
	root := cfg.HTTP().WebRoot
	if root == "" {
		root = "assets/web_root"
	}
	path := filepath.Join(root, "ks.cfg")
	data, err := os.ReadFile(path)
	if err != nil {
		return "", false
	}
	return string(data), true
}
