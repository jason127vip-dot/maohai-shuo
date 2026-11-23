// Package wechat implements Official Account login and optional profile authorization.
// OpenID is obtained server-to-server, never accepted from a browser parameter.
package wechat

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
	"unicode"

	"github.com/jason127vip-dot/maohai-shuo/backend/internal/model"
)

type Exchanger interface {
	Exchange(context.Context, string) (string, error)
}

type ProfileExchanger interface {
	ExchangeProfile(context.Context, string) (string, model.UserProfile, error)
}

type oauthToken struct {
	OpenID      string `json:"openid"`
	AccessToken string `json:"access_token"`
	Scope       string `json:"scope"`
	ErrorCode   int    `json:"errcode"`
}

type Client struct {
	AppID, AppSecret string
	HTTP             *http.Client
}

func NewClient(appID, secret string) *Client {
	return &Client{AppID: appID, AppSecret: secret, HTTP: &http.Client{
		Timeout:       10 * time.Second,
		CheckRedirect: func(_ *http.Request, _ []*http.Request) error { return http.ErrUseLastResponse },
	}}
}

func (c *Client) Exchange(ctx context.Context, code string) (string, error) {
	token, err := c.exchangeToken(ctx, code, "snsapi_base")
	return token.OpenID, err
}

func (c *Client) exchangeToken(ctx context.Context, code, scope string) (oauthToken, error) {
	q := url.Values{"appid": {c.AppID}, "secret": {c.AppSecret}, "code": {code}, "grant_type": {"authorization_code"}}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://api.weixin.qq.com/sns/oauth2/access_token?"+q.Encode(), nil)
	if err != nil {
		return oauthToken{}, errors.New("微信授权请求无法创建")
	}
	resp, err := c.HTTP.Do(req)
	// Transport errors may contain the full request URL (including AppSecret).
	if err != nil {
		return oauthToken{}, errors.New("微信授权服务暂时不可用")
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return oauthToken{}, errors.New("微信授权服务返回异常")
	}
	var body oauthToken
	decoder := json.NewDecoder(io.LimitReader(resp.Body, 32<<10))
	if decoder.Decode(&body) != nil || body.ErrorCode != 0 || body.OpenID == "" || len(body.OpenID) > 128 || body.AccessToken == "" || !hasScope(body.Scope, scope) {
		return oauthToken{}, errors.New("微信授权未完成，请重新登录")
	}
	return body, nil
}

func hasScope(scope, expected string) bool {
	for _, s := range strings.Split(scope, ",") {
		if s == expected {
			return true
		}
	}
	return false
}

func AuthorizeURL(appID, callback, state string) string {
	return authorizeURL(appID, callback, state, "snsapi_base")
}

func ProfileAuthorizeURL(appID, callback, state string) string {
	return authorizeURL(appID, callback, state, "snsapi_userinfo")
}

func authorizeURL(appID, callback, state, scope string) string {
	// Keep the parameter order used by the Official Account authorization URL.
	return "https://open.weixin.qq.com/connect/oauth2/authorize?appid=" + url.QueryEscape(appID) + "&redirect_uri=" + url.QueryEscape(callback) + "&response_type=code&scope=" + scope + "&state=" + url.QueryEscape(state) + "#wechat_redirect"
}

// OAuth tokens stay in this call and are never returned to the browser or stored.
func (c *Client) ExchangeProfile(ctx context.Context, code string) (string, model.UserProfile, error) {
	token, err := c.exchangeToken(ctx, code, "snsapi_userinfo")
	if err != nil {
		return "", model.UserProfile{}, err
	}
	q := url.Values{"access_token": {token.AccessToken}, "openid": {token.OpenID}, "lang": {"zh_CN"}}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://api.weixin.qq.com/sns/userinfo?"+q.Encode(), nil)
	if err != nil {
		return "", model.UserProfile{}, errors.New("微信资料请求无法创建")
	}
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return "", model.UserProfile{}, errors.New("微信资料暂时不可用")
	}
	defer resp.Body.Close()
	var body struct {
		OpenID    string `json:"openid"`
		Nickname  string `json:"nickname"`
		AvatarURL string `json:"headimgurl"`
		ErrorCode int    `json:"errcode"`
	}
	if resp.StatusCode != http.StatusOK || json.NewDecoder(io.LimitReader(resp.Body, 32<<10)).Decode(&body) != nil || body.ErrorCode != 0 || body.OpenID != token.OpenID {
		return "", model.UserProfile{}, errors.New("微信资料未能读取")
	}
	name := []rune(strings.TrimSpace(strings.Map(func(r rune) rune {
		if unicode.IsControl(r) {
			return -1
		}
		return r
	}, body.Nickname)))
	if len(name) > 64 {
		name = name[:64]
	}
	profile := model.UserProfile{Nickname: string(name), AvatarURL: safeAvatarURL(body.AvatarURL)}
	if profile.Nickname == "" && profile.AvatarURL == "" {
		return "", model.UserProfile{}, errors.New("微信资料为空，请稍后重试")
	}
	return token.OpenID, profile, nil
}

func safeAvatarURL(raw string) string {
	if raw == "" || len(raw) > 2048 {
		return ""
	}
	u, err := url.Parse(raw)
	if err != nil || u.User != nil || u.Port() != "" || (u.Scheme != "http" && u.Scheme != "https") {
		return ""
	}
	switch u.Host {
	case "thirdwx.qlogo.cn", "wx.qlogo.cn", "wx.qpic.cn":
		u.Scheme = "https"
		u.Fragment = ""
		return u.String()
	default:
		return ""
	}
}
