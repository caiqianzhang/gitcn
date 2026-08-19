package speedtest

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/caiqianzhang/gitcn/internal/config"
)

func TestHTMLBodyWithPlainContentTypeIsDead(t *testing.T) {
	// 假代理把自身首页误标为 text/plain，且体长 >100B —— 只有 HTML 体嗅探能识别它。
	html := []byte("<html><head><title>IP 数据查询站</title></head><body><h1>本页并非代理内容，而是该站点自己的首页，长度远超一百字节</h1></body></html>")
	if len(html) <= 100 {
		t.Fatalf("测试前提：HTML 体应 >100B，got %d", len(html))
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		w.Write(html)
	}))
	defer srv.Close()

	cfg := config.Default()
	cfg.Timeout = 2 * time.Second
	cfg.Concurrency = 2
	cfg.TestFileRawURL = "/t"
	res := RunWithBuilder(context.Background(), []string{"a.com"}, cfg,
		func(host, path string) string { return srv.URL + path })[0]
	if !res.Dead {
		t.Fatal("返回 HTML 体的响应应判 dead（无论 Content-Type）")
	}
}

func TestTinyStubIsDead(t *testing.T) {
	// 200 + 短 text/plain stub（无 HTML 标记）是假代理的一种失效形态 → dead。
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		w.Write([]byte("ok"))
	}))
	defer srv.Close()

	cfg := config.Default()
	cfg.Timeout = 2 * time.Second
	cfg.Concurrency = 2
	cfg.TestFileRawURL = "/t"
	res := RunWithBuilder(context.Background(), []string{"a.com"}, cfg,
		func(host, path string) string { return srv.URL + path })[0]
	if !res.Dead {
		t.Fatal("短 text/plain stub 应判 dead（<100B 守卫）")
	}
}

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
