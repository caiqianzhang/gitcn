package nodes

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/caiqianzhang/gitcn/internal/config"
)

func TestBestUsesCacheWhenFresh(t *testing.T) {
	withTempConfig(t)
	cfg := config.Default()
	cfg.CacheTTL = time.Hour
	if err := saveCache(&Cache{
		Nodes:     []Node{{Host: "fast.com", LatencyMS: 50}, {Host: "slow.com", LatencyMS: 900}},
		FetchedAt: time.Now(),
	}); err != nil {
		t.Fatal(err)
	}
	cands, err := Best(context.Background(), cfg)
	if err != nil {
		t.Fatalf("Best: %v", err)
	}
	if len(cands) == 0 || cands[0] != "fast.com" {
		t.Fatalf("candidates = %v, want fast.com first", cands)
	}
}

func TestBestSpeedtestsWhenCacheStale(t *testing.T) {
	withTempConfig(t)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/png")
		w.Write(make([]byte, 862))
	}))
	defer srv.Close()

	cfg := config.Default()
	cfg.CacheTTL = time.Hour
	cfg.MirrorSources = []string{"http://127.0.0.1:1/x"}
	_ = saveCache(&Cache{
		Nodes:     []Node{{Host: "dead1.com", Dead: true}},
		FetchedAt: time.Now().Add(-2 * time.Hour),
	})

	cfg.BuildURL = func(host, path string) string { return srv.URL + "/" + path }

	cands, err := Best(context.Background(), cfg)
	if err != nil {
		t.Fatalf("Best: %v", err)
	}
	if len(cands) == 0 {
		t.Fatal("no candidates")
	}
	c, _ := loadCache()
	alive := 0
	for _, n := range c.Nodes {
		if !n.Dead {
			alive++
		}
	}
	if alive == 0 {
		t.Fatal("cache not refreshed with alive nodes")
	}
}

func TestBestRevivesDeadNodeOnExpiry(t *testing.T) {
	withTempConfig(t)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/png")
		w.Write(make([]byte, 862))
	}))
	defer srv.Close()

	cfg := config.Default()
	cfg.CacheTTL = time.Hour
	cfg.MirrorSources = []string{"http://127.0.0.1:1/x"} // 必失败 → 走缓存节点池
	// 缓存过期且含一个 dead 节点：过期重测应让它有机会复活。
	_ = saveCache(&Cache{
		Nodes: []Node{
			{Host: "was-dead.com", LatencyMS: 200, Dead: true},
			{Host: "ok.com", LatencyMS: 50},
		},
		FetchedAt: time.Now().Add(-2 * time.Hour),
	})
	cfg.BuildURL = func(host, path string) string { return srv.URL + "/" + path }

	if _, err := Best(context.Background(), cfg); err != nil {
		t.Fatalf("Best: %v", err)
	}
	c, _ := loadCache()
	byHost := map[string]Node{}
	for _, n := range c.Nodes {
		byHost[n.Host] = n
	}
	if n, ok := byHost["was-dead.com"]; !ok {
		t.Fatal("was-dead.com 应重新测速并保留在缓存")
	} else if n.Dead {
		t.Fatal("过期重测后可达的节点应复活（Dead=false）")
	}
}

func TestBestStillDeadStaysExcluded(t *testing.T) {
	withTempConfig(t)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "zombie") {
			w.Header().Set("Content-Type", "text/html")
			w.Write([]byte("<html>仍然在线的假代理</html>"))
			return
		}
		w.Header().Set("Content-Type", "image/png")
		w.Write(make([]byte, 862))
	}))
	defer srv.Close()

	cfg := config.Default()
	cfg.CacheTTL = time.Hour
	cfg.MirrorSources = []string{"http://127.0.0.1:1/x"}
	_ = saveCache(&Cache{
		Nodes: []Node{
			{Host: "zombie.com", LatencyMS: 10, Dead: true},
			{Host: "ok.com", LatencyMS: 50},
		},
		FetchedAt: time.Now().Add(-2 * time.Hour),
	})
	cfg.BuildURL = func(host, path string) string { return srv.URL + "/" + host + path }

	cands, err := Best(context.Background(), cfg)
	if err != nil {
		t.Fatalf("Best: %v", err)
	}
	for _, c := range cands {
		if c == "zombie.com" {
			t.Fatal("仍为 dead 的节点不应成为候选")
		}
	}
	c, _ := loadCache()
	for _, n := range c.Nodes {
		if n.Host == "zombie.com" && !n.Dead {
			t.Fatal("重测后仍失败的节点应保持 dead")
		}
	}
}

func TestBestSortsCacheNodesDeterministically(t *testing.T) {
	withTempConfig(t)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/png")
		w.Write(make([]byte, 862))
	}))
	defer srv.Close()

	cfg := config.Default()
	cfg.CacheTTL = time.Hour
	cfg.MirrorSources = []string{"http://127.0.0.1:1/x"}
	_ = saveCache(&Cache{
		Nodes:     []Node{{Host: "zeta.com", LatencyMS: 10}, {Host: "alpha.com", LatencyMS: 5}},
		FetchedAt: time.Now().Add(-2 * time.Hour),
	})
	cfg.BuildURL = func(host, path string) string { return srv.URL + "/" + path }

	if _, err := Best(context.Background(), cfg); err != nil {
		t.Fatalf("Best: %v", err)
	}
	c, _ := loadCache()
	if len(c.Nodes) != 2 {
		t.Fatalf("缓存节点数 = %d, want 2", len(c.Nodes))
	}
	if c.Nodes[0].Host != "alpha.com" || c.Nodes[1].Host != "zeta.com" {
		t.Fatalf("缓存应按 Host 排序, got %q, %q", c.Nodes[0].Host, c.Nodes[1].Host)
	}
}
