package logger

import (
	"bytes"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/gin-gonic/gin"
)

// mockWriter 内存日志写入器，用于断言日志输出。
type mockWriter struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (m *mockWriter) Write(p []byte) (int, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.buf.Write(p)
}

func (m *mockWriter) String() string {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.buf.String()
}

// setupLogger 将控制台输出替换为 mockWriter 并重建日志器，测试结束后恢复。
func setupLogger(t *testing.T, level Level) *mockWriter {
	t.Helper()
	mw := &mockWriter{}
	mu.Lock()
	consoleOut = mw
	mu.Unlock()
	if err := Init("", 0, 0, level); err != nil {
		t.Fatalf("init logger: %v", err)
	}
	t.Cleanup(func() {
		mu.Lock()
		consoleOut = os.Stdout
		mu.Unlock()
		_ = Init("", 0, 0, LevelInfo)
	})
	return mw
}

func TestLogLevelFilter(t *testing.T) {
	mw := setupLogger(t, LevelWarn)
	Debug("debug-msg")
	Info("info-msg")
	Warn("warn-msg")
	Error("error-msg")
	out := mw.String()
	if strings.Contains(out, "debug-msg") || strings.Contains(out, "info-msg") {
		t.Errorf("debug/info should be filtered out, got: %s", out)
	}
	if !strings.Contains(out, "warn-msg") || !strings.Contains(out, "error-msg") {
		t.Errorf("warn/error should be logged, got: %s", out)
	}
}

func TestLogFormat(t *testing.T) {
	mw := setupLogger(t, LevelDebug)
	Info("hello %s", "world")
	out := mw.String()
	if !strings.Contains(out, "hello world") {
		t.Errorf("message missing, got: %s", out)
	}
	if !strings.Contains(out, "level=INFO") {
		t.Errorf("level attr missing, got: %s", out)
	}
	if !strings.Contains(out, "time=") {
		t.Errorf("time attr missing, got: %s", out)
	}
	if !strings.HasSuffix(out, "\n") {
		t.Error("log should end with newline")
	}
}

func TestErrorSource(t *testing.T) {
	mw := setupLogger(t, LevelDebug)
	Error("boom")
	out := mw.String()
	if !strings.Contains(out, "source=logger_test.go:") {
		t.Errorf("error should carry source location, got: %s", out)
	}
}

func TestSetLevel(t *testing.T) {
	mw := setupLogger(t, LevelError)
	SetLevel(LevelDebug)
	Debug("now-visible")
	if !strings.Contains(mw.String(), "now-visible") {
		t.Error("SetLevel should raise visibility")
	}
	SetLevel(LevelError)
}

func TestWith(t *testing.T) {
	mw := setupLogger(t, LevelDebug)
	With("mac", "aa:bb:cc", "ip", "10.0.0.1").Info("device-seen")
	out := mw.String()
	if !strings.Contains(out, "mac=aa:bb:cc") || !strings.Contains(out, "ip=10.0.0.1") {
		t.Errorf("With fields missing, got: %s", out)
	}
}

func TestFromContext(t *testing.T) {
	mw := setupLogger(t, LevelDebug)
	ctx := WithRequestID(t.Context(), "req-abc")
	FromContext(ctx).Info("ctx-log")
	out := mw.String()
	if !strings.Contains(out, "request_id=req-abc") {
		t.Errorf("request_id missing, got: %s", out)
	}
}

func TestFromGin(t *testing.T) {
	mw := setupLogger(t, LevelDebug)
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest("GET", "/api/hosts", nil)
	c.Set(RequestIDKey, "req-gin")
	FromGin(c).Info("gin-log")
	out := mw.String()
	if !strings.Contains(out, "req-gin") || !strings.Contains(out, "/api/hosts") {
		t.Errorf("gin context fields missing, got: %s", out)
	}
}

func TestRecover(t *testing.T) {
	mw := setupLogger(t, LevelDebug)
	func() {
		defer Recover("test-goroutine")
		panic("boom")
	}()
	out := mw.String()
	if !strings.Contains(out, "test-goroutine") || !strings.Contains(out, "boom") {
		t.Errorf("panic not recorded, got: %s", out)
	}
	if !strings.Contains(out, "goroutine test-goroutine panic") {
		t.Errorf("panic message missing, got: %s", out)
	}
}

func TestInitWithFile(t *testing.T) {
	mw := setupLogger(t, LevelInfo)
	dir := t.TempDir()
	filePath := filepath.Join(dir, "logs", "test.log")
	if err := Init(filePath, 1, 2, LevelInfo); err != nil {
		t.Fatalf("init with file: %v", err)
	}
	Info("file-log-test")

	// 关闭文件 writer，避免占用句柄导致 TempDir 无法清理
	mu.Lock()
	if fileOut != nil {
		_ = fileOut.Close()
		fileOut = nil
	}
	mu.Unlock()

	data, err := os.ReadFile(filePath)
	if err != nil {
		t.Fatalf("read log file: %v", err)
	}
	if !bytes.Contains(data, []byte("file-log-test")) {
		t.Errorf("log file content missing, got: %s", data)
	}
	if !bytes.Contains(data, []byte(`"level":"INFO"`)) {
		t.Errorf("file should be JSON formatted, got: %s", data)
	}
	// 控制台同步输出
	if !strings.Contains(mw.String(), "file-log-test") {
		t.Error("console should also receive the log")
	}
}

func TestInitDirFail(t *testing.T) {
	// 目录路径不可创建（文件占用目录名）时应返回 error 且不 panic
	dir := t.TempDir()
	blocker := filepath.Join(dir, "blocker")
	if err := os.WriteFile(blocker, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := Init(filepath.Join(blocker, "sub", "test.log"), 1, 2, LevelInfo); err == nil {
		t.Error("Init should return error when dir creation fails")
	}
	// 降级后仍可输出
	Info("still-working")
}
