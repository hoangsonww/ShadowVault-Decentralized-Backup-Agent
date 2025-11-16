package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/multiformats/go-multiaddr"
	"github.com/spf13/cobra"
	"go.etcd.io/bbolt"

	"github.com/hoangsonww/backupagent/config"
	"github.com/hoangsonww/backupagent/internal/agent"
	"github.com/hoangsonww/backupagent/internal/persistence"
	"github.com/libp2p/go-libp2p/core/peer"
)

var (
	cfgFile    string
	passphrase string
)

func main() {
	root := &cobra.Command{
		Use:   "peerctl",
		Short: "Manage peers in backupagent network",
	}
	root.PersistentFlags().StringVarP(&cfgFile, "config", "c", "config.yaml", "path to config")
	root.PersistentFlags().StringVarP(&passphrase, "pass", "p", "", "passphrase (required)")

	addCmd := &cobra.Command{
		Use:   "add [multiaddr]",
		Short: "Add and connect to a peer (multiaddr format)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if passphrase == "" {
				return fmt.Errorf("passphrase is required")
			}
			maddrStr := args[0]
			cfg, err := config.Load(cfgFile)
			if err != nil {
				return err
			}
			ag, err := agent.New(cfg, passphrase)
			if err != nil {
				return err
			}
			maddr, err := multiaddr.NewMultiaddr(maddrStr)
			if err != nil {
				return err
			}
			info, err := peer.AddrInfoFromP2pAddr(maddr)
			if err != nil {
				return err
			}
			if err := ag.P2P.Host.Connect(context.Background(), *info); err != nil {
				return err
			}
			// persist peer
			err = ag.DB.Update(func(tx *bbolt.Tx) error {
				b := tx.Bucket([]byte(persistence.BucketPeers))
				val, _ := json.Marshal(info)
				return b.Put([]byte(info.ID.String()), val)
			})
			if err != nil {
				return err
			}
			fmt.Printf("Added and connected to peer %s\n", info.ID.String())
			return nil
		},
	}

	removeCmd := &cobra.Command{
		Use:   "remove [peerID]",
		Short: "Remove a peer from stored peer list",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if passphrase == "" {
				return fmt.Errorf("passphrase is required")
			}
			peerID := args[0]
			cfg, err := config.Load(cfgFile)
			if err != nil {
				return err
			}
			ag, err := agent.New(cfg, passphrase)
			if err != nil {
				return err
			}
			err = ag.DB.Update(func(tx *bbolt.Tx) error {
				b := tx.Bucket([]byte(persistence.BucketPeers))
				return b.Delete([]byte(peerID))
			})
			if err != nil {
				return err
			}
			fmt.Printf("Removed peer %s\n", peerID)
			return nil
		},
	}

	listCmd := &cobra.Command{
		Use:   "list",
		Short: "List stored peers",
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
			err = ag.DB.View(func(tx *bbolt.Tx) error {
				b := tx.Bucket([]byte(persistence.BucketPeers))
				return b.ForEach(func(k, v []byte) error {
					fmt.Printf("PeerID: %s\n", string(k))
					return nil
				})
			})
			return err
		},
	}

	root.AddCommand(addCmd, removeCmd, listCmd)
	if err := root.Execute(); err != nil {
		fmt.Println("peerctl error:", err)
		os.Exit(1)
	}
}
