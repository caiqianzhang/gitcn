package nodes

import (
	_ "embed"
	"encoding/json"
	"fmt"
)

//go:embed nodes.json
var seedRaw []byte

type NodeList struct {
	Version   int      `json:"version"`
	UpdatedAt string   `json:"updated_at"`
	Nodes     []string `json:"nodes"`
}

// Seed 返回内置种子节点列表。
func Seed() ([]string, error) {
	var l NodeList
	if err := json.Unmarshal(seedRaw, &l); err != nil {
		return nil, fmt.Errorf("解析内置节点失败: %w", err)
	}
	if len(l.Nodes) == 0 {
		return nil, fmt.Errorf("内置节点列表为空")
	}
	return l.Nodes, nil
}
