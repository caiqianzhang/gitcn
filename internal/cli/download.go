package cli

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/caiqianzhang/gitcn/internal/config"
	"github.com/caiqianzhang/gitcn/internal/proxy"
)

// downloadFileFn 提供测试注入点（seam）。
var downloadFileFn = downloadFile

func cmdDownload(args []string) error {
	var out string
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
		default:
			urls = append(urls, args[i])
		}
	}
	if len(urls) != 1 {
		return fmt.Errorf("用法: gitcn download <url> [-o 文件名]")
	}
	if hasOut && out == "" {
		return fmt.Errorf("-o 需要非空文件名")
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

	candidates, err := bestNodes(context.Background(), cfg)
	if err != nil {
		// 无可用节点时直连兜底，与 clone/fetch/pull 保持一致。
		if cfg.Verbose {
			fmt.Fprintf(os.Stderr, "提示: %v，将直连执行\n", err)
		}
		return downloadDirect(urlArg, out, cfg)
	}

	err = proxy.RunWithFailover(candidates, func(node string) error {
		warnIfUserinfo(urlArg)
		rewritten, err := proxy.Rewrite(urlArg, node)
		if err != nil {
			return err
		}
		if cfg.Verbose {
			fmt.Printf("→ 使用节点 %s\n", node)
		}
		n, err := downloadFileFn(rewritten, out)
		if err != nil {
			return err
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
func downloadDirect(urlArg, out string, cfg *config.Config) error {
	if cfg.Verbose {
		fmt.Println("→ 直连下载")
	}
	n, err := downloadFileFn(urlArg, out)
	if err != nil {
		os.Remove(out)
		return err
	}
	fmt.Printf("已保存 %s (%d 字节)\n", out, n)
	return nil
}

// downloadFile 下载 URL 到 out，带简单进度；拒绝 text/html 假代理响应。
func downloadFile(url, out string) (int64, error) {
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return 0, err
	}
	req.Header.Set("User-Agent", "github.com/caiqianzhang/gitcn/"+Version)
	client := &http.Client{Timeout: 10 * time.Minute}
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
