package cli

import (
	"errors"
	"fmt"

	"github.com/caiqianzhang/gitcn/internal/proxy"
)

// Version 由构建时注入或手动更新。
const Version = "0.1.0"

// ExitCode 返回命令错误对应的进程退出码（透传 git 的真实退出码）。
func ExitCode(err error) int {
	var ee *proxy.ExitError
	if errors.As(err, &ee) {
		return ee.Code
	}
	return 1
}

func Run(args []string) error {
	if len(args) == 0 {
		printUsage()
		return nil
	}
	switch args[0] {
	case "-h", "--help", "help":
		printUsage()
		return nil
	case "clone":
		return cmdClone(args[1:])
	case "fetch":
		return cmdFetch(args[1:])
	case "pull":
		return cmdPull(args[1:])
	case "download":
		return cmdDownload(args[1:])
	case "nodes":
		return cmdNodes(args[1:])
	case "config":
		return cmdConfig(args[1:])
	case "version", "-v":
		fmt.Println("gitcn", Version)
		return nil
	default:
		return fmt.Errorf("未知命令 %q，运行 gitcn help 查看用法", args[0])
	}
}

func printUsage() {
	fmt.Println(`gitcn - 国内 GitHub 访问加速工具

用法:
  gitcn clone <url|owner/repo> [git参数...]   加速克隆
  gitcn fetch [git参数...]                     加速拉取(当前仓库)
  gitcn pull [git参数...]                      加速拉取并合并(当前仓库)
  gitcn download <url> [-o 文件名]             加速下载 Release/Raw/Archive/Gist
  gitcn nodes list|update|add <domain>         管理代理节点
  gitcn config [key value]                     查看或修改配置
  gitcn version                                显示版本`)
}
