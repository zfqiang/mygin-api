package middleware

import (
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
)

var rateLimitScript = redis.NewScript(`
local count = redis.call("INCR", KEYS[1])
if count == 1 then
  redis.call("PEXPIRE", KEYS[1], ARGV[1])
end
return {count, redis.call("PTTL", KEYS[1])}
`)

func RateLimit(client *redis.Client, limit int64, window time.Duration, logger *slog.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		now := time.Now()
		windowMillis := window.Milliseconds()
		nowMillis := now.UnixMilli()
		windowID := nowMillis / windowMillis
		windowTTL := windowMillis - nowMillis%windowMillis
		key := fmt.Sprintf("rate_limit:%s:%d", c.ClientIP(), windowID)

		result, err := rateLimitScript.Run(c.Request.Context(), client,
			[]string{key}, windowTTL).Slice()
		if err != nil {
			logger.Error("rate limit check failed", "error", err, "client_ip", c.ClientIP())
			c.AbortWithStatusJSON(http.StatusServiceUnavailable, gin.H{"error": "rate limiter unavailable"})
			return
		}
		if len(result) != 2 {
			logger.Error("rate limit returned unexpected result", "result", result)
			c.AbortWithStatusJSON(http.StatusServiceUnavailable, gin.H{"error": "rate limiter unavailable"})
			return
		}

		count, countErr := redisInt64(result[0])
		ttlMillis, ttlErr := redisInt64(result[1])
		if countErr != nil || ttlErr != nil {
			logger.Error("rate limit returned invalid result", "count_error", countErr, "ttl_error", ttlErr)
			c.AbortWithStatusJSON(http.StatusServiceUnavailable, gin.H{"error": "rate limiter unavailable"})
			return
		}
		if count > limit {
			retryAfter := (ttlMillis + int64(time.Second/time.Millisecond) - 1) /
				int64(time.Second/time.Millisecond)
			if retryAfter < 1 {
				retryAfter = 1
			}
			c.Header("Retry-After", strconv.FormatInt(retryAfter, 10))
			c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{"error": "rate limit exceeded"})
			return
		}

		c.Next()
	}
}

func redisInt64(value any) (int64, error) {
	switch typed := value.(type) {
	case int64:
		return typed, nil
	case string:
		return strconv.ParseInt(typed, 10, 64)
	case []byte:
		return strconv.ParseInt(string(typed), 10, 64)
	default:
		return 0, fmt.Errorf("unexpected Redis integer type %T", value)
	}
}
