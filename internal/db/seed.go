package db

import (
	"time"

	"pxe-server/internal/model"
)

// defaultKSTemplateContent 内置默认 KS 模板（BC Linux for Euler 21.10），不可删除/修改。
const defaultKSTemplateContent = `# BC Linux for Euler 21.10 无人值守安装 KS 模板
# 占位符 @@PXE_SERVER@@ 由服务端自动替换；
# %pre / %post 动态内容由服务端注入。

graphical
reboot
keyboard --xlayouts='us'
lang en_US.UTF-8
timezone Asia/Shanghai --utc
firstboot --disable
services --enabled="chronyd"
firewall --disabled
selinux --disabled
rootpw --plaintext @@ROOT_PASSWORD@@ 

%include /tmp/arch-repo
%include /tmp/partinfo



%packages
@^minimal-environment
wget
bash-completion
lldpad
ipmitool
network-scripts
lrzsz
python3-IPy
tcpdump
telnet
vim
%end

%post
# 动态 %post 内容由服务端注入
%end

%addon com_redhat_kdump --disable --reserve-mb='128'
%end
%addon org_bclinux_security_enhancement --disable
%end
`

// EnsureDefaultKSTemplate 确保内置默认 KS 模板存在（is_default=1, active=1）。
// 若已存在默认模板则跳过；若被误删则重新创建。
func EnsureDefaultKSTemplate() error {
	row := DB.QueryRow(`SELECT id, content FROM ks_template WHERE is_default=1 LIMIT 1`)
	var id int64
	var content string
	if err := row.Scan(&id, &content); err == nil {
		if content != defaultKSTemplateContent {
			_, err = DB.Exec(`UPDATE ks_template SET content=?, name=? WHERE id=?`,
				defaultKSTemplateContent, "默认 KS 模板", id)
		}
		return err
	}
	tpl := &model.KSTemplate{
		Name:       "默认 KS 模板",
		OsType:     "EulerOS",
		Content:    defaultKSTemplateContent,
		Active:     1,
		IsDefault:  1,
		CreateTime: time.Now(),
	}
	_, err := CreateKSTemplate(tpl)
	return err
}

// defaultIPxeScriptContent 内置默认 iPXE 脚本内容。
const defaultIPxeScriptContent = `#!ipxe
set boot_root http://@@PXE_SERVER@@
set ks_root http://@@PXE_SERVER@@

:start
menu iPXE Boot Menu
item --gap --             -------- Boot Options --------
item bclinux              BC Linux for Euler 21.10
item shell                iPXE Shell
item reboot               Reboot
choose --default bclinux --timeout 5000 target && goto ${target}

:bclinux
cpuid --ext 29 && set arch_override aarch64 || set arch_override x86_64
iseq ${buildarch} arm64 && set arch_override aarch64 ||
set arch_override ${arch_override}
set image_name bclinux-euler2110
kernel ${boot_root}/repo/${image_name}/${arch_override}/images/pxeboot/vmlinuz initrd=initrd.img inst.ks=${ks_root}/ks.cfg inst.stage2=${boot_root}/repo/${image_name}/${arch_override} net.ifnames=0 biosdevname=0
initrd ${boot_root}/repo/${image_name}/${arch_override}/images/pxeboot/initrd.img
boot

:shell
shell

:reboot
reboot
`

// EnsureDefaultIPxeScript 确保内置默认 iPXE 脚本存在（is_default=1, active=1）。
// 若已存在默认脚本则跳过；若被误删则重新创建。
func EnsureDefaultIPxeScript() error {
	row := DB.QueryRow(`SELECT id, content FROM ipxe_script WHERE is_default=1 LIMIT 1`)
	var id int64
	var content string
	if err := row.Scan(&id, &content); err == nil {
		if content != defaultIPxeScriptContent {
			_, err = DB.Exec(`UPDATE ipxe_script SET content=?, name=? WHERE id=?`,
				defaultIPxeScriptContent, "默认 iPXE 脚本", id)
		}
		return err
	}
	scr := &model.IPxeScript{
		Name:       "默认 iPXE 脚本",
		Content:    defaultIPxeScriptContent,
		Active:     1,
		IsDefault:  1,
		CreateTime: time.Now(),
	}
	_, err := CreateIPxeScript(scr)
	return err
}

// defaultDeployScriptContent 内置默认部署脚本。
const defaultDeployScriptContent = `#!/bin/bash
# 远端运维部署脚本：系统安装完成后由 %post 阶段拉取执行。
# 参考 pxe/ipxe/scripts/deploy.sh 的生产部署流程，保持 Bond 网络、IPMI 识别、
# 主机名、YUM 源、内核优化、NIC 驱动、完成上报等核心逻辑。
# 占位符 @@PXE_SERVER@@ 由服务端自动替换为 PXE 服务 IP。

set -E -o pipefail

PXE_SERVER="@@PXE_SERVER@@"
pxe_server=${PXE_SERVER}

# 网络接口（按实际硬件拓扑修改）
bond0_if1=eno1
bond0_if2=eno2
# 存储网络
bond2_if1=enp23s0f1
bond2_if2=enp150s0f1
# 服务网络
bond1_if1=enp23s0f0
bond1_if2=enp150s0f0

# 服务端与本地路径
yum_server=${PXE_SERVER}
node_info_file=/root/node-info.txt
network_dir=/etc/sysconfig/network-scripts
log_file=/var/log/pxe-deploy.log
state_dir=/var/lib/pxe-deploy
artifact_dir=${state_dir}/artifacts
firmware_marker=${state_dir}/nic-firmware-installed
completion_marker=${state_dir}/completion-reported
lock_file=/run/pxe-deploy.lock

# NIC 软件（按实际版本/架构修改）
ARCH=$(uname -m)
case "${ARCH}" in
    aarch64)
        hinic_rpm=hinic3-17.13.1.0_4.19.90_2107.6.0.0192.8.oe1.bclinux.aarch64-1.oe1.bclinux.aarch64.rpm
        hisdk_rpm=hisdk3-17.13.1.0_4.19.90_2107.6.0.0192.8.oe1.bclinux.aarch64-1.oe1.bclinux.aarch64.rpm
        ;;
    x86_64)
        hinic_rpm=hinic3-17.13.1.0_4.19.90_2107.6.0.0100.oe1.bclinux.x86_64-1.oe1.bclinux.x86_64.rpm
        hisdk_rpm=hisdk3-17.13.1.0_4.19.90_2107.6.0.0100.oe1.bclinux.x86_64-1.oe1.bclinux.x86_64.rpm
        ;;
    *)
        hinic_rpm=
        hisdk_rpm=
        ;;
esac
firmware_archive=NIC-FW-17.13.1.0.tar
firmware_dir=NIC-FW-17.13.1.0

log() {
    printf '[%s] [deploy] %s\n' "$(date '+%Y-%m-%d %H:%M:%S')" "$*"
}

log_error() {
    local exit_code=$?
    printf '[%s] [deploy] ERROR at line %s, exit code %s\n' \
        "$(date '+%Y-%m-%d %H:%M:%S')" "$1" "${exit_code}"
}

trap 'log_error ${LINENO}' ERR

initialize_runtime() {
    touch "${log_file}"
    chmod 600 "${log_file}"
    exec > >(tee -a "${log_file}") 2>&1
    mkdir -p "${state_dir}" "${artifact_dir}"
}

require_command() {
    command -v "$1" >/dev/null 2>&1 || {
        echo "ERROR: required command not found: $1" >&2
        return 1
    }
}

preflight() {
    local command_name failed=0
    local required_commands=(
        awk chage find flock grep hostnamectl ipmitool iptables modprobe
        passwd rpm sed swapoff sysctl systemctl tar tee tr useradd usermod wget yum
    )

    if [ "$(id -u)" -ne 0 ]; then
        echo "ERROR: deploy.sh must run as root" >&2
    fi
    for command_name in "${required_commands[@]}"; do
        require_command "${command_name}" || failed=1
    done
    [ "${failed}" -eq 0 ] || return 1
    [ -r "${node_info_file}" ] || {
        echo "ERROR: node information file is not readable: ${node_info_file}" >&2
    }
    mkdir -p "${network_dir}"
}

acquire_execution_lock() {
    exec 9>"${lock_file}"
    if ! flock -n 9; then
        echo "ERROR: another deploy.sh process is already running" >&2
        return 1
    fi
}

load_node_info() {
    local row_ipmi row_hostname
    local row_bond0_ip row_bond0_mask row_bond0_gateway
    local row_bond0_ipv6 row_bond0_ipv6mask row_bond0_ipv6gateway
    local row_bond2_ip row_bond2_mask row_bond2_gateway
    local row_bond2_ipv6 row_bond2_ipv6mask row_bond2_ipv6gateway
    local row_bond1_ip row_bond1_mask row_bond1_gateway

    while read -r row_ipmi row_hostname \
        row_bond0_ip row_bond0_mask row_bond0_gateway \
        row_bond0_ipv6 row_bond0_ipv6mask row_bond0_ipv6gateway \
        row_bond2_ip row_bond2_mask row_bond2_gateway \
        row_bond2_ipv6 row_bond2_ipv6mask row_bond2_ipv6gateway \
        row_bond1_ip row_bond1_mask row_bond1_gateway; do
        [ "${row_ipmi}" = "${ipmi_addr}" ] || continue

        target_hostname=${row_hostname}
        bond0_ip=${row_bond0_ip}
        bond0_mask=${row_bond0_mask}
        bond0_gateway=${row_bond0_gateway}
        bond0_ipv6=${row_bond0_ipv6}
        bond0_ipv6mask=${row_bond0_ipv6mask}
        bond0_ipv6gateway=${row_bond0_ipv6gateway}
        bond2_ip=${row_bond2_ip}
        bond2_mask=${row_bond2_mask}
        bond2_gateway=${row_bond2_gateway}
        bond2_ipv6=${row_bond2_ipv6}
        bond2_ipv6mask=${row_bond2_ipv6mask}
        bond2_ipv6gateway=${row_bond2_ipv6gateway}
        bond1_ip=${row_bond1_ip}
        bond1_mask=${row_bond1_mask}
        bond1_gateway=${row_bond1_gateway}
        return 0
    done < "${node_info_file}"

    return 1
}

write_slave_config() {
    local interface=$1 bond=$2
    cat > "${network_dir}/ifcfg-${interface}" << EOF
TYPE=Ethernet
NAME=${interface}
DEVICE=${interface}
ONBOOT=yes
MASTER=${bond}
SLAVE=yes
BOOTPROTO=none
EOF
}

write_interface_config() {
    local interface=$1
    cat > "${network_dir}/ifcfg-${interface}" << EOF
TYPE=Ethernet
NAME=${interface}
DEVICE=${interface}
ONBOOT=yes
BOOTPROTO=none
EOF
}

write_bond_config() {
    local bond=$1 interface1=$2 interface2=$3 ip=$4 mask=$5
    local gateway=$6 ipv6=$7 ipv6_prefix=$8 ipv6_gateway=$9

    if [ -z "${ip}" ]; then
        log "remove ${bond} configuration: IP address is empty"
        rm -f "${network_dir}/ifcfg-${bond}" \
            "${network_dir}/ifcfg-${interface1}" \
            "${network_dir}/ifcfg-${interface2}"
        return
    fi

    cat > "${network_dir}/ifcfg-${bond}" << EOF
DEVICE=${bond}
NAME=${bond}
BONDING_OPTS="mode=4 miimon=100 xmit_hash_policy=layer3+4"
TYPE=Bond
BONDING_MASTER=yes
BOOTPROTO=static
ONBOOT=yes
IPADDR=${ip}
NETMASK=${mask}
EOF

    [ -z "${gateway}" ] || echo "GATEWAY=${gateway}" >> "${network_dir}/ifcfg-${bond}"
    if [ -n "${ipv6}" ]; then
        cat >> "${network_dir}/ifcfg-${bond}" << EOF
IPV6INIT=yes
IPV6_AUTOCONF=no
IPV6ADDR=${ipv6}/${ipv6_prefix}
IPV6_DEFAULTGW=${ipv6_gateway}
EOF
    fi

    write_slave_config "${interface1}" "${bond}"
    write_slave_config "${interface2}" "${bond}"
}

sanitize_report_value() {
    printf '%s' "$1" | tr -d '\r\n' | sed 's/[^A-Za-z0-9.:-]/_/g'
}

urlencode_report_value() {
    local value=$1 encoded="" char hex i
    LC_ALL=C
    for ((i = 0; i < ${#value}; i++)); do
        char=${value:i:1}
        case "${char}" in
            [A-Za-z0-9.~_-]) encoded="${encoded}${char}" ;;
            *)
                printf -v hex '%%%02X' "'${char}"
                encoded="${encoded}${hex}"
                ;;
        esac
    done
    printf '%s' "${encoded}"
}

get_interface_lldp() {
    local interface=$1 mac switch_name port_id
    mac=$(tr '[:lower:]' '[:upper:]' < "/sys/class/net/${interface}/address" 2>/dev/null || printf 'unknown')
    switch_name=$(lldptool -t -n -i "${interface}" -V sysName 2>/dev/null | \
        awk '!/System Name|Agent/ {gsub(/^[[:space:]]+|[[:space:]]+$/, ""); if (length) {print; exit}}')
    port_id=$(lldptool -t -n -i "${interface}" -V portID 2>/dev/null | \
        awk '!/Port ID|Agent/ {gsub(/^[[:space:]]+|[[:space:]]+$/, ""); if (length) {print; exit}}')
    mac=$(sanitize_report_value "${mac:-unknown}")
    switch_name=$(sanitize_report_value "${switch_name:-unknown}")
    port_id=$(sanitize_report_value "${port_id:-unknown}")
    printf '%s=%s|%s|%s' "${interface}" "${mac}" "${switch_name}" "${port_id}"
}

get_physical_interfaces() {
    local interface_path
    for interface_path in /sys/class/net/*; do
        [ -e "${interface_path}/device" ] || continue
        printf '%s\n' "${interface_path##*/}"
    done
}

report_completion() {
    local report_hostname report_ipmi report_ip report_mac report_arch report_url
    local report_interfaces report_lldp interface interface_separator lldp_separator
    local -a physical_interfaces

    report_hostname=$(hostname 2>/dev/null || cat /etc/hostname 2>/dev/null || printf 'unknown')
    report_hostname=$(sanitize_report_value "${report_hostname}")
    report_ipmi=$(sanitize_report_value "${ipmi_addr:-unknown}")
    report_ip=$(sanitize_report_value "${bond0_ip:-unknown}")
    report_arch=$(sanitize_report_value "$(uname -m)")

    if [ -r /sys/class/net/bond0/address ]; then
        report_mac=$(tr '[:lower:]' '[:upper:]' < /sys/class/net/bond0/address)
    else
        report_mac=unknown
    fi

    mapfile -t physical_interfaces < <(get_physical_interfaces)
    report_interfaces=""
    report_lldp=""
    interface_separator=""
    lldp_separator=""
    for interface in "${physical_interfaces[@]}"; do
        report_interfaces="${report_interfaces}${interface_separator}${interface}"
        report_lldp="${report_lldp}${lldp_separator}$(get_interface_lldp "${interface}")"
        interface_separator=","
        lldp_separator=";"
    done

    report_url="http://${pxe_server}/install-complete"
    report_url="${report_url}?status=success&hostname=${report_hostname}"
    report_url="${report_url}&ipmi=${report_ipmi}&mac=${report_mac}"
    report_url="${report_url}&ip=${report_ip}&arch=${report_arch}"
    report_url="${report_url}&interfaces=$(urlencode_report_value "${report_interfaces}")"
    report_url="${report_url}&lldp=$(urlencode_report_value "${report_lldp}")"

    if curl -sS -m 5 "${report_url}" >/dev/null 2>&1; then
        log "installation completion reported to PXE server"
        return 0
    else
        echo "WARNING: failed to report installation completion" >&2
        return 1
    fi
}

download_file() {
    local url=$1 target=$2 temporary_file
    temporary_file="${target}.tmp"
    rm -f "${temporary_file}"
    if wget -q -T 30 -t 3 -O "${temporary_file}" "${url}"; then
        mv -f "${temporary_file}" "${target}"
        return 0
    fi
    rm -f "${temporary_file}"
    echo "ERROR: failed to download ${url}" >&2
    return 1
}

install_rpm_if_needed() {
    local rpm_file=$1 package_name expected_version installed_version
    package_name=$(rpm -qp --queryformat '%{NAME}' "${rpm_file}") || return 1
    expected_version=$(rpm -qp --queryformat '%{VERSION}-%{RELEASE}.%{ARCH}' "${rpm_file}") || return 1
    installed_version=$(rpm -q --queryformat '%{VERSION}-%{RELEASE}.%{ARCH}' "${package_name}" 2>/dev/null || true)
    if [ "${installed_version}" = "${expected_version}" ]; then
        log "skip ${package_name}: version ${expected_version} is already installed"
        return 0
    fi
    rpm -Uvh "${rpm_file}"
}

ensure_rpm_installed() {
    local url=$1 rpm_file=$2
    if [ -f "${rpm_file}" ] && install_rpm_if_needed "${rpm_file}"; then
        return 0
    fi
    download_file "${url}" "${rpm_file}" && install_rpm_if_needed "${rpm_file}"
}

configure_security_baseline() {
    log "disable firewalld"
    systemctl disable --now firewalld
}

load_ipmi_modules() {
    log "load IPMI modules"
    modprobe ipmi_msghandler
    modprobe ipmi_si
    modprobe ipmi_devintf
}

load_server_identity() {
    ipmi_addr=$(ipmitool lan print 1 | awk -F: '/IP Address[[:space:]]*:/ {gsub(/[[:space:]]/, "", $2); print $2; exit}')
    if [ -z "${ipmi_addr}" ]; then
        echo "ERROR: failed to read the IPMI address" >&2
    fi

    log "load node configuration for IPMI ${ipmi_addr}"
    if ! load_node_info; then
        echo "ERROR: IPMI ${ipmi_addr} was not found in ${node_info_file}" >&2
    fi
}

configure_hostname() {
    log "set hostname to ${target_hostname}"
    hostnamectl set-hostname "${target_hostname}"
    printf '%s\n' "${target_hostname}" > /etc/hostname
}

configure_network() {
    log "write bond network configuration"
    write_bond_config bond0 "${bond0_if1}" "${bond0_if2}" \
        "${bond0_ip}" "${bond0_mask}" "${bond0_gateway}" \
        "${bond0_ipv6}" "${bond0_ipv6mask}" "${bond0_ipv6gateway}"
    write_bond_config bond2 "${bond2_if1}" "${bond2_if2}" \
        "${bond2_ip}" "${bond2_mask}" "" \
        "${bond2_ipv6}" "${bond2_ipv6mask}" "${bond2_ipv6gateway}"
    write_bond_config bond1 "${bond1_if1}" "${bond1_if2}" \
        "${bond1_ip}" "${bond1_mask}" "" "" "" ""
    if [ -z "${bond1_ip}" ]; then
        log "bond1 IP address is empty; enable its physical interfaces without bonding"
        write_interface_config "${bond1_if1}"
        write_interface_config "${bond1_if2}"
    fi
}

configure_repositories() {
    log "configure hosts and YUM repository"
    cat > /etc/hosts << EOF
127.0.0.1   localhost localhost.localdomain localhost4 localhost4.localdomain4
::1         localhost localhost.localdomain localhost6 localhost6.localdomain6

#YUM
${yum_server} mirrors.bclinux.org
EOF

    mkdir -p /etc/yum.repos.d/bak
    find /etc/yum.repos.d -maxdepth 1 -type f -name '*.repo' \
        -exec mv -f {} /etc/yum.repos.d/bak/ \;

    # 根据当前生效镜像生成 YUM 源
    # @@PXE_IMAGE_NAME@@ 由服务端下发时替换为当前生效的镜像名称
    local image_name="@@PXE_IMAGE_NAME@@"
    if [ "${image_name}" = "@@PXE_IMAGE_NAME@@" ]; then
        image_name="euler"
        log "WARNING: @@PXE_IMAGE_NAME@@ not replaced, using default: ${image_name}"
    fi
    local repo_arch
    case "$(uname -m)" in
        x86_64)  repo_arch="x86_64" ;;
        aarch64) repo_arch="aarch64" ;;
        *)       repo_arch="x86_64" ;;
    esac

    cat > /etc/yum.repos.d/pxeYum.repo << REPOEOF
[pxeYum]
name=BC-Linux-release - pxeYum
baseurl=http://${pxe_server}/${image_name}/${repo_arch}/
gpgcheck=0
enabled=1
gpgkey=file:///etc/pki/rpm-gpg/RPM-GPG-KEY-BCLinux-For-Euler
REPOEOF
    log "YUM repo configured: baseurl=http://${pxe_server}/${image_name}/${repo_arch}/"

    if [ -f /etc/yum/pluginconf.d/license-manager.conf ]; then
        sed -i 's/enabled=1/enabled=0/g' /etc/yum/pluginconf.d/license-manager.conf
    fi
}

configure_system_services() {
    log "configure system services"
    systemctl mask systemd-journald-audit.socket
    rm -f /usr/lib/systemd/system/sockets.target.wants/systemd-journald-audit.socket
}

configure_kernel() {
    log "disable swap and apply kernel parameters"
    sed -i '/^[[:space:]]*[^#].*[[:space:]]swap[[:space:]]/s/^/#/' /etc/fstab
    swapoff -a

    modprobe nf_conntrack
    cat > /etc/sysctl.d/99-optimize.conf << 'EOF'
net.netfilter.nf_conntrack_max = 262144
net.nf_conntrack_max = 262144
net.ipv4.ip_forward = 1
net.ipv4.ip_nonlocal_bind = 1
net.ipv4.conf.default.accept_redirects = 0
net.ipv4.conf.all.accept_redirects = 0
net.ipv4.conf.all.send_redirects = 0
net.ipv4.conf.all.rp_filter = 0
net.ipv4.conf.default.rp_filter = 1
net.ipv4.conf.all.accept_source_route = 0
net.ipv4.icmp_echo_ignore_broadcasts = 1
net.ipv4.tcp_keepalive_intvl = 3
net.ipv4.tcp_keepalive_time = 30
net.ipv4.tcp_keepalive_probes = 8
net.ipv4.tcp_retries2 = 5
net.ipv6.conf.all.forwarding = 1
net.ipv6.ip_nonlocal_bind = 1
net.ipv6.route.max_size = 2147483647
kernel.pid_max = 316448
EOF
    sysctl -q -p /etc/sysctl.d/99-optimize.conf
}

configure_network_services() {
    log "enable network services"
    systemctl enable --now lldpad
    systemctl disable  NetworkManager
    systemctl enable network
    if systemctl is-active --quiet network; then
        systemctl restart network
    else
        systemctl start network
    fi
}

configure_firewall() {
    log "reset and disable iptables"
    iptables -F
    iptables -X
    systemctl stop iptables
    systemctl disable iptables
    chmod +x /root/*.sh
}

downgrade_x86_64_kernel() {
    local rpm_name
    local -a kernel_rpms=(
        kernel-4.19.90-2107.6.0.0100.oe1.bclinux.x86_64.rpm
        kernel-devel-4.19.90-2107.6.0.0100.oe1.bclinux.x86_64.rpm
        kernel-tools-4.19.90-2107.6.0.0100.oe1.bclinux.x86_64.rpm
    )
    local -a rpm_files=()

    [ "${ARCH}" = "x86_64" ] || return 0

    log "download and downgrade x86_64 kernel packages"
    for rpm_name in "${kernel_rpms[@]}"; do
        download_file "http://${pxe_server}/uploads/${rpm_name}" \
            "${artifact_dir}/${rpm_name}" || return 1
        rpm_files+=("${artifact_dir}/${rpm_name}")
    done

    yum -y downgrade "${rpm_files[@]}"
}

configure_nic_software() {
    log "install NIC drivers and firmware"
    ensure_rpm_installed "http://${pxe_server}/uploads/${hinic_rpm}" \
        "${artifact_dir}/${hinic_rpm}"
    ensure_rpm_installed "http://${pxe_server}/uploads/${hisdk_rpm}" \
        "${artifact_dir}/${hisdk_rpm}"

    if [ -f "${firmware_marker}" ] && grep -qxF "${firmware_dir}" "${firmware_marker}"; then
        log "skip NIC firmware: installation was already completed"
    elif download_file "http://${pxe_server}/uploads/${firmware_archive}" \
        "${artifact_dir}/${firmware_archive}" && \
        tar -xf "${artifact_dir}/${firmware_archive}" -C "${state_dir}" && \
        (cd "${state_dir}/${firmware_dir}" && bash install.sh); then
        printf '%s\n' "${firmware_dir}" > "${firmware_marker}"
    else
        echo "WARNING: NIC firmware installation failed; the next run will retry" >&2
    fi
}

report_completion_once() {
    if [ -f "${completion_marker}" ]; then
        log "skip completion report: success was already reported"
    elif report_completion; then
        touch "${completion_marker}"
    fi
}

main() {
    preflight || exit 1
    acquire_execution_lock || exit 1
    initialize_runtime

    configure_security_baseline
    load_ipmi_modules
    load_server_identity || exit 1
    configure_hostname
    configure_network
    configure_repositories
    configure_system_services
    configure_kernel
    configure_network_services
    configure_firewall
    downgrade_x86_64_kernel || exit 1
    configure_nic_software
    report_completion_once

    log "deployment completed"
}

main "$@"`

// EnsureDefaultDeployScript 确保内置默认部署脚本存在且内容为最新（is_default=1, active=1）。
func EnsureDefaultDeployScript() error {
	row := DB.QueryRow(`SELECT id, content FROM deploy_script WHERE is_default=1 LIMIT 1`)
	var id int64
	var content string
	if err := row.Scan(&id, &content); err == nil {
		// 已存在，强制更新内容
		if content != defaultDeployScriptContent {
			_, err = DB.Exec(`UPDATE deploy_script SET content=?, name=? WHERE id=?`,
				defaultDeployScriptContent, "默认部署脚本", id)
		}
		return err
	}
	s := &model.DeployScript{
		Name:       "默认部署脚本",
		Content:    defaultDeployScriptContent,
		Active:     1,
		IsDefault:  1,
		CreateTime: time.Now(),
	}
	_, err := CreateDeployScript(s)
	return err
}
