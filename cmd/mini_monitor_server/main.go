package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/spf13/cobra"

	"mini_monitor_server/internal/config"
	"mini_monitor_server/internal/daemon"
	"mini_monitor_server/internal/model"
	"mini_monitor_server/internal/report"
	"mini_monitor_server/internal/storage"
)

var (
	version    = "dev"
	configPath string
)

func main() {
	rootCmd := &cobra.Command{
		Use:   "mini_monitor_server",
		Short: "Lightweight monitoring daemon for small servers",
	}
	rootCmd.PersistentFlags().StringVarP(&configPath, "config", "c", "", "config file path")

	rootCmd.AddCommand(daemonCmd(), reportCmd(), checkCmd(), versionCmd())

	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func daemonCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "daemon",
		Short: "Start the monitoring daemon",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := loadConfig()
			if err != nil {
				return err
			}

			d, err := daemon.New(cfg)
			if err != nil {
				return fmt.Errorf("init daemon: %w", err)
			}

			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			sigCh := make(chan os.Signal, 1)
			signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
			go func() {
				sig := <-sigCh
				slog.Info("received signal", "signal", sig)
				cancel()
			}()

			return d.Run(ctx)
		},
	}
}

func reportCmd() *cobra.Command {
	var jsonFormat bool
	cmd := &cobra.Command{
		Use:   "report",
		Short: "Print current system report",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := loadConfig()
			if err != nil {
				return err
			}

			store, err := storage.New(cfg.Storage.Dir)
			if err != nil {
				return fmt.Errorf("init storage: %w", err)
			}

			var (
				snap        *model.Snapshot
				firingRules []string
			)

			state, err := store.LoadState()
			if err != nil {
				return fmt.Errorf("load state: %w", err)
			}
			if state != nil && state.LastSnapshot != nil {
				snap = state.LastSnapshot
				firingRules = firingRulesFromState(state.Rules)
			} else {
				snap, err = daemon.CollectSnapshot(context.Background(), cfg)
				if err != nil {
					return fmt.Errorf("collect: %w", err)
				}
			}

			if jsonFormat {
				data, err := report.JSONReport(snap, firingRules)
				if err != nil {
					return err
				}
				fmt.Println(string(data))
			} else {
				fmt.Print(report.TextReport(snap, firingRules, cfg.Report.IncludeHistoryDays, nil))
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&jsonFormat, "json", false, "output in JSON format")
	return cmd
}

func checkCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "check",
		Short: "Validate config file",
		RunE: func(cmd *cobra.Command, args []string) error {
			if _, err := loadConfig(); err != nil {
				return err
			}
			fmt.Println("config is valid")
			return nil
		},
	}
}

func versionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print version",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Printf("mini_monitor_server %s\n", version)
		},
	}
}

func loadConfig() (*config.Config, error) {
	if configPath == "" {
		return nil, fmt.Errorf("config path is required (-c flag)")
	}
	cfg, err := config.Load(configPath)
	if err != nil {
		return nil, err
	}
	if err := config.Validate(cfg); err != nil {
		return nil, fmt.Errorf("config validation failed: %w", err)
	}
	return cfg, nil
}

func firingRulesFromState(states map[string]model.RuleRuntimeState) []string {
	var rules []string
	for name, state := range states {
		if state.Status == "firing" {
			rules = append(rules, name)
		}
	}
	return rules
}
