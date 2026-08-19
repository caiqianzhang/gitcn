package nodes

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"time"

	"github.com/caiqianzhang/gitcn/internal/config"
)

var hostRe = regexp.MustCompile(`^[A-Za-z0-9]([A-Za-z0-9-]{0,61}[A-Za-z0-9])?(\.[A-Za-z0-9]([A-Za-z0-9-]{0,61}[A-Za-z0-9])?)*$`)

// isValidHost 校验裸域名或 IP，拒绝 scheme/路径/空格等垃圾输入。
func isValidHost(s string) bool {
	if s == "" || len(s) > 253 {
		return false
	}
	if net.ParseIP(s) != nil {
		return true
	}
	return hostRe.MatchString(s)
}

// Node 是缓存中单个节点的状态。
type Node struct {
	Host      string `json:"host"`
	LatencyMS int64  `json:"latency_ms,omitempty"`
	Dead      bool   `json:"dead,omitempty"`
	TestedAt  string `json:"tested_at,omitempty"`
}

// Cache 是本地节点缓存。
type Cache struct {
	Nodes     []Node    `json:"nodes"`
	FetchedAt time.Time `json:"fetched_at"`
}

func cachePath() (string, error) {
	dir, err := config.Dir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "cache.json"), nil
}

func loadCache() (*Cache, error) {
	p, err := cachePath()
	if err != nil {
		return nil, err
	}
	b, err := os.ReadFile(p)
	if err != nil {
		if os.IsNotExist(err) {
			return &Cache{}, nil
		}
		return nil, err
	}
	var c Cache
	if err := json.Unmarshal(b, &c); err != nil {
		return nil, err
	}
	return &c, nil
}

func saveCache(c *Cache) error {
	p, err := cachePath()
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

// List 返回节点主机名：优先远端 JSON，其次缓存中非 dead 节点，最后内置种子。
func List(ctx context.Context, cfg *config.Config) ([]string, error) {
	if hosts, err := fetchRemote(ctx, cfg); err == nil && len(hosts) > 0 {
		return hosts, nil
	}
	if cache, err := loadCache(); err == nil {
		var hosts []string
		for _, n := range cache.Nodes {
			if !n.Dead {
				hosts = append(hosts, n.Host)
			}
		}
		if len(hosts) > 0 {
			return hosts, nil
		}
	}
	return Seed()
}

// fetchRemote 按镜像源顺序轮询，成功即返回并写缓存。
func fetchRemote(ctx context.Context, cfg *config.Config) ([]string, error) {
	client := &http.Client{Timeout: cfg.Timeout}
	var lastErr error
	for _, src := range cfg.MirrorSources {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, src, nil)
		if err != nil {
			lastErr = err
			continue
		}
		req.Header.Set("User-Agent", "gitcn")
		resp, err := client.Do(req)
		if err != nil {
			lastErr = err
			continue
		}
		body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
		resp.Body.Close()
		if err != nil {
			lastErr = err
			continue
		}
		if resp.StatusCode != http.StatusOK {
			lastErr = fmt.Errorf("%s: HTTP %d", src, resp.StatusCode)
			continue
		}
		var l NodeList
		if err := json.Unmarshal(body, &l); err != nil || len(l.Nodes) == 0 {
			lastErr = fmt.Errorf("%s: 节点列表解析失败", src)
			continue
		}
		nodes := make([]Node, 0, len(l.Nodes))
		for _, h := range l.Nodes {
			nodes = append(nodes, Node{Host: h})
		}
		_ = saveCache(&Cache{Nodes: nodes, FetchedAt: time.Now()})
		return l.Nodes, nil
	}
	if lastErr == nil {
		lastErr = fmt.Errorf("未配置镜像源")
	}
	return nil, lastErr
}

// Add 追加一个节点到缓存。
func Add(host string) error {
	if !isValidHost(host) {
		return fmt.Errorf("无效节点域名: %q（请输入裸域名或 IP，如 gh.example.com）", host)
	}
	cache, err := loadCache()
	if err != nil {
		return err
	}
	for _, n := range cache.Nodes {
		if n.Host == host {
			return fmt.Errorf("节点 %s 已存在", host)
		}
	}
	cache.Nodes = append(cache.Nodes, Node{Host: host, TestedAt: time.Now().Format(time.RFC3339)})
	// 置零 FetchedAt 使缓存立即过期，保证新节点下次参与测速选优。
	return saveCache(&Cache{Nodes: cache.Nodes, FetchedAt: time.Time{}})
}

// LoadCacheForTest 返回当前节点缓存（命名含 ForTest，后续可改名）。
func LoadCacheForTest() (*Cache, error) {
	return loadCache()
}

// Fetch 只从远端镜像源拉取节点列表，不落到缓存/种子。
// 成功返回 (hosts, true, nil)；全部镜像失败返回 (nil, false, err)。
func Fetch(ctx context.Context, cfg *config.Config) ([]string, bool, error) {
	hosts, err := fetchRemote(ctx, cfg)
	if err != nil {
		return nil, false, err
	}
	return hosts, true, nil
}
