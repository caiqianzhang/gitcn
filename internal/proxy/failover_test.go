package proxy

import (
	"errors"
	"strings"
	"testing"
)

func TestRunWithFailoverSuccessOnSecond(t *testing.T) {
	calls := 0
	err := RunWithFailover([]string{"a", "b"}, func(node string) error {
		calls++
		if node == "a" {
			return errors.New("boom")
		}
		return nil
	})
	if err != nil {
		t.Fatalf("RunWithFailover: %v", err)
	}
	if calls != 2 {
		t.Fatalf("calls = %d, want 2", calls)
	}
}

func TestRunWithFailoverAllFail(t *testing.T) {
	err := RunWithFailover([]string{"a", "b"}, func(node string) error {
		return errors.New("fail " + node)
	})
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "a") || !strings.Contains(err.Error(), "b") {
		t.Fatalf("错误应包含各节点信息: %v", err)
	}
}
