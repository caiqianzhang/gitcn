package speedtest

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/caiqianzhang/gitcn/internal/config"
)

type Result struct {
	Node      string
	LatencyMS int64
	Dead      bool
}

// URLBuilder 构造测试请求地址；默认拼接 https://节点/测试文件，测试时可注入。
type URLBuilder func(host, testPath string) string

func defaultURL(host, testPath string) string {
	return fmt.Sprintf("https://%s/%s", host, strings.TrimPrefix(testPath, "/"))
}

// Run 并发测试全部节点，返回按（存活优先、延迟升序）排序的结果。
func Run(ctx context.Context, hosts []string, cfg *config.Config) []Result {
	build := defaultURL
	if cfg.BuildURL != nil {
		build = cfg.BuildURL
	}
	return RunWithBuilder(ctx, hosts, cfg, build)
}

// RunWithBuilder 与 Run 相同，但允许注入 URL 构造器（测试用）。
func RunWithBuilder(ctx context.Context, hosts []string, cfg *config.Config, build URLBuilder) []Result {
	results := make([]Result, len(hosts))
	sem := make(chan struct{}, cfg.Concurrency)
	var wg sync.WaitGroup
	for i, h := range hosts {
		wg.Add(1)
		sem <- struct{}{}
		go func(i int, h string) {
			defer wg.Done()
			defer func() { <-sem }()
			results[i] = test(ctx, h, cfg, build)
		}(i, h)
	}
	wg.Wait()
	sort.Slice(results, func(i, j int) bool {
		if results[i].Dead != results[j].Dead {
			return !results[i].Dead
		}
		return results[i].LatencyMS < results[j].LatencyMS
	})
	return results
}

func test(ctx context.Context, host string, cfg *config.Config, build URLBuilder) Result {
	url := build(host, cfg.TestFileRawURL)
	start := time.Now()
	client := &http.Client{Timeout: cfg.Timeout}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return Result{Node: host, Dead: true}
	}
	req.Header.Set("User-Agent", "gitcn")
	resp, err := client.Do(req)
	if err != nil {
		return Result{Node: host, Dead: true}
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 8<<10))
	elapsed := time.Since(start)
	if err != nil || resp.StatusCode != http.StatusOK {
		return Result{Node: host, Dead: true}
	}
	ct := resp.Header.Get("Content-Type")
	// 防假代理：返回体过小或为 HTML 首页 → dead。
	// 保留 <100B 守卫：真实探测文件恒 >100B（默认 README 约 3KB），从无误伤合法文件，
	// 但能拦截「200 + 短 text/plain stub」这类无 HTML 标记的假代理；
	// 另加 HTML 体嗅探，覆盖被误标 Content-Type 的 HTML 首页。
	lower := strings.ToLower(strings.TrimSpace(string(body)))
	isHTML := strings.Contains(ct, "text/html") ||
		strings.HasPrefix(lower, "<!doctype html") ||
		strings.HasPrefix(lower, "<html")
	if len(body) < 100 || isHTML {
		return Result{Node: host, Dead: true}
	}
	return Result{Node: host, LatencyMS: elapsed.Milliseconds()}
}
