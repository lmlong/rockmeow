package cli

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/lingguard/internal/agent"
	"github.com/lingguard/internal/config"
	"github.com/lingguard/internal/providers"
	"github.com/lingguard/internal/skills"
	"github.com/lingguard/internal/tools"
	"github.com/lingguard/pkg/stream"
	"github.com/spf13/cobra"
)

var message string

var agentCmd = &cobra.Command{
	Use:   "agent",
	Short: "Chat with the agent",
	Run: func(cmd *cobra.Command, args []string) {
		cfg, err := config.Load(cfgPath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error loading config: %v\n", err)
			os.Exit(1)
		}

		ag, err := createAgent(cfg)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error creating agent: %v\n", err)
			os.Exit(1)
		}

		ctx := context.Background()

		if message != "" {
			// 单次消息模式 (使用流式输出)
			err := ag.ProcessMessageStream(ctx, "cli-session", message, func(event stream.StreamEvent) {
				switch event.Type {
				case stream.EventText:
					fmt.Print(event.Content)
				case stream.EventToolStart:
					fmt.Printf("\n⚙️ 执行工具: %s...\n", event.ToolName)
				case stream.EventToolEnd:
					if event.ToolError != "" {
						fmt.Printf("❌ 工具执行失败: %s\n", event.ToolError)
					}
				case stream.EventDone:
					fmt.Println()
				case stream.EventError:
					fmt.Fprintf(os.Stderr, "\n❌ 错误: %v\n", event.Error)
				}
			})
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				os.Exit(1)
			}
		} else {
			// 交互模式
			runInteractiveMode(ctx, ag)
		}
	},
}

func init() {
	agentCmd.Flags().StringVarP(&message, "message", "m", "", "Message to send")
}

func createAgent(cfg *config.Config) (*agent.Agent, error) {
	// 1. 创建 Provider 注册表
	registry := providers.NewRegistry()
	if err := registry.InitFromConfig(cfg); err != nil {
		return nil, err
	}

	// 2. 通过 provider 配置获取 Provider
	providerName := cfg.Agents.Provider
	provider, ok := registry.MatchProvider(providerName)
	if !ok {
		return nil, fmt.Errorf("provider not found: %s", providerName)
	}

	// 3. 设置默认 Provider
	registry.SetDefault(providerName)

	// 4. 创建 Skills Loader
	var skillsLoader *skills.Loader
	var builtinDirs []string

	// 如果配置了内置技能目录，使用配置的
	if cfg.Agents.SkillsBuiltinDir != "" {
		builtinDirs = append(builtinDirs, cfg.Agents.SkillsBuiltinDir)
	} else {
		// 自动发现内置技能目录
		execPath, _ := os.Executable()
		home, _ := os.UserHomeDir()
		cwd, _ := os.Getwd()

		// 候选路径（按优先级排序）
		candidatePaths := []string{
			// 1. 相对于可执行文件的 skills 目录
			filepath.Join(filepath.Dir(execPath), "skills"),
			// 2. 相对于可执行文件的上级目录
			filepath.Join(filepath.Dir(execPath), "..", "skills"),
			// 3. 用户主目录下的 .lingguard/skills
			filepath.Join(home, ".lingguard", "skills"),
			// 4. 当前工作目录下的 skills
			filepath.Join(cwd, "skills"),
		}

		// 去重：使用 map 记录已添加的绝对路径
		seen := make(map[string]bool)
		for _, p := range candidatePaths {
			absPath, err := filepath.Abs(p)
			if err != nil {
				absPath = p
			}
			if _, err := os.Stat(p); err == nil && !seen[absPath] {
				seen[absPath] = true
				builtinDirs = append(builtinDirs, p)
			}
		}
	}

	workspaceSkills := cfg.Agents.SkillsWorkspace

	if len(builtinDirs) > 0 || workspaceSkills != "" {
		skillsLoader = skills.NewLoader(builtinDirs, workspaceSkills)
		if len(builtinDirs) > 0 {
			fmt.Printf("Skills loaded from: %v\n", builtinDirs)
		}
	}

	// 5. 创建 Agent
	ag := agent.NewAgent(&cfg.Agents, provider, skillsLoader)

	// 6. 注册工具
	workspace := cfg.Agents.Workspace
	if workspace == "" {
		workspace = cfg.Tools.Workspace
	}
	ag.RegisterTool(tools.NewShellTool(workspace, cfg.Tools.RestrictToWorkspace))
	ag.RegisterTool(tools.NewFileTool(workspace, cfg.Tools.RestrictToWorkspace))

	// 7. 注册技能工具（支持按需加载技能）
	ag.RegisterSkillTool()

	// 8. 注册子代理工具（支持后台任务）
	ag.RegisterSubagentTools()

	// 9. 注册记忆工具（参考 nanobot）
	ag.RegisterMemoryTool()

	return ag, nil
}

func runInteractiveMode(ctx context.Context, ag *agent.Agent) {
	reader := bufio.NewReader(os.Stdin)
	sessionID := "cli-interactive"

	fmt.Println("LingGuard Interactive Mode")
	fmt.Println("Type 'exit' or 'quit' to exit.")
	fmt.Println()

	for {
		fmt.Print("> ")
		input, _ := reader.ReadString('\n')
		input = strings.TrimSpace(input)

		if input == "" {
			continue
		}

		if input == "exit" || input == "quit" {
			fmt.Println("Goodbye!")
			break
		}

		// 使用流式输出
		fmt.Println()
		err := ag.ProcessMessageStream(ctx, sessionID, input, func(event stream.StreamEvent) {
			switch event.Type {
			case stream.EventText:
				// 实时打印增量文本
				fmt.Print(event.Content)
			case stream.EventToolStart:
				// 显示工具执行状态
				fmt.Printf("\n⚙️ 执行工具: %s...\n", event.ToolName)
			case stream.EventToolEnd:
				// 工具执行完成
				if event.ToolError != "" {
					fmt.Printf("❌ 工具执行失败: %s\n", event.ToolError)
				}
			case stream.EventDone:
				// 完成后换行
				fmt.Println()
			case stream.EventError:
				fmt.Fprintf(os.Stderr, "\n❌ 错误: %v\n", event.Error)
			}
		})

		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		}
		fmt.Println()
	}
}
