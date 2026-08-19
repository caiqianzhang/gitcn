package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"
)

// Config 保存 gitcn 的可配置项。
type Config struct {
	CacheTTL       time.Duration `json:"cache_ttl"`
	MirrorSources  []string      `json:"mirror_sources"`
	Timeout        time.Duration `json:"timeout"`
	MaxRetry       int           `json:"max_retry"`
	Concurrency    int           `json:"concurrency"`
	Verbose        bool          `json:"verbose"`
	TestFileRawURL string        `json:"test_file_raw_url"`
	// BuildURL 仅供测试注入测速 URL 构造器，不参与 JSON 序列化。
	BuildURL func(host, testPath string) string `json:"-"`
}

func Default() *Config {
	return &Config{
		CacheTTL:       15 * time.Minute,
		MirrorSources:  []string{},
		Timeout:        5 * time.Second,
		MaxRetry:       3,
		Concurrency:    10,
		Verbose:        false,
		TestFileRawURL: "https://raw.githubusercontent.com/microsoft/terminal/refs/heads/main/res/terminal/images/SmallTile.scale-125.png",
	}
}

// Dir 返回配置目录并确保其存在。
func Dir() (string, error) {
	base, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	dir := filepath.Join(base, "gitcn")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	return dir, nil
}

// Path 返回配置文件路径。
func Path() (string, error) {
	d, err := Dir()
	if err != nil {
		return "", err
	}
	return filepath.Join(d, "config.json"), nil
}

// Load 读取配置，文件不存在时返回默认值。
func Load() (*Config, error) {
	p, err := Path()
	if err != nil {
		return nil, err
	}
	b, err := os.ReadFile(p)
	if err != nil {
		if os.IsNotExist(err) {
			return Default(), nil
		}
		return nil, err
	}
	c := Default()
	if err := json.Unmarshal(b, c); err != nil {
		return nil, err
	}
	c.sanitize()
	return c, nil
}

// sanitize 钳制非法配置值，防止损坏的 config.json 导致死锁/panic。
func (c *Config) sanitize() {
	d := Default()
	if c.CacheTTL <= 0 {
		c.CacheTTL = d.CacheTTL
	}
	if c.Timeout <= 0 {
		c.Timeout = d.Timeout
	}
	if c.MaxRetry < 1 {
		c.MaxRetry = d.MaxRetry
	}
	if c.Concurrency < 1 {
		c.Concurrency = d.Concurrency
	}
}

// Save 写回配置。
func Save(c *Config) error {
	p, err := Path()
	if err != nil {
		return err
	}
	b, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	return writeFileAtomic(p, b)
}

// writeFileAtomic 先写临时文件再重命名，避免崩溃时留下损坏的 JSON。
func writeFileAtomic(p string, b []byte) error {
	tmp := p + ".tmp"
	if err := os.WriteFile(tmp, b, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, p)
}
