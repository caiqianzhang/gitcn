package cli

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/caiqianzhang/gitcn/internal/config"
)

func TestDownloadFileToTarget(t *testing.T) {
	content := []byte("hello gitcn download")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/octet-stream")
		w.Header().Set("Content-Length", fmt.Sprintf("%d", len(content)))
		w.Write(content)
	}))
	defer srv.Close()

	dir := t.TempDir()
	out := filepath.Join(dir, "app.zip")
	got, err := downloadFile(srv.URL, out)
	if err != nil {
		t.Fatalf("downloadFile: %v", err)
	}
	if got != int64(len(content)) {
		t.Fatalf("downloaded = %d, want %d", got, len(content))
	}
	b, _ := os.ReadFile(out)
	if string(b) != string(content) {
		t.Fatalf("内容不符: %q", b)
	}
}

func TestDownloadFileRejectsHTMLFake(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.Write([]byte("<html>fake site</html>"))
	}))
	defer srv.Close()

	if _, err := downloadFile(srv.URL, filepath.Join(t.TempDir(), "x")); err == nil {
		t.Fatal("应拒绝 text/html 假代理响应")
	}
}

func TestDownloadRejectsBadArgs(t *testing.T) {
	if err := cmdDownload([]string{"https://github.com/a/b/releases/download/v1/x.zip", "extra"}); err == nil {
		t.Fatal("多余位置参数应报错")
	}
	if err := cmdDownload([]string{"-o", "", "https://github.com/a/b/releases/download/v1/x.zip"}); err == nil {
		t.Fatal("-o 空文件名应报错")
	}
}

func TestDownloadFallsBackDirectWhenNoNodes(t *testing.T) {
	withTempXDG(t)
	origBest, origDl := bestNodes, downloadFileFn
	var gotURL, gotOut string
	bestNodes = func(ctx context.Context, cfg *config.Config) ([]string, error) {
		return nil, fmt.Errorf("所有节点均不可用")
	}
	downloadFileFn = func(url, out string) (int64, error) {
		gotURL, gotOut = url, out
		fmt.Printf("已保存 %s (3 字节)\n", out)
		return 3, nil
	}
	defer func() { bestNodes, downloadFileFn = origBest, origDl }()

	raw := "https://raw.githubusercontent.com/foo/bar/main/run.sh"
	if err := cmdDownload([]string{raw}); err != nil {
		t.Fatalf("cmdDownload: %v", err)
	}
	if gotURL != raw {
		t.Fatalf("直连 URL = %q, want %q", gotURL, raw)
	}
	if gotOut != "run.sh" {
		t.Fatalf("输出文件名 = %q, want run.sh", gotOut)
	}
}
