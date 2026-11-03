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

	"gopkg.in/yaml.v3"
)

type Config struct {
	Server struct {
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
	base, err := filepath.Abs(filepath.Dir(path))
	if err != nil {
		return c, p, err
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
	// Deliberately fail closed until WeChat OAuth has been implemented and reviewed.
	ip := net.ParseIP(c.Server.Host)
	if c.Server.Environment != "development" || c.Auth.Mode != "development" || ip == nil || !ip.IsLoopback() {
		return errors.New("当前版本只支持绑定回环地址的 development 身份模式；正式微信授权尚待接入")
	}
	if c.Server.Port < 1 || c.Server.Port > 65535 || c.Server.ReadTimeoutSeconds < 1 || c.Server.ShutdownTimeoutSeconds < 1 {
		return errors.New("server 端口或超时配置不正确")
	}
	if len(c.Auth.SigningKey) < 32 || c.Auth.CookieName == "" || c.Auth.SessionHours < 1 || c.Auth.SessionHours > 720 {
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
