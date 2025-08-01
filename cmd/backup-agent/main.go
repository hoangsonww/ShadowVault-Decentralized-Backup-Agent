package main

import (
	"context"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/hoangsonww/backupagent/config"
	"github.com/hoangsonww/backupagent/internal/agent"
)

var (
	cfgFile   string
	passphrase string
)

func main() {
	root := &cobra.Command{
		Use:   "backup-agent",
		Short: "Decentralized Encrypted Backup Agent",
	}

	root.PersistentFlags().StringVarP(&cfgFile, "config", "c", "config.yaml", "Path to config file")
	root.PersistentFlags().StringVarP(&passphrase, "pass", "p", "", "Passphrase for encryption (required)")

	initCmd := &cobra.Command{
		Use:   "daemon",
		Short: "Start the backup agent daemon",
		RunE: func(cmd *cobra.Command, args []string) error {
			if passphrase == "" {
				return fmt.Errorf("passphrase is required")
			}
			cfg, err := config.Load(cfgFile)
			if err != nil {
				return err
			}
			ag, err := agent.New(cfg, passphrase)
			if err != nil {
				return err
			}
			return ag.RunDaemon(context.Background())
		},
	}

	snapCmd := &cobra.Command{
		Use:   "snapshot [path]",
		Short: "Take snapshot of a directory",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if passphrase == "" {
				return fmt.Errorf("passphrase is required")
			}
			cfg, err := config.Load(cfgFile)
			if err != nil {
				return err
			}
			ag, err := agent.New(cfg, passphrase)
			if err != nil {
				return err
			}
			return ag.CreateAndSaveSnapshot(args[0])
		},
	}

	root.AddCommand(initCmd, snapCmd)
	if err := root.Execute(); err != nil {
		fmt.Println("Error:", err)
		os.Exit(1)
	}
}
