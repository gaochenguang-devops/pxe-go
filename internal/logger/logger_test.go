package logger

import (
	"bytes"
	"os"
	"strings"
	"sync"
	"testing"
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

// 在测试内替换全局 writer，并在结束后恢复。
func setupLogger(t *testing.T, level Level) *mockWriter {
	t.Helper()
	mw := &mockWriter{}
	mu.Lock()
	origConsole := consoleOut
	consoleOut = mw
	writers = []writer{mw}
	minLevel = level
	mu.Unlock()
	t.Cleanup(func() {
		mu.Lock()
		consoleOut = origConsole
		writers = []writer{origConsole}
		minLevel = LevelInfo
		mu.Unlock()
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
	// 格式: [YYYY-MM-DD HH:MM:SS] [INFO] hello world
	if !strings.Contains(out, "[INFO] hello world") {
		t.Errorf("log format wrong, got: %s", out)
	}
	if !strings.HasPrefix(out, "[") {
		t.Errorf("log should start with timestamp, got: %s", out)
	}
	if !strings.HasSuffix(out, "\n") {
		t.Error("log should end with newline")
	}
}

func TestSetLevel(t *testing.T) {
	mw := setupLogger(t, LevelError)
	SetLevel(LevelDebug)
	Debug("now-visible")
	if !strings.Contains(mw.String(), "now-visible") {
		t.Error("SetLevel should raise visibility")
	}
}

func TestInitWithFile(t *testing.T) {
	dir := t.TempDir()
	filePath := dir + "/logs/test.log"
	// 初始化到临时文件，应不报错
	Init(filePath, 1, 2, LevelInfo)
	Info("file-log-test")
	// 关闭文件 writer，避免占用句柄导致 TempDir 无法清理
	mu.Lock()
	if fj, ok := fileOut.(interface{ Close() error }); ok {
		_ = fj.Close()
	}
	fileOut = nil
	writers = []writer{consoleOut}
	mu.Unlock()
	// 验证文件已写入
	data, err := os.ReadFile(filePath)
	if err != nil {
		t.Fatalf("read log file: %v", err)
	}
	if !strings.Contains(string(data), "file-log-test") {
		t.Errorf("log file content missing, got: %s", data)
	}
}
