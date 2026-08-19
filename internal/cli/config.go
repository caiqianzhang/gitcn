package cli

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/caiqianzhang/gitcn/internal/config"
)

func cmdConfig(args []string) error {
	if len(args) == 0 {
		c, err := config.Load()
		if err != nil {
			return err
		}
		b, _ := json.MarshalIndent(c, "", "  ")
		p, _ := config.Path()
		fmt.Printf("%s\n%s\n", b, p)
		return nil
	}
	if len(args) != 2 {
		return fmt.Errorf("用法: gitcn config <key> <value>")
	}
	return setConfig(args[0], args[1])
}

func setConfig(key, value string) error {
	c, err := config.Load()
	if err != nil {
		return err
	}
	switch key {
	case "cache_ttl":
		d, err := time.ParseDuration(value)
		if err != nil {
			return fmt.Errorf("cache_ttl 需要时长，如 30m/1h: %v", err)
		}
		c.CacheTTL = d
	case "timeout":
		d, err := time.ParseDuration(value)
		if err != nil {
			return fmt.Errorf("timeout 需要时长，如 3s: %v", err)
		}
		c.Timeout = d
	case "mirror_sources":
		var srcs []string
		for _, s := range strings.Split(value, ",") {
			if s = strings.TrimSpace(s); s != "" {
				srcs = append(srcs, s)
			}
		}
		c.MirrorSources = srcs
	case "max_retry":
		n, err := strconv.Atoi(value)
		if err != nil || n < 1 {
			return fmt.Errorf("max_retry 需要正整数")
		}
		c.MaxRetry = n
	case "concurrency":
		n, err := strconv.Atoi(value)
		if err != nil || n < 1 {
			return fmt.Errorf("concurrency 需要正整数")
		}
		c.Concurrency = n
	case "verbose":
		b, err := strconv.ParseBool(value)
		if err != nil {
			return fmt.Errorf("verbose 需要 true/false")
		}
		c.Verbose = b
	default:
		return fmt.Errorf("未知配置项 %q（可选: cache_ttl/timeout/mirror_sources/max_retry/concurrency/verbose）", key)
	}
	return config.Save(c)
}
