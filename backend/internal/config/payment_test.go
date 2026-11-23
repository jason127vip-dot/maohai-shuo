package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPaymentConfigurationRequiresHTTPSPersistentIdentityAndServerPlans(t *testing.T) {
	c, _, err := Load(filepath.Join("..", "..", "configs", "config.example.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	c.Auth.Mode = "wechat"
	c.Auth.SigningKey = strings.Repeat("x", 40)
	c.Database.Driver = "postgres"
	c.Database.URL = "postgres://test.invalid/db?sslmode=require"
	c.Wechat.AppID = "wx0123456789abcdef"
	c.Wechat.AppSecret = "test-app-secret"
	c.Wechat.FrontendURL = "https://example.test"
	c.Wechat.CallbackURL = "https://example.test/api/auth/wechat/callback"
	c.Server.AllowedOrigins = []string{c.Wechat.FrontendURL}
	c.Payment.Enabled = true
	c.Payment.MchID = "1234567890"
	c.Payment.CertificateSerial = "1234"
	c.Payment.APIv3Key = strings.Repeat("k", 32)
	c.Payment.PublicKeyID = "PUB_KEY_ID_TEST"
	c.Payment.NotifyURL = "https://example.test/api/payments/wechat/notify"
	if err = c.Validate(); err != nil {
		t.Fatal("valid payment configuration rejected", err)
	}
	for _, item := range []struct {
		name   string
		change func(*Config)
	}{
		{"http", func(c *Config) { c.Payment.NotifyURL = "http://example.test/api/payments/wechat/notify" }},
		{"development", func(c *Config) { c.Auth.Mode = "development" }},
		{"other-site", func(c *Config) { c.Payment.NotifyURL = "https://other.test/api/payments/wechat/notify" }},
		{"nonstandard-port", func(c *Config) { c.Payment.NotifyURL = "https://example.test:8088/api/payments/wechat/notify" }},
		{"v2-key-length", func(c *Config) { c.Payment.APIv3Key = "short" }},
		{"bad-public-id", func(c *Config) { c.Payment.PublicKeyID = "wrong" }},
		{"unsupported-plan", func(c *Config) { c.Payment.Plans = c.Payment.Plans[:1] }},
		{"invalid-ttl", func(c *Config) { c.Payment.OrderMinutes = 0 }},
	} {
		t.Run(item.name, func(t *testing.T) {
			v := c
			item.change(&v)
			if v.Validate() == nil {
				t.Fatal("invalid payment config accepted")
			}
		})
	}
}
func TestPaymentSidecarResolvesPathsWithoutLoggingSecrets(t *testing.T) {
	dir := t.TempDir()
	raw, err := os.ReadFile(filepath.Join("..", "..", "configs", "config.example.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	// Keep the main configuration's prompt path valid when copied to a temp dir.
	prompts, _ := filepath.Abs(filepath.Join("..", "..", "configs", "prompts.yaml"))
	s := string(raw)
	s = s[:strings.Index(s, "# Payment settings")] + "\npayment_file: private/pay.yaml\n"
	s = strings.Replace(s, "prompts_file: prompts.yaml", "prompts_file: \""+prompts+"\"", 1)
	if err = os.MkdirAll(filepath.Join(dir, "private"), 0700); err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(filepath.Join(dir, "config.yaml"), []byte(s), 0600); err != nil {
		t.Fatal(err)
	}
	sidecar := []byte("enabled: false\nmch_id: test\nprivate_key_path: keys/key.pem\npublic_key_path: keys/public.pem\nmerchant_certificate_path: keys/cert.pem\napi_v3_key: private-value-no-output\n")
	if err = os.WriteFile(filepath.Join(dir, "private", "pay.yaml"), sidecar, 0600); err != nil {
		t.Fatal(err)
	}
	c, _, err := Load(filepath.Join(dir, "config.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if c.Payment.PrivateKeyPath != filepath.Join(dir, "private", "keys", "key.pem") || len(c.Payment.Plans) != 2 {
		t.Fatal("sidecar paths or default catalog wrong")
	}
}
