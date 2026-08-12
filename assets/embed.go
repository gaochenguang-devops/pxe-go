// Package assets 存放随二进制打包的静态资源。
package assets

import "embed"

// UI 前端管理后台静态资源（web_root/ui）。
// 通过 //go:embed 将 index.html 编译进二进制，支持单文件分发（无需依赖磁盘目录）。
// 注意：仅嵌入 web_root/ui，web_root 下的系统安装源（rpm/ISO 解压等）体积大且运行时可变，不嵌入。
//
//go:embed web_root/ui
var UI embed.FS
