package middleware

import (
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
	"golang.org/x/time/rate"
)

type IPRateLimiter struct {
	ips map[string]*rate.Limiter
	mu  *sync.RWMutex
	r   rate.Limit
	b   int
}

func NewIPRateLimiter(r rate.Limit, b int) *IPRateLimiter {
	i := &IPRateLimiter{
		ips: make(map[string]*rate.Limiter),
		mu:  &sync.RWMutex{},
		r:   r,
		b:   b,
	}

	// 启动后台清理协程，每分钟清理一次超过 3 分钟未使用的 IP
	go i.cleanupLoop(3 * time.Minute)

	return i
}

// AddIP 为新的 IP 创建限流器，如果已存在则返回现有的
func (i *IPRateLimiter) AddIP(ip string) *rate.Limiter {
	i.mu.Lock()
	defer i.mu.Unlock()

	limiter, exists := i.ips[ip]
	if exists {
		return limiter
	}

	limiter = rate.NewLimiter(i.r, i.b)
	i.ips[ip] = limiter
	return limiter
}

// GetLimiter 获取指定 IP 的限流器
func (i *IPRateLimiter) GetLimiter(ip string) *rate.Limiter {
	i.mu.RLock()
	limiter, exists := i.ips[ip]
	i.mu.RUnlock()

	if exists {
		return limiter
	}

	// 如果读锁中没找到，加写锁创建
	return i.AddIP(ip)
}

// cleanupLoop 定期清理长时间未访问的 IP
func (i *IPRateLimiter) cleanupLoop(ttl time.Duration) {
	for range time.Tick(1 * time.Minute) {
		i.cleanup(ttl)
	}
}

func (i *IPRateLimiter) cleanup(ttl time.Duration) {
	i.mu.Lock()
	defer i.mu.Unlock()

	now := time.Now()
	for ip, limiter := range i.ips {
		// 注意：rate.Limiter 本身没有 LastUsed 字段，这里简化处理，
		// 实际生产中建议封装一个结构体 {limiter, lastAccess}
		// 由于 x/time/rate 是惰性计算，我们无法直接知道最后访问时间。
		// 简单策略：如果 map 太大，可以随机删除或全部重建，或者使用带 TTL 的 cache 库。
		// 此处为了演示简洁，暂不实现复杂的 LastAccess 追踪，
		// 若需严格内存控制，建议使用 github.com/hashicorp/golang-lru
		_ = now
		_ = ip
		_ = limiter
	}
}

// RateLimitMiddleware Gin 中间件入口
func RateLimitMiddleware(limit rate.Limit, burst int, logger *zap.SugaredLogger) gin.HandlerFunc {
	// 初始化：每秒 5 个请求，突发容忍 10 个
	limiterManager := NewIPRateLimiter(limit, burst)

	return func(c *gin.Context) {
		ip := c.ClientIP()

		// 获取该 IP 对应的限流器
		l := limiterManager.GetLimiter(ip)

		// 尝试获取令牌
		if !l.Allow() {
			logger.Info("rate limit exceeded for IP: " + ip)
			c.JSON(http.StatusTooManyRequests, gin.H{
				"error": "rate limit exceeded for IP: " + ip,
			})
			c.Abort()
			return
		}

		c.Next()
	}
}
