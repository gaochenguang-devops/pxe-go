// Package logger 提供分级日志（控制台 + 文件按大小切割）。
package logger

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

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

var levelNames = map[Level]string{
	LevelDebug: "DEBUG",
	LevelInfo:  "INFO",
	LevelWarn:  "WARN",
	LevelError: "ERROR",
}

var (
	mu     sync.Mutex
	minLevel = LevelInfo
	writers []writer
)

type writer interface {
	Write(p []byte) (int, error)
}

var (
	consoleOut writer = os.Stdout
	fileOut    writer
)

// Init 初始化日志系统。
// filePath 为空则仅输出控制台。maxSizeMB 为单文件最大 MB 数，maxBackups 为保留备份数。
func Init(filePath string, maxSizeMB, maxBackups int, level Level) {
	mu.Lock()
	defer mu.Unlock()
	minLevel = level
	writers = writers[:0]
	writers = append(writers, consoleOut)
	if filePath != "" {
		if err := os.MkdirAll(filepath.Dir(filePath), 0755); err == nil {
			fileOut = &lumberjack.Logger{
				Filename:   filePath,
				MaxSize:    maxSizeMB,
				MaxBackups: maxBackups,
				MaxAge:     30,
				Compress:   true,
			}
			writers = append(writers, fileOut)
		}
	}
}

// SetLevel 动态调整日志级别。
func SetLevel(level Level) {
	mu.Lock()
	defer mu.Unlock()
	minLevel = level
}

func logf(lv Level, format string, args ...any) {
	if lv < minLevel {
		return
	}
	msg := fmt.Sprintf(format, args...)
	ts := time.Now().Format("2006-01-02 15:04:05")
	line := fmt.Sprintf("[%s] [%s] %s\n", ts, levelNames[lv], msg)
	mu.Lock()
	defer mu.Unlock()
	for _, w := range writers {
		w.Write([]byte(line))
	}
}

// Debug 输出调试日志。
func Debug(format string, args ...any) { logf(LevelDebug, format, args...) }

// Info 输出信息日志。
func Info(format string, args ...any) { logf(LevelInfo, format, args...) }

// Warn 输出警告日志。
func Warn(format string, args ...any) { logf(LevelWarn, format, args...) }

// Error 输出错误日志。
func Error(format string, args ...any) { logf(LevelError, format, args...) }
