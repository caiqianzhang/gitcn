package cli

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"regexp"
	"strings"

	"github.com/caiqianzhang/gitcn/internal/config"
	"github.com/caiqianzhang/gitcn/internal/nodes"
	"github.com/caiqianzhang/gitcn/internal/proxy"
)

// runGit / bestNodes 提供测试注入点（seam）。
var (
	runGit    = proxy.RunGit
	bestNodes = nodes.Best
)

// gitSlowOpts 让 git 在下载长时间无数据时主动失败，避免代理“吞连接”导致永久卡死。
var gitSlowOpts = []string{"-c", "http.lowSpeedLimit=1", "-c", "http.lowSpeedTime=30"}

// warnIfUserinfo 提示 URL 中的用户信息会被剥离（不泄露给代理节点）。
func warnIfUserinfo(raw string) {
	u, err := url.Parse(raw)
	if err != nil || u.User == nil {
		return
	}
	fmt.Fprintln(os.Stderr, "gitcn: 警告: 已从 URL 移除用户信息，避免泄露给代理节点")
}

var shorthandRe = regexp.MustCompile(`^[A-Za-z0-9_.-]+/[A-Za-z0-9_.-]+$`)

func isShorthand(s string) bool {
	return shorthandRe.MatchString(s)
}

func normalizeCloneArg(raw string) string {
	if isShorthand(raw) {
		return "https://github.com/" + raw
	}
	return raw
}

func cmdClone(ctx context.Context, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("用法: gitcn clone <url|owner/repo> [git参数...]")
	}
	target := normalizeCloneArg(args[0])
	rest := args[1:]

	kind, _, err := proxy.Detect(target)
	if err != nil {
		// 非 GitHub 链接不做代理改写，直连执行（与 fetch/pull 行为一致）。
		return runGit(append(gitSlowOpts, append([]string{"clone", target}, rest...)...))
	}
	if kind != proxy.KindGit {
		return fmt.Errorf("该链接不是仓库地址，请用 gitcn download")
	}

	cfg, err := config.Load()
	if err != nil {
		return err
	}
	candidates, err := bestNodes(ctx, cfg)
	if err != nil {
		if cfg.Verbose {
			fmt.Fprintf(os.Stderr, "提示: %v，将直连执行\n", err)
		}
		return runGit(append(gitSlowOpts, append([]string{"clone", target}, rest...)...))
	}
	if cfg.Verbose {
		fmt.Printf("候选节点: %s\n", strings.Join(candidates, ", "))
	}
	return proxy.RunWithFailover(candidates, func(node string) error {
		warnIfUserinfo(target)
		rewritten, err := proxy.Rewrite(target, node)
		if err != nil {
			return err
		}
		if cfg.Verbose {
			fmt.Printf("→ 使用节点 %s\n", node)
		}
		err = runGit(append(gitSlowOpts, append([]string{"clone", rewritten}, rest...)...))
		if err != nil {
			nodes.MarkDead(node) // 失败节点标记 dead，下次不再优先选择
		}
		return err
	})
}

func cmdFetch(ctx context.Context, args []string) error { return fetchOrPull(ctx, args, "fetch") }

func cmdPull(ctx context.Context, args []string) error { return fetchOrPull(ctx, args, "pull") }

func fetchOrPull(ctx context.Context, args []string, verb string) error {
	raw, err := repoRemoteURL()
	if err != nil {
		return fmt.Errorf("当前目录不是 git 仓库或没有 origin: %w", err)
	}
	if _, _, err := proxy.Detect(raw); err != nil {
		return runGit(append(gitSlowOpts, append([]string{verb}, args...)...))
	}

	cfg, err := config.Load()
	if err != nil {
		return err
	}
	candidates, err := bestNodes(ctx, cfg)
	if err != nil {
		return runGit(append(gitSlowOpts, append([]string{verb}, args...)...)) // 直连兜底
	}
	return proxy.RunWithFailover(candidates, func(node string) error {
		warnIfUserinfo(raw)
		rewritten, err := proxy.Rewrite(raw, node)
		if err != nil {
			return err
		}
		gitArgs := append(gitSlowOpts, "-c", "remote.origin.url="+rewritten, verb)
		gitArgs = append(gitArgs, args...)
		err = runGit(gitArgs)
		if err != nil {
			nodes.MarkDead(node)
		}
		return err
	})
}

func repoRemoteURL() (string, error) {
	out, err := exec.Command("git", "remote", "get-url", "origin").Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}
