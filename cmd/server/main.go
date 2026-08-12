// 入口：定义版本变量并执行 Cobra 根命令。
package main

import (
	"fmt"
	"os"
)

// version 通过 -ldflags "-X main.version=<tag>" 注入，未指定时默认为 dev。
var version = "dev"

func main() {
	cmd := newRootCmd()
	cmd.SetArgs(normalizeArgs(os.Args[1:]))
	if err := cmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
