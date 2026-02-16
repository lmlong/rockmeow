package cli

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/lingguard/internal/agent"
	"github.com/lingguard/internal/config"
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
			// 单次消息模式
			err := ag.ProcessMessageStream(ctx, "cli-session", message, printStreamEvent)
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
	builder := NewAgentBuilder(cfg)
	builder.InitSkills(true)
	return builder.Build()
}

// printStreamEvent 打印流式事件
func printStreamEvent(event stream.StreamEvent) {
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

		fmt.Println()
		err := ag.ProcessMessageStream(ctx, sessionID, input, printStreamEvent)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		}
		fmt.Println()
	}
}
