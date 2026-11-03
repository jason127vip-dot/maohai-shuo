package middleware

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jason127vip-dot/maohai-shuo/backend/internal/model"
)

func RequestLog(log *slog.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, err := model.NewID()
		if err != nil {
			id = "unavailable"
		}
		c.Header("X-Request-Id", id)
		start := time.Now()
		c.Next()
		log.Info("http_request", "request_id", id, "method", c.Request.Method, "route", c.FullPath(), "status", c.Writer.Status(), "elapsed_ms", time.Since(start).Milliseconds())
	}
}
func Recovery(log *slog.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			if recover() != nil {
				log.Error("http_handler_panic")
				c.AbortWithStatusJSON(500, gin.H{"code": "INTERNAL_ERROR", "message": "服务暂时不可用，请稍后再试"})
			}
		}()
		c.Next()
	}
}
func OriginCheck(allowed []string) gin.HandlerFunc {
	origins := map[string]bool{}
	for _, origin := range allowed {
		origins[origin] = true
	}
	return func(c *gin.Context) {
		if c.Request.Method == http.MethodPost {
			origin := c.GetHeader("Origin")
			if origin != "" && !origins[origin] {
				c.AbortWithStatusJSON(403, gin.H{"code": "ORIGIN_REJECTED", "message": "请求来源不被允许"})
				return
			}
		}
		c.Next()
	}
}
