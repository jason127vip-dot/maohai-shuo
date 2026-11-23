package handler

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"html"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jason127vip-dot/maohai-shuo/backend/internal/config"
	"github.com/jason127vip-dot/maohai-shuo/backend/internal/middleware"
	"github.com/jason127vip-dot/maohai-shuo/backend/internal/model"
	"github.com/jason127vip-dot/maohai-shuo/backend/internal/repository"
	"github.com/jason127vip-dot/maohai-shuo/backend/internal/wechat"
)

const oauthCookiePath = "/api/auth/wechat"

func digest(s string) string { h := sha256.Sum256([]byte(s)); return hex.EncodeToString(h[:]) }

func returnPath(raw string) (string, bool) {
	if raw == "" {
		return "/", true
	}
	if len(raw) > 2048 || strings.ContainsAny(raw, "\\\r\n\x00") {
		return "", false
	}
	u, err := url.Parse(raw)
	if err != nil || u.IsAbs() || u.Host != "" || u.Opaque != "" || u.RawPath != "" || !strings.HasPrefix(raw, "/") || strings.HasPrefix(raw, "//") {
		return "", false
	}
	valid := u.Path == "/" || u.Path == "/privacy" || u.Path == "/history" || u.Path == "/credits" || u.Path == "/pets" || u.Path == "/growth"
	if strings.HasPrefix(u.Path, "/generations/") {
		id := strings.TrimPrefix(u.Path, "/generations/")
		_, err := hex.DecodeString(id)
		valid = len(id) == 32 && err == nil
	}
	// Only application routes; discard tracking queries/fragments and OAuth codes.
	return u.Path, valid
}

func registerWechatAuth(router *gin.Engine, cfg config.Config, store repository.Store, exchange wechat.Exchanger, log *slog.Logger) {
	name := cfg.Auth.CookieName + "_oauth"
	cookie := func(c *gin.Context, value string, maxAge int) {
		expires := time.Now().Add(time.Duration(maxAge) * time.Second)
		http.SetCookie(c.Writer, &http.Cookie{Name: name, Value: value, Path: oauthCookiePath, HttpOnly: true, Secure: middleware.WechatCookieSecure(cfg), SameSite: http.SameSiteLaxMode, MaxAge: maxAge, Expires: expires})
	}
	headers := func(c *gin.Context) {
		c.Header("Cache-Control", "no-store")
		c.Header("Referrer-Policy", "no-referrer")
		c.Header("X-Content-Type-Options", "nosniff")
	}
	failure := func(c *gin.Context, status int, message string) {
		// No provider errors, OpenIDs, codes, state or secrets are rendered/logged.
		c.Header("Content-Security-Policy", "default-src 'none'; style-src 'unsafe-inline'; base-uri 'none'; frame-ancestors 'none'")
		c.Data(status, "text/html; charset=utf-8", []byte(`<!doctype html><html lang="zh-CN"><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"><title>微信登录</title><body style="padding:32px;font:18px sans-serif"><h1>暂时无法登录</h1><p>`+html.EscapeString(message)+`</p><a href="`+html.EscapeString(strings.TrimRight(cfg.Wechat.FrontendURL, "/")+"/")+`">返回毛孩儿说重新登录</a></body></html>`))
	}
	start := func(c *gin.Context) {
		headers(c)
		if !strings.Contains(strings.ToLower(c.GetHeader("User-Agent")), "micromessenger") {
			failure(c, 400, "请在微信内打开毛孩儿说后登录。")
			return
		}
		path, ok := returnPath(c.Query("return_to"))
		if !ok {
			c.JSON(400, gin.H{"code": "INVALID_RETURN_PATH", "message": "登录返回地址不正确"})
			return
		}
		state, err := model.NewID()
		if err != nil {
			failure(c, 500, "会话创建失败，请稍后重试。")
			return
		}
		browser, err := model.NewID()
		if err != nil {
			failure(c, 500, "会话创建失败，请稍后重试。")
			return
		}
		ctx, cancel := context.WithTimeout(c.Request.Context(), 5*time.Second)
		defer cancel()
		purpose := "login"
		if c.GetString("userID") != "" {
			purpose = "profile"
		}
		err = store.CreateOAuthState(ctx, repository.OAuthState{Key: digest(state), BrowserHash: digest(browser), ReturnPath: path, Purpose: purpose, UserID: c.GetString("userID"), ExpiresAt: time.Now().Add(10 * time.Minute)})
		if err != nil {
			failure(c, 503, "数据服务暂时不可用，请稍后重试。")
			return
		}
		cookie(c, browser, 600)
		if purpose == "profile" {
			c.Redirect(http.StatusFound, wechat.ProfileAuthorizeURL(cfg.Wechat.AppID, cfg.Wechat.CallbackURL, state))
			return
		}
		c.Redirect(http.StatusFound, wechat.AuthorizeURL(cfg.Wechat.AppID, cfg.Wechat.CallbackURL, state))
	}
	router.GET(oauthCookiePath+"/start", start)
	router.GET(oauthCookiePath+"/profile/start", middleware.WechatSession(cfg, store), start)
	router.GET(oauthCookiePath+"/callback", func(c *gin.Context) {
		headers(c)
		state := c.Query("state")
		browser, _ := c.Cookie(name)
		if len(state) != 32 || len(browser) != 32 {
			failure(c, 400, "登录已过期，请返回重新登录。")
			return
		}
		ctx, cancel := context.WithTimeout(c.Request.Context(), 20*time.Second)
		defer cancel()
		pending, err := store.ConsumeOAuthState(ctx, digest(state), digest(browser))
		if err != nil {
			if errors.Is(err, model.ErrNotFound) {
				failure(c, 400, "登录已过期或已使用，请返回重新登录。")
			} else {
				failure(c, 503, "数据服务暂时不可用，请稍后重试。")
			}
			return
		}
		cookie(c, "", -1)
		code := c.Query("code")
		if pending.Purpose == "profile" {
			// The stored user ID binds this optional authorization to the account
			// that requested it. Never register/rebind an account in this branch.
			result := "skipped"
			profileExchange, supported := exchange.(wechat.ProfileExchanger)
			if code != "" && len(code) <= 512 && pending.UserID != "" && supported {
				openID, profile, err := profileExchange.ExchangeProfile(ctx, code)
				if err == nil && openID != "" && len(openID) <= 128 {
					err = store.SaveWechatProfile(ctx, pending.UserID, cfg.Wechat.AppID, openID, profile)
				}
				result = "failed"
				if err == nil && openID != "" && len(openID) <= 128 {
					result = "synced"
				}
			}
			path, ok := returnPath(pending.ReturnPath)
			if !ok {
				path = "/history"
			}
			c.Redirect(http.StatusSeeOther, strings.TrimRight(cfg.Wechat.FrontendURL, "/")+path+"?wechat_profile="+result)
			return
		}
		if pending.Purpose != "" && pending.Purpose != "login" {
			failure(c, 400, "授权类型无效，请返回重试。")
			return
		}
		if code == "" || len(code) > 512 {
			failure(c, 400, "微信授权未完成，请返回重新登录。")
			return
		}
		openID, err := exchange.Exchange(ctx, code)
		if err != nil {
			log.Warn("wechat_exchange_failed")
			failure(c, 502, "微信授权暂未完成，请返回重新登录。")
			return
		}
		if openID == "" || len(openID) > 128 {
			failure(c, 502, "微信授权返回异常，请返回重新登录。")
			return
		}
		id, err := store.ResolveWechatUser(ctx, cfg.Wechat.AppID, openID, cfg.Quota.InitialCredits)
		if err != nil {
			log.Warn("wechat_identity_failed")
			failure(c, 503, "账号暂时无法读取，请返回重新登录。")
			return
		}
		path, ok := returnPath(pending.ReturnPath)
		if !ok {
			path = "/"
		}
		middleware.SetWechatSession(c.Writer, cfg, id)
		c.Redirect(http.StatusSeeOther, strings.TrimRight(cfg.Wechat.FrontendURL, "/")+path)
	})
}
