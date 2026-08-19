package main

import (
	"fmt"
	"os"

	"github.com/caiqianzhang/gitcn/internal/cli"
)

func main() {
	if err := cli.Run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "gitcn:", err)
		os.Exit(cli.ExitCode(err))
	}
}
