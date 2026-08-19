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
	os.Setenv("XDG_CONFIG_HOME", dir)
	t.Cleanup(func() { os.Unsetenv("XDG_CONFIG_HOME") })
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
