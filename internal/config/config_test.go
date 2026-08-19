package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestDefaultHasSaneValues(t *testing.T) {
	c := Default()
	if c.CacheTTL != 15*time.Minute {
		t.Fatalf("CacheTTL = %v, want 15m", c.CacheTTL)
	}
	if c.MaxRetry != 3 || c.Concurrency != 10 || c.Timeout != 5*time.Second {
		t.Fatalf("defaults wrong: %+v", c)
	}
	if len(c.MirrorSources) != 0 {
		t.Fatalf("默认 MirrorSources 应为空, got %v", c.MirrorSources)
	}
}

func TestLoadSanitizesBadValues(t *testing.T) {
	dir := t.TempDir()
	os.Setenv("XDG_CONFIG_HOME", dir)
	defer os.Unsetenv("XDG_CONFIG_HOME")

	bad := `{"cache_ttl":0,"timeout":0,"max_retry":-1,"concurrency":0}`
	if err := os.MkdirAll(filepath.Join(dir, "gitcn"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "gitcn", "config.json"), []byte(bad), 0o644); err != nil {
		t.Fatal(err)
	}
	c, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if c.CacheTTL != 15*time.Minute {
		t.Fatalf("CacheTTL = %v, want 15m", c.CacheTTL)
	}
	if c.Timeout != 5*time.Second {
		t.Fatalf("Timeout = %v, want 5s", c.Timeout)
	}
	if c.MaxRetry != 3 {
		t.Fatalf("MaxRetry = %d, want 3", c.MaxRetry)
	}
	if c.Concurrency != 10 {
		t.Fatalf("Concurrency = %d, want 10", c.Concurrency)
	}
}

func TestDownloadTimeoutDefaultAndSanitize(t *testing.T) {
	if d := Default(); d.DownloadTimeout != 10*time.Minute {
		t.Fatalf("默认 DownloadTimeout = %v, want 10m", d.DownloadTimeout)
	}
	cfg := &Config{DownloadTimeout: -1 * time.Second}
	cfg.sanitize()
	if cfg.DownloadTimeout != 10*time.Minute {
		t.Fatalf("非法 DownloadTimeout 应钳制为默认, got %v", cfg.DownloadTimeout)
	}
}

func TestSaveLoadRoundtrip(t *testing.T) {
	dir := t.TempDir()
	os.Setenv("XDG_CONFIG_HOME", dir)
	defer os.Unsetenv("XDG_CONFIG_HOME")

	c := Default()
	c.CacheTTL = 5 * time.Minute
	c.MirrorSources = []string{"https://example.com/nodes.json"}
	if err := Save(c); err != nil {
		t.Fatalf("Save: %v", err)
	}
	got, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if got.CacheTTL != 5*time.Minute {
		t.Fatalf("CacheTTL = %v", got.CacheTTL)
	}
	if got.MirrorSources[0] != "https://example.com/nodes.json" {
		t.Fatalf("MirrorSources = %v", got.MirrorSources)
	}
}

func TestLoadMissingReturnsDefault(t *testing.T) {
	dir := t.TempDir()
	os.Setenv("XDG_CONFIG_HOME", dir)
	defer os.Unsetenv("XDG_CONFIG_HOME")

	c, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if c.CacheTTL != 15*time.Minute {
		t.Fatalf("CacheTTL = %v, want 15m", c.CacheTTL)
	}
}

func TestDirCreates(t *testing.T) {
	dir := t.TempDir()
	os.Setenv("XDG_CONFIG_HOME", dir)
	defer os.Unsetenv("XDG_CONFIG_HOME")

	d, err := Dir()
	if err != nil {
		t.Fatalf("Dir: %v", err)
	}
	if _, err := os.Stat(filepath.Join(d, "config.json")); !os.IsNotExist(err) {
		t.Fatal("config.json should not exist yet")
	}
}
