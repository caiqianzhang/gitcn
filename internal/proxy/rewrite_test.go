package proxy

import (
	"strings"
	"testing"
)

func TestDetectKinds(t *testing.T) {
	cases := []struct {
		in   string
		kind Kind
		want string
	}{
		{"https://github.com/foo/bar.git", KindGit, "https://github.com/foo/bar.git"},
		{"https://github.com/foo/bar", KindGit, "https://github.com/foo/bar"},
		{"https://github.com/foo/bar/releases/download/v1.0/app.zip", KindRelease, "https://github.com/foo/bar/releases/download/v1.0/app.zip"},
		{"https://raw.githubusercontent.com/foo/bar/main/run.sh", KindRaw, "https://raw.githubusercontent.com/foo/bar/main/run.sh"},
		{"https://github.com/foo/bar/archive/refs/tags/v1.0.0.zip", KindArchive, "https://github.com/foo/bar/archive/refs/tags/v1.0.0.zip"},
		{"https://gist.github.com/abc123/file.sh", KindGist, "https://gist.github.com/abc123/file.sh"},
		{"https://kkgithub.com/foo/bar", KindGit, "https://github.com/foo/bar"},
	}
	for _, c := range cases {
		kind, norm, err := Detect(c.in)
		if err != nil {
			t.Fatalf("Detect(%q): %v", c.in, err)
		}
		if kind != c.kind {
			t.Errorf("Detect(%q) kind = %v, want %v", c.in, kind, c.kind)
		}
		if norm != c.want {
			t.Errorf("Detect(%q) norm = %q, want %q", c.in, norm, c.want)
		}
	}
}

func TestDetectRejectsNonGitHub(t *testing.T) {
	for _, in := range []string{"https://gitlab.com/foo/bar", "https://example.com/x", "not a url"} {
		if _, _, err := Detect(in); err == nil {
			t.Errorf("Detect(%q) 应报错", in)
		}
	}
}

func TestRewrite(t *testing.T) {
	got, err := Rewrite("https://github.com/foo/bar.git", "gh.dpik.top")
	if err != nil {
		t.Fatal(err)
	}
	want := "https://gh.dpik.top/https://github.com/foo/bar.git"
	if got != want {
		t.Fatalf("Rewrite = %q, want %q", got, want)
	}
}

func TestDetectNormalizesWWW(t *testing.T) {
	for _, in := range []string{
		"https://www.github.com/foo/bar",
		"https://www.raw.githubusercontent.com/foo/bar/main/x.sh",
	} {
		_, norm, err := Detect(in)
		if err != nil {
			t.Fatalf("Detect(%q): %v", in, err)
		}
		if strings.Contains(norm, "www.") {
			t.Errorf("Detect(%q) norm 仍含 www: %q", in, norm)
		}
	}
}

func TestRewriteStripsUserinfo(t *testing.T) {
	got, err := Rewrite("https://ghp_TOKEN@github.com/foo/bar.git", "gh.dpik.top")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(got, "ghp_TOKEN") || strings.Contains(got, "@") {
		t.Fatalf("Rewrite 泄露 userinfo: %q", got)
	}
	want := "https://gh.dpik.top/https://github.com/foo/bar.git"
	if got != want {
		t.Fatalf("Rewrite = %q, want %q", got, want)
	}
}
