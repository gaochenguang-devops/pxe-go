# pxe-server 使用手册

本手册面向运维/实施人员，介绍如何从零部署 pxe-server，并完成一次完整的 PXE 网络装机。项目介绍与架构说明见根目录 `README.md`；如需**脚本化/自动化调用接口**（不通过 Web 后台）完成装机与管理，请参阅《[接口文档](API.md)》。

---

## 目录

1. [部署安装](#一部署安装)
2. [快速上手（5 分钟跑通）](#二快速上手5-分钟跑通)
3. [Web 后台操作指南](#三web-后台操作指南)
4. [装机全流程（分步）](#四装机全流程分步)
5. [配置项速查表](#五配置项速查表)
6. [目录与数据说明](#六目录与数据说明)
7. [常见问题排查](#七常见问题排查)
8. [运维建议](#八运维建议)

---

## 一、部署安装

### 1.1 环境要求

| 项目 | 要求 | 说明 |
|------|------|------|
| 操作系统 | Linux（推荐 CentOS / openEuler / Ubuntu） | 需 root 权限绑定 UDP 67/69 端口 |
| Go | ≥ 1.22 | 仅编译时需要，运行不依赖 |
| `ipmitool` | 可选 | 仅 IPMI 远程电源管理功能需要 |
| 网络 | 服务器 IP 固定，与待装机机器同网段 | DHCP 广播可达 |

> pxe-server 是**单二进制**程序，无 Nginx / dnsmasq / MySQL 等外部依赖。SQLite 内嵌，前端已编译进二进制。

### 1.2 编译产物

```bash
# 方式一：在当前平台编译
make build            # 产物 pxe-server

# 方式二：交叉编译 Linux amd64 / arm64（推荐部署到 Linux 服务器用）
make linux            # 产物 pxe-server-linux-amd64
make linux-arm        # 产物 pxe-server-linux-arm64
```

> 交叉编译用 `CGO_ENABLED=0`（纯 Go SQLite），产物为静态二进制，无 glibc 依赖，可直接拷贝到服务器运行。

### 1.3 部署到服务器

```bash
# 1. 将产物与 assets 目录拷贝到服务器（/opt/pxe-server）
scp pxe-server-linux-amd64 root@<服务器IP>:/opt/pxe-server/pxe-server
scp -r assets root@<服务器IP>:/opt/pxe-server/

# 2. 确保可执行权限
chmod +x /opt/pxe-server/pxe-server

# 3. 手动启动验证（前台运行，观察日志）
cd /opt/pxe-server && ./pxe-server -db data/pxe-server.db -log logs/pxe-server.log -level info
```

### 1.4 注册为系统服务（开机自启）

```bash
# 拷贝 systemd 服务文件
sudo cp deploy/pxe-server.service /etc/systemd/system/

# 按实际路径修改服务文件（默认 /opt/pxe-server）
sudo vi /etc/systemd/system/pxe-server.service

# 重新加载并启用
sudo systemctl daemon-reload
sudo systemctl enable --now pxe-server

# 查看状态与日志
sudo systemctl status pxe-server
sudo journalctl -u pxe-server -f
```

### 1.5 防火墙放行

必须放行以下端口（以 firewalld 为例）：

```bash
sudo firewall-cmd --permanent --add-service=dhcp        # UDP 67
sudo firewall-cmd --permanent --add-port=69/udp         # TFTP
sudo firewall-cmd --permanent --add-port=80/tcp         # HTTP 管理后台
sudo firewall-cmd --reload
```

> 若管理后台端口不是 80，请放行实际端口（见「HTTP 后台配置」）。

---

## 二、快速上手（5 分钟跑通）

以下用最简步骤让 pxe-server 能启动、能登录后台。

```bash
# 1. 编译并启动
make build
./pxe-server -db data/pxe-server.db -log logs/pxe-server.log -level info
```

启动后：

1. 浏览器访问 `http://<服务器IP>/`
2. 默认账号：`admin`，密码：`admin123`
3. 进入「DHCP/TFTP 配置」，填写：
   - **PXE 服务 IP**：服务器 IP（如 `192.168.10.10`）
   - **地址池**：待装机网段可用地址（如 `192.168.10.100` ~ `192.168.10.200`）
   - **网关 / 子网掩码 / DNS**：按实际网络
4. 保存配置（自动热加载生效，无需重启）

至此 DHCP/TFTP/HTTP 三服务已就绪。上传引导文件与安装源后即可开始装机（见下文）。

---

## 三、Web 后台操作指南

### 3.1 登录

- 地址：`http://<服务器IP>/`
- 默认账号 `admin` / 密码 `admin123`，**首次登录后请尽快修改**。

### 3.2 资源管理（上传引导文件/安装源）

用途：上传 iPXE 引导固件、内核、initrd、脚本、系统安装源。

操作步骤：

1. 进入「资源管理」。
2. 配置上传项：
   - **资源类型**：iPXE 固件 / 内核 / initrd / 脚本 / KS 模板 / 安装源
   - **架构**：BIOS / x86_64 UEFI / aarch64 UEFI / 通用
   - **存储目标**：TFTP 根目录（引导文件）或 HTTP 资源目录（安装源）
3. 上传文件，或选择 `.zip` 批量导入（自动解压归档）。

典型上传清单：

| 文件 | 类型 | 架构 | 目标 |
|------|------|------|------|
| `undionly.kpxe` | iPXE 固件 | BIOS | TFTP 根目录 |
| `ipxe-x86_64.efi` | iPXE 固件 | x86_64 UEFI | TFTP 根目录 |
| `ipxe-aarch64.efi` | iPXE 固件 | aarch64 UEFI | TFTP 根目录 |
| `vmlinuz` / `initrd.img`（各架构） | 内核/initrd | 对应架构 | TFTP 根目录 |
| `euler2110` 安装源 | 安装源 | 通用 | HTTP 资源目录 |

> 安装源会自动解压归档到 `web_root/repo/{镜像名}/{架构}`。

### 3.3 DHCP / TFTP 配置

| 配置项 | 说明 |
|--------|------|
| PXE 服务 IP | 本机 IP，用于下发 DHCP/TFTP 服务器地址 |
| 地址池 | 自动分配区间（起始 ~ 结束 IP） |
| 子网掩码 / 网关 / DNS | 下发给客户端的网络参数 |
| 租期 | 租约时长（秒） |
| BIOS/x86/aarch64 引导文件 | 各架构下发的引导文件名 |
| iPXE 脚本名 | 客户端二次引导后拉取的脚本名（默认 `autoexec.ipxe`） |

修改保存后**自动热加载**，无需重启进程。

### 3.4 主机资产与 IPMI 运维

**新增主机**：填写 MAC 地址、主机名、IPMI 地址/账号/密码、绑定镜像。
- `ipmi_addr` 全局唯一，重复录入会被拦截。

**IPMI 电源管理**（需服务器安装 `ipmitool`）：
- 开机 / 关机 / 硬重启 / 查询电源状态
- 设置下次启动为 PXE / 硬盘

### 3.5 KS 模板

- 在线编辑 KS 模板，占位符 `@@PXE_SERVER@@` 自动替换为当前 PXE 服务 IP。
- 支持实时预览。
- `%pre`（架构识别、软件源、分区）与 `%post`（拉取部署脚本）由服务端动态生成。

### 3.6 系统镜像

- 创建镜像记录，关联 vmlinuz、initrd、KS 模板、仓库路径（`/repo/{镜像名}/{架构}`）。
- 主机绑定镜像后，装机时按镜像拉取对应的启动文件与安装源。

---

## 四、装机全流程（分步）

以一台 x86_64 UEFI 裸机安装为例：

1. **准备引导文件**：按 3.2 上传 `ipxe-x86_64.efi`、`vmlinuz`、`initrd.img`。
2. **准备安装源**：上传系统 ISO（自动解压到 `repo/`），确认安装源 URL 可访问：
   ```
   curl -I http://<PXE IP>/repo/euler2110/x86_64/
   ```
3. **配置 DHCP**：见 3.3。
4. **录入主机并绑定镜像**：见 3.4 与 3.6。
5. **设置 PXE 启动并重启**：
   - 若主机带 IPMI：在后台用「PXE 启动」设置下次启动，并点击重启。
   - 否则手动进 BIOS 设置从网络启动。
6. **观察引导流程**：
   - BIOS 客户端获取 `undionly.kpxe`；UEFI 客户端获取对应 `.efi`。
   - iPXE 菜单出现「安装 Euler / 重启 / 本地硬盘启动」。
7. **选择安装**：自动拉取渲染后的 `ks.cfg` 完成无人值守装机，`%post` 阶段拉取执行部署脚本。

> 装机侧无需修改原有 KS / 部署脚本，与既有 dnsmasq 流程 1:1 对齐。

---

## 五、配置项速查表

| 命令行参数 | 默认值 | 说明 |
|-----------|--------|------|
| `-db` | `data/pxe-server.db` | SQLite 数据库路径 |
| `-log` | `logs/pxe-server.log` | 日志文件路径（留空仅控制台） |
| `-level` | `info` | 日志级别：`debug`/`info`/`warn`/`error` |

| Web 配置 | 默认值 | 说明 |
|---------|--------|------|
| 后台账号 | `admin` | 可在「HTTP 后台配置」修改 |
| 后台密码 | `admin123` | 首次登录后请修改 |

---

## 六、目录与数据说明

```
pxe-server
├── cmd/server              程序入口
├── internal/               业务逻辑（config/db/service/middleware/util）
├── web/ui                  Web 后台前端源码（编译进二进制）
├── assets
│   ├── tftp_root/          TFTP 根目录（引导固件、autoexec.ipxe）
│   └── web_root/
│       ├── repo/           系统镜像安装源（{镜像名}/{x86_64|aarch64}）
│       ├── uploads/        资源上传
│       ├── deploy.sh       部署脚本（装机 %post 拉取）
│       └── ks.cfg          KS 模板
├── data/                   SQLite 数据库（运行时生成）
└── logs/                   日志（运行时生成）
```

> `data/`、`logs/`、`web_root/` 下的大体积安装源均为**运行时生成/下载**数据，已被 `.gitignore` 忽略，不纳入版本库。

---

## 七、常见问题排查

### 7.1 客户端无法获取 IP / 引导文件

- 确认 pxe-server 已监听 `0.0.0.0:67`：`ss -lunp | grep :67`
- 确认防火墙放行 UDP 67/69。
- 确认「DHCP 配置」中地址池、PXE 服务 IP 正确。
- 确认服务器 IP 固定（不能是 DHCP 自动获取）。

### 7.2 UEFI 客户端引导失败

- 确认已上传对应架构的 `.efi` 引导文件到 TFTP 根目录。
- 确认「引导文件」配置与上传文件名一致。
- 确认 BIOS 已启用 UEFI 网络启动。

### 7.3 能进 iPXE 菜单，但选择安装后报找不到安装源

- 用浏览器/curl 验证安装源 URL：
  ```
  curl -I http://<PXE IP>/repo/<镜像名>/<架构>/
  ```
- 确认安装源目录结构为 `repo/{镜像名}/{架构}/`，且包含 `repodata/`。

### 7.4 修改配置后不生效

- 配置是**热加载**的（约 5 秒内刷新），请稍等。
- 若仍不生效，检查日志确认版本号已递增，必要时重启服务。

### 7.5 IPMI 操作失败

- 确认服务器已安装 `ipmitool`：`which ipmitool`
- 确认能手动通过 ipmitool 访问：`ipmitool -I lanplus -H <IPMI> -U <user> -P <pass> power status`
- 检查 IPMI 地址/账号/密码是否填写正确。

### 7.6 查看运行日志

```bash
# 系统服务日志
sudo journalctl -u pxe-server -f

# 文件日志
tail -f /opt/pxe-server/logs/pxe-server.log
```

---

## 八、运维建议

- **安全**：修改默认后台密码；如无需公网访问，管理后台尽量限制在内网。
- **备份**：定期备份 `data/pxe-server.db`（含配置、主机资产、模板）。
- **升级**：替换二进制后重启服务；前端已编译进二进制，无需单独部署。
- **资源**：大体积安装源放在 `web_root/repo/`，注意磁盘空间（按装机并发数规划）。
- **测试**：建议先在隔离网段用小规模机器验证完整装机流程，再推广到生产。
