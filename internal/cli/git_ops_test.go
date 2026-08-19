package cli

import (
	"context"
	"fmt"
	"testing"

	"github.com/caiqianzhang/gitcn/internal/config"
)

func TestIsShorthand(t *testing.T) {
	cases := map[string]bool{
		"octocat/Hello-World":  true,
		"a/b":                  true,
		"https://github.com/x": false,
		"git@github.com:x/y":   false,
		"justonepart":          false,
		"a/b/c":                false,
	}
	for in, want := range cases {
		if got := isShorthand(in); got != want {
			t.Errorf("isShorthand(%q) = %v, want %v", in, got, want)
		}
	}
}

func TestNormalizeCloneArg(t *testing.T) {
	got := normalizeCloneArg("octocat/Hello-World")
	if got != "https://github.com/octocat/Hello-World" {
		t.Fatalf("normalizeCloneArg = %q", got)
	}
	if normalizeCloneArg("https://github.com/foo/bar") != "https://github.com/foo/bar" {
		t.Fatal("完整 URL 不应被改写")
	}
}

func TestCloneFallsBackDirectForNonGitHub(t *testing.T) {
	orig := runGit
	var got []string
	runGit = func(args []string) error {
		got = append([]string{}, args...)
		return nil
	}
	defer func() { runGit = orig }()

	if err := cmdClone([]string{"https://gitlab.com/foo/bar", "--depth", "1"}); err != nil {
		t.Fatalf("cmdClone: %v", err)
	}
	found := false
	for i, a := range got {
		if a == "clone" && i+1 < len(got) && got[i+1] == "https://gitlab.com/foo/bar" {
			found = true
		}
	}
	if !found || !containsSlowOpts(got) {
		t.Fatalf("直连 git 参数错误: %v", got)
	}
}

func containsSlowOpts(args []string) bool {
	for _, a := range args {
		if a == "http.lowSpeedLimit=1" || a == "http.lowSpeedTime=30" {
			return true
		}
	}
	return false
}

func TestCloneFallsBackDirectWhenNoNodes(t *testing.T) {
	withTempXDG(t)
	origRun, origBest := runGit, bestNodes
	var got []string
	runGit = func(args []string) error {
		got = append([]string{}, args...)
		return nil
	}
	bestNodes = func(ctx context.Context, cfg *config.Config) ([]string, error) {
		return nil, fmt.Errorf("所有节点均不可用")
	}
	defer func() { runGit, bestNodes = origRun, origBest }()

	if err := cmdClone([]string{"octocat/Hello-World"}); err != nil {
		t.Fatalf("cmdClone: %v", err)
	}
	found := false
	for i, a := range got {
		if a == "clone" && i+1 < len(got) && got[i+1] == "https://github.com/octocat/Hello-World" {
			found = true
		}
	}
	if !found || !containsSlowOpts(got) {
		t.Fatalf("直连 git 参数错误: %v", got)
	}
}
