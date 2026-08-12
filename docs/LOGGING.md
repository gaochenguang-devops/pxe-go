# 日志规范

本文档定义 pxe-go 后端日志记录的标准，覆盖级别选择、字段命名、结构化用法与请求追踪，确保线上问题可观测、可定位。

## 1. 基础设施

日志基础设施为 `internal/logger` 包，基于 Go 标准库 `log/slog` 构建：

- **控制台**：人类可读的 text 格式（`time=... level=INFO msg=...`）
- **文件**：JSON 格式（每行一条，便于日志平台采集），经 lumberjack 自动轮转（单文件 20MB、保留 10 份、30 天、压缩）

启动参数：

```bash
./pxe-server -log logs/pxe-server.log -level info
```

- `-log` 留空则仅控制台输出；日志目录不可创建时降级为仅控制台，不影响服务启动
- `-level` 支持 `debug/info/warn/error`，运行中可通过 `logger.SetLevel` 动态调整

## 2. 级别选择标准

| 级别 | 何时使用 | 示例 |
|---|---|---|
| `Debug` | 热路径调试信息，生产默认关闭 | DHCP 包交换、HTTP 请求参数 |
| `Info` | 正常业务事件、生命周期事件 | 服务启动/停止、资源创建/删除、装机流程节点 |
| `Warn` | 可恢复/尽力而为操作失败，不影响主流程 | 自动激活失败、统计查询失败、文件重生成失败 |
| `Error` | 关键路径失败，导致功能不可用或数据不一致 | DB 写入失败、服务启动失败、文件落盘失败 |

判断原则：**失败是否会导致用户可感知的功能失效或数据不一致**——是则 `Error`（通常需中止并向调用方返回错误），否则 `Warn`（记录后继续）。

## 3. 调用方式

### 3.1 兼容的格式化风格（适用于简单消息）

```go
logger.Info("创建部署脚本: name=%s", name)
logger.Error("数据库初始化失败: %v", err)
```

第一个参数是 `fmt.Sprintf` 格式串。**禁止**在消息中拼接动态结构字段，应使用 3.2 的结构化方式。

### 3.2 结构化风格（推荐，带字段）

```go
logger.With("mac", mac, "ip", ip, "subnet", subnet).Info("DHCP 请求")
logger.With("image_id", id, "size_mb", size).Warn("设置默认镜像失败")
```

- 字段**先 `With` 携带、消息固定为纯文本**，不要把字段当作 `Info` 的格式化参数
- 字段统一英文 `snake_case`：`request_id`、`client_ip`、`cost_ms`、`image_id`、`mac`、`ip`、`subnet`、`status`、`source`

### 3.3 请求上下文日志（HTTP Handler）

所有 HTTP handler 内日志必须通过 `logger.FromGin(c)` 获取，自动携带 `request_id/path/method/client_ip`：

```go
func (s *Server) handleXxx(c *gin.Context) {
    logger.FromGin(c).With("id", id).Info("操作成功")
    // 失败时：
    logger.FromGin(c).Error("查询失败: %v", err)
}
```

- `request_id` 由 `middleware.RequestID()` 中间件生成/透传（`X-Request-Id`），可跨访问日志与业务日志关联排查
- 非 HTTP 场景（后台任务）可用 `logger.FromContext(ctx)`，需先经 `logger.WithRequestID(ctx, id)` 注入

## 4. goroutine 崩溃兜底

所有长期运行的 goroutine（服务监听、配置热加载、定时任务）必须在入口 `defer logger.Recover("名字")`，panic 时记录完整堆栈而非崩溃整个进程：

```go
go func() {
    defer logger.Recover("dhcp-serve")
    if err := srv.Serve(); err != nil {
        logger.Error("DHCP Serve 异常退出: %v", err)
    }
}()
```

## 5. 错误处理约定

- **禁止**静默吞错：`_ = db.Xxx()`、`_, _ = DB.Exec(...)` 这类忽略必须在关键路径中止返回、降级路径记录 `Warn`
- 资源关闭（`Close()`）、响应写出（`c.AbortWithError`）、忽略替换计数等常规惯例可保留 `_ =`
- 关键路径错误向调用方返回时使用 `fmt.Errorf("...: %w", err)` 包裹上下文，方便上层定位
- `Error` 级别日志自动附带 `source` 字段（`文件:行号`），可直接定位代码位置

## 6. 敏感信息规范

- **严禁**在日志中记录明文密码、token、密钥
- 涉及敏感字段只记录存在性等布尔信息：

```go
// 正确：仅记录是否设置了密码
logger.FromGin(c).With("name", req.Name, "has_root_password", req.RootPassword != "").Info("创建 KS 模板")
```

- 数据库层面密码存储走 `util.EncryptPassword` 加密，日志层面同样禁止输出

## 7. 输出示例

控制台（text）：

```
time=2026-08-12 10:00:00 level=INFO msg=HTTP 请求完成 path=/api/hosts method=GET client_ip=192.168.1.5 request_id=9f3a2b1c status=200 cost_ms=12
```

文件（JSON）：

```json
{"time":"2026-08-12T10:00:00.123+08:00","level":"INFO","msg":"HTTP 请求完成","path":"/api/hosts","method":"GET","client_ip":"192.168.1.5","request_id":"9f3a2b1c","status":200,"cost_ms":12}
```
