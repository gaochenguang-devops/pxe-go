package middleware

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
)

func TestTokenLifecycle(t *testing.T) {
	gin.SetMode(gin.TestMode)
	// 清理全局 token 状态，避免测试间干扰
	tokenMu.Lock()
	tokenSess = map[string]time.Time{}
	tokenMu.Unlock()

	tk := IssueToken()
	if tk == "" {
		t.Fatal("IssueToken returned empty")
	}
	if !ValidToken(tk) {
		t.Error("valid token should pass")
	}
	// 不存在的 token 无效
	if ValidToken("nonexistent") {
		t.Error("nonexistent token should be invalid")
	}
	// 注销后失效
	InvalidateToken(tk)
	if ValidToken(tk) {
		t.Error("invalidated token should be invalid")
	}
}

func TestRandTokenFormat(t *testing.T) {
	const chars = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	tk := randToken()
	if len(tk) != 32 {
		t.Errorf("randToken length = %d, want 32", len(tk))
	}
	for _, c := range tk {
		if !strings.ContainsRune(chars, c) {
			t.Errorf("randToken contains invalid char: %q", c)
		}
	}
	// 多次生成应不同
	tk2 := randToken()
	if tk == tk2 {
		t.Error("randToken should produce different tokens")
	}
}

func TestCORS(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(CORS())
	r.GET("/test", func(c *gin.Context) { c.String(http.StatusOK, "ok") })

	// OPTIONS 预检请求应返回 204
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodOptions, "/test", nil)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusNoContent {
		t.Errorf("OPTIONS status = %d, want 204", w.Code)
	}

	// 正常 GET 应带 CORS 头
	w2 := httptest.NewRecorder()
	req2 := httptest.NewRequest(http.MethodGet, "/test", nil)
	r.ServeHTTP(w2, req2)
	if w2.Code != http.StatusOK {
		t.Errorf("GET status = %d, want 200", w2.Code)
	}
	if w2.Header().Get("Access-Control-Allow-Origin") != "*" {
		t.Error("missing CORS allow-origin header")
	}
}

func TestAuthRequired(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(AuthRequired())
	r.GET("/protected", func(c *gin.Context) { c.String(http.StatusOK, "ok") })

	// 无 token → 401
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Errorf("no token status = %d, want 401", w.Code)
	}

	// 有效 token → 200
	tk := IssueToken()
	defer InvalidateToken(tk)
	w2 := httptest.NewRecorder()
	req2 := httptest.NewRequest(http.MethodGet, "/protected", nil)
	req2.Header.Set("Authorization", tk)
	r.ServeHTTP(w2, req2)
	if w2.Code != http.StatusOK {
		t.Errorf("valid token status = %d, want 200", w2.Code)
	}
}

func TestRateLimit(t *testing.T) {
	gin.SetMode(gin.TestMode)
	// 重置全局限流器，避免测试间干扰
	ipLimiter.mu.Lock()
	ipLimiter.visits = map[string]*visitInfo{}
	ipLimiter.mu.Unlock()

	r := gin.New()
	r.Use(RateLimit(2, 60000000000)) // 窗口 60s，rate=2
	r.GET("/limit", func(c *gin.Context) { c.String(http.StatusOK, "ok") })

	// 前 2 次通过
	for i := 0; i < 2; i++ {
		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/limit", nil)
		req.RemoteAddr = "1.2.3.4:1234"
		r.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Errorf("request %d status = %d, want 200", i, w.Code)
		}
	}
	// 第 3 次被限流
	w3 := httptest.NewRecorder()
	req3 := httptest.NewRequest(http.MethodGet, "/limit", nil)
	req3.RemoteAddr = "1.2.3.4:1234"
	r.ServeHTTP(w3, req3)
	if w3.Code != http.StatusTooManyRequests {
		t.Errorf("3rd request status = %d, want 429", w3.Code)
	}
}

// 确保并发签发 token 不产生竞态。
func TestIssueTokenConcurrent(t *testing.T) {
	gin.SetMode(gin.TestMode)
	var wg sync.WaitGroup
	seen := make(map[string]bool)
	var mu sync.Mutex
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			tk := IssueToken()
			mu.Lock()
			defer mu.Unlock()
			if seen[tk] {
				t.Error("duplicate token generated")
			}
			seen[tk] = true
		}()
	}
	wg.Wait()
}
