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
