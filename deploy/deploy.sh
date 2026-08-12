#!/bin/bash
set -e

# ============================================================
#  PXE Server Linux 部署脚本
#  用法: sudo bash deploy.sh [install|update|uninstall|status]
# ============================================================

APP_NAME="pxe-server"
INSTALL_DIR="/opt/${APP_NAME}"
SERVICE_NAME="pxe-server.service"
ARCH=$(uname -m)

# 颜色
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

log_info()  { echo -e "${GREEN}[INFO]${NC}  $1"; }
log_warn()  { echo -e "${YELLOW}[WARN]${NC}  $1"; }
log_error() { echo -e "${RED}[ERROR]${NC} $1"; }

# 检查 root
check_root() {
    if [[ $EUID -ne 0 ]]; then
        log_error "请使用 root 权限运行: sudo bash $0 $*"
        exit 1
    fi
}

# 检测架构
detect_arch() {
    case "$ARCH" in
        x86_64)  echo "amd64" ;;
        aarch64) echo "arm64" ;;
        *)       log_error "不支持的架构: $ARCH"; exit 1 ;;
    esac
}

# 二进制命名与 CI/Release 产物一致（pxe-server-linux-<arch>）
BIN_NAME="pxe-server-linux-$(detect_arch)"

# 安装
do_install() {
    check_root
    local go_arch=$(detect_arch)
    log_info "检测到架构: ${ARCH} (${go_arch})"

    # 创建目录结构
    log_info "创建目录结构..."
    mkdir -p "${INSTALL_DIR}"/{data,logs,assets/web_root,assets/tftp_root}
    mkdir -p "${INSTALL_DIR}/assets/web_root/repo"
    mkdir -p "${INSTALL_DIR}/assets/web_root/uploads"

    # 复制文件
    log_info "复制程序文件..."
    if [[ -f "./${BIN_NAME}" ]]; then
        cp "./${BIN_NAME}" "${INSTALL_DIR}/${APP_NAME}"
        chmod +x "${INSTALL_DIR}/${APP_NAME}"
    elif [[ -f "${INSTALL_DIR}/${APP_NAME}" ]]; then
        log_info "二进制文件已存在，跳过复制"
    else
        log_error "找不到二进制文件 ${BIN_NAME}，请先执行 make linux 编译"
        exit 1
    fi

    # 复制 assets（如果当前目录有）
    if [[ -d "./assets" ]]; then
        log_info "复制静态资源 assets/ ..."
        cp -r ./assets/* "${INSTALL_DIR}/assets/"
    fi

    # 安装 systemd 服务
    log_info "安装 systemd 服务..."
    if [[ -f "./deploy/pxe-server.service" ]]; then
        cp ./deploy/pxe-server.service /etc/systemd/system/${SERVICE_NAME}
    elif [[ -f "${INSTALL_DIR}/pxe-server.service" ]]; then
        cp "${INSTALL_DIR}/pxe-server.service" /etc/systemd/system/${SERVICE_NAME}
    fi

    # 修正 service 文件中的路径为实际安装路径
    sed -i "s|/opt/pxe-server|${INSTALL_DIR}|g" /etc/systemd/system/${SERVICE_NAME}

    systemctl daemon-reload
    systemctl enable "${SERVICE_NAME}"

    log_info "安装完成！"
    echo ""
    echo "  启动服务:   sudo systemctl start ${SERVICE_NAME}"
    echo "  查看状态:   sudo systemctl status ${SERVICE_NAME}"
    echo "  查看日志:   sudo journalctl -u ${SERVICE_NAME} -f"
    echo "  文件日志:   tail -f ${INSTALL_DIR}/logs/pxe-server.log"
    echo "  管理界面:   http://<服务器IP>"
    echo ""
}

# 更新（保留数据和日志）
do_update() {
    check_root
    log_info "更新程序..."

    # 停止服务
    if systemctl is-active --quiet "${SERVICE_NAME}" 2>/dev/null; then
        log_info "停止服务..."
        systemctl stop "${SERVICE_NAME}"
    fi

    # 备份旧二进制
    if [[ -f "${INSTALL_DIR}/${APP_NAME}" ]]; then
        cp "${INSTALL_DIR}/${APP_NAME}" "${INSTALL_DIR}/${APP_NAME}.bak.$(date +%Y%m%d_%H%M%S)"
    fi

    # 替换二进制
    if [[ -f "./${BIN_NAME}" ]]; then
        cp "./${BIN_NAME}" "${INSTALL_DIR}/${APP_NAME}"
        chmod +x "${INSTALL_DIR}/${APP_NAME}"
    else
        log_error "找不到 ${BIN_NAME}，请先执行 make linux 编译"
        exit 1
    fi

    # 复制静态资源
    if [[ -d "./assets" ]]; then
        log_info "更新静态资源..."
        cp -r ./assets/* "${INSTALL_DIR}/assets/"
    fi

    # 重启服务
    systemctl start "${SERVICE_NAME}"
    log_info "更新完成！服务已重启"
}

# 卸载
do_uninstall() {
    check_root
    read -p "确认卸载 ${APP_NAME}？此操作不可逆！(yes/no): " confirm
    if [[ "$confirm" != "yes" ]]; then
        log_info "已取消"
        exit 0
    fi

    # 停止并禁用服务
    if systemctl is-active --quiet "${SERVICE_NAME}" 2>/dev/null; then
        log_info "停止服务..."
        systemctl stop "${SERVICE_NAME}"
    fi
    if systemctl is-enabled --quiet "${SERVICE_NAME}" 2>/dev/null; then
        systemctl disable "${SERVICE_NAME}"
    fi

    # 删除 service 文件
    rm -f "/etc/systemd/system/${SERVICE_NAME}"
    systemctl daemon-reload

    # 删除安装目录
    read -p "是否删除所有数据（${INSTALL_DIR}）？(yes/no): " del_data
    if [[ "$del_data" == "yes" ]]; then
        rm -rf "${INSTALL_DIR}"
        log_info "已删除 ${INSTALL_DIR}"
    else
        log_info "安装目录保留在 ${INSTALL_DIR}"
    fi

    log_info "卸载完成"
}

# 状态
do_status() {
    echo "=== ${APP_NAME} 状态 ==="
    echo ""

    # systemd 状态
    if systemctl is-active --quiet "${SERVICE_NAME}" 2>/dev/null; then
        echo -e "  systemd 服务: ${GREEN}运行中${NC}"
    elif systemctl is-failed --quiet "${SERVICE_NAME}" 2>/dev/null; then
        echo -e "  systemd 服务: ${RED}失败${NC}"
    else
        echo -e "  systemd 服务: ${YELLOW}未运行${NC}"
    fi

    echo "  安装目录: ${INSTALL_DIR}"
    echo "  二进制:   ${INSTALL_DIR}/${APP_NAME}"
    if [[ -f "${INSTALL_DIR}/${APP_NAME}" ]]; then
        echo "  版本信息: $("${INSTALL_DIR}/${APP_NAME}" -v 2>/dev/null || echo "无法获取")"
    fi

    echo ""
    echo "  磁盘使用:"
    du -sh "${INSTALL_DIR}" 2>/dev/null || true
    echo ""
    echo "  最近日志 (journalctl -n 10):"
    journalctl -u "${SERVICE_NAME}" -n 10 --no-pager 2>/dev/null || echo "  无日志"
}

# 帮助
usage() {
    echo "用法: sudo bash $0 <命令>"
    echo ""
    echo "命令:"
    echo "  install     首次安装（复制文件 + 注册 systemd 服务）"
    echo "  update      更新程序（替换二进制 + 重启服务，保留数据）"
    echo "  uninstall   卸载（停止服务 + 删除文件）"
    echo "  status      查看服务状态"
    echo ""
    echo "首次部署步骤:"
    echo "  1. 在开发机上执行: make linux  （或从 GitHub Release 下载 pxe-server-linux-<arch>.tar.gz）"
    echo "  2. 将 pxe-server-linux-<arch> 和 deploy/ 目录上传到服务器"
    echo "  3. 在服务器上执行: sudo bash deploy/deploy.sh install"
    echo ""
}

# 入口
case "${1:-}" in
    install)   do_install ;;
    update)    do_update ;;
    uninstall) do_uninstall ;;
    status)    do_status ;;
    *)         usage ;;
esac
