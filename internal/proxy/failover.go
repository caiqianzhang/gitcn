package proxy

import (
	"fmt"
	"strings"
)

// RunWithFailover 依次用候选节点调用 fn；任一成功即返回 nil，全部失败返回汇总错误。
func RunWithFailover(candidates []string, fn func(node string) error) error {
	var errs []string
	for _, node := range candidates {
		if err := fn(node); err == nil {
			return nil
		} else {
			errs = append(errs, fmt.Sprintf("  节点 %s: %v", node, err))
		}
	}
	return fmt.Errorf("候选节点全部失败:\n%s", strings.Join(errs, "\n"))
}
