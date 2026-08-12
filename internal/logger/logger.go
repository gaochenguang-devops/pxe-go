// Package logger 提供结构化分级日志（控制台文本 + 文件 JSON 双输出）。
//
// 基础设施基于标准库 log/slog 构建，兼容旧版调用方式：
//
//	logger.Info("创建记录: name=%s id=%d", name, id)
//
// 同时支持结构化字段、请求上下文关联与 goroutine 崩溃兜底：
//
//	logger.With("mac", mac, "ip", ip).Info("DHCP 请求")
//	defer logger.Recover("dhcp-serve") // 捕获 panic 记录堆栈
//	log := logger.FromGin(c)           // 自动携带 request_id/path/method/client_ip
package logger

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"sync/atomic"

	"github.com/gin-gonic/gin"
	"gopkg.in/natefinch/lumberjack.v2"
)

// Level 日志级别。
type Level int

const (
	LevelDebug Level = iota
	LevelInfo
	LevelWarn
	LevelError
)

func (l Level) slog() slog.Level {
	switch l {
	case LevelDebug:
		return slog.LevelDebug
	case LevelWarn:
		return slog.LevelWarn
	case LevelError:
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}

// RequestIDKey 是请求追踪 ID 在 gin.Context / context.Context 中使用的 key。
// 中间件与业务代码统一通过该 key 读写 request_id。
const RequestIDKey = "X-Request-Id"

// Logger 是带结构化字段的日志器，通过 Init / With / FromGin / FromContext 获取。
type Logger struct {
	sl *slog.Logger
}

var (
	mu         sync.Mutex
	levelVar   = new(slog.LevelVar) // 全局动态级别，所有 handler 共享
	global     atomic.Pointer[Logger]
	consoleOut io.Writer = os.Stdout
	fileOut    io.Closer
)

func init() {
	global.Store(newSlogLogger(nil, LevelInfo))
}

// Init 初始化日志系统。
// filePath 为空则仅输出控制台；maxSizeMB 为单文件最大 MB 数，maxBackups 为保留备份数。
// 目录创建失败时返回 error，此时仍保持控制台输出可用。
func Init(filePath string, maxSizeMB, maxBackups int, level Level) error {
	mu.Lock()
	defer mu.Unlock()

	// 关闭旧文件 writer，避免多次 Init 造成句柄泄漏
	if fileOut != nil {
		_ = fileOut.Close()
		fileOut = nil
	}

	var file io.Writer
	if filePath != "" {
		if err := os.MkdirAll(filepath.Dir(filePath), 0o755); err != nil {
			global.Store(newSlogLogger(nil, level))
			return fmt.Errorf("logger.Init: 创建日志目录失败: %w", err)
		}
		lj := &lumberjack.Logger{
			Filename:   filePath,
			MaxSize:    maxSizeMB,
			MaxBackups: maxBackups,
			MaxAge:     30,
			Compress:   true,
		}
		file = lj
		fileOut = lj
	}
	global.Store(newSlogLogger(file, level))
	return nil
}

// SetLevel 动态调整日志级别。
func SetLevel(level Level) {
	levelVar.Set(level.slog())
}

// newSlogLogger 基于全局输出端构建 slog.Logger，控制台走 text、文件走 JSON。
func newSlogLogger(file io.Writer, level Level) *Logger {
	levelVar.Set(level.slog())
	handlers := []slog.Handler{newTextHandler(consoleOut)}
	if file != nil {
		handlers = append(handlers, newJSONHandler(file))
	}
	return &Logger{sl: slog.New(&multiHandler{handlers: handlers, level: levelVar})}
}

// 控制台时间格式为人类可读；文件 JSON 保留标准 RFC3339 便于机器解析。
func replaceTime(_ []string, a slog.Attr) slog.Attr {
	if a.Key == slog.TimeKey {
		a.Value = slog.StringValue(a.Value.Time().Format("2006-01-02 15:04:05"))
	}
	return a
}

func newTextHandler(w io.Writer) slog.Handler {
	return slog.NewTextHandler(w, &slog.HandlerOptions{
		Level:       levelVar,
		ReplaceAttr: replaceTime,
	})
}

func newJSONHandler(w io.Writer) slog.Handler {
	return slog.NewJSONHandler(w, &slog.HandlerOptions{Level: levelVar})
}

// multiHandler 将同一日志记录分发到多个 handler（控制台 + 文件）。
// 级别过滤统一由共享的 levelVar 决定。
type multiHandler struct {
	handlers []slog.Handler
	level    *slog.LevelVar
}

func (h *multiHandler) Enabled(ctx context.Context, lv slog.Level) bool {
	return lv >= h.level.Level()
}

func (h *multiHandler) Handle(ctx context.Context, r slog.Record) error {
	for _, hh := range h.handlers {
		if err := hh.Handle(ctx, r); err != nil {
			return err
		}
	}
	return nil
}

func (h *multiHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	handlers := make([]slog.Handler, len(h.handlers))
	for i, hh := range h.handlers {
		handlers[i] = hh.WithAttrs(attrs)
	}
	return &multiHandler{handlers: handlers, level: h.level}
}

func (h *multiHandler) WithGroup(name string) slog.Handler {
	handlers := make([]slog.Handler, len(h.handlers))
	for i, hh := range h.handlers {
		handlers[i] = hh.WithGroup(name)
	}
	return &multiHandler{handlers: handlers, level: h.level}
}

// caller 返回调用位置 "文件:行号"，用于错误日志定位。
// skip=3 时定位到业务调用点（logger.Info / l.Info 均为单层包装）。
func caller(skip int) string {
	_, file, line, ok := runtime.Caller(skip)
	if !ok {
		return ""
	}
	return fmt.Sprintf("%s:%d", filepath.Base(file), line)
}

func (l *Logger) logf(lv slog.Level, skip int, format string, args ...any) {
	sl := l.sl
	if sl == nil || !sl.Enabled(context.Background(), lv) {
		return
	}
	msg := fmt.Sprintf(format, args...)
	if lv >= slog.LevelError {
		sl.LogAttrs(context.Background(), lv, msg, slog.String("source", caller(skip)))
		return
	}
	sl.Log(context.Background(), lv, msg)
}

// Debug 输出调试日志。
func (l *Logger) Debug(format string, args ...any) { l.logf(slog.LevelDebug, 3, format, args...) }

// Info 输出信息日志。
func (l *Logger) Info(format string, args ...any) { l.logf(slog.LevelInfo, 3, format, args...) }

// Warn 输出警告日志。
func (l *Logger) Warn(format string, args ...any) { l.logf(slog.LevelWarn, 3, format, args...) }

// Error 输出错误日志，并附带调用位置 source 字段。
func (l *Logger) Error(format string, args ...any) { l.logf(slog.LevelError, 3, format, args...) }

// With 返回携带附加字段的派生日志器（键值对成对出现，如 With("mac", m, "ip", i)）。
func (l *Logger) With(kv ...any) *Logger {
	if l.sl == nil {
		return global.Load()
	}
	return &Logger{sl: l.sl.With(kv...)}
}

// Debug 输出调试日志。
func Debug(format string, args ...any) { global.Load().logf(slog.LevelDebug, 3, format, args...) }

// Info 输出信息日志。
func Info(format string, args ...any) { global.Load().logf(slog.LevelInfo, 3, format, args...) }

// Warn 输出警告日志。
func Warn(format string, args ...any) { global.Load().logf(slog.LevelWarn, 3, format, args...) }

// Error 输出错误日志，并附带调用位置 source 字段。
func Error(format string, args ...any) { global.Load().logf(slog.LevelError, 3, format, args...) }

// With 返回携带附加字段的全局派生日志器。
func With(kv ...any) *Logger { return global.Load().With(kv...) }

// Recover 用于 goroutine 崩溃兜底：defer logger.Recover("goroutine 名") 捕获 panic
// 并记录完整堆栈，避免未捕获 panic 导致整个进程崩溃。
func Recover(name string) {
	if r := recover(); r != nil {
		buf := make([]byte, 64<<10)
		n := runtime.Stack(buf, false)
		sl := global.Load().sl
		if sl != nil {
			sl.LogAttrs(context.Background(), slog.LevelError,
				fmt.Sprintf("goroutine %s panic", name),
				slog.Any("panic", r),
				slog.String("stack", string(buf[:n])),
			)
		}
	}
}

type ctxKey struct{}

// WithRequestID 将 request ID 写入 context，供非 HTTP 场景透传追踪。
func WithRequestID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, ctxKey{}, id)
}

// RequestIDFrom 从 context 读取 request ID，未设置时返回空串。
func RequestIDFrom(ctx context.Context) string {
	if id, ok := ctx.Value(ctxKey{}).(string); ok {
		return id
	}
	return ""
}

// FromContext 返回携带请求上下文字段的日志器（request_id）。
func FromContext(ctx context.Context) *Logger {
	l := global.Load()
	if id := RequestIDFrom(ctx); id != "" {
		l = l.With("request_id", id)
	}
	return l
}

// FromGin 返回携带当前 HTTP 请求上下文字段的日志器（request_id/path/method/client_ip）。
// 需在 RequestID 中间件之后使用，否则 request_id 为空。
func FromGin(c *gin.Context) *Logger {
	l := global.Load()
	if c == nil {
		return l
	}
	if id, ok := c.Get(RequestIDKey); ok {
		if s, ok := id.(string); ok && s != "" {
			l = l.With("request_id", s)
		}
	}
	return l.With(
		"path", c.Request.URL.Path,
		"method", c.Request.Method,
		"client_ip", c.ClientIP(),
	)
}
