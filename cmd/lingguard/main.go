package main

import (
	"os"
	"path/filepath"

	"github.com/lingguard/cmd/cli"
)

func main() {
	// 设置配置路径（按优先级）
	configPath := os.Getenv("LINGGUARD_CONFIG")
	if configPath == "" {
		// 默认使用 ~/.lingguard/config.json
		home, _ := os.UserHomeDir()
		configPath = filepath.Join(home, ".lingguard", "config.json")
	}

	if err := cli.Execute(configPath); err != nil {
		os.Exit(1)
	}
}
