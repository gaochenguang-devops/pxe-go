# Go 一体化 PXE 装机服务（pxe-server）

用 Go 单二进制实现的 PXE 网络装机服务，**替换 dnsmasq + Nginx + iPXE + Kickstart 整套 PXE 能力**。进程内通过 Goroutine 并发运行标准 DHCP、TFTP、Gin HTTP 三大服务，无需任何外置依赖（无 dnsmasq / Nginx / Apache / MySQL）。持久化使用 SQLite，配置全 Web 可视化修改并热加载生效。

支持 **BIOS x86、x86_64 UEFI、aarch64 ARM UEFI** 三架构 PXE 引导，业务逻辑 1:1 对齐现有 dnsmasq+ipxe+ks 装机流程，装机侧无需修改原有 KS / 部署脚本。

> 📖 部署、操作与排障请参阅《[使用手册](docs/USER_MANUAL.md)》，脚本化/自动化 PXE 装机请参阅《[接口文档](docs/API.md)》。本文档侧重功能与设计介绍。

---

## 一、功能特性

- **标准 DHCP 服务**（UDP 67，基于 `github.com/insomniacslk/dhcp`）：地址池分配、子网掩码/网关/DNS/租期下发、Option 66 指定 TFTP、Option 67 下发引导文件、按 **Option 93(client-arch)** 区分架构、识别 **Option 175 iPXE** 后下发 `autoexec.ipxe`。地址池**完全由子网（subnet）列表定义**（类似 ISC DHCP `subnet ... { option routers ...; range ...; }`），按请求来源 IP 或 giaddr（中继）自动匹配子网并下发对应网络参数，无匹配子网时拒绝分配。
- **TFTP 服务**（UDP 69，基于 `github.com/pin/tftp`）：专属根目录只读下发引导固件、`autoexec.ipxe`；成熟库自动处理 `blksize`/`tsize` 等 option 协商（RFC 2347/2348/2349），路径防目录穿越、访问日志。
- **HTTP Gin 服务**：Web 管理后台（登录鉴权）+ 静态安装源 + iPXE 动态脚本 + KS 模板渲染。
- **主机资产表**：`ipmi_addr` 唯一索引，Web 新增重复 IPMI 直接拦截报错；支持 IPMI 开机/关机/硬重启/PXE 启动/BIOS 启动（调用服务器 `ipmitool`）。
- **Web 资源批量导入**：单文件上传 / ZIP 压缩包批量导入，可选存储至 TFTP 根目录或 HTTP 资源目录，SQLite 记录资源元数据。
- **KS 模板渲染**：全局替换 `@@PXE_SERVER@@`、`%pre` 架构识别与 LVM 分区、`%post` 拉取部署脚本，按客户端 MAC 匹配绑定主机输出专属 ks.cfg。
- **配置热加载**：所有配置持久化于 `sys_config` 表，各服务定时轮询配置版本变化，动态刷新，不重启进程、不中断进行中的传输。
- **安全规范**：入参强校验、路径锁定工作目录、IPMI 密码加密存储、优雅关停、UDP 限流防泛洪、全量操作审计日志。

---

## 二、目录结构

```
pxe-server
├── cmd/server/main.go          入口：初始化组件、启动多服务 Goroutine、信号捕获优雅退出
├── internal
│   ├── config                   配置监听、热加载管理
│   ├── logger                   分级日志（控制台 + 文件切割）
│   ├── db                       SQLite 初始化、CRUD、唯一性校验、默认数据种子
│   ├── model                    数据库实体、配置结构体、请求入参
│   ├── service
│   │   ├── dhcp                 标准 DHCP、架构识别、Option 下发、地址池、租约
│   │   ├── tftp                 TFTP 文件服务、路径安全校验、限流
│   │   ├── httpapi              Gin 实例、路由、各 handler、静态资源
│   │   ├── ipxe                 autoexec.ipxe 脚本动态拼装
│   │   ├── ksrender             KS 占位符替换、%pre/%post 动态生成
│   │   └── ipmi                 ipmitool 命令封装执行
│   ├── handler                  （控制器逻辑在 httpapi 内实现）
│   ├── middleware               登录鉴权、跨域、请求日志、接口限流
│   └── util                     工具集：IP/MAC 校验、路径防穿越、ZIP 解压、字符串替换、密码加密
├── web
│   └── ui                        Web 管理后台前端源码（编译进二进制，支持单文件分发）
├── assets
│   ├── tftp_root                TFTP 文件存储目录（含默认 autoexec.ipxe）
│   └── web_root                 HTTP 安装源、部署脚本、KS 模板（运行时生成，不入版本库）
│       └── repo                系统镜像安装源（{镜像名}/{x86_64|aarch64}）
├── deploy/pxe-server.service    systemd 开机自启配置
├── docs
│   ├── USER_MANUAL.md          使用手册（部署/操作/装机流程/排障）
│   ├── API.md                  接口文档（脚本化 PXE 装机与管理）
│   └── openapi.yaml            OpenAPI 3.0 规范（可导入 Postman/Swagger）
├── Makefile
├── go.mod / go.sum
└── README.md
```

---

## 三、编译与运行

### 环境要求

- Go >= 1.22
- Linux 服务器（DHCP/TFTP 需绑定 UDP 67/69 端口，需 root 权限）
- `ipmitool`（仅 IPMI 运维功能需要，可按需安装）

### 编译

```bash
# 当前平台
make build            # 或 go build -o pxe-server ./cmd/server

# Linux amd64 交叉编译
make linux

# Linux arm64 交叉编译
make linux-arm
```

### 运行

```bash
# 前台运行
./pxe-server -db data/pxe-server.db -log logs/pxe-server.log -level info

# 参数说明
#   -db       SQLite 数据库路径（默认 data/pxe-server.db）
#   -log      日志文件路径（默认 logs/pxe-server.log，留空仅控制台）
#   -level    日志级别：debug/info/warn/error
```

首次启动会自动建表并写入默认配置、内置 Euler KS 模板与占位镜像。

### systemd 开机自启（Linux）

```bash
sudo cp deploy/pxe-server.service /etc/systemd/system/
sudo mkdir -p /opt/pxe-server
# 将编译产物与 assets 目录放入 /opt/pxe-server/
sudo systemctl daemon-reload
sudo systemctl enable --now pxe-server
sudo systemctl status pxe-server
```

---

## 四、Web 后台操作指引

启动后浏览器访问 `http://<服务器IP>/`，默认账号 `admin` / 密码 `admin123`（可在「DHCP/TFTP 配置 → HTTP 后台配置」中修改）。

### 1. 资源管理

- 选择资源类型（iPXE 固件 / 内核 / initrd / 脚本 / KS 模板 / 安装源）、架构（BIOS / x86_64 UEFI / aarch64 UEFI / 通用）、存储目标（TFTP 根目录 / HTTP 资源目录）。
- **单文件上传**：上传 `undionly.kpxe`、`ipxe-x86_64.efi`、`ipxe-aarch64.efi`、各架构 `vmlinuz`、`initrd.img`、iPXE 脚本。
- **ZIP 批量导入**：选择 `.zip` 压缩包（内含整套安装源），后端自动解压归档到目标目录。
- 上传后自动记录元数据（文件名、存储路径、架构、类型、上传时间、文件大小）。

### 2. DHCP / TFTP 配置

- 配置服务本机 `PXE_IP`、地址池、子网掩码、网关、DNS、租期、TFTP 服务 IP、BIOS/x86/aarch64 引导文件、iPXE 脚本名。
- 保存后自动热加载，无需重启主程序。

### 3. 主机资产与 IPMI 运维

- **新增/编辑主机**：填写 MAC、主机名、IPMI 地址、IPMI 账号密码、绑定镜像。
- **IPMI 地址全局唯一**：重复录入会被后端拦截并提示「该IPMI地址已绑定其他主机」。
- **IPMI 运维**：一键开机、硬重启、关机、查询电源状态、设置 PXE/硬盘启动。

### 4. KS 模板与系统镜像

- **KS 模板在线编辑**：可实时预览，占位符 `@@PXE_SERVER@@` 会被替换为当前 PXE 服务 IP。
- **系统镜像**：关联 vmlinuz、initrd、KS 模板、仓库路径，供 iPXE 引导与 KS 渲染使用。

### 5. 操作审计日志

- 全量记录配置修改、文件上传、IPMI 开关机、主机绑定等操作留痕。

---

## 五、默认资源模板（对齐生产 `pxe/install-pxe-common.sh` 体系）

> 以下脚本与配置均参考 `pxe/install-pxe-common.sh` 及 `pxe/ipxe/` 下的生产脚本编写，
> 保留服务端 `@@PXE_SERVER@@` 占位符（请求时自动替换），实现与原 dnsmasq 装机流程 1:1 对齐。

- `assets/tftp_root/autoexec.ipxe`：BC Linux for Euler 21.10 启动菜单（`cpuid`/`buildarch` 架构自动识别），完整内核启动参数（`inst.repo`、`inst.stage2`、`ksdevice=bootif`、`BOOTIF`、串口 console、`inst.text`）。
- `assets/web_root/ks.cfg`：生产对齐 KS 模板——`%include /tmp/arch-repo` 与 `%include /tmp/partinfo`（由服务端动态 `%pre` 生成）、真实 LVM 分区方案（`bel` 卷组 + swap/root/home/var）、生产 `%packages` 包列表、`%addon` 段；`%post` 动态拉取部署脚本。
- `assets/web_root/deploy.sh`：生产对齐部署脚本——Bond 网络（bond0/bond1/bond2）、IPMI 识别匹配 node-info、主机名、YUM 源、内核参数优化、SSH 免密、NIC 驱动/固件安装、内核降级、装机完成上报。
- `assets/web_root/lldp.sh`：生产对齐——LLDP 邻居发现（交换机/端口/MAC）。
- `assets/web_root/node-info.txt`：空格分隔节点信息表（IPMI 主机名 bond0 bond2 bond1），列格式与生产一致，供 deploy.sh 按 IPMI 匹配。

---

## 六、全流程测试步骤

### 1）Web 批量导入资源

1. 登录后台 → 「资源管理」。
2. 上传 iPXE 固件：`undionly.kpxe`（BIOS）、`ipxe-x86_64.efi`（x86_64 UEFI）、`ipxe-aarch64.efi`（aarch64 UEFI），目标选「TFTP 根目录」。
3. 上传各架构 `vmlinuz`、`initrd.img`、iPXE 脚本（目标 TFTP）。
4. 上传安装源镜像（如 `euler2110` 的 `x86_64/`、`aarch64/`，可打 ZIP，目标「HTTP 资源目录」），自动解压归档至 `web_root/repo/{镜像名}/{架构}`。

### 2）配置 DHCP 网段与 PXE 服务 IP

- 「DHCP/TFTP 配置」中填写 PXE 服务 IP、地址池、网关、DNS 等，保存。

### 3）录入主机并验证重复 IPMI 拦截

- 「主机资产」新增主机，填写 MAC、IPMI 地址等。
- 再次录入同一 IPMI 地址，页面应提示「该IPMI地址已绑定其他主机」并拒绝。

### 4）绑定镜像，IPMI 重启验证双架构 PXE 引导

1. 「主机资产」中为主机绑定系统镜像。
2. 使用 IPMI「PXE启动」设置下次启动为 PXE 并重启主机。
3. BIOS 客户端应获取 `undionly.kpxe`，UEFI 客户端获取对应 `.efi`，iPXE 菜单可正常选择「安装 Euler / 重启 / 本地硬盘启动」。
4. 选择安装后自动走 HTTP 拉取渲染后的 `ks.cfg`，完成全自动 KS 装机，`%post` 阶段拉取执行部署脚本。

---

## 七、关键设计说明

- **架构识别**：DHCP Option 93（client-arch）：`0`→BIOS、`7/9`→x86_64 UEFI、`11`→aarch64 UEFI；识别 Option 175（iPXE vendor）后下发 `autoexec.ipxe`。
- **成熟库支撑**：DHCP 报文解析/构造基于 `insomniacslk/dhcp`（`dhcpv4` + `server4`，自动处理 `0.0.0.0` 来源转广播回包），TFTP 基于 `pin/tftp`（自动处理 `blksize`/`tsize`/`OACK` 等 option 协商）。二者均比自研实现更健壮，规避了地址族、option 解析、源端口等易错点。
- **DHCP IPv4 广播**：DHCP 套接字强制 `udp4` 监听并回退到 `0.0.0.0:67`（源端口天然为 67，符合 RFC 2131），通过 `setBroadcastOn`（分平台 `sockopt_unix.go`/`sockopt_windows.go`）启用 `SO_BROADCAST`，避免 Windows 下"地址族不匹配/地址无效"发送失败。
- **路径安全**：TFTP 与静态资源读取均经过 `util.SafeJoinWithin` 严格校验，杜绝目录穿越。
- **IPMI 密码**：数据库中以 XOR+Base64 加密存储，读取时解密，禁止明文裸存。
- **配置热加载**：`sys_config` 表维护各模块 `*_config_version`，修改配置时递增版本号，服务每 5 秒轮询检测并刷新。
- **优雅关停**：捕获 SIGINT/SIGTERM，依次安全关闭 DHCP → TFTP → HTTP，断开 SQLite 连接。
