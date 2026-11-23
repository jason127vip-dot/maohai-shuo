package middleware

import (
	"context"
	"errors"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jason127vip-dot/maohai-shuo/backend/internal/config"
	"github.com/jason127vip-dot/maohai-shuo/backend/internal/model"
	"github.com/jason127vip-dot/maohai-shuo/backend/internal/repository"
)

func WechatCookieSecure(cfg config.Config) bool {
	callback, err := url.Parse(cfg.Wechat.CallbackURL)
	return err == nil && callback.Scheme == "https"
}

func SetWechatSession(w http.ResponseWriter, cfg config.Config, id string) {
	expires := time.Now().Add(time.Duration(cfg.Auth.SessionHours) * time.Hour)
	value := id + "." + strconv.FormatInt(expires.Unix(), 10)
	// A different signing purpose prevents development cookies being trusted as
	// logged-in WeChat sessions even if an upgrade retains the old signing key.
	token := "wx." + value + "." + sign(value, "wechat-session:"+cfg.Wechat.AppID+":"+cfg.Auth.SigningKey)
	http.SetCookie(w, &http.Cookie{Name: cfg.Auth.CookieName, Value: token, Path: "/api", HttpOnly: true, Secure: WechatCookieSecure(cfg), SameSite: http.SameSiteLaxMode, Expires: expires, MaxAge: cfg.Auth.SessionHours * 3600})
}

func WechatSession(cfg config.Config, store repository.IdentityStore) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("Cache-Control", "private, no-store")
		token, _ := c.Cookie(cfg.Auth.CookieName)
		id, valid := verify(strings.TrimPrefix(token, "wx."), "wechat-session:"+cfg.Wechat.AppID+":"+cfg.Auth.SigningKey)
		if !strings.HasPrefix(token, "wx.") || !valid {
			c.AbortWithStatusJSON(401, gin.H{"code": "WECHAT_LOGIN_REQUIRED", "message": "请在微信中登录后继续"})
			return
		}
		ctx, cancel := context.WithTimeout(c.Request.Context(), 5*time.Second)
		defer cancel()
		err := store.TouchWechatUser(ctx, id, cfg.Wechat.AppID)
		if errors.Is(err, model.ErrNotFound) {
			c.AbortWithStatusJSON(401, gin.H{"code": "WECHAT_LOGIN_REQUIRED", "message": "请重新使用微信登录"})
			return
		}
		if err != nil {
			c.AbortWithStatusJSON(503, gin.H{"code": "DATABASE_UNAVAILABLE", "message": "数据服务暂时不可用，请稍后再试"})
			return
		}
		c.Set("userID", id)
		c.Next()
	}
}
