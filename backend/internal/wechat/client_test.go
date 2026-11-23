package wechat

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
)

type transport func(*http.Request) (*http.Response, error)

func (f transport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestExchangeUsesBackendCredentialsAndValidatesResponse(t *testing.T) {
	for _, test := range []struct {
		name, body string
		status     int
		ok         bool
	}{
		{"success", `{"openid":"user-id","access_token":"secret-token","scope":"snsapi_base"}`, 200, true},
		{"wechat-error", `{"errcode":40029,"errmsg":"private diagnostic"}`, 200, false},
		{"missing-openid", `{"access_token":"token","scope":"snsapi_base"}`, 200, false},
		{"missing-token", `{"openid":"user-id","scope":"snsapi_base"}`, 200, false},
		{"wrong-scope", `{"openid":"user-id","access_token":"token","scope":"unknown"}`, 200, false},
		{"invalid-json", `<!doctype html>`, 200, false},
		{"redirect", ``, 302, false},
		{"upstream-503", `unavailable`, 503, false},
	} {
		t.Run(test.name, func(t *testing.T) {
			c := NewClient("test-app", "test-secret")
			c.HTTP.Transport = transport(func(r *http.Request) (*http.Response, error) {
				q := r.URL.Query()
				if r.URL.Scheme != "https" || r.URL.Host != "api.weixin.qq.com" || r.URL.Path != "/sns/oauth2/access_token" || q.Get("appid") != "test-app" || q.Get("secret") != "test-secret" || q.Get("code") != "once" || q.Get("grant_type") != "authorization_code" {
					t.Fatal("unexpected token exchange request")
				}
				return &http.Response{StatusCode: test.status, Body: io.NopCloser(strings.NewReader(test.body)), Header: make(http.Header)}, nil
			})
			id, err := c.Exchange(context.Background(), "once")
			if (err == nil) != test.ok || (test.ok && id != "user-id") {
				t.Fatalf("unexpected exchange outcome: %q %v", id, err)
			}
			if err != nil && (strings.Contains(err.Error(), "test-secret") || strings.Contains(err.Error(), "private diagnostic")) {
				t.Fatal("secrets leaked")
			}
		})
	}
}

func TestTransportErrorDoesNotExposeCredentialURL(t *testing.T) {
	c := NewClient("app", "do-not-log-secret")
	c.HTTP.Transport = transport(func(*http.Request) (*http.Response, error) {
		return nil, errors.New("transport error contains do-not-log-secret")
	})
	_, err := c.Exchange(context.Background(), "private-code")
	if err == nil || strings.Contains(err.Error(), "do-not-log-secret") || strings.Contains(err.Error(), "private-code") {
		t.Fatal("credential URL leaked", err)
	}
}

func TestProfileExchangeValidatesIdentityAndAvatar(t *testing.T) {
	for _, test := range []struct {
		name, body string
		ok         bool
	}{
		{"success", `{"openid":"owner","nickname":" 毛孩🐾 ","headimgurl":"http://thirdwx.qlogo.cn/avatar"}`, true},
		{"identity-mismatch", `{"openid":"other","nickname":"wrong"}`, false},
		{"error", `{"errcode":40003}`, false},
	} {
		t.Run(test.name, func(t *testing.T) {
			c := NewClient("app", "private-secret")
			c.HTTP.Transport = transport(func(r *http.Request) (*http.Response, error) {
				body := `{"openid":"owner","access_token":"private-token","scope":"snsapi_userinfo"}`
				if r.URL.Host != "api.weixin.qq.com" || r.URL.Scheme != "https" {
					t.Fatal("unexpected credential destination")
				}
				if r.URL.Path == "/sns/userinfo" {
					if r.URL.Query().Get("access_token") != "private-token" || r.URL.Query().Get("openid") != "owner" {
						t.Fatal("userinfo did not use exchanged identity")
					}
					body = test.body
				} else if r.URL.Path != "/sns/oauth2/access_token" {
					t.Fatal("unexpected endpoint")
				}
				return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(body))}, nil
			})
			id, profile, err := c.ExchangeProfile(context.Background(), "code")
			if (err == nil) != test.ok {
				t.Fatal("unexpected result", err)
			}
			if test.ok && (id != "owner" || profile.Nickname != "毛孩🐾" || profile.AvatarURL != "https://thirdwx.qlogo.cn/avatar") {
				t.Fatal("profile normalization failed", profile)
			}
		})
	}
	for _, raw := range []string{"javascript:alert(1)", "https://evil.test/image", "https://thirdwx.qlogo.cn.evil.test/image", "https://user@thirdwx.qlogo.cn/image", "https://thirdwx.qlogo.cn:8443/image"} {
		if safeAvatarURL(raw) != "" {
			t.Fatal("untrusted avatar URL accepted", raw)
		}
	}
}
