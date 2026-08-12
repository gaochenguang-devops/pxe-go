// Package web 存放前端管理后台静态资源。
// 前端源码独立于 assets/web_root（后者包含体积大且运行时变化的系统安装源），
// 通过 //go:embed 将 ui 编译进二进制，支持单文件分发（无需依赖磁盘目录）。
package web

import "embed"

// UI 前端管理后台静态资源（ui）。
//
//go:embed ui
var UI embed.FS
