package nodes

import (
	"context"
	"fmt"
	"sort"
	"time"

	"github.com/caiqianzhang/gitcn/internal/config"
	"github.com/caiqianzhang/gitcn/internal/speedtest"
)

// Best 返回按延迟排序的候选节点（最多 cfg.MaxRetry 个）。
// 缓存未过期且有可用延迟数据时直接返回；否则对当前节点池测速并刷新缓存。
func Best(ctx context.Context, cfg *config.Config) ([]string, error) {
	if cache, err := loadCache(); err == nil && time.Since(cache.FetchedAt) < cfg.CacheTTL {
		var alive []Node
		for _, n := range cache.Nodes {
			if !n.Dead && n.LatencyMS > 0 {
				alive = append(alive, n)
			}
		}
		if len(alive) > 0 {
			return firstNodes(alive, cfg.MaxRetry), nil
		}
	}

	hosts, err := List(ctx, cfg)
	if err != nil {
		return nil, err
	}
	results := speedtest.Run(ctx, hosts, cfg)

	var cached []Node
	now := time.Now().Format(time.RFC3339)
	for _, r := range results {
		cached = append(cached, Node{Host: r.Node, LatencyMS: r.LatencyMS, Dead: r.Dead, TestedAt: now})
	}
	// 只保留本轮测速过的节点并按 Host 排序：裁剪已不在节点池的旧节点，令 cache.json 输出确定。
	sort.Slice(cached, func(i, j int) bool { return cached[i].Host < cached[j].Host })
	_ = saveCache(&Cache{Nodes: cached, FetchedAt: time.Now()})

	var alive []Node
	for _, nd := range cached {
		if !nd.Dead {
			alive = append(alive, nd)
		}
	}
	if len(alive) == 0 {
		return nil, fmt.Errorf("所有节点均不可用")
	}
	return firstNodes(alive, cfg.MaxRetry), nil
}

func firstNodes(nodes []Node, n int) []string {
	if len(nodes) < n {
		n = len(nodes)
	}
	sort.Slice(nodes, func(i, j int) bool { return nodes[i].LatencyMS < nodes[j].LatencyMS })
	out := make([]string, 0, n)
	for _, nd := range nodes[:n] {
		out = append(out, nd.Host)
	}
	return out
}
