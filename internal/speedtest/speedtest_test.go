package speedtest

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/caiqianzhang/gitcn/internal/config"
)

func TestRunMarksDeadAndSorts(t *testing.T) {
	good := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/png")
		w.Write(make([]byte, 862))
	}))
	defer good.Close()
	fake := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.Write([]byte("<html>IP 数据查询站</html>"))
	}))
	defer fake.Close()

	cfg := config.Default()
	cfg.Timeout = 2 * time.Second
	cfg.Concurrency = 4
	cfg.TestFileRawURL = "/test.png"

	results := RunWithBuilder(context.Background(), []string{"a.com", "b.com"}, cfg,
		func(host, path string) string {
			if host == "a.com" {
				return good.URL + path
			}
			return fake.URL + path
		})

	byHost := map[string]Result{}
	for _, r := range results {
		byHost[r.Node] = r
	}
	if byHost["a.com"].Dead {
		t.Fatal("a.com 不应 dead")
	}
	if byHost["a.com"].LatencyMS < 0 {
		t.Fatal("a.com 延迟异常")
	}
	if !byHost["b.com"].Dead {
		t.Fatal("b.com（假代理 text/html）应被识别为 dead")
	}
}
