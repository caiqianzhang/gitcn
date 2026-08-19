package cli

import (
	"context"
	"fmt"

	"github.com/caiqianzhang/gitcn/internal/config"
	"github.com/caiqianzhang/gitcn/internal/nodes"
)

func cmdNodes(ctx context.Context, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("用法: gitcn nodes list|update|add <域名>")
	}
	switch args[0] {
	case "list":
		return nodesList()
	case "update":
		cfg, err := config.Load()
		if err != nil {
			return err
		}
		return nodesUpdate(ctx, cfg)
	case "add":
		if len(args) < 2 {
			return fmt.Errorf("用法: gitcn nodes add <域名>")
		}
		return nodesAdd(args[1])
	default:
		return fmt.Errorf("未知子命令 %q", args[0])
	}
}

func nodesList() error {
	c, err := nodes.LoadCache()
	if err != nil {
		return err
	}
	if len(c.Nodes) == 0 {
		hosts, err := nodes.Seed()
		if err != nil {
			return err
		}
		fmt.Printf("共 %d 个种子节点（尚未测速）\n", len(hosts))
		return nil
	}
	fmt.Printf("%-32s %-12s %s\n", "节点", "延迟", "测速时间")
	for _, n := range c.Nodes {
		state := "未知"
		if n.Dead {
			state = "dead"
		} else if n.LatencyMS > 0 {
			state = fmt.Sprintf("%dms", n.LatencyMS)
		}
		fmt.Printf("%-32s %-12s %s\n", n.Host, state, n.TestedAt)
	}
	return nil
}

func nodesUpdate(ctx context.Context, cfg *config.Config) error {
	hosts, ok, err := nodes.Fetch(ctx, cfg)
	if err != nil {
		return fmt.Errorf("远端节点更新失败: %v（可继续使用缓存/内置节点）", err)
	}
	if !ok {
		return fmt.Errorf("远端节点更新失败（可继续使用缓存/内置节点）")
	}
	fmt.Printf("已更新 %d 个节点\n", len(hosts))
	return nil
}

func nodesAdd(host string) error {
	return nodes.Add(host)
}
