// Package cli CLI 命令 - Doctor 诊断
package cli

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"time"

	"github.com/fatih/color"
	"github.com/lingguard/internal/config"
	"github.com/lingguard/internal/providers"
	"github.com/lingguard/pkg/embedding"
	"github.com/lingguard/pkg/llm"
	"github.com/spf13/cobra"
)

// 注意: expandPath 函数在 config_cmd.go 中定义

var (
	doctorFix bool
)

var doctorCmd = &cobra.Command{
	Use:   "doctor",
	Short: "Run system diagnostics",
	Long:  `Run comprehensive diagnostics to check system health and configuration.`,
	Run: func(cmd *cobra.Command, args []string) {
		runDoctor()
	},
}

func init() {
	doctorCmd.Flags().BoolVar(&doctorFix, "fix", false, "Automatically fix issues where possible")
	rootCmd.AddCommand(doctorCmd)
}

// DiagnosticResult 诊断结果
type DiagnosticResult struct {
	Name    string
	Status  string // "ok", "warn", "error", "skip"
	Message string
	Fix     string // 修复建议
	CanFix  bool   // 是否可以自动修复
	Fixed   bool   // 是否已修复
}

// Doctor 诊断器
type Doctor struct {
	configPath string
	config     *config.Config
	results    []DiagnosticResult
	fixMode    bool
}

func runDoctor() {
	printHeader("LingGuard Doctor")

	doctor := &Doctor{
		configPath: cfgPath,
		fixMode:    doctorFix,
	}

	// 1. 配置文件检查
	doctor.checkConfig()

	// 2. Provider 检查
	doctor.checkProviders()

	// 3. 记忆系统检查
	doctor.checkMemory()

	// 4. 工作目录检查
	doctor.checkWorkspace()

	// 5. 渠道配置检查
	doctor.checkChannels()

	// 6. 系统环境检查
	doctor.checkSystem()

	// 7. MCP 服务检查
	doctor.checkMCP()

	// 打印结果
	doctor.printResults()
}

func printHeader(title string) {
	bold := color.New(color.Bold, color.FgCyan)
	bold.Printf("\n🦞 %s\n", title)
	bold.Println(strings.Repeat("─", 50))
}

func (d *Doctor) addResult(name, status, message, fix string, canFix bool) {
	d.results = append(d.results, DiagnosticResult{
		Name:    name,
		Status:  status,
		Message: message,
		Fix:     fix,
		CanFix:  canFix,
	})
}

// checkConfig 检查配置文件
func (d *Doctor) checkConfig() {
	// 检查配置文件是否存在
	if _, err := os.Stat(d.configPath); os.IsNotExist(err) {
		d.addResult("Config File", "error",
			fmt.Sprintf("Config file not found: %s", d.configPath),
			"Run 'lingguard init' to create a config file", true)
		return
	}

	// 加载配置
	cfg, err := config.Load(d.configPath)
	if err != nil {
		d.addResult("Config File", "error",
			fmt.Sprintf("Failed to load config: %v", err),
			"Check config file format (JSON)", false)
		return
	}
	d.config = cfg

	d.addResult("Config File", "ok",
		fmt.Sprintf("Config loaded: %s", d.configPath), "", false)

	// 检查必填字段
	if cfg.Agents.Provider == "" {
		d.addResult("Agent Provider", "error",
			"Agent provider not configured",
			"Set 'agents.provider' in config", true)
	} else {
		d.addResult("Agent Provider", "ok",
			fmt.Sprintf("Provider: %s", cfg.Agents.Provider), "", false)
	}

	// 检查系统提示
	if cfg.Agents.SystemPrompt == "" {
		d.addResult("System Prompt", "warn",
			"System prompt is empty",
			"Consider setting 'agents.systemPrompt'", false)
	}
}

// checkProviders 检查 Provider 配置
func (d *Doctor) checkProviders() {
	if d.config == nil {
		return
	}

	// 检查默认 Provider
	defaultProvider := d.config.Agents.Provider
	if defaultProvider == "" {
		d.addResult("Default Provider", "error",
			"No default provider configured",
			"Set 'agents.provider' in config", true)
		return
	}

	// 检查默认 Provider 的配置
	hasDefaultConfig := false
	for name, pc := range d.config.Providers {
		if name == defaultProvider {
			hasDefaultConfig = true
			if pc.APIKey == "" || pc.APIKey == "sk-xxx" || pc.APIKey == "xxx" {
				d.addResult("Provider API Key", "error",
					fmt.Sprintf("API key not configured for '%s'", name),
					fmt.Sprintf("Set 'providers.%s.apiKey' in config", name), false)
			} else {
				// 尝试测试连接
				d.testProviderConnection(name, pc)
			}
			break
		}
	}

	if !hasDefaultConfig {
		d.addResult("Default Provider", "error",
			fmt.Sprintf("Provider '%s' not found in providers config", defaultProvider),
			fmt.Sprintf("Add 'providers.%s' section in config", defaultProvider), true)
	}

	// 统计配置的 Provider 数量
	configuredCount := 0
	for _, pc := range d.config.Providers {
		if pc.APIKey != "" && pc.APIKey != "sk-xxx" && pc.APIKey != "xxx" {
			configuredCount++
		}
	}
	d.addResult("Providers", "ok",
		fmt.Sprintf("%d providers configured", configuredCount), "", false)
}

// testProviderConnection 测试 Provider 连接
func (d *Doctor) testProviderConnection(providerName string, pc config.ProviderConfig) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	// 创建 Registry 并初始化
	registry := providers.NewRegistry()
	if err := registry.InitFromConfig(d.config); err != nil {
		d.addResult("Provider Test", "error",
			fmt.Sprintf("Failed to init providers: %v", err),
			"Check provider configuration", false)
		return
	}

	// 获取 Provider
	provider, ok := registry.Get(providerName)
	if !ok {
		d.addResult("Provider Test", "error",
			fmt.Sprintf("Provider '%s' not found", providerName), "", false)
		return
	}

	// 简单的连接测试（发送一个最小请求）
	testReq := &llm.Request{
		Model: provider.Model(),
		Messages: []llm.Message{
			{Role: "user", Content: "hi"},
		},
		MaxTokens: 5,
	}

	_, err := provider.Complete(ctx, testReq)
	if err != nil {
		// 检查是否是 API Key 问题
		errMsg := err.Error()
		if strings.Contains(errMsg, "401") || strings.Contains(errMsg, "unauthorized") || strings.Contains(errMsg, "invalid") {
			d.addResult("Provider Connection", "error",
				fmt.Sprintf("API key invalid for '%s'", providerName),
				"Check your API key", false)
		} else if strings.Contains(errMsg, "timeout") || strings.Contains(errMsg, "connection") {
			d.addResult("Provider Connection", "warn",
				fmt.Sprintf("Connection timeout for '%s' (may be network issue)", providerName),
				"Check network connection", false)
		} else {
			d.addResult("Provider Connection", "warn",
				fmt.Sprintf("Provider '%s' test failed: %v", providerName, err),
				"Check provider configuration", false)
		}
	} else {
		d.addResult("Provider Connection", "ok",
			fmt.Sprintf("Provider '%s' connected successfully", providerName), "", false)
	}
}

// checkMemory 检查记忆系统
func (d *Doctor) checkMemory() {
	if d.config == nil {
		return
	}

	memoryDir := expandPath(d.getMemoryPath())

	// 检查记忆目录
	if _, err := os.Stat(memoryDir); os.IsNotExist(err) {
		if d.fixMode {
			if err := os.MkdirAll(memoryDir, 0755); err == nil {
				d.addResult("Memory Directory", "ok",
					fmt.Sprintf("Created memory directory: %s", memoryDir), "", true)
				d.results[len(d.results)-1].Fixed = true
				return
			}
		}
		d.addResult("Memory Directory", "error",
			fmt.Sprintf("Memory directory not found: %s", memoryDir),
			"Create the directory or run 'lingguard doctor --fix'", true)
		return
	}

	d.addResult("Memory Directory", "ok",
		fmt.Sprintf("Memory directory exists: %s", memoryDir), "", false)

	// 检查 MEMORY.md
	memoryFile := filepath.Join(memoryDir, "MEMORY.md")
	if _, err := os.Stat(memoryFile); os.IsNotExist(err) {
		if d.fixMode {
			// 创建默认 MEMORY.md
			defaultContent := `# Memory

This file stores long-term memories and important facts.

## User Preferences
<!-- 用户偏好设置 -->

## Project Context
<!-- 项目上下文信息 -->

## Important Facts
<!-- 重要事实记录 -->
`
			if err := os.WriteFile(memoryFile, []byte(defaultContent), 0644); err == nil {
				d.addResult("Memory File", "ok",
					"Created MEMORY.md", "", true)
				d.results[len(d.results)-1].Fixed = true
				return
			}
		}
		d.addResult("Memory File", "warn",
			"MEMORY.md not found",
			"Run 'lingguard doctor --fix' to create", true)
	} else {
		d.addResult("Memory File", "ok", "MEMORY.md exists", "", false)
	}

	// 检查向量存储
	if d.config.Agents.MemoryConfig != nil && d.config.Agents.MemoryConfig.Vector != nil {
		if d.config.Agents.MemoryConfig.Vector.Enabled {
			d.checkVectorStore()
		} else {
			d.addResult("Vector Store", "skip", "Vector search is disabled", "", false)
		}
	}
}

// checkVectorStore 检查向量存储
func (d *Doctor) checkVectorStore() {
	memoryDir := expandPath(d.getMemoryPath())
	vectorsDB := filepath.Join(memoryDir, "vectors.db")

	if _, err := os.Stat(vectorsDB); os.IsNotExist(err) {
		d.addResult("Vector Store", "warn",
			"Vector database not initialized (will be created on first use)",
			"Send a message to initialize", false)
		return
	}

	// 检查 Embedding 配置
	vecConfig := d.config.Agents.MemoryConfig.Vector
	if vecConfig.Embedding.Provider == "" {
		d.addResult("Embedding Config", "error",
			"Embedding provider not configured",
			"Set 'memory.vector.embedding.provider' in config", false)
		return
	}

	d.addResult("Vector Store", "ok",
		fmt.Sprintf("Vector database exists (%s)", vectorsDB), "", false)

	// 测试 Embedding 服务
	d.testEmbeddingService()
}

// getMemoryPath 获取记忆存储路径
func (d *Doctor) getMemoryPath() string {
	if d.config.Agents.MemoryConfig.Path != "" {
		return d.config.Agents.MemoryConfig.Path
	}
	// 默认路径
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".lingguard", "memory")
}

// testEmbeddingService 测试 Embedding 服务
func (d *Doctor) testEmbeddingService() {
	embeddingCfg := d.config.Agents.MemoryConfig.Vector.Embedding

	// 获取 API Key（优先使用 embedding 配置中的，否则从 provider 继承）
	apiKey := embeddingCfg.APIKey
	apiBase := embeddingCfg.APIBase
	if apiKey == "" {
		if providerConfig, ok := d.config.Providers[embeddingCfg.Provider]; ok {
			apiKey = providerConfig.APIKey
			if apiBase == "" {
				apiBase = providerConfig.APIBase
			}
		}
	}

	if apiKey == "" {
		d.addResult("Embedding Service", "error",
			fmt.Sprintf("API key not found for embedding provider '%s'", embeddingCfg.Provider),
			fmt.Sprintf("Set 'providers.%s.apiKey' or 'memory.vector.embedding.apiKey'", embeddingCfg.Provider), false)
		return
	}

	// 创建 Embedding 客户端
	embConfig := &embedding.Config{
		Provider:  embeddingCfg.Provider,
		Model:     embeddingCfg.Model,
		Dimension: embeddingCfg.Dimension,
		APIKey:    apiKey,
		APIBase:   apiBase,
	}

	var client embedding.Model
	switch embeddingCfg.Provider {
	case "qwen":
		client = embedding.NewQwenEmbedding(embConfig)
	default:
		d.addResult("Embedding Service", "warn",
			fmt.Sprintf("Unsupported embedding provider: %s", embeddingCfg.Provider),
			"Use 'qwen' for embedding", false)
		return
	}

	// 测试 Embedding
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	_, err := client.Embed(ctx, "test")
	if err != nil {
		d.addResult("Embedding Service", "error",
			fmt.Sprintf("Embedding test failed: %v", err),
			"Check embedding provider configuration", false)
		return
	}

	d.addResult("Embedding Service", "ok",
		fmt.Sprintf("Embedding service working (provider: %s, model: %s)",
			embeddingCfg.Provider, embeddingCfg.Model), "", false)
}

// checkWorkspace 检查工作目录
func (d *Doctor) checkWorkspace() {
	if d.config == nil {
		return
	}

	workspace := expandPath(d.config.Agents.Workspace)

	if _, err := os.Stat(workspace); os.IsNotExist(err) {
		if d.fixMode {
			if err := os.MkdirAll(workspace, 0755); err == nil {
				d.addResult("Workspace", "ok",
					fmt.Sprintf("Created workspace: %s", workspace), "", true)
				d.results[len(d.results)-1].Fixed = true
				return
			}
		}
		d.addResult("Workspace", "error",
			fmt.Sprintf("Workspace not found: %s", workspace),
			"Create the directory or run 'lingguard doctor --fix'", true)
		return
	}

	d.addResult("Workspace", "ok",
		fmt.Sprintf("Workspace exists: %s", workspace), "", false)

	// 检查/创建技能目录 (ClawHub 安装的技能)
	skillsDir := filepath.Join(workspace, "skills")
	if _, err := os.Stat(skillsDir); os.IsNotExist(err) {
		if d.fixMode {
			if err := os.MkdirAll(skillsDir, 0755); err == nil {
				d.addResult("Workspace Skills", "ok",
					fmt.Sprintf("Created skills directory: %s", skillsDir), "", true)
				d.results[len(d.results)-1].Fixed = true
			}
		} else {
			d.addResult("Workspace Skills", "info",
				fmt.Sprintf("Skills directory not found: %s", skillsDir),
				"Run 'lingguard doctor --fix' to create", false)
		}
	} else {
		d.addResult("Workspace Skills", "ok",
			fmt.Sprintf("Skills directory exists: %s", skillsDir), "", false)
	}

	// 检查写入权限
	testFile := filepath.Join(workspace, ".write_test")
	if err := os.WriteFile(testFile, []byte("test"), 0644); err != nil {
		d.addResult("Workspace Writable", "error",
			"Workspace is not writable",
			"Check directory permissions", false)
	} else {
		os.Remove(testFile)
		d.addResult("Workspace Writable", "ok", "Workspace is writable", "", false)
	}

	// 检查磁盘空间
	d.checkDiskSpace(workspace)
}

// checkDiskSpace 检查磁盘空间
func (d *Doctor) checkDiskSpace(path string) {
	var stat syscall.Statfs_t
	if err := syscall.Statfs(path, &stat); err != nil {
		d.addResult("Disk Space", "skip",
			fmt.Sprintf("Could not check disk space: %v", err), "", false)
		return
	}

	// 计算可用空间（GB）
	availGB := float64(stat.Bavail*uint64(stat.Bsize)) / 1024 / 1024 / 1024

	if availGB < 1 {
		d.addResult("Disk Space", "error",
			fmt.Sprintf("Very low disk space: %.2f GB available", availGB),
			"Free up disk space", false)
	} else if availGB < 5 {
		d.addResult("Disk Space", "warn",
			fmt.Sprintf("Low disk space: %.2f GB available", availGB),
			"Consider freeing up disk space", false)
	} else {
		d.addResult("Disk Space", "ok",
			fmt.Sprintf("%.2f GB available", availGB), "", false)
	}
}

// checkChannels 检查渠道配置
func (d *Doctor) checkChannels() {
	if d.config == nil {
		return
	}

	// 检查飞书
	if d.config.Channels.Feishu != nil && d.config.Channels.Feishu.Enabled {
		feishu := d.config.Channels.Feishu
		issues := []string{}

		if feishu.AppID == "" {
			issues = append(issues, "App ID not configured")
		}
		if feishu.AppSecret == "" {
			issues = append(issues, "App Secret not configured")
		}

		if len(issues) > 0 {
			d.addResult("Feishu Channel", "error",
				strings.Join(issues, ", "),
				"Set 'channels.feishu.appId' and 'channels.feishu.appSecret'", false)
		} else {
			d.addResult("Feishu Channel", "ok", "Feishu channel configured", "", false)
		}
	} else {
		d.addResult("Feishu Channel", "skip", "Feishu channel not enabled", "", false)
	}
}

// checkSystem 检查系统环境
func (d *Doctor) checkSystem() {
	// Go 版本
	d.addResult("Go Version", "ok", runtime.Version(), "", false)

	// 操作系统
	d.addResult("OS", "ok", fmt.Sprintf("%s/%s", runtime.GOOS, runtime.GOARCH), "", false)

	// 检查 git
	if _, err := exec.LookPath("git"); err != nil {
		d.addResult("Git", "warn", "Git not found in PATH", "Install git for version control features", false)
	} else {
		d.addResult("Git", "ok", "Git available", "", false)
	}

	// 检查日志文件
	if d.config != nil && d.config.Logging.Output != "" {
		logPath := expandPath(d.config.Logging.Output)
		if info, err := os.Stat(logPath); err == nil {
			sizeMB := float64(info.Size()) / 1024 / 1024
			if sizeMB > 100 {
				d.addResult("Log File", "warn",
					fmt.Sprintf("Log file is large: %.2f MB", sizeMB),
					"Consider rotating or clearing logs", false)
			} else {
				d.addResult("Log File", "ok",
					fmt.Sprintf("Log file: %s (%.2f MB)", logPath, sizeMB), "", false)
			}
		}
	}
}

// checkMCP 检查 MCP 服务
func (d *Doctor) checkMCP() {
	if d.config == nil || d.config.Tools.MCPServers == nil || len(d.config.Tools.MCPServers) == 0 {
		d.addResult("MCP Servers", "skip", "No MCP servers configured", "", false)
		return
	}

	for serverName, server := range d.config.Tools.MCPServers {
		if server.Command != "" {
			// Stdio MCP
			if _, err := exec.LookPath(server.Command); err != nil {
				d.addResult(fmt.Sprintf("MCP '%s'", serverName), "warn",
					fmt.Sprintf("Command not found: %s", server.Command),
					"Install the required command", false)
			} else {
				d.addResult(fmt.Sprintf("MCP '%s'", serverName), "ok",
					fmt.Sprintf("Command available: %s", server.Command), "", false)
			}
		} else if server.URL != "" {
			// HTTP MCP
			d.addResult(fmt.Sprintf("MCP '%s'", serverName), "ok",
				fmt.Sprintf("HTTP endpoint: %s", server.URL), "", false)
		}
	}
}

// printResults 打印诊断结果
func (d *Doctor) printResults() {
	fmt.Println()

	okColor := color.New(color.FgGreen)
	warnColor := color.New(color.FgYellow)
	errorColor := color.New(color.FgRed)
	skipColor := color.New(color.FgHiBlack)
	bold := color.New(color.Bold)

	// 按类别分组打印
	categories := map[string][]DiagnosticResult{
		"Configuration": {},
		"Providers":     {},
		"Memory":        {},
		"System":        {},
	}

	for _, r := range d.results {
		// 简单分类
		switch {
		case strings.Contains(r.Name, "Config") || (strings.Contains(r.Name, "Provider") && !strings.Contains(r.Name, "Connection") && !strings.Contains(r.Name, "Embedding")):
			categories["Configuration"] = append(categories["Configuration"], r)
		case strings.Contains(r.Name, "Provider") || strings.Contains(r.Name, "Embedding"):
			categories["Providers"] = append(categories["Providers"], r)
		case strings.Contains(r.Name, "Memory") || strings.Contains(r.Name, "Vector") || strings.Contains(r.Name, "Workspace"):
			categories["Memory"] = append(categories["Memory"], r)
		default:
			categories["System"] = append(categories["System"], r)
		}
	}

	statusIcon := func(status string) string {
		switch status {
		case "ok":
			return "✓"
		case "warn":
			return "⚠"
		case "error":
			return "✗"
		default:
			return "○"
		}
	}

	statusColor := func(status string) *color.Color {
		switch status {
		case "ok":
			return okColor
		case "warn":
			return warnColor
		case "error":
			return errorColor
		default:
			return skipColor
		}
	}

	for _, cat := range []string{"Configuration", "Providers", "Memory", "System"} {
		results := categories[cat]
		if len(results) == 0 {
			continue
		}

		bold.Printf("\n%s\n", cat)
		fmt.Println(strings.Repeat("─", 40))

		for _, r := range results {
			icon := statusIcon(r.Status)
			c := statusColor(r.Status)

			if r.Fixed {
				c.Printf("  %s [FIXED] %s\n", icon, r.Name)
			} else {
				c.Printf("  %s %s\n", icon, r.Name)
			}

			if r.Message != "" {
				fmt.Printf("    %s\n", r.Message)
			}
			if r.Fix != "" && r.Status != "ok" {
				skipColor.Printf("    → %s\n", r.Fix)
			}
		}
	}

	// 统计
	fmt.Println()
	okCount := 0
	warnCount := 0
	errorCount := 0
	for _, r := range d.results {
		switch r.Status {
		case "ok":
			okCount++
		case "warn":
			warnCount++
		case "error":
			errorCount++
		}
	}

	bold.Print("Summary: ")
	okColor.Printf("%d ok  ", okCount)
	if warnCount > 0 {
		warnColor.Printf("%d warnings  ", warnCount)
	}
	if errorCount > 0 {
		errorColor.Printf("%d errors", errorCount)
	}
	fmt.Println()

	if errorCount > 0 {
		fmt.Println("\nRun 'lingguard doctor --fix' to attempt automatic fixes.")
	} else if warnCount > 0 {
		fmt.Println("\nSystem is operational but has some warnings.")
	} else {
		fmt.Println("\nAll systems operational! ✓")
	}
}
