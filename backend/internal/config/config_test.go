package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestExampleAndProductionGuard(t *testing.T) {
	path := filepath.Join("..", "..", "configs", "config.example.yaml")
	c, p, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if !filepath.IsAbs(c.Storage.RootDir) || !filepath.IsAbs(c.PromptsFile) {
		t.Fatal("paths are not relative to config directory")
	}
	prompt, err := p.Build("mischief", "cartoon")
	if err != nil || !strings.Contains(prompt, "毛色") {
		t.Fatal("prompt not built")
	}
	if _, err = p.Build("unknown", "cartoon"); err == nil {
		t.Fatal("invalid scene accepted")
	}
	c.Server.Environment = "production"
	if c.Validate() == nil {
		t.Fatal("development identity allowed in production")
	}
	c.Server.Environment = "development"
	c.Server.Host = "0.0.0.0"
	if c.Validate() == nil {
		t.Fatal("development identity exposed publicly")
	}
}
func TestConfigErrorsDoNotEchoSecrets(t *testing.T) {
	path := filepath.Join(t.TempDir(), "bad.yaml")
	secret := "do-not-log-this-secret"
	if err := os.WriteFile(path, []byte("server:\n  port: "+secret+"\n"), 0600); err != nil {
		t.Fatal(err)
	}
	_, _, err := Load(path)
	if err == nil || strings.Contains(err.Error(), secret) {
		t.Fatal("invalid config exposed value or was accepted")
	}
}

func TestWechatConfigurationRequiresPersistentIdentityAndFixedOrigins(t *testing.T) {
	base, _, err := Load(filepath.Join("..", "..", "configs", "config.example.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	base.Server.Environment = "production"
	base.Auth.Mode = "wechat"
	base.Auth.SigningKey = "a-unique-production-signing-key-with-32-bytes"
	base.Database.Driver = "postgres"
	base.Database.URL = "postgresql://example.invalid/db?sslmode=require"
	base.Wechat.AppID = "wx0123456789abcdef"
	base.Wechat.AppSecret = "test-secret"
	base.Wechat.CallbackURL = "http://maohai.test/api/auth/wechat/callback"
	base.Wechat.FrontendURL = "http://maohai.test:8088"
	base.Server.AllowedOrigins = []string{base.Wechat.FrontendURL}
	if err = base.Validate(); err != nil {
		t.Fatal("valid HTTP80 callback rejected", err)
	}
	for _, test := range []struct {
		name   string
		change func(*Config)
	}{
		{"memory", func(c *Config) { c.Database.Driver = "memory" }},
		{"missing-secret", func(c *Config) { c.Wechat.AppSecret = "" }},
		{"callback-8088", func(c *Config) { c.Wechat.CallbackURL = "http://maohai.test:8088/api/auth/wechat/callback" }},
		{"callback-query", func(c *Config) { c.Wechat.CallbackURL += "?secret=unsafe" }},
		{"wrong-host", func(c *Config) { c.Wechat.FrontendURL = "http://evil.test" }},
		{"scheme-mismatch", func(c *Config) { c.Wechat.FrontendURL = "https://maohai.test" }},
		{"unknown-origin", func(c *Config) { c.Server.AllowedOrigins = nil }},
		{"development-key", func(c *Config) { c.Auth.SigningKey = "local-development-key-change-before-sharing" }},
		{"bad-cookie", func(c *Config) { c.Auth.CookieName = "bad;cookie" }},
	} {
		t.Run(test.name, func(t *testing.T) {
			cfg := base
			test.change(&cfg)
			if cfg.Validate() == nil {
				t.Fatal("unsafe configuration accepted")
			}
		})
	}
}
