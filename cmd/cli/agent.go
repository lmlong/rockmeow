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
	provider, spec := registry.MatchProvider(providerName)
	if provider == nil {
		return nil, fmt.Errorf("provider not found: %s", providerName)
	}
	_ = spec // spec 可用于获取 DisplayName 等信息

	// 3. 设置默认 Provider
	registry.SetDefault(providerName)

	// 4. 创建 Skills Loader
	// 支持从多个目录加载技能：
	// 1. 程序执行目录的 skills/builtin/（内置技能）
	// 2. ~/.lingguard/skills/（用户技能）
	var skillDirs []string
	home, _ := os.UserHomeDir()

	// 内置技能目录：程序执行目录的 skills/builtin/
	execPath, _ := os.Executable()
	execDir := filepath.Dir(execPath)
	builtinDir := filepath.Join(execDir, "skills", "builtin")
	if _, err := os.Stat(builtinDir); err == nil {
		skillDirs = append(skillDirs, builtinDir)
		fmt.Printf("Built-in skills: %s\n", builtinDir)
	}

	// 用户技能目录：~/.lingguard/skills/
	userSkillsDir := filepath.Join(home, ".lingguard", "skills")
	if _, err := os.Stat(userSkillsDir); err == nil {
		skillDirs = append(skillDirs, userSkillsDir)
		fmt.Printf("User skills: %s\n", userSkillsDir)
	}

	var skillsLoader *skills.Loader
	if len(skillDirs) > 0 {
		skillsLoader = skills.NewLoader(skillDirs, "")
	}

	// 5. 创建 Agent
	ag := agent.NewAgent(&cfg.Agents, provider, skillsLoader)

	// 6. 创建工作目录管理器
	workspace := cfg.Agents.Workspace
	if workspace == "" {
		workspace = cfg.Tools.Workspace
	}
	workspaceMgr := tools.NewWorkspaceManager(workspace, cfgPath)

	// 7. 注册工具
	ag.RegisterTool(tools.NewShellTool(workspaceMgr, cfg.Tools.RestrictToWorkspace))
	ag.RegisterTool(tools.NewFileTool(workspaceMgr, cfg.Tools.RestrictToWorkspace))
	ag.RegisterTool(tools.NewWorkspaceTool(workspaceMgr))

	// 注册 Web 工具
	braveAPIKey := cfg.Tools.BraveAPIKey
	if braveAPIKey == "" {
		braveAPIKey = os.Getenv("BRAVE_API_KEY")
	}
	ag.RegisterTool(tools.NewWebSearchTool(braveAPIKey, 5))
	ag.RegisterTool(tools.NewWebFetchTool(cfg.Tools.WebMaxChars))

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
