package handler

import (
	"context"
	"net/url"
	"strings"
	"testing"
)

func TestOptionalProfileBoundToExistingIdentity(t *testing.T) {
	for _, test := range []struct {
		name, code, result string
		fail               bool
	}{
		{"success", "owner", "synced", false},
		{"cancel", "", "skipped", false},
		{"different-wechat", "other", "failed", false},
		{"upstream-error", "owner", "failed", true},
	} {
		t.Run(test.name, func(t *testing.T) {
			f := loginSetup(t)
			if w := f.request("/api/auth/wechat/profile/start?return_to=%2Fhistory"); w.Code != 401 {
				t.Fatal("profile sync did not require login")
			}
			session := f.login(t, "owner")
			w := f.request("/api/auth/wechat/profile/start?return_to=%2Fhistory", session)
			if w.Code != 302 {
				t.Fatal(w.Code, w.Body.String())
			}
			target, err := url.Parse(w.Header().Get("Location"))
			if err != nil || target.Host != "open.weixin.qq.com" || target.Query().Get("scope") != "snsapi_userinfo" {
				t.Fatal("wrong profile authorization scope")
			}
			browser := w.Result().Cookies()[0]
			path := "/api/auth/wechat/callback?state=" + target.Query().Get("state") + "&code=" + test.code
			f.exchange.fail = test.fail
			callback := f.request(path, browser)
			if callback.Code != 303 || callback.Header().Get("Location") != f.cfg.Wechat.FrontendURL+"/history?wechat_profile="+test.result {
				t.Fatal("optional profile failed", callback.Code, callback.Body.String())
			}
			if replay := f.request(path, browser); replay.Code != 400 {
				t.Fatal("profile OAuth state replay accepted")
			}
			for _, cookie := range callback.Result().Cookies() {
				if cookie.Name == f.cfg.Auth.CookieName {
					t.Fatal("profile authorization replaced login identity")
				}
			}
			profile, err := f.store.UserProfile(context.Background(), strings.Split(session.Value, ".")[1])
			if err != nil || (profile.Nickname != "") != (test.result == "synced") {
				t.Fatal("unexpected stored profile", profile, err)
			}
			if me := f.request("/api/me", session); me.Code != 200 || !strings.Contains(me.Body.String(), `"remainingCredits":2`) {
				t.Fatal("profile sync altered quota or session")
			}
		})
	}
}
