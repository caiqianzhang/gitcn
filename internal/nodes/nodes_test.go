package nodes

import "testing"

func TestSeedParses(t *testing.T) {
	hosts, err := Seed()
	if err != nil {
		t.Fatalf("Seed: %v", err)
	}
	if len(hosts) < 50 {
		t.Fatalf("种子节点太少: %d, want >= 50", len(hosts))
	}
	seen := map[string]bool{}
	for _, h := range hosts {
		if h == "" {
			t.Fatal("存在空节点")
		}
		if seen[h] {
			t.Fatalf("重复节点: %s", h)
		}
		seen[h] = true
	}
}
