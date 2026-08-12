// Package middleware 提供登录鉴权、跨域、请求日志、接口限流等 Gin 中间件。
package middleware

import (
	crand "crypto/rand"
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"

	"pxe-server/internal/logger"
)

// 会话 token 存储（内存态，重启失效；生产可扩展为持久化）。
var (
	tokenMu     sync.RWMutex
	tokenSess   = map[string]time.Time{}
	tokenExpire = 24 * time.Hour
)

// IssueToken 签发登录 token。
func IssueToken() string {
	token := randToken()
	tokenMu.Lock()
	tokenSess[token] = time.Now().Add(tokenExpire)
	tokenMu.Unlock()
	return token
}

// InvalidateToken 注销 token。
func InvalidateToken(token string) {
	tokenMu.Lock()
	delete(tokenSess, token)
	tokenMu.Unlock()
}

// ValidToken 校验 token 是否有效。
func ValidToken(token string) bool {
	tokenMu.RLock()
	exp, ok := tokenSess[token]
	tokenMu.RUnlock()
	if !ok {
		return false
	}
	if time.Now().After(exp) {
		tokenMu.Lock()
		delete(tokenSess, token)
		tokenMu.Unlock()
		return false
	}
	return true
}

func randToken() string {
	// 使用 crypto/rand 生成 32 字节随机数，保证并发下不碰撞且不可预测
	b := make([]byte, 24)
	if _, err := crand.Read(b); err != nil {
		// 极罕见失败时回退到时间 + 计数兜底，避免返回空 token
		seed := time.Now().UnixNano() + randFallbackCounter
		randFallbackCounter++
		for i := range b {
			seed = seed*1103515245 + 12345
			b[i] = byte(seed >> (i % 8 * 8) & 0xff)
		}
	}
	const chars = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	out := make([]byte, 32)
	for i := range out {
		out[i] = chars[int(b[i%len(b)])%len(chars)]
	}
	return string(out)
}

var randFallbackCounter int64

// AuthRequired 登录鉴权中间件。
func AuthRequired() gin.HandlerFunc {
	return func(c *gin.Context) {
		token := c.GetHeader("Authorization")
		if token == "" {
			token = c.Query("token")
		}
		if token == "" || !ValidToken(token) {
			c.JSON(http.StatusUnauthorized, gin.H{"code": 401, "msg": "未登录或登录已过期"})
			c.Abort()
			return
		}
		c.Next()
	}
}

// CORS 跨域中间件。
func CORS() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("Access-Control-Allow-Origin", "*")
		c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		c.Header("Access-Control-Allow-Headers", "Origin, Content-Type, Authorization")
		if c.Request.Method == http.MethodOptions {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}
		c.Next()
	}
}

// RequestLogger 请求日志中间件。
func RequestLogger() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		clientIP := ClientIP(c)
		c.Next()
		logger.Info("HTTP %s %s from %s status=%d cost=%v",
			c.Request.Method, c.Request.URL.Path, clientIP, c.Writer.Status(), time.Since(start))
	}
}

// ClientIP 获取客户端 IP。
func ClientIP(c *gin.Context) string {
	if ip := c.ClientIP(); ip != "" {
		return ip
	}
	return ""
}

// limiter 简单 IP 级限流器。
type limiter struct {
	mu      sync.Mutex
	visits  map[string]*visitInfo
	rate    int   // 窗口内允许次数
	window  time.Duration
	cleanAt time.Time
}

type visitInfo struct {
	count  int
	first  time.Time
	blocked time.Time
}

var ipLimiter = &limiter{
	visits: map[string]*visitInfo{},
	rate:   600,
	window: time.Minute,
}

// RateLimit 接口限流中间件（简单令牌桶，IP 维度）。
func RateLimit(rate int, window time.Duration) gin.HandlerFunc {
	return func(c *gin.Context) {
		if rate <= 0 {
			rate = ipLimiter.rate
		}
		if window <= 0 {
			window = ipLimiter.window
		}
		ip := ClientIP(c)
		now := time.Now()
		ipLimiter.mu.Lock()
		// 定期清理过期记录
		if now.After(ipLimiter.cleanAt) {
			for k, v := range ipLimiter.visits {
				if now.Sub(v.first) > window {
					delete(ipLimiter.visits, k)
				}
			}
			ipLimiter.cleanAt = now.Add(window)
		}
		v := ipLimiter.visits[ip]
		if v == nil {
			v = &visitInfo{first: now}
			ipLimiter.visits[ip] = v
		}
		if now.Sub(v.first) > window {
			v.first = now
			v.count = 0
		}
		v.count++
		over := v.count > rate
		ipLimiter.mu.Unlock()

		if over {
			c.JSON(http.StatusTooManyRequests, gin.H{"code": 429, "msg": "请求过于频繁"})
			c.Abort()
			return
		}
		c.Next()
	}
}
