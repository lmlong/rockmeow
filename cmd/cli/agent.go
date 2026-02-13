package cli

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/lingguard/internal/agent"
	"github.com/lingguard/internal/config"
	"github.com/lingguard/internal/providers"
	"github.com/lingguard/internal/tools"
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
			// 单次消息模式
			response, err := ag.ProcessMessage(ctx, "cli-session", message)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				os.Exit(1)
			}
			fmt.Println(response)
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

	// 4. 创建 Agent
	ag := agent.NewAgent(&cfg.Agents, provider)

	// 5. 注册工具
	workspace := cfg.Agents.Workspace
	if workspace == "" {
		workspace = cfg.Tools.Workspace
	}
	ag.RegisterTool(tools.NewShellTool(workspace, cfg.Tools.RestrictToWorkspace))
	ag.RegisterTool(tools.NewFileTool(workspace, cfg.Tools.RestrictToWorkspace))

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

		response, err := ag.ProcessMessage(ctx, sessionID, input)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			continue
		}

		fmt.Println()
		fmt.Println(response)
		fmt.Println()
	}
}
