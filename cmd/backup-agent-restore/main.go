package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/hoangsonww/backupagent/config"
	"github.com/hoangsonww/backupagent/internal/agent"
	"github.com/hoangsonww/backupagent/internal/versioning"
)

var (
	cfgFile    string
	passphrase string
)

func main() {
	root := &cobra.Command{
		Use:   "restore-agent",
		Short: "Restore a snapshot from repository",
	}
	root.PersistentFlags().StringVarP(&cfgFile, "config", "c", "config.yaml", "Path to config file")
	root.PersistentFlags().StringVarP(&passphrase, "pass", "p", "", "Passphrase for decryption (required)")

	restoreCmd := &cobra.Command{
		Use:   "restore [snapshot-id] [target-dir]",
		Short: "Restore snapshot to target directory",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			if passphrase == "" {
				return fmt.Errorf("passphrase is required")
			}
			snapshotID := args[0]
			target := args[1]
			cfg, err := config.Load(cfgFile)
			if err != nil {
				return err
			}
			ag, err := agent.New(cfg, passphrase)
			if err != nil {
				return err
			}
			snap, err := versioning.LoadSnapshot(ag.DB, snapshotID)
			if err != nil {
				return err
			}

			if err := os.MkdirAll(target, 0755); err != nil {
				return err
			}
			output := filepath.Join(target, fmt.Sprintf("restored_%s.bin", snapshotID))
			f, err := os.Create(output)
			if err != nil {
				return err
			}
			defer f.Close()
			for _, h := range snap.Chunks {
				data, err := ag.Store.GetChunk(h)
				if err != nil {
					return fmt.Errorf("failed to get chunk %s: %w", h, err)
				}
				if _, err := f.Write(data); err != nil {
					return err
				}
			}
			fmt.Printf("Restored snapshot %s to %s\n", snapshotID, output)
			return nil
		},
	}

	root.AddCommand(restoreCmd)
	if err := root.ExecuteContext(context.Background()); err != nil {
		fmt.Println("Error:", err)
		os.Exit(1)
	}
}
