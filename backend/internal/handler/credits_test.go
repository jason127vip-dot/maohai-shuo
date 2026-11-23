package handler

import "testing"

func TestWechatLoginReturnsToCredits(t *testing.T) {
	f := loginSetup(t)
	state, browser := f.start(t, "/credits?source=quota#plans")
	w := f.request("/api/auth/wechat/callback?state="+state+"&code=credits-user", browser)
	if w.Code != 303 || w.Header().Get("Location") != f.cfg.Wechat.FrontendURL+"/credits" {
		t.Fatalf("login did not return to credits: %d %s", w.Code, w.Header().Get("Location"))
	}
}

func TestCreditsReturnPathRejectsExternalAndUnknownRoutes(t *testing.T) {
	for _, path := range []string{"https://example.com/credits", "//example.com/credits", "/credits/other", "/credits%2fother", "/credits\\other", "/credits\r\nLocation: https://example.com"} {
		if _, ok := returnPath(path); ok {
			t.Errorf("accepted invalid return path %q", path)
		}
	}
}
