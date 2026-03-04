package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/lingguard/internal/config"
	"github.com/lingguard/internal/cron"
	"github.com/lingguard/pkg/utils"
	"github.com/spf13/cobra"
)

var cronCmd = &cobra.Command{
	Use:   "cron",
	Short: "Manage scheduled tasks",
	Long:  `Manage scheduled tasks (cron jobs) for automated message delivery.`,
}

var cronListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all scheduled tasks",
	Run: func(cmd *cobra.Command, args []string) {
		_, service, err := initCronService()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		defer service.Stop()

		includeDisabled, _ := cmd.Flags().GetBool("all")
		jobs := service.ListJobs(includeDisabled)

		if len(jobs) == 0 {
			fmt.Println("No scheduled tasks found.")
			return
		}

		fmt.Println("Scheduled Tasks:")
		fmt.Println("─────────────────────────────────────────────────────────────────────────")
		for _, job := range jobs {
			status := "✓"
			if !job.Enabled {
				status = "✗"
			}
			nextRun := formatNextRun(job.State.NextRunAtMs)
			lastRun := formatLastRun(job.State.LastRunAtMs, job.State.LastStatus)

			// 显示执行模式
			mode := "📢 Notify"
			if job.Payload.Execute {
				mode = "🤖 Execute"
			}

			fmt.Printf("ID: %s | Status: %s | Name: %s\n", job.ID, status, job.Name)
			fmt.Printf("    Schedule: %s | Next: %s | Mode: %s\n", formatSchedule(job.Schedule), nextRun, mode)
			fmt.Printf("    Message: %s\n", utils.TruncateString(job.Payload.Message, 50))
			if lastRun != "" {
				fmt.Printf("    Last Run: %s\n", lastRun)
			}
			fmt.Println("─────────────────────────────────────────────────────────────────────────")
		}
	},
}

var cronAddCmd = &cobra.Command{
	Use:   "add <name> <schedule> <message>",
	Short: "Add a new scheduled task",
	Long: `Add a new scheduled task.

Schedule formats:
  - every:<duration>  - Repeat every duration (e.g., "every:1h", "every:30m")
  - at:<datetime>     - Run once at specific time (e.g., "at:2024-12-25 09:00")
  - cron:<expr>       - Cron expression (e.g., "cron:0 9 * * *")

Timezone:
  Use --tz flag to specify timezone for cron expressions (e.g., "America/New_York", "Asia/Shanghai")
  Default is local timezone.`,
	Args: cobra.ExactArgs(3),
	Run: func(cmd *cobra.Command, args []string) {
		_, service, err := initCronService()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		defer service.Stop()

		name := args[0]
		scheduleStr := args[1]
		message := args[2]

		tz, _ := cmd.Flags().GetString("tz")
		schedule, err := parseScheduleWithTZ(scheduleStr, tz)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error parsing schedule: %v\n", err)
			os.Exit(1)
		}

		var opts []cron.JobOption

		if deliver, _ := cmd.Flags().GetBool("deliver"); deliver {
			channel, _ := cmd.Flags().GetString("channel")
			to, _ := cmd.Flags().GetString("to")
			opts = append(opts, cron.WithDeliver(channel, to))
		}

		if deleteAfter, _ := cmd.Flags().GetBool("delete-after"); deleteAfter {
			opts = append(opts, cron.WithDeleteAfterRun())
		}

		if execute, _ := cmd.Flags().GetBool("execute"); execute {
			opts = append(opts, cron.WithExecute(true))
		}

		job, err := service.AddJob(name, *schedule, message, opts...)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error adding job: %v\n", err)
			os.Exit(1)
		}

		fmt.Printf("Task added successfully!\n")
		fmt.Printf("  ID: %s\n", job.ID)
		fmt.Printf("  Name: %s\n", job.Name)
		fmt.Printf("  Schedule: %s\n", formatSchedule(job.Schedule))
		if job.Schedule.TZ != "" {
			fmt.Printf("  Timezone: %s\n", job.Schedule.TZ)
		}
		fmt.Printf("  Next Run: %s\n", formatNextRun(job.State.NextRunAtMs))
		if job.Payload.Execute {
			fmt.Printf("  Mode: 🤖 Execute + Notify\n")
		} else {
			fmt.Printf("  Mode: 📢 Notify only\n")
		}
	},
}

var cronRemoveCmd = &cobra.Command{
	Use:   "remove <job-id>",
	Short: "Remove a scheduled task",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		_, service, err := initCronService()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		defer service.Stop()

		jobID := args[0]
		if service.RemoveJob(jobID) {
			fmt.Printf("Task %s removed successfully.\n", jobID)
		} else {
			fmt.Fprintf(os.Stderr, "Task %s not found.\n", jobID)
			os.Exit(1)
		}
	},
}

var cronEnableCmd = &cobra.Command{
	Use:   "enable <job-id>",
	Short: "Enable a scheduled task",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		_, service, err := initCronService()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		defer service.Stop()

		jobID := args[0]
		job := service.EnableJob(jobID, true)
		if job != nil {
			fmt.Printf("Task %s enabled.\n", jobID)
			fmt.Printf("  Next Run: %s\n", formatNextRun(job.State.NextRunAtMs))
		} else {
			fmt.Fprintf(os.Stderr, "Task %s not found.\n", jobID)
			os.Exit(1)
		}
	},
}

var cronDisableCmd = &cobra.Command{
	Use:   "disable <job-id>",
	Short: "Disable a scheduled task",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		_, service, err := initCronService()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		defer service.Stop()

		jobID := args[0]
		job := service.EnableJob(jobID, false)
		if job != nil {
			fmt.Printf("Task %s disabled.\n", jobID)
		} else {
			fmt.Fprintf(os.Stderr, "Task %s not found.\n", jobID)
			os.Exit(1)
		}
	},
}

var cronRunCmd = &cobra.Command{
	Use:   "run <job-id>",
	Short: "Manually run a scheduled task",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		_, service, err := initCronService()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		defer service.Stop()

		jobID := args[0]
		force, _ := cmd.Flags().GetBool("force")

		job, err := service.RunJob(jobID, force)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error running task: %v\n", err)
			os.Exit(1)
		}

		fmt.Printf("Task %s executed.\n", jobID)
		fmt.Printf("  Status: %s\n", job.State.LastStatus)
		if job.State.LastError != "" {
			fmt.Printf("  Error: %s\n", job.State.LastError)
		}
		if job.State.LastResponse != "" {
			fmt.Printf("  Response: %s\n", utils.TruncateString(job.State.LastResponse, 200))
		}
	},
}

var cronUpdateCmd = &cobra.Command{
	Use:   "update <job-id>",
	Short: "Update a scheduled task",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		_, service, err := initCronService()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		defer service.Stop()

		jobID := args[0]
		opts := cron.UpdateJobOptions{}

		if name, _ := cmd.Flags().GetString("name"); name != "" {
			opts.Name = &name
		}
		if scheduleStr, _ := cmd.Flags().GetString("schedule"); scheduleStr != "" {
			tz, _ := cmd.Flags().GetString("tz")
			schedule, err := parseScheduleWithTZ(scheduleStr, tz)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error parsing schedule: %v\n", err)
				os.Exit(1)
			}
			opts.Schedule = schedule
		}
		if message, _ := cmd.Flags().GetString("message"); message != "" {
			opts.Message = &message
		}
		if cmd.Flags().Changed("execute") {
			execute, _ := cmd.Flags().GetBool("execute")
			opts.Execute = &execute
		}
		if cmd.Flags().Changed("enabled") {
			enabled, _ := cmd.Flags().GetBool("enabled")
			opts.Enabled = &enabled
		}

		job, err := service.UpdateJob(jobID, opts)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error updating task: %v\n", err)
			os.Exit(1)
		}

		fmt.Printf("Task updated successfully!\n")
		fmt.Printf("  ID: %s\n", job.ID)
		fmt.Printf("  Name: %s\n", job.Name)
		fmt.Printf("  Schedule: %s\n", formatSchedule(job.Schedule))
		fmt.Printf("  Next Run: %s\n", formatNextRun(job.State.NextRunAtMs))
		if job.Payload.Execute {
			fmt.Printf("  Mode: 🤖 Execute + Notify\n")
		} else {
			fmt.Printf("  Mode: 📢 Notify only\n")
		}
	},
}

var cronStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show cron service status",
	Run: func(cmd *cobra.Command, args []string) {
		_, service, err := initCronService()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		defer service.Stop()

		status := service.Status()
		data, _ := json.MarshalIndent(status, "", "  ")
		fmt.Println(string(data))
	},
}

func init() {
	rootCmd.AddCommand(cronCmd)
	cronCmd.AddCommand(cronListCmd)
	cronCmd.AddCommand(cronAddCmd)
	cronCmd.AddCommand(cronRemoveCmd)
	cronCmd.AddCommand(cronEnableCmd)
	cronCmd.AddCommand(cronDisableCmd)
	cronCmd.AddCommand(cronRunCmd)
	cronCmd.AddCommand(cronUpdateCmd)
	cronCmd.AddCommand(cronStatusCmd)

	cronListCmd.Flags().BoolP("all", "a", false, "Include disabled tasks")
	cronAddCmd.Flags().BoolP("deliver", "d", false, "Deliver response to channel")
	cronAddCmd.Flags().StringP("channel", "c", "", "Target channel (e.g., feishu)")
	cronAddCmd.Flags().StringP("to", "t", "", "Target user/group ID")
	cronAddCmd.Flags().BoolP("delete-after", "", false, "Delete after execution")
	cronAddCmd.Flags().BoolP("execute", "e", false, "Execute mode: run Agent to process task before notifying")
	cronAddCmd.Flags().StringP("tz", "z", "", "Timezone for cron expression")
	cronRunCmd.Flags().BoolP("force", "f", false, "Force run even if disabled")

	// update 命令的 flags
	cronUpdateCmd.Flags().StringP("name", "n", "", "New task name")
	cronUpdateCmd.Flags().StringP("schedule", "s", "", "New schedule (e.g., cron:0 9 * * *)")
	cronUpdateCmd.Flags().StringP("message", "m", "", "New task message")
	cronUpdateCmd.Flags().BoolP("execute", "e", false, "Enable execute mode (run Agent before notifying)")
	cronUpdateCmd.Flags().Bool("no-execute", false, "Disable execute mode (notify only)")
	cronUpdateCmd.Flags().Bool("enabled", true, "Enable or disable the task")
	cronUpdateCmd.Flags().StringP("tz", "z", "", "Timezone for cron expression")
}

func initCronService() (*config.Config, *cron.Service, error) {
	cfg, err := config.Load(cfgPath)
	if err != nil {
		return nil, nil, fmt.Errorf("load config: %w", err)
	}

	if err := cfg.Validate(); err != nil {
		return nil, nil, fmt.Errorf("config validation: %w", err)
	}

	storePath := utils.ExpandHome("~/.lingguard/cron/jobs.json")
	if cfg.Tools.Cron != nil && cfg.Tools.Cron.StorePath != "" {
		storePath = utils.ExpandHome(cfg.Tools.Cron.StorePath)
	}

	service := cron.NewService(storePath, nil)
	if err := service.Start(); err != nil {
		return nil, nil, fmt.Errorf("start cron service: %w", err)
	}

	return cfg, service, nil
}

func parseScheduleWithTZ(s string, tz string) (*cron.CronSchedule, error) {
	parts := strings.SplitN(s, ":", 2)
	if len(parts) != 2 {
		return nil, fmt.Errorf("invalid schedule format, use: every:<duration>, at:<datetime>, or cron:<expr>")
	}

	kind := strings.ToLower(parts[0])
	value := parts[1]

	switch kind {
	case "every":
		duration, err := time.ParseDuration(value)
		if err != nil {
			return nil, fmt.Errorf("invalid duration: %w", err)
		}
		return &cron.CronSchedule{
			Kind:    cron.ScheduleKindEvery,
			EveryMs: duration.Milliseconds(),
		}, nil

	case "at":
		t, err := utils.ParseTime(value)
		if err != nil {
			return nil, fmt.Errorf("invalid datetime: %w", err)
		}
		return &cron.CronSchedule{
			Kind: cron.ScheduleKindAt,
			AtMs: t.UnixMilli(),
		}, nil

	case "cron":
		return &cron.CronSchedule{
			Kind: cron.ScheduleKindCron,
			Expr: value,
			TZ:   tz,
		}, nil

	default:
		return nil, fmt.Errorf("unknown schedule kind: %s", kind)
	}
}

func formatSchedule(s cron.CronSchedule) string {
	switch s.Kind {
	case cron.ScheduleKindEvery:
		return fmt.Sprintf("every %s", time.Duration(s.EveryMs)*time.Millisecond)
	case cron.ScheduleKindAt:
		return fmt.Sprintf("at %s", time.UnixMilli(s.AtMs).Format("2006-01-02 15:04:05"))
	case cron.ScheduleKindCron:
		if s.TZ != "" {
			return fmt.Sprintf("cron: %s (TZ: %s)", s.Expr, s.TZ)
		}
		return fmt.Sprintf("cron: %s", s.Expr)
	default:
		return string(s.Kind)
	}
}

func formatNextRun(ms int64) string {
	if ms == 0 {
		return "not scheduled"
	}
	return time.UnixMilli(ms).Format("2006-01-02 15:04:05")
}

func formatLastRun(ms int64, status cron.JobStatus) string {
	if ms == 0 {
		return ""
	}
	return fmt.Sprintf("%s (%s)", time.UnixMilli(ms).Format("2006-01-02 15:04:05"), status)
}
