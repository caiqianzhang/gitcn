package proxy

import (
	"os"
	"os/exec"
)

// ExitError 包装 git 的非零退出。
type ExitError struct {
	Code int
	Err  error
}

func (e *ExitError) Error() string { return e.Err.Error() }

// RunGit 在终端中执行系统 git，透传 stdin/stdout/stderr。
func RunGit(args []string) error {
	cmd := exec.Command("git", args...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		if ee, ok := err.(*exec.ExitError); ok {
			return &ExitError{Code: ee.ExitCode(), Err: err}
		}
		return err
	}
	return nil
}
