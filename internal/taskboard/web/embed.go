// Package web Web UI 静态文件
package web

import "embed"

// StaticFiles 静态文件嵌入
//
//go:embed static/*
var StaticFiles embed.FS
