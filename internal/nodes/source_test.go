package nodes

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/caiqianzhang/gitcn/internal/config"
)

func withTempConfig(t *testing.T) {
	t.Helper()
	dir := t.TempDir()
	os.Setenv("GITCN_CONFIG_DIR", dir)
	t.Cleanup(func() { os.Unsetenv("GITCN_CONFIG_DIR") })
}

func TestFetchRemoteSuccess(t *testing.T) {
	withTempConfig(t)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(NodeList{Version: 1, Nodes: []string{"a.com", "b.com"}})
	}))
	defer srv.Close()

	cfg := config.Default()
	cfg.MirrorSources = []string{srv.URL}

	got, err := fetchRemote(context.Background(), cfg)
	if err != nil {
		t.Fatalf("fetchRemote: %v", err)
	}
	if len(got) != 2 || got[0] != "a.com" {
		t.Fatalf("got = %v", got)
	}
}

func TestFetchRemoteFallsToNextMirror(t *testing.T) {
	withTempConfig(t)
	dead := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer dead.Close()
	ok := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(NodeList{Version: 1, Nodes: []string{"c.com"}})
	}))
	defer ok.Close()

	cfg := config.Default()
	cfg.MirrorSources = []string{dead.URL, ok.URL}

	got, err := fetchRemote(context.Background(), cfg)
	if err != nil {
		t.Fatalf("fetchRemote: %v", err)
	}
	if len(got) != 1 || got[0] != "c.com" {
		t.Fatalf("got = %v", got)
	}
}

func TestListFallsBackToSeed(t *testing.T) {
	withTempConfig(t)
	cfg := config.Default()
	cfg.MirrorSources = []string{"http://127.0.0.1:1/nodes.json"} // 必然失败

	hosts, err := List(context.Background(), cfg)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(hosts) < 50 {
		t.Fatalf("expected seed fallback, got %d hosts", len(hosts))
	}
}

func TestAddRejectsInvalidHosts(t *testing.T) {
	withTempConfig(t)
	for _, bad := range []string{"https://x.com", "bad host", "x/y", "", "x:443", "a b.com"} {
		if err := Add(bad); err == nil {
			t.Errorf("Add(%q) 应报错", bad)
		}
	}
}

func TestAddAcceptsValidHosts(t *testing.T) {
	withTempConfig(t)
	for _, good := range []string{"gh.example.com", "1.2.3.4", "a-b.example.co", "2001:db8::1"} {
		if err := Add(good); err != nil {
			t.Errorf("Add(%q) 应成功: %v", good, err)
		}
	}
}

func TestAddInvalidatesCache(t *testing.T) {
	withTempConfig(t)
	if err := saveCache(&Cache{Nodes: []Node{{Host: "old.com", LatencyMS: 50}}, FetchedAt: time.Now()}); err != nil {
		t.Fatal(err)
	}
	if err := Add("new.com"); err != nil {
		t.Fatal(err)
	}
	c, err := loadCache()
	if err != nil {
		t.Fatal(err)
	}
	if !c.FetchedAt.IsZero() {
		t.Fatalf("Add 后 FetchedAt 应置零，got %v", c.FetchedAt)
	}
}

func TestMarkDeadPersists(t *testing.T) {
	withTempConfig(t)
	if err := saveCache(&Cache{
		Nodes:     []Node{{Host: "dead-target.com", LatencyMS: 50}, {Host: "ok.com", LatencyMS: 80}},
		FetchedAt: time.Now(),
	}); err != nil {
		t.Fatal(err)
	}
	if err := MarkDead("dead-target.com"); err != nil {
		t.Fatalf("MarkDead: %v", err)
	}
	c, err := LoadCache()
	if err != nil {
		t.Fatal(err)
	}
	byHost := map[string]Node{}
	for _, n := range c.Nodes {
		byHost[n.Host] = n
	}
	if !byHost["dead-target.com"].Dead {
		t.Fatal("MarkDead 后节点应为 dead")
	}
	if byHost["ok.com"].Dead {
		t.Fatal("其他节点不应受影响")
	}
}

func TestMarkDeadUnknownHost(t *testing.T) {
	withTempConfig(t)
	if err := MarkDead("not-in-cache.com"); err != nil {
		t.Fatalf("缓存外节点 MarkDead 应 no-op，got %v", err)
	}
	c, err := loadCache()
	if err != nil {
		t.Fatal(err)
	}
	for _, n := range c.Nodes {
		if n.Host == "not-in-cache.com" {
			t.Fatal("no-op 不应写入缓存")
		}
	}
}
