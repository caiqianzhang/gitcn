package cli

import (
	"context"
	"testing"

	"github.com/caiqianzhang/gitcn/internal/config"
	"github.com/caiqianzhang/gitcn/internal/nodes"
)

func withTempConfigDir(t *testing.T) {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("GITCN_CONFIG_DIR", dir)
}

func TestNodesAddPersists(t *testing.T) {
	withTempConfigDir(t)
	if err := nodesAdd("my.node.example"); err != nil {
		t.Fatalf("nodesAdd: %v", err)
	}
	c, err := nodes.LoadCache()
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, n := range c.Nodes {
		if n.Host == "my.node.example" {
			found = true
		}
	}
	if !found {
		t.Fatal("添加的节点未持久化")
	}
}

func TestNodesUpdateFetchesRemote(t *testing.T) {
	withTempConfigDir(t)
	cfg, _ := config.Load()
	cfg.MirrorSources = []string{"http://127.0.0.1:1/nodes.json"}
	if err := nodesUpdate(context.Background(), cfg); err == nil {
		t.Fatal("远端不可达时 update 应报错")
	}
}
