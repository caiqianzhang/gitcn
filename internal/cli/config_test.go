package cli

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/caiqianzhang/gitcn/internal/config"
	"github.com/caiqianzhang/gitcn/internal/proxy"
)

func TestSetConfigMirrorSourcesTrims(t *testing.T) {
	withTempConfigDir(t)
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

	err := cmdClone(context.Background(), []string{"https://gitlab.com/foo/bar"})
	if err == nil {
		t.Fatal("应返回错误")
	}
	if code := ExitCode(err); code != 128 {
		t.Fatalf("ExitCode = %d, want 128", code)
	}
}

func TestSetConfigDownloadTimeout(t *testing.T) {
	withTempConfigDir(t)
	if err := setConfig("download_timeout", "20m"); err != nil {
		t.Fatalf("setConfig: %v", err)
	}
	c, err := config.Load()
	if err != nil {
		t.Fatal(err)
	}
	if c.DownloadTimeout != 20*time.Minute {
		t.Fatalf("DownloadTimeout = %v, want 20m", c.DownloadTimeout)
	}
	if err := setConfig("download_timeout", "abc"); err == nil {
		t.Fatal("非法时长应报错")
	}
}

func TestSetConfigTestFileRawURL(t *testing.T) {
	withTempConfigDir(t)
	if err := setConfig("test_file_raw_url", "  https://raw.githubusercontent.com/caiqianzhang/gitcn/main/LICENSE  "); err != nil {
		t.Fatalf("setConfig: %v", err)
	}
	c, err := config.Load()
	if err != nil {
		t.Fatal(err)
	}
	want := "https://raw.githubusercontent.com/caiqianzhang/gitcn/main/LICENSE"
	if c.TestFileRawURL != want {
		t.Fatalf("TestFileRawURL = %q, want %q", c.TestFileRawURL, want)
	}
}
