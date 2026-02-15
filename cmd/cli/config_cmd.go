package cli

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/lingguard/internal/config"
	"github.com/spf13/cobra"
)

var configCmd = &cobra.Command{
	Use:   "config",
	Short: "Manage configuration",
	Long:  `Manage LingGuard configuration settings.`,
}

var workspaceCmd = &cobra.Command{
	Use:   "workspace [path]",
	Short: "Get or set workspace directory",
	Long: `Get or set the workspace directory.

Without arguments, shows the current workspace.
With a path argument, sets the workspace to that path.

The path can use ~ for home directory expansion.
Example: lingguard config workspace ~/my-project`,
	Args: cobra.MaximumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		if len(args) == 0 {
			showWorkspace()
		} else {
			setWorkspace(args[0])
		}
	},
}

func init() {
	rootCmd.AddCommand(configCmd)
	configCmd.AddCommand(workspaceCmd)
}

func showWorkspace() {
	cfg, err := loadConfig()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	workspace := cfg.Agents.Workspace
	if workspace == "" {
		workspace = cfg.Tools.Workspace
	}
	if workspace == "" {
		workspace = "~/.lingguard/workspace"
	}

	// 展开路径用于显示
	expanded := expandPath(workspace)
	fmt.Printf("Workspace: %s\n", workspace)
	fmt.Printf("Expanded:  %s\n", expanded)

	// 检查目录是否存在
	if _, err := os.Stat(expanded); os.IsNotExist(err) {
		fmt.Println("Status:    Directory does not exist (will be created on first use)")
	} else {
		fmt.Println("Status:    Directory exists")
	}
}

func setWorkspace(path string) {
	cfg, err := loadConfig()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading config: %v\n", err)
		os.Exit(1)
	}

	// 规范化路径（保留 ~ 格式用于配置文件）
	normalized := path
	if len(path) >= 2 && path[:2] == "~/" {
		// 保持 ~ 格式
		normalized = path
	} else {
		// 转换为绝对路径
		abs, err := filepath.Abs(path)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error resolving path: %v\n", err)
			os.Exit(1)
		}
		normalized = abs
	}

	// 更新配置
	cfg.Agents.Workspace = normalized
	cfg.Tools.Workspace = normalized

	// 保存配置
	if err := cfg.Save(cfgPath); err != nil {
		fmt.Fprintf(os.Stderr, "Error saving config: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Workspace set to: %s\n", normalized)
	fmt.Printf("Expanded path:     %s\n", expandPath(normalized))
	fmt.Printf("Config saved to:   %s\n", cfgPath)
}

func loadConfig() (*config.Config, error) {
	// 检查配置文件是否存在
	if _, err := os.Stat(cfgPath); os.IsNotExist(err) {
		return nil, fmt.Errorf("config file not found: %s\nRun 'lingguard init' to create one", cfgPath)
	}

	return config.Load(cfgPath)
}

func expandPath(path string) string {
	if len(path) >= 2 && path[:2] == "~/" {
		home, err := os.UserHomeDir()
		if err != nil {
			return path
		}
		return filepath.Join(home, path[2:])
	}
	return path
}
