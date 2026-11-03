package middleware

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jason127vip-dot/maohai-shuo/backend/internal/config"
	"github.com/jason127vip-dot/maohai-shuo/backend/internal/model"
	"github.com/jason127vip-dot/maohai-shuo/backend/internal/repository"
)

func sign(value, key string) string {
	h := hmac.New(sha256.New, []byte(key))
	h.Write([]byte(value))
	return hex.EncodeToString(h.Sum(nil))
}
func verify(value, key string) (string, bool) {
	parts := strings.Split(value, ".")
	if len(parts) != 3 {
		return "", false
	}
	if len(parts[0]) != 32 {
		return "", false
	}
	if _, err := hex.DecodeString(parts[0]); err != nil {
		return "", false
	}
	exp, err := strconv.ParseInt(parts[1], 10, 64)
	if err != nil || exp <= time.Now().Unix() {
		return "", false
	}
	expected := sign(parts[0]+"."+parts[1], key)
	if !hmac.Equal([]byte(expected), []byte(parts[2])) {
		return "", false
	}
	return parts[0], true
}

// DevelopmentSession is only wired after config validation restricts it to loopback.
// It is not a substitute for WeChat OpenID identity or production authentication.
func DevelopmentSession(cfg config.Config, store repository.Store) gin.HandlerFunc {
	return func(c *gin.Context) {
		token, _ := c.Cookie(cfg.Auth.CookieName)
		user, valid := verify(token, cfg.Auth.SigningKey)
		if !valid {
			var err error
			user, err = model.NewID()
			if err != nil {
				c.AbortWithStatusJSON(500, gin.H{"code": "SESSION_ERROR", "message": "会话创建失败"})
				return
			}
			expires := time.Now().Add(time.Duration(cfg.Auth.SessionHours) * time.Hour)
			value := user + "." + strconv.FormatInt(expires.Unix(), 10)
			value += "." + sign(value, cfg.Auth.SigningKey)
			http.SetCookie(c.Writer, &http.Cookie{Name: cfg.Auth.CookieName, Value: value, Path: "/api", HttpOnly: true, SameSite: http.SameSiteLaxMode, Expires: expires, MaxAge: cfg.Auth.SessionHours * 3600})
		}
		ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
		defer cancel()
		if store.EnsureUser(ctx, user, cfg.Quota.InitialCredits) != nil {
			c.AbortWithStatusJSON(503, gin.H{"code": "DATABASE_UNAVAILABLE", "message": "数据服务暂时不可用，请稍后再试"})
			return
		}
		c.Set("userID", user)
		c.Next()
	}
}
