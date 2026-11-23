package config

import (
	"errors"
	"fmt"
	"io"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/jason127vip-dot/maohai-shuo/backend/internal/model"
	"gopkg.in/yaml.v3"
)

type Config struct {
	Payment     Payment `yaml:"payment"`
	PaymentFile string  `yaml:"payment_file"`
	Server      struct {
		Environment            string   `yaml:"environment"`
		Host                   string   `yaml:"host"`
		Port                   int      `yaml:"port"`
		AllowedOrigins         []string `yaml:"allowed_origins"`
		ReadTimeoutSeconds     int      `yaml:"read_timeout_seconds"`
		ShutdownTimeoutSeconds int      `yaml:"shutdown_timeout_seconds"`
	} `yaml:"server"`
	Database struct {
		Driver         string `yaml:"driver"`
		URL            string `yaml:"url"`
		MaxConnections int32  `yaml:"max_connections"`
	} `yaml:"database"`
	Auth struct {
		Mode         string `yaml:"mode"`
		CookieName   string `yaml:"cookie_name"`
		SigningKey   string `yaml:"signing_key"`
		SessionHours int    `yaml:"session_hours"`
	} `yaml:"auth"`
	Wechat struct {
		AppID       string `yaml:"app_id"`
		AppSecret   string `yaml:"app_secret"`
		CallbackURL string `yaml:"callback_url"`
		FrontendURL string `yaml:"frontend_url"`
	} `yaml:"wechat"`
	AI struct {
		Provider              string `yaml:"provider"`
		TimeoutSeconds        int    `yaml:"timeout_seconds"`
		DemoDelayMilliseconds int    `yaml:"demo_delay_milliseconds"`
		Providers             struct {
			Doubao Doubao `yaml:"doubao"`
		} `yaml:"providers"`
	} `yaml:"ai"`
	Worker struct {
		Concurrency       int `yaml:"concurrency"`
		PollMilliseconds  int `yaml:"poll_milliseconds"`
		StaleAfterSeconds int `yaml:"stale_after_seconds"`
	} `yaml:"worker"`
	Storage struct {
		Driver                   string `yaml:"driver"`
		RootDir                  string `yaml:"root_dir"`
		MaxUploadMB              int64  `yaml:"max_upload_mb"`
		MaxInputPixels           int64  `yaml:"max_input_pixels"`
		InputMaxEdge             int    `yaml:"input_max_edge"`
		OutputSize               int    `yaml:"output_size"`
		OriginalRetentionSeconds int    `yaml:"original_retention_seconds"`
		FontPath                 string `yaml:"font_path"`
		BrandText                string `yaml:"brand_text"`
	} `yaml:"storage"`
	Quota struct {
		InitialCredits        int `yaml:"initial_credits"`
		MaxActiveTasksPerUser int `yaml:"max_active_tasks_per_user"`
	} `yaml:"quota"`
	PromptsFile string `yaml:"prompts_file"`
}

type Payment struct {
	Enabled                 bool                `yaml:"enabled"`
	MchID                   string              `yaml:"mch_id"`
	CertificateSerial       string              `yaml:"certificate_serial"`
	MerchantCertificatePath string              `yaml:"merchant_certificate_path"`
	PrivateKeyPath          string              `yaml:"private_key_path"`
	APIv3Key                string              `yaml:"api_v3_key"`
	PublicKeyID             string              `yaml:"public_key_id"`
	PublicKeyPath           string              `yaml:"public_key_path"`
	NotifyURL               string              `yaml:"notify_url"`
	OrderMinutes            int                 `yaml:"order_minutes"`
	HTTPTimeoutSeconds      int                 `yaml:"http_timeout_seconds"`
	Plans                   []model.PaymentPlan `yaml:"plans"`
}

type Doubao struct {
	BaseURL                string `yaml:"base_url"`
	APIKey                 string `yaml:"api_key"`
	Model                  string `yaml:"model"`
	Size                   string `yaml:"size"`
	DisableGroupGeneration bool   `yaml:"disable_group_generation"`
}

type Prompts struct {
	Template string            `yaml:"template"`
	Scenes   map[string]string `yaml:"scenes"`
	Styles   map[string]string `yaml:"styles"`
}

func readYAML(path string, out any) error {
	f, err := os.Open(path)
	if err != nil {
		return errors.New("配置文件无法读取，请检查路径和权限")
	}
	defer f.Close()
	decoder := yaml.NewDecoder(io.LimitReader(f, 1<<20))
	decoder.KnownFields(true)
	if decoder.Decode(out) != nil {
		return errors.New("配置 YAML 格式、字段名称或类型不正确")
	}
	var extra any
	if decoder.Decode(&extra) != io.EOF {
		return errors.New("配置文件只允许一个 YAML 文档")
	}
	return nil
}

func Load(path string) (Config, Prompts, error) {
	var c Config
	var p Prompts
	if err := readYAML(path, &c); err != nil {
		return c, p, err
	}
	// Deployment secrets can stay outside the YAML and all frontend artifacts.
	if value := os.Getenv("MAOHAI_WECHAT_APP_SECRET"); value != "" {
		c.Wechat.AppSecret = value
	}
	if value := os.Getenv("MAOHAI_AUTH_SIGNING_KEY"); value != "" {
		c.Auth.SigningKey = value
	}
	base, err := filepath.Abs(filepath.Dir(path))
	if err != nil {
		return c, p, err
	}
	if c.PaymentFile != "" {
		if c.Payment.Enabled || c.Payment.MchID != "" {
			return c, p, errors.New("payment 与 payment_file 只能配置一个来源")
		}
		if !filepath.IsAbs(c.PaymentFile) {
			c.PaymentFile = filepath.Join(base, c.PaymentFile)
		}
		if err = readYAML(c.PaymentFile, &c.Payment); err != nil {
			return c, p, errors.New("独立支付配置文件无法加载")
		}
	}
	if value := os.Getenv("MAOHAI_PAYMENT_API_V3_KEY"); value != "" {
		c.Payment.APIv3Key = value
	}
	resolve := func(value string) string {
		if value == "" || filepath.IsAbs(value) {
			return value
		}
		return filepath.Join(base, value)
	}
	c.PromptsFile = resolve(c.PromptsFile)
	c.Storage.RootDir = resolve(c.Storage.RootDir)
	c.Storage.FontPath = resolve(c.Storage.FontPath)
	paymentBase := base
	if c.PaymentFile != "" {
		paymentBase = filepath.Dir(c.PaymentFile)
	}
	for _, path := range []*string{&c.Payment.PrivateKeyPath, &c.Payment.PublicKeyPath, &c.Payment.MerchantCertificatePath} {
		if *path != "" && !filepath.IsAbs(*path) {
			*path = filepath.Join(paymentBase, *path)
		}
	}
	if len(c.Payment.Plans) == 0 {
		c.Payment.Plans = []model.PaymentPlan{{ID: "month", Name: "月卡", AmountFen: 480, Credits: 4, Months: 1}, {ID: "year", Name: "年卡", AmountFen: 5800, Credits: 48, Months: 12}}
	}
	if err = c.Validate(); err != nil {
		return c, p, err
	}
	if err = readYAML(c.PromptsFile, &p); err != nil {
		return c, p, fmt.Errorf("提示词配置无法加载: %w", err)
	}
	if !strings.Contains(p.Template, "{scene}") || !strings.Contains(p.Template, "{style}") {
		return c, p, errors.New("提示词模板缺少 scene 或 style 占位符")
	}
	for _, id := range []string{"mischief", "snack", "sleep", "door"} {
		if strings.TrimSpace(p.Scenes[id]) == "" {
			return c, p, errors.New("场景提示词不完整")
		}
	}
	for _, id := range []string{"sketch", "cartoon", "watercolor"} {
		if strings.TrimSpace(p.Styles[id]) == "" {
			return c, p, errors.New("画风提示词不完整")
		}
	}
	return c, p, nil
}

func (c Config) Validate() error {
	if c.Payment.Enabled {
		p := c.Payment
		u, e := url.Parse(p.NotifyURL)
		front, fe := url.Parse(c.Wechat.FrontendURL)
		if c.Auth.Mode != "wechat" || c.Database.Driver != "postgres" || e != nil || fe != nil || u.Scheme != "https" || u.Hostname() == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" || u.Path != "/api/payments/wechat/notify" || u.RawPath != "" || front.Scheme != "https" || front.Host != u.Host || (u.Port() != "" && u.Port() != "443") || (front.Port() != "" && front.Port() != "443") {
			return errors.New("启用支付需要微信持久账号和同站标准HTTPS地址，notify_url 路径为 /api/payments/wechat/notify")
		}
		if !asciiPaymentKey(p.APIv3Key) || len(p.APIv3Key) != 32 || strings.TrimSpace(p.APIv3Key) != p.APIv3Key || p.MchID == "" || p.CertificateSerial == "" || p.MerchantCertificatePath == "" || p.PrivateKeyPath == "" || p.PublicKeyPath == "" || !strings.HasPrefix(p.PublicKeyID, "PUB_KEY_ID_") || strings.ContainsAny(p.MchID+p.CertificateSerial+p.PublicKeyID, "\r\n\"\\") {
			return errors.New("支付商户号、证书、私钥、公钥ID及32字节APIv3密钥未完整配置")
		}
		if p.OrderMinutes < 5 || p.OrderMinutes > 60 || p.HTTPTimeoutSeconds < 1 || p.HTTPTimeoutSeconds > 15 {
			return errors.New("支付订单有效期需5至60分钟，HTTP超时需1至15秒")
		}
		if len(p.Plans) != 2 {
			return errors.New("支付套餐需配置月卡和年卡")
		}
		seen := map[string]bool{}
		for _, plan := range p.Plans {
			if seen[plan.ID] || plan.Name == "" || len([]rune(plan.Name)) > 30 || plan.AmountFen < 1 || plan.AmountFen > 100000 || plan.Credits < 1 || plan.Credits > 10000 || !(plan.ID == "month" && plan.Months == 1 || plan.ID == "year" && plan.Months == 12) {
				return errors.New("支付套餐标识、金额（分）、次数或自然月数配置不正确")
			}
			seen[plan.ID] = true
		}
	}
	// Both modes stay behind the existing IIS/Nginx reverse proxy.
	ip := net.ParseIP(c.Server.Host)
	if ip == nil || !ip.IsLoopback() {
		return errors.New("Go 服务必须绑定回环地址，通过 IIS/Nginx 对外提供服务")
	}
	if c.Server.Environment != "development" && c.Server.Environment != "production" {
		return errors.New("server.environment 必须为 development 或 production")
	}
	if c.Auth.Mode != "development" && c.Auth.Mode != "wechat" {
		return errors.New("auth.mode 必须为 development 或 wechat")
	}
	if c.Auth.Mode == "development" && c.Server.Environment != "development" {
		return errors.New("production 环境必须使用 wechat 身份模式")
	}
	if c.Auth.Mode == "wechat" {
		if c.Database.Driver != "postgres" {
			return errors.New("微信身份模式必须使用 PostgreSQL 持久化账号")
		}
		callback, err := url.Parse(c.Wechat.CallbackURL)
		front, frontErr := url.Parse(c.Wechat.FrontendURL)
		validURL := func(u *url.URL) bool {
			return u != nil && (u.Scheme == "http" || u.Scheme == "https") && u.Hostname() != "" && u.User == nil && u.RawQuery == "" && u.Fragment == "" && !u.ForceQuery && u.Opaque == ""
		}
		standardPort := callback != nil && (callback.Port() == "" || callback.Scheme == "http" && callback.Port() == "80" || callback.Scheme == "https" && callback.Port() == "443")
		if err != nil || frontErr != nil || !validURL(callback) || !validURL(front) || callback.Path != "/api/auth/wechat/callback" || callback.RawPath != "" || (front.Path != "" && front.Path != "/") || front.RawPath != "" || callback.Scheme != front.Scheme || callback.Hostname() != front.Hostname() || !standardPort {
			return errors.New("微信 callback_url 必须使用标准端口及 /api/auth/wechat/callback；frontend_url 为同协议同域名的网站来源（可带端口）")
		}
		if !strings.HasPrefix(c.Wechat.AppID, "wx") || len(c.Wechat.AppID) != 18 || strings.TrimSpace(c.Wechat.AppSecret) == "" {
			return errors.New("微信身份模式需要有效 AppID 和 AppSecret")
		}
		if c.Auth.SigningKey == "local-development-key-change-before-sharing" || strings.HasPrefix(c.Auth.SigningKey, "REPLACE_") {
			return errors.New("微信身份模式必须配置独立的随机会话签名密钥")
		}
		origin := front.Scheme + "://" + front.Host
		found := false
		for _, allowed := range c.Server.AllowedOrigins {
			if allowed == origin {
				found = true
			}
		}
		if !found {
			return errors.New("allowed_origins 必须包含微信 frontend_url 对应的网站来源")
		}
	}
	if c.Server.Port < 1 || c.Server.Port > 65535 || c.Server.ReadTimeoutSeconds < 1 || c.Server.ShutdownTimeoutSeconds < 1 {
		return errors.New("server 端口或超时配置不正确")
	}
	if len(c.Auth.SigningKey) < 32 || c.Auth.CookieName == "" || c.Auth.SessionHours < 1 || c.Auth.SessionHours > 720 || !validCookieName(c.Auth.CookieName) {
		return errors.New("auth 签名密钥至少 32 字符，且必须配置 Cookie 名称与有效期")
	}
	if c.Database.Driver != "memory" && c.Database.Driver != "postgres" {
		return errors.New("database.driver 必须为 memory 或 postgres")
	}
	if c.Database.MaxConnections < 1 || c.Database.MaxConnections > 50 {
		return errors.New("database.max_connections 必须在 1 到 50 之间")
	}
	if c.Database.Driver == "postgres" {
		u, err := url.Parse(c.Database.URL)
		if err != nil || u.Host == "" || (u.Scheme != "postgresql" && u.Scheme != "postgres") || u.Query().Get("sslmode") == "" || u.Query().Get("sslmode") == "disable" {
			return errors.New("database.url 必须为带 SSL 配置的 PostgreSQL 连接串")
		}
	}
	if c.AI.Provider != "demo" && c.AI.Provider != "doubao" {
		return errors.New("ai.provider 当前支持 demo 或 doubao")
	}
	if c.AI.TimeoutSeconds < 1 || c.AI.TimeoutSeconds > 600 || c.AI.DemoDelayMilliseconds < 0 || c.AI.DemoDelayMilliseconds > 30000 {
		return errors.New("AI 超时或演示延迟配置不正确")
	}
	if c.AI.Provider == "doubao" {
		d := c.AI.Providers.Doubao
		u, err := url.Parse(d.BaseURL)
		if err != nil || u.Scheme != "https" || u.Host == "" || u.User != nil || u.RawQuery != "" {
			return errors.New("豆包 base_url 必须是无凭据和查询参数的 HTTPS 地址")
		}
		if d.APIKey == "" || d.Model == "" || d.Size == "" || c.Storage.FontPath == "" {
			return errors.New("真实豆包模式需要配置 API Key、模型 ID、尺寸和中文角标字体")
		}
	}
	if c.Worker.Concurrency < 1 || c.Worker.Concurrency > 10 || c.Worker.PollMilliseconds < 100 || c.Worker.StaleAfterSeconds < c.AI.TimeoutSeconds+30 {
		return errors.New("worker 并发或超时配置不正确，stale_after_seconds 至少比 AI 超时多 30 秒")
	}
	s := c.Storage
	if s.Driver != "local" || s.RootDir == "" || s.MaxUploadMB < 1 || s.MaxUploadMB > 15 || s.MaxInputPixels < 16384 || s.MaxInputPixels > 40000000 || s.InputMaxEdge < 128 || s.InputMaxEdge > 4096 || s.OutputSize < 256 || s.OutputSize > 2048 || s.OriginalRetentionSeconds < c.Worker.StaleAfterSeconds || s.BrandText == "" {
		return errors.New("storage 配置不正确，当前仅支持 local，原图保留时间应覆盖任务超时")
	}
	if c.Quota.InitialCredits < 0 || c.Quota.InitialCredits > 100 || c.Quota.MaxActiveTasksPerUser < 1 || c.Quota.MaxActiveTasksPerUser > 5 {
		return errors.New("quota 配置超出允许范围")
	}
	if c.PromptsFile == "" {
		return errors.New("必须配置 prompts_file")
	}
	for _, origin := range c.Server.AllowedOrigins {
		u, err := url.Parse(origin)
		if err != nil || u.Host == "" || (u.Scheme != "http" && u.Scheme != "https") || u.Path != "" || u.RawQuery != "" || u.User != nil {
			return errors.New("allowed_origins 只能填写完整网站来源，不包含路径")
		}
	}
	return nil
}

func validCookieName(name string) bool {
	for _, r := range name {
		if !(r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '_' || r == '-') {
			return false
		}
	}
	return !strings.HasPrefix(name, "__Host-") && !strings.HasPrefix(name, "__Secure-")
}

func (c Config) Address() string          { return net.JoinHostPort(c.Server.Host, fmt.Sprint(c.Server.Port)) }
func (c Config) AITimeout() time.Duration { return time.Duration(c.AI.TimeoutSeconds) * time.Second }

func (p Prompts) Build(scene, style string) (string, error) {
	a, ok := p.Scenes[scene]
	if !ok {
		return "", errors.New("不支持的场景")
	}
	b, ok := p.Styles[style]
	if !ok {
		return "", errors.New("不支持的画风")
	}
	return strings.NewReplacer("{scene}", a, "{style}", b).Replace(p.Template), nil
}

func asciiPaymentKey(value string) bool {
	for _, ch := range value {
		if !(ch >= 'a' && ch <= 'z' || ch >= 'A' && ch <= 'Z' || ch >= '0' && ch <= '9') {
			return false
		}
	}
	return true
}
