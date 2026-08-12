# pxe-server 接口文档

本接口文档用于**不通过 Web 后台，直接以 HTTP 接口完成 PXE 装机与管理**。适合脚本化、自动化运维场景（如 Ansible、Jenkins、CI 批量装机）。

接口全部基于 JSON（上传/下载类除外），服务端监听 HTTP（默认 `0.0.0.0:80`）。

> 🧪 **OpenAPI 规范**：本接口已提供 `openapi.yaml` 规范文件，可导入 **Postman / Swagger UI / Apifox** 等工具，自动生成可视化接口与调试请求。导入后按章节可直接调用，无需手写 curl。

---

## 目录

1. [通用约定](#一通用约定)
2. [鉴权与登录](#二鉴权与登录)
3. [装机核心流程（推荐顺序）](#三装机核心流程推荐顺序)
4. [配置接口](#四配置接口)
5. [资源与镜像接口](#五资源与镜像接口)
6. [主机与 IPMI 接口](#六主机与-ipmi-接口)
7. [KS 与 iPXE 接口](#七ks-与-ipxe-接口)
8. [装机上报与记录](#八装机上报与记录)
9. [文件管理接口](#九文件管理接口)
10. [服务状态与日志](#十服务状态与日志)
11. [客户端直连接口（无需登录）](#十一客户端直连接口无需登录)
12. [附录：curl 脚本示例](#十二附录curl-脚本示例)
13. [OpenAPI 导入指南](#十三openapi-导入指南)

---

## 一、通用约定

### 1.1 请求地址

```
http://<PXE_SERVER_IP>/api/...
```

### 1.2 鉴权

- **管理接口**（`/api/*`）需要携带 `Authorization` 请求头：`Authorization: <token>`
- token 通过 `POST /api/login` 获取
- **客户端直连接口**（`/install-complete`、`/deploy.sh`、`/ks.cfg` 等）无需登录

### 1.3 统一响应格式

成功：
```json
{ "code": 0, "msg": "操作成功", "data": { ... } }
```

失败（HTTP 状态码对应错误）：
```json
{ "code": 401, "msg": "用户名或密码错误" }
```

| code | 含义 |
|------|------|
| 0 | 成功 |
| 400 | 参数错误 |
| 401 | 未认证/凭证错误 |
| 404 | 资源不存在 |
| 409 | 冲突（如 IPMI 已绑定） |
| 500 | 服务器内部错误 |

---

## 二、鉴权与登录

### 2.1 登录

`POST /api/login`

无需鉴权。请求体：
```json
{ "username": "admin", "password": "admin123" }
```

响应：
```json
{ "code": 0, "msg": "登录成功", "data": { "token": "aBcDeF..." } }
```

> **保存 token**：后续所有管理接口在请求头加 `Authorization: <token>`。

### 2.2 登出

`POST /api/logout`

使当前 token 失效。需鉴权。无需请求体。

### 2.3 修改密码

`PUT /api/password`

```json
{ "old_password": "admin123", "new_password": "newPass" }
```

---

## 三、装机核心流程（推荐顺序）

以下是最简的**纯 API 批量装机**步骤（假设服务端已上传好引导文件与安装源）：

### 步骤 1：登录获取 token

```bash
TOKEN=$(curl -s -X POST http://<IP>/api/login -H 'Content-Type: application/json' \
  -d '{"username":"admin","password":"admin123"}' | jq -r '.data.token')
```

### 步骤 2：配置 DHCP（首次：全局参数 + 子网地址池）

```bash
# 全局参数
curl -s -X PUT http://<IP>/api/config/dhcp -H "Authorization: $TOKEN" \
  -H 'Content-Type: application/json' -d '{
    "enabled": true,
    "pxe_ip": "<服务器IP>",
    "boot_file_bios": "undionly.kpxe",
    "boot_file_x86": "ipxe-x86_64.efi",
    "boot_file_arm": "ipxe-aarch64.efi"
  }'

# 配置子网地址池（一个或多个）
curl -s -X POST http://<IP>/api/config/dhcp/subnets -H "Authorization: $TOKEN" \
  -H 'Content-Type: application/json' -d '{
    "name": "办公子网",
    "ip_pool_start": "192.168.10.100",
    "ip_pool_end": "192.168.10.200",
    "subnet_mask": "255.255.255.0",
    "gateway": "192.168.10.1",
    "dns_servers": "192.168.10.1",
    "enabled": true
  }'
```

### 步骤 3：录入主机（含 IPMI）

```bash
curl -s -X POST http://<IP>/api/host -H "Authorization: $TOKEN" \
  -H 'Content-Type: application/json' -d '{
    "hostname": "node-001",
    "ipmi_addr": "10.0.0.101",
    "ipmi_user": "admin",
    "ipmi_pass": "secret"
  }'
```

### 步骤 4：设置下次启动为 PXE 并重启（IPMI 带外）

```bash
# 设置 PXE 启动
curl -s -X POST http://<IP>/api/host/<host_id>/ipmi/boot -H "Authorization: $TOKEN" \
  -H 'Content-Type: application/json' -d '{"device":"pxe"}'
# 重启
curl -s -X POST http://<IP>/api/host/<host_id>/ipmi/power -H "Authorization: $TOKEN" \
  -H 'Content-Type: application/json' -d '{"action":"cycle"}'
```

### 步骤 5：观察装机进度

```bash
# 查询装机完成记录（含成功/失败统计）
curl -s http://<IP>/api/install-records -H "Authorization: $TOKEN"
```

> 客户端装机完成后会回调 `GET /install-complete?hostname=...`（见第十一章），后台自动记录。

---

## 四、配置接口

### 4.1 获取全局配置

`GET /api/config`

响应 `data` 为配置键值 Map：
```json
{
  "code": 0,
  "data": {
    "dhcp_enabled": "true",
    "dhcp_pxe_ip": "192.168.10.10",
    "http_admin_user": "admin",
    "tftp_root_dir": "assets/tftp_root",
    "...": "..."
  }
}
```

### 4.2 更新 DHCP 配置

`PUT /api/config/dhcp`

所有字段可选（部分更新，传哪个改哪个），保存后**自动热重载**。地址池不再在此配置，由子网接口（见 4.5）管理：
```json
{
  "enabled": true,
  "listen_ip": "0.0.0.0",
  "interface": "eth0",
  "pxe_ip": "192.168.10.10",
  "lease_time": 3600,
  "boot_file_bios": "undionly.kpxe",
  "boot_file_x86": "ipxe-x86_64.efi",
  "boot_file_arm": "ipxe-aarch64.efi",
  "ipxe_script": "autoexec.ipxe"
}
```

### 4.3 更新 TFTP 配置

`PUT /api/config/tftp`

```json
{
  "enabled": true,
  "listen_ip": "0.0.0.0",
  "root_dir": "assets/tftp_root",
  "transfer_timeout": 5,
  "max_connections": 32,
  "access_log": true
}
```

### 4.4 更新 HTTP 配置

`PUT /api/config/http`

```json
{
  "listen_addr": "0.0.0.0:80",
  "web_root": "assets/web_root",
  "admin_user": "admin",
  "admin_password": "newPassword"
}
```

> 修改 `listen_addr` 后需重启进程生效；其余字段热重载。

### 4.5 DHCP 子网管理（纯子网方式）

DHCP 的**地址池完全由子网（subnet）列表定义**，不再保留单网段字段。配置多个子网后，服务器按以下优先级自动匹配对应子网，并下发该子网的地址池、掩码、网关与 DNS；**无匹配子网时拒绝分配（NAK/忽略）**：

1. **GIADDR**（跨网段中继）——优先级最高，只要存在即为客户端所在子网网关
2. **来源 IP**（客户端已持有 IP，如续约）
3. **服务器监听/本机接口 IP**（同网段直连）——广播、源 `0.0.0.0`、无 GIADDR 时，用服务器所在网卡 IP 匹配子网

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | `/api/config/dhcp/subnets` | 子网列表 |
| POST | `/api/config/dhcp/subnets` | 新增子网 |
| PUT | `/api/config/dhcp/subnets/:id` | 更新子网 |
| DELETE | `/api/config/dhcp/subnets/:id` | 删除子网 |

新增/更新子网请求体：
```json
{
  "name": "办公子网",
  "ip_pool_start": "192.168.1.100",
  "ip_pool_end": "192.168.1.200",
  "subnet_mask": "255.255.255.0",
  "gateway": "192.168.1.1",
  "dns_servers": "192.168.1.1",
  "enabled": true
}
```

> 增删改子网后自动递增 DHCP 配置版本并热重载，无需重启。

**对应 ISC DHCP 配置参考**（传统 dnsmasq/dhcpd 写法与本接口的映射）：

```bash
# ISC dhcpd 写法                    →  本接口子网字段
subnet 10.122.240.128 netmask 255.255.255.192 {
  option routers 10.122.240.190;    →  gateway: "10.122.240.190"
  range 10.122.240.169 10.122.240.187;  →  ip_pool_start/ip_pool_end
}                                     →  subnet_mask: "255.255.255.192"
```

---

## 五、资源与镜像接口

### 5.1 资源列表

`GET /api/resource`

返回已上传资源（引导固件、内核、initrd、安装源等）：
```json
{
  "code": 0,
  "data": [
    { "id": 1, "name": "undionly.kpxe", "res_type": "firmware", "arch_type": "bios", "size": 267432, "upload_time": "..." }
  ]
}
```

> `res_type`：firmware / kernel / initrd / script / ks_template / repo / other
> `arch_type`：bios / x86_uefi / aarch64_uefi / all

### 5.2 镜像列表

`GET /api/image`

```json
{
  "code": 0,
  "data": [
    { "id": 1, "name": "euler1", "x86_repo_path": "/repo/euler1/x86_64", "arm_repo_path": "/repo/euler1/aarch64", "active": 1 }
  ]
}
```

### 5.3 上传系统镜像（ISO）

`POST /api/image/upload` （multipart/form-data）

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `name` | text | 是 | 镜像名（如 `euler1`，仅字母数字 `_` `-`） |
| `x86_iso` | file | 否 | x86_64 架构 ISO |
| `arm_iso` | file | 否 | aarch64 架构 ISO |

一次可传一个或两个架构。上传后自动解压到 `web_root/repo/{name}/{arch}` 并落库。

```bash
curl -s -X POST http://<IP>/api/image/upload -H "Authorization: $TOKEN" \
  -F "name=euler1" -F "x86_iso=@/path/euler-x86.iso" -F "arm_iso=@/path/euler-arm.iso"
```

### 5.4 删除镜像

`DELETE /api/image/:id`

### 5.5 设置默认镜像

`POST /api/image/:id/active`

设为默认安装镜像（全局唯一生效）。

### 5.6 上传镜像引导文件（vmlinuz/initrd）

`POST /api/image/:id/boot-file` （multipart/form-data）

| 字段 | 说明 |
|------|------|
| `arch` | 架构：`x86_64` / `aarch64` |
| `file` | 引导文件（`vmlinuz` 或 `initrd.img`） |

文件上传到 `web_root/repo/{镜像名}/{arch}/images/pxeboot/`。

---

## 六、主机与 IPMI 接口

### 6.1 主机列表

`GET /api/host?page=1&pageSize=20&search=关键字`

- 不带分页参数返回全量；`search` 按主机名/IPMI 模糊匹配
- 分页时响应含 `total` / `page` / `pageSize` / `totalPages`
- **IPMI 密码脱敏**返回（`ipmi_pass` 为空，`ipmi_pass_masked: "******"`）

### 6.2 新增主机

`POST /api/host`

```json
{
  "hostname": "node-001",
  "ipmi_addr": "10.0.0.101",
  "ipmi_user": "admin",
  "ipmi_pass": "secret",
  "install_status": "idle"
}
```

- `ipmi_addr` 全局唯一，重复返回 409
- 密码自动加密存储

### 6.3 编辑主机

`PUT /api/host/:id`

字段同新增。不传 `ipmi_pass`（或传 `"******"`）则保留原密码。

### 6.4 删除主机

`DELETE /api/host/:id`

### 6.5 IPMI 电源操作

`POST /api/host/:id/ipmi/power`

```json
{ "action": "on" }
```
`action`：`on` / `off` / `cycle` / `reset` / `status`

### 6.6 查询电源状态

`GET /api/host/:id/ipmi/status`

```json
{ "code": 0, "data": { "status": "on" } }
```

### 6.7 设置启动设备

`POST /api/host/:id/ipmi/boot`

```json
{ "device": "pxe" }
```
`device`：`pxe`（下次 PXE 启动）或 `disk`（硬盘启动）。

### 6.8 主机资源（Bond 网络）

以下接口管理 `HostResource`（用于生成 node-info.txt）：

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | `/api/host-resource/list` | 列表 |
| GET | `/api/host-resource/template` | 下载 Excel 模板 |
| POST | `/api/host-resource/import` | 导入 Excel（multipart，字段 `file`） |
| GET | `/api/host-resource/excel/export` | 导出 Excel |
| GET | `/api/host-resource/node-info/export` | 导出 node-info.txt |
| GET | `/api/host-resource/node-info` | 获取 node-info.txt 内容 |
| POST | `/api/host-resource/batch/export` | 导出选中（body：`{"ids":[1,2,3]}`） |
| POST | `/api/host-resource/batch/delete` | 批量删除（body：`{"ids":[1,2,3]}`） |

新增/编辑主机资源字段（`POST /api/host-resource` 不存在，需通过 Excel 导入；如需直接新增请用数据库或反馈需求）：
`ipmi_addr`、`hostname`、`bond0_ip`、`bond0_mask`、`bond0_gateway`、`bond0_ipv6`、`bond0_ipv6mask`、`bond0_ipv6gw`、`bond2_*`（同 bond0）、`bond1_ip`、`bond1_mask`、`bond1_gateway`

---

## 七、KS 与 iPXE 接口

### 7.1 KS 模板列表

`GET /api/ks/template`

### 7.2 新增 KS 模板

`POST /api/ks/template`

```json
{
  "name": "euler-standard",
  "os_type": "euler",
  "content": "graphical\nrootpw --iscrypted ...\n%pre\n...\n%end\n",
  "root_password": "myRootPass"
}
```

### 7.3 编辑 / 删除 / 设为生效

- `PUT /api/ks/template/:id`
- `DELETE /api/ks/template/:id`
- `POST /api/ks/template/:id/active`

### 7.4 渲染生效 KS

`GET /api/ks/template/render`

返回当前生效 KS 模板渲染结果（占位符已替换 + `%pre`/`%post` 动态注入）。

### 7.5 iPXE 脚本 CRUD

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | `/api/ipxe/script` | 列表 |
| POST | `/api/ipxe/script` | 新增（`{name,content}`） |
| PUT | `/api/ipxe/script/:id` | 编辑 |
| DELETE | `/api/ipxe/script/:id` | 删除 |
| POST | `/api/ipxe/script/:id/active` | 设为生效（全局唯一） |

### 7.6 按镜像渲染 autoexec.ipxe

`GET /api/ipxe/script/render?image_id=1&name=euler1&arch=x86_64`

根据系统镜像生成 autoexec.ipxe 内容（安装源指向 `/repo/{name}/{arch}`）。参数三选一即可。

### 7.7 部署脚本 CRUD

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | `/api/deploy/script` | 列表 |
| POST | `/api/deploy/script` | 新增（`{name,content}`） |
| PUT | `/api/deploy/script/:id` | 编辑 |
| DELETE | `/api/deploy/script/:id` | 删除 |
| POST | `/api/deploy/script/:id/active` | 设为生效 |
| GET | `/api/deploy/script/:id/content` | 获取内容 |

---

## 八、装机上报与记录

### 8.1 装机完成上报

`GET /install-complete`（**无需登录**，客户端通过 wget 回调）

```
GET /install-complete?status=success&hostname=node-001&ipmi=10.0.0.101&mac=00:11:22:33:44:55&ip=192.168.10.101&arch=x86_64&interfaces=eth0,eth1&lldp=switch1
```

| 参数 | 说明 |
|------|------|
| `status` | 默认 `success` |
| `hostname` / `ipmi` / `mac` / `ip` / `arch` / `interfaces` / `lldp` | 上报字段 |

### 8.2 查询装机记录

`GET /api/install-records?limit=50&offset=0`

```json
{
  "code": 0,
  "data": [ { "hostname": "...", "status": "success", "report_time": "..." } ],
  "total": 120, "success": 118, "failed": 2, "noLldp": 5
}
```

---

## 九、文件管理接口

上传到 `web_root/uploads/`，公开可访问。

| 方法 | 路径 | 说明 |
|------|------|------|
| POST | `/api/file/upload` | 上传（multipart，字段 `file`） |
| GET | `/api/file/list` | 列表 |
| DELETE | `/api/file/:name` | 删除 |

上传后文件可通过 `GET /uploads/{文件名}` 公开下载。

---

## 十、服务状态与日志

### 10.1 服务状态

`GET /api/status`

```json
{
  "code": 0,
  "data": [
    { "name": "DHCP 服务", "port": "0.0.0.0:67", "status": "running" },
    { "name": "TFTP 服务", "port": "0.0.0.0:69", "status": "running" },
    { "name": "HTTP 服务", "status": "running" },
    { "name": "SQLite", "status": "running" }
  ]
}
```

### 10.2 操作审计日志

`GET /api/operlog`

### 10.3 实时日志

`GET /api/logfile`

---

## 十一、客户端直连接口（无需登录）

以下接口是**装机过程中客户端自动访问**的，无需 token，限流阈值较高：

| 接口 | 用途 |
|------|------|
| `GET /install-complete` | 装机完成后回调上报 |
| `GET /deploy.sh` | 部署脚本（`@@PXE_SERVER@@` 自动替换为实际 IP） |
| `GET /lldp.sh` | LLDP 邻居发现脚本 |
| `GET /node-info.txt` | 节点信息表（bond 网络） |
| `GET /ks.cfg` | 通用 KS（渲染当前生效模板） |
| `GET /ks/{mac}/ks.cfg` | 按主机 MAC 渲染专属 KS |
| `GET /repo/{镜像名}/{架构}/...` | 系统安装源（YUM repo）静态文件 |
| `GET /uploads/{文件名}` | 上传的公开文件 |
| `GET /files/*filepath` | web_root 下静态文件 |
| `GET /vmlinuz`、`/initrd.img` 等 | web_root 根下任意文件 |

---

## 十二、附录：curl 脚本示例

### 完整批量装机脚本（bash + curl + jq）

```bash
#!/usr/bin/env bash
set -euo pipefail
IP="192.168.10.10"
USER="admin"; PASS="admin123"

# 1. 登录
TOKEN=$(curl -s -X POST "http://$IP/api/login" \
  -H 'Content-Type: application/json' \
  -d "{\"username\":\"$USER\",\"password\":\"$PASS\"}" | jq -r '.data.token')
AUTH="Authorization: $TOKEN"

# 2. 确保 DHCP 配置（全局参数 + 子网地址池）
curl -s -X PUT "http://$IP/api/config/dhcp" -H "$AUTH" -H 'Content-Type: application/json' \
  -d "{\"enabled\":true,\"pxe_ip\":\"$IP\",\"boot_file_bios\":\"undionly.kpxe\",\"boot_file_x86\":\"ipxe-x86_64.efi\",\"boot_file_arm\":\"ipxe-aarch64.efi\"}"
# 配置子网（幂等：先查再增）
if ! curl -s "http://$IP/api/config/dhcp/subnets" -H "$AUTH" | jq -e '.data[] | select(.ip_pool_start=="192.168.10.100")' >/dev/null 2>&1; then
  curl -s -X POST "http://$IP/api/config/dhcp/subnets" -H "$AUTH" -H 'Content-Type: application/json' \
    -d '{"name":"办公子网","ip_pool_start":"192.168.10.100","ip_pool_end":"192.168.10.200","subnet_mask":"255.255.255.0","gateway":"192.168.10.1","dns_servers":"192.168.10.1","enabled":true}'
fi

# 3. 录入主机并记录 id
HOST_JSON=$(curl -s -X POST "http://$IP/api/host" -H "$AUTH" -H 'Content-Type: application/json' \
  -d '{"hostname":"node-001","ipmi_addr":"10.0.0.101","ipmi_user":"admin","ipmi_pass":"secret"}')
HOST_ID=$(echo "$HOST_JSON" | jq -r '.id')
echo "host id: $HOST_ID"

# 4. 设置 PXE 启动并重启
curl -s -X POST "http://$IP/api/host/$HOST_ID/ipmi/boot" -H "$AUTH" -H 'Content-Type: application/json' -d '{"device":"pxe"}'
curl -s -X POST "http://$IP/api/host/$HOST_ID/ipmi/power" -H "$AUTH" -H 'Content-Type: application/json' -d '{"action":"cycle"}'

# 5. 轮询装机记录
sleep 60
curl -s "http://$IP/api/install-records" -H "$AUTH" | jq '{total, success, failed}'
```

### 上传镜像示例

```bash
curl -s -X POST "http://$IP/api/image/upload" -H "$AUTH" \
  -F "name=euler1" \
  -F "x86_iso=@./euler-2110-x86.iso" \
  -F "arm_iso=@./euler-2110-arm.iso"
```

### 通过接口验证安装源可用

```bash
curl -sI "http://$IP/repo/euler1/x86_64/" | head -1   # 期望 HTTP/1.1 200
```

---

## 十三、OpenAPI 导入指南

本接口提供 `docs/openapi.yaml`（OpenAPI 3.0），可用可视化工具直接导入并调试。

### 导入 Postman

1. Postman → **Import** → **Upload Files** → 选择 `docs/openapi.yaml`
2. 导入后自动生成各接口的请求模板
3. 在 Collection 的 **Variables** 中设置 `PXE_IP` 为实际服务器 IP
4. 先调用 `POST /api/login`，在响应中找到 `data.token`
5. 在 Collection 级配置 `Authorization` 为 **Bearer Token**，粘贴 token，即可调试全部管理接口

### 使用 Swagger UI

```bash
# 任意目录起一个静态服务托管 openapi.yaml
docker run -d -p 8080:8080 -e SWAGGER_JSON=/docs/openapi.yaml \
  -v "$PWD/docs/openapi.yaml:/docs/openapi.yaml" swaggerapi/swagger-ui
# 浏览器访问 http://<本机>:8080
```

### 使用 Apifox

1. Apifox → 新建接口 → **导入数据** → **OpenAPI** → 选择 `docs/openapi.yaml`
2. 自动生成文档与调试环境，设置 baseURL 与登录 token 后可在线联调。

> 提示：管理接口均带 Bearer 鉴权（见 `components.securitySchemes`），导入后需先登录获取 token 再调试。

### 补充示例：完整主机列表响应

`GET /api/host?page=1&pageSize=20`

```json
{
  "code": 0,
  "msg": "查询成功",
  "data": {
    "list": [
      {
        "id": 1,
        "hostname": "node-001",
        "mac_addr": "",
        "ipmi_addr": "10.0.0.101",
        "ipmi_user": "admin",
        "ipmi_pass_masked": "******",
        "install_status": "idle",
        "create_time": "2026-08-12 10:00:00"
      }
    ],
    "total": 1,
    "page": 1,
    "pageSize": 20,
    "totalPages": 1
  }
}
```
