package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/spf13/cobra"

	"runloop/internal/config"
	"runloop/internal/daemon"
)

func NewRootCommand(name string) *cobra.Command {
	cmd := &cobra.Command{
		Use:   name,
		Short: "Runloop local workflow executor",
	}
	cmd.AddCommand(initCommand())
	cmd.AddCommand(healthCommand())
	cmd.AddCommand(daemonCommand())
	cmd.AddCommand(inboxCommand())
	cmd.AddCommand(workflowsCommand())
	cmd.AddCommand(runsCommand())
	cmd.AddCommand(sourcesCommand())
	return cmd
}

func NewDaemonRootCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "runloopd",
		Short: "Runloop background daemon",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
			defer stop()
			d, err := daemon.New(ctx, slog.Default())
			if err != nil {
				return err
			}
			return d.Run(ctx)
		},
	}
}

func initCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "init",
		Short: "Create local Runloop config and sample workflow",
		RunE: func(cmd *cobra.Command, args []string) error {
			paths, err := config.DefaultPaths()
			if err != nil {
				return err
			}
			if err := config.WriteInitial(paths); err != nil {
				return err
			}
			_, err = fmt.Fprintf(cmd.OutOrStdout(), "initialized Runloop config at %s\n", paths.ConfigDir)
			return err
		},
	}
}

func healthCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "health",
		Short: "Check daemon health",
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := NewClient()
			if err != nil {
				return err
			}
			var out map[string]any
			if err := client.Get("/api/health", &out); err != nil {
				return err
			}
			return printJSON(cmd, out)
		},
	}
}

func daemonCommand() *cobra.Command {
	cmd := &cobra.Command{Use: "daemon", Short: "Daemon commands"}
	cmd.AddCommand(&cobra.Command{
		Use:   "start",
		Short: "Start the daemon in the foreground",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
			defer stop()
			d, err := daemon.New(ctx, slog.Default())
			if err != nil {
				return err
			}
			return d.Run(ctx)
		},
	})
	return cmd
}

func inboxCommand() *cobra.Command {
	cmd := &cobra.Command{Use: "inbox", Short: "Inspect and add inbox items"}
	cmd.AddCommand(&cobra.Command{
		Use:   "list",
		Short: "List inbox items",
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := NewClient()
			if err != nil {
				return err
			}
			var out any
			if err := client.Get("/api/inbox", &out); err != nil {
				return err
			}
			return printJSON(cmd, out)
		},
	})
	cmd.AddCommand(&cobra.Command{
		Use:   "show <id>",
		Args:  cobra.ExactArgs(1),
		Short: "Show an inbox item",
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := NewClient()
			if err != nil {
				return err
			}
			var out any
			if err := client.Get("/api/inbox/"+args[0], &out); err != nil {
				return err
			}
			return printJSON(cmd, out)
		},
	})
	cmd.AddCommand(&cobra.Command{
		Use:   "archive <id>",
		Args:  cobra.ExactArgs(1),
		Short: "Archive an inbox item",
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := NewClient()
			if err != nil {
				return err
			}
			var out any
			if err := client.Post("/api/inbox/"+args[0]+"/archive", nil, &out); err != nil {
				return err
			}
			return printJSON(cmd, out)
		},
	})
	cmd.AddCommand(&cobra.Command{
		Use:   "ignore <id>",
		Args:  cobra.ExactArgs(1),
		Short: "Ignore an inbox item",
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := NewClient()
			if err != nil {
				return err
			}
			var out any
			if err := client.Post("/api/inbox/"+args[0]+"/ignore", nil, &out); err != nil {
				return err
			}
			return printJSON(cmd, out)
		},
	})
	add := &cobra.Command{
		Use:   "add",
		Short: "Add a manual inbox item",
		RunE: func(cmd *cobra.Command, args []string) error {
			source, _ := cmd.Flags().GetString("source")
			externalID, _ := cmd.Flags().GetString("external-id")
			title, _ := cmd.Flags().GetString("title")
			jsonValue, _ := cmd.Flags().GetString("json")
			var payload map[string]any
			if err := json.Unmarshal([]byte(jsonValue), &payload); err != nil {
				return err
			}
			client, err := NewClient()
			if err != nil {
				return err
			}
			var out any
			err = client.Post("/api/inbox", map[string]any{"source": source, "externalId": externalID, "title": title, "payload": payload}, &out)
			if err != nil {
				return err
			}
			return printJSON(cmd, out)
		},
	}
	add.Flags().String("source", "manual", "source id")
	add.Flags().String("external-id", "", "external id")
	add.Flags().String("title", "", "title")
	add.Flags().String("json", "{}", "JSON payload")
	_ = add.MarkFlagRequired("external-id")
	_ = add.MarkFlagRequired("title")
	cmd.AddCommand(add)
	return cmd
}

func workflowsCommand() *cobra.Command {
	cmd := &cobra.Command{Use: "workflows", Short: "Inspect workflows"}
	cmd.AddCommand(&cobra.Command{
		Use:   "list",
		Short: "List workflows",
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := NewClient()
			if err != nil {
				return err
			}
			var out any
			if err := client.Get("/api/workflows", &out); err != nil {
				return err
			}
			return printJSON(cmd, out)
		},
	})
	cmd.AddCommand(&cobra.Command{
		Use:   "show <id>",
		Args:  cobra.ExactArgs(1),
		Short: "Show a workflow",
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := NewClient()
			if err != nil {
				return err
			}
			var out any
			if err := client.Get("/api/workflows/"+args[0], &out); err != nil {
				return err
			}
			return printJSON(cmd, out)
		},
	})
	cmd.AddCommand(&cobra.Command{
		Use:   "enable <id>",
		Args:  cobra.ExactArgs(1),
		Short: "Enable a workflow",
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := NewClient()
			if err != nil {
				return err
			}
			var out any
			if err := client.Post("/api/workflows/"+args[0]+"/enable", nil, &out); err != nil {
				return err
			}
			return printJSON(cmd, out)
		},
	})
	cmd.AddCommand(&cobra.Command{
		Use:   "disable <id>",
		Args:  cobra.ExactArgs(1),
		Short: "Disable a workflow",
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := NewClient()
			if err != nil {
				return err
			}
			var out any
			if err := client.Post("/api/workflows/"+args[0]+"/disable", nil, &out); err != nil {
				return err
			}
			return printJSON(cmd, out)
		},
	})
	return cmd
}

func runsCommand() *cobra.Command {
	cmd := &cobra.Command{Use: "runs", Short: "Inspect workflow runs"}
	cmd.AddCommand(&cobra.Command{
		Use:   "list",
		Short: "List runs",
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := NewClient()
			if err != nil {
				return err
			}
			var out any
			if err := client.Get("/api/runs", &out); err != nil {
				return err
			}
			return printJSON(cmd, out)
		},
	})
	cmd.AddCommand(&cobra.Command{
		Use:   "show <id>",
		Args:  cobra.ExactArgs(1),
		Short: "Show a run",
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := NewClient()
			if err != nil {
				return err
			}
			var out any
			if err := client.Get("/api/runs/"+args[0], &out); err != nil {
				return err
			}
			return printJSON(cmd, out)
		},
	})
	return cmd
}

func sourcesCommand() *cobra.Command {
	cmd := &cobra.Command{Use: "sources", Short: "Inspect and test sources"}
	cmd.AddCommand(&cobra.Command{
		Use:   "list",
		Short: "List registered sources",
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := NewClient()
			if err != nil {
				return err
			}
			var out any
			if err := client.Get("/api/sources", &out); err != nil {
				return err
			}
			return printJSON(cmd, out)
		},
	})
	cmd.AddCommand(&cobra.Command{
		Use:   "test <id>",
		Args:  cobra.ExactArgs(1),
		Short: "Run the source's connectivity check",
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := NewClient()
			if err != nil {
				return err
			}
			var out any
			if err := client.Post("/api/sources/"+args[0]+"/test", nil, &out); err != nil {
				return err
			}
			return printJSON(cmd, out)
		},
	})
	return cmd
}

func printJSON(cmd *cobra.Command, value any) error {
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	_, err = fmt.Fprintln(cmd.OutOrStdout(), string(data))
	return err
}
