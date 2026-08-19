package proxy

import (
	"fmt"
	"net/url"
	"strings"
)

type Kind int

const (
	KindGit Kind = iota
	KindRelease
	KindRaw
	KindArchive
	KindGist
)

// Detect 判断 URL 类型并返回归一化 URL（kkgithub.com → github.com）。
func Detect(raw string) (Kind, string, error) {
	s := strings.TrimSpace(raw)
	if s == "" {
		return 0, "", fmt.Errorf("链接为空")
	}
	u, err := url.Parse(s)
	if err != nil || u.Host == "" {
		return 0, "", fmt.Errorf("无效链接: %s", raw)
	}
	host := strings.ToLower(u.Host)
	base := ""
	// 注意顺序：更具体的后缀必须先判断（gist/kkgithub 也以 github.com 结尾）。
	switch {
	case strings.HasSuffix(host, "raw.githubusercontent.com"):
		base = "raw.githubusercontent.com"
	case strings.HasSuffix(host, "gist.github.com"):
		base = "gist.github.com"
	case strings.HasSuffix(host, "kkgithub.com"):
		base = "kkgithub.com"
	case strings.HasSuffix(host, "github.com"):
		base = "github.com"
	default:
		return 0, "", fmt.Errorf("仅支持 GitHub 系链接: %s", raw)
	}
	if base == "kkgithub.com" {
		u.Host = "github.com"
	}
	// 统一去除 www. 前缀，避免改写后残留 www 导致部分代理不识别。
	if strings.HasPrefix(u.Host, "www.") {
		u.Host = strings.TrimPrefix(u.Host, "www.")
	}
	// 剥离 userinfo，防止令牌/账号信息随改写后的 URL 泄露给代理节点。
	u.User = nil
	norm := u.String()
	path := u.Path
	switch {
	case strings.Contains(path, "/releases/download/"):
		return KindRelease, norm, nil
	case base == "raw.githubusercontent.com" || strings.HasPrefix(path, "/raw/"):
		return KindRaw, norm, nil
	case strings.Contains(path, "/archive/refs/"):
		return KindArchive, norm, nil
	case base == "gist.github.com":
		return KindGist, norm, nil
	default:
		return KindGit, norm, nil
	}
}

// Rewrite 把 GitHub URL 改写为 https://<node>/<原URL>。
func Rewrite(raw, node string) (string, error) {
	_, norm, err := Detect(raw)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("https://%s/%s", node, norm), nil
}
