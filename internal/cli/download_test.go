package cli

import (
	"context"
	"crypto/sha256"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

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
	got, err := downloadFile(context.Background(), srv.URL, out, time.Minute)
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

	if _, err := downloadFile(context.Background(), srv.URL, filepath.Join(t.TempDir(), "x"), time.Minute); err == nil {
		t.Fatal("应拒绝 text/html 假代理响应")
	}
}

func TestDownloadRejectsBadArgs(t *testing.T) {
	if err := cmdDownload(context.Background(), []string{"https://github.com/a/b/releases/download/v1/x.zip", "extra"}); err == nil {
		t.Fatal("多余位置参数应报错")
	}
	if err := cmdDownload(context.Background(), []string{"-o", "", "https://github.com/a/b/releases/download/v1/x.zip"}); err == nil {
		t.Fatal("-o 空文件名应报错")
	}
}

func TestDownloadFallsBackDirectWhenNoNodes(t *testing.T) {
	withTempConfigDir(t)
	origBest, origDl := bestNodes, downloadFileFn
	var gotURL, gotOut string
	bestNodes = func(ctx context.Context, cfg *config.Config) ([]string, error) {
		return nil, fmt.Errorf("所有节点均不可用")
	}
	downloadFileFn = func(ctx context.Context, url, out string, timeout time.Duration) (int64, error) {
		gotURL, gotOut = url, out
		fmt.Printf("已保存 %s (3 字节)\n", out)
		return 3, nil
	}
	defer func() { bestNodes, downloadFileFn = origBest, origDl }()

	raw := "https://raw.githubusercontent.com/foo/bar/main/run.sh"
	if err := cmdDownload(context.Background(), []string{raw}); err != nil {
		t.Fatalf("cmdDownload: %v", err)
	}
	if gotURL != raw {
		t.Fatalf("直连 URL = %q, want %q", gotURL, raw)
	}
	if gotOut != "run.sh" {
		t.Fatalf("输出文件名 = %q, want run.sh", gotOut)
	}
}

func TestDownloadFileRespectsTimeout(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(200 * time.Millisecond)
	}))
	defer srv.Close()

	if _, err := downloadFile(context.Background(), srv.URL, filepath.Join(t.TempDir(), "x"), 20*time.Millisecond); err == nil {
		t.Fatal("超过 download_timeout 应使下载失败")
	}
}

func TestDownloadFileHonorsCancel(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(200 * time.Millisecond)
	}))
	defer srv.Close()

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // 模拟 Ctrl+C
	if _, err := downloadFile(ctx, srv.URL, filepath.Join(t.TempDir(), "x"), time.Minute); err == nil {
		t.Fatal("已取消的 ctx 应立即中断下载")
	}
}

func TestVerifySHA256(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "f")
	content := []byte("fixture bytes for checksum")
	if err := os.WriteFile(p, content, 0o644); err != nil {
		t.Fatal(err)
	}
	good := fmt.Sprintf("%x", sha256.Sum256(content))
	if err := verifySHA256(p, good); err != nil {
		t.Fatalf("匹配的哈希应通过: %v", err)
	}
	if err := verifySHA256(p, strings.Repeat("0", 64)); err == nil {
		t.Fatal("不匹配的哈希应报错")
	}
}

func TestDownloadVerifiesSHA256Success(t *testing.T) {
	withTempConfigDir(t)
	content := []byte("fixture bytes for checksum")
	want := fmt.Sprintf("%x", sha256.Sum256(content))
	origBest, origDl := bestNodes, downloadFileFn
	bestNodes = func(ctx context.Context, cfg *config.Config) ([]string, error) {
		return []string{"node.example"}, nil
	}
	downloadFileFn = func(ctx context.Context, url, out string, timeout time.Duration) (int64, error) {
		if err := os.WriteFile(out, content, 0o644); err != nil {
			return 0, err
		}
		fmt.Printf("已保存 %s (%d 字节)\n", out, len(content))
		return int64(len(content)), nil
	}
	defer func() { bestNodes, downloadFileFn = origBest, origDl }()

	raw := "https://github.com/a/b/releases/download/v1/x.zip"
	out := filepath.Join(t.TempDir(), "x.zip")
	if err := cmdDownload(context.Background(), []string{raw, "-o", out, "--sha256", want}); err != nil {
		t.Fatalf("校验匹配应成功: %v", err)
	}
	if _, err := os.Stat(out); err != nil {
		t.Fatal("校验通过后文件应保留")
	}
}

func TestDownloadSHA256MismatchFailsAndRemovesFile(t *testing.T) {
	withTempConfigDir(t)
	content := []byte("fixture bytes for checksum")
	wrong := strings.Repeat("0", 64)
	origBest, origDl := bestNodes, downloadFileFn
	bestNodes = func(ctx context.Context, cfg *config.Config) ([]string, error) {
		return []string{"node.example"}, nil
	}
	downloadFileFn = func(ctx context.Context, url, out string, timeout time.Duration) (int64, error) {
		if err := os.WriteFile(out, content, 0o644); err != nil {
			return 0, err
		}
		fmt.Printf("已保存 %s (%d 字节)\n", out, len(content))
		return int64(len(content)), nil
	}
	defer func() { bestNodes, downloadFileFn = origBest, origDl }()

	raw := "https://github.com/a/b/releases/download/v1/x.zip"
	out := filepath.Join(t.TempDir(), "x.zip")
	if err := cmdDownload(context.Background(), []string{raw, "-o", out, "--sha256", wrong}); err == nil {
		t.Fatal("校验不匹配应报错")
	}
	if _, err := os.Stat(out); !os.IsNotExist(err) {
		t.Fatal("校验失败后应删除文件")
	}
}

func TestDownloadRejectsInvalidSHA256(t *testing.T) {
	withTempConfigDir(t)
	raw := "https://github.com/a/b/releases/download/v1/x.zip"
	if err := cmdDownload(context.Background(), []string{raw, "--sha256", "not-a-hex"}); err == nil {
		t.Fatal("非法 --sha256 值应报错")
	}
}
