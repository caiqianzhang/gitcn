package cli

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/caiqianzhang/gitcn/internal/config"
	"github.com/caiqianzhang/gitcn/internal/nodes"
	"github.com/caiqianzhang/gitcn/internal/proxy"
)

// downloadFileFn 提供测试注入点（seam）。
var downloadFileFn = downloadFile

func cmdDownload(ctx context.Context, args []string) error {
	var out, wantSHA string
	hasOut := false
	var urls []string
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "-o", "--output":
			if i+1 >= len(args) {
				return fmt.Errorf("-o 需要文件名")
			}
			i++
			out = args[i]
			hasOut = true
		case "--sha256":
			if i+1 >= len(args) {
				return fmt.Errorf("--sha256 需要 64 位十六进制哈希")
			}
			i++
			wantSHA = args[i]
		default:
			urls = append(urls, args[i])
		}
	}
	if len(urls) != 1 {
		return fmt.Errorf("用法: gitcn download <url> [-o 文件名] [--sha256 <hex>]")
	}
	if hasOut && out == "" {
		return fmt.Errorf("-o 需要非空文件名")
	}
	if wantSHA != "" && !isSHA256Hex(wantSHA) {
		return fmt.Errorf("--sha256 需要 64 位十六进制哈希")
	}
	urlArg := urls[0]
	if _, _, err := proxy.Detect(urlArg); err != nil {
		return err
	}

	cfg, err := config.Load()
	if err != nil {
		return err
	}
	if out == "" {
		out = filenameFromURL(urlArg)
	}

	candidates, err := bestNodes(ctx, cfg)
	if err != nil {
		// 无可用节点时直连兜底，与 clone/fetch/pull 保持一致。
		if cfg.Verbose {
			fmt.Fprintf(os.Stderr, "提示: %v，将直连执行\n", err)
		}
		return downloadDirect(ctx, urlArg, out, cfg, wantSHA)
	}

	err = proxy.RunWithFailover(candidates, func(node string) error {
		if err := ctx.Err(); err != nil {
			return err // Ctrl+C/SIGTERM 已触发，停止切换下一候选
		}
		warnIfUserinfo(urlArg)
		rewritten, err := proxy.Rewrite(urlArg, node)
		if err != nil {
			return err
		}
		if cfg.Verbose {
			fmt.Printf("→ 使用节点 %s\n", node)
		}
		n, err := downloadFileFn(ctx, rewritten, out, cfg.DownloadTimeout)
		if err != nil {
			nodes.MarkDead(node)
			return err
		}
		if wantSHA != "" {
			if verr := verifySHA256(out, wantSHA); verr != nil {
				os.Remove(out)
				nodes.MarkDead(node) // 节点内容被篡改/损坏，标记后自动切换下一候选
				return verr
			}
		}
		fmt.Printf("已保存 %s (%d 字节)\n", out, n)
		return nil
	})
	if err != nil {
		// 全部节点失败：清理半成品文件，避免残留部分下载内容。
		os.Remove(out)
		if cfg.Verbose {
			fmt.Fprintf(os.Stderr, "提示: 代理节点均失败，可尝试直连下载\n")
		}
		return err
	}
	return nil
}

// downloadDirect 直连下载（无可用节点时的兜底）。
func downloadDirect(ctx context.Context, urlArg, out string, cfg *config.Config, wantSHA string) error {
	if cfg.Verbose {
		fmt.Println("→ 直连下载")
	}
	n, err := downloadFileFn(ctx, urlArg, out, cfg.DownloadTimeout)
	if err != nil {
		os.Remove(out)
		return err
	}
	if wantSHA != "" {
		if verr := verifySHA256(out, wantSHA); verr != nil {
			os.Remove(out)
			return verr
		}
	}
	fmt.Printf("已保存 %s (%d 字节)\n", out, n)
	return nil
}

// downloadFile 下载 URL 到 out，带简单进度；拒绝 text/html 假代理响应。
// ctx 用于让 Ctrl+C/SIGTERM 中断进行中的下载。
func downloadFile(ctx context.Context, url, out string, timeout time.Duration) (int64, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return 0, err
	}
	req.Header.Set("User-Agent", "github.com/caiqianzhang/gitcn/"+Version)
	client := &http.Client{Timeout: timeout}
	resp, err := client.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	if ct := resp.Header.Get("Content-Type"); strings.Contains(ct, "text/html") {
		return 0, fmt.Errorf("节点返回 text/html（可能不支持该链接）")
	}
	f, err := os.Create(out)
	if err != nil {
		return 0, err
	}
	defer f.Close()

	total := resp.ContentLength
	var n int64
	buf := make([]byte, 64<<10)
	lastPrint := time.Now()
	for {
		read, rerr := resp.Body.Read(buf)
		if read > 0 {
			w, werr := f.Write(buf[:read])
			n += int64(w)
			if werr != nil {
				return n, werr
			}
			if total > 0 && time.Since(lastPrint) > time.Second {
				fmt.Printf("\r下载中 %d%%", n*100/total)
				lastPrint = time.Now()
			}
		}
		if rerr == io.EOF {
			break
		}
		if rerr != nil {
			return n, rerr
		}
	}
	if total > 0 {
		fmt.Printf("\r下载完成 100%%\n")
	} else {
		fmt.Println()
	}
	return n, nil
}

func filenameFromURL(raw string) string {
	u := raw
	if i := strings.Index(raw, "://"); i >= 0 {
		u = raw[i+3:]
	}
	if i := strings.IndexAny(u, "?#"); i >= 0 {
		u = u[:i]
	}
	segs := strings.Split(u, "/")
	name := segs[len(segs)-1]
	if name == "" {
		name = "download.bin"
	}
	return filepath.Base(name)
}

// isSHA256Hex 判断是否为 64 位十六进制（大小写均可）。
func isSHA256Hex(s string) bool {
	if len(s) != 64 {
		return false
	}
	for _, c := range s {
		switch {
		case c >= '0' && c <= '9', c >= 'a' && c <= 'f', c >= 'A' && c <= 'F':
		default:
			return false
		}
	}
	return true
}

// verifySHA256 计算文件的 SHA-256 并与期望值比对，不匹配返回错误。
func verifySHA256(path, want string) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return err
	}
	got := hex.EncodeToString(h.Sum(nil))
	if got != strings.ToLower(want) {
		return fmt.Errorf("SHA-256 校验失败: 期望 %s，实际 %s", want, got)
	}
	return nil
}
