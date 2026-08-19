package nodes

import (
	"context"
	"net/http"
	"net/http/httptest"
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
