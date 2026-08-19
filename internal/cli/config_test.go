package cli

import (
	"fmt"
	"testing"

	"github.com/caiqianzhang/gitcn/internal/config"
	"github.com/caiqianzhang/gitcn/internal/proxy"
)

func TestSetConfigMirrorSourcesTrims(t *testing.T) {
	withTempXDG(t)
	if err := setConfig("mirror_sources", " https://a.com/nodes.json , b.com ,, "); err != nil {
		t.Fatalf("setConfig: %v", err)
	}
	c, err := config.Load()
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"https://a.com/nodes.json", "b.com"}
	if len(c.MirrorSources) != 2 || c.MirrorSources[0] != want[0] || c.MirrorSources[1] != want[1] {
		t.Fatalf("MirrorSources = %v, want %v", c.MirrorSources, want)
	}
}

func TestExitCodePropagatesGitCode(t *testing.T) {
	orig := runGit
	runGit = func(args []string) error {
		return &proxy.ExitError{Code: 128, Err: fmt.Errorf("boom")}
	}
	defer func() { runGit = orig }()

	err := cmdClone([]string{"https://gitlab.com/foo/bar"})
	if err == nil {
		t.Fatal("应返回错误")
	}
	if code := ExitCode(err); code != 128 {
		t.Fatalf("ExitCode = %d, want 128", code)
	}
}
