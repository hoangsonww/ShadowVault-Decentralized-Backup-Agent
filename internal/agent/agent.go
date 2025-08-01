package agent

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"github.com/hoangsonww/backupagent/config"
	"github.com/hoangsonww/backupagent/internal/auth"
	"github.com/hoangsonww/backupagent/internal/crypto"
	"github.com/hoangsonww/backupagent/internal/p2p"
	"github.com/hoangsonww/backupagent/internal/persistence"
	"github.com/hoangsonww/backupagent/internal/storage"
	"github.com/hoangsonww/backupagent/internal/versioning"
	"github.com/hoangsonww/backupagent/snapshots"
)

type Agent struct {
	Config    *config.Config
	DB        *persistence.DB
	Store     *storage.Store
	P2P       *p2p.P2PHost
	ACL       *auth.ACL
	SignerPub []byte
	SignerPriv []byte
}

func New(cfg *config.Config, passphrase string) (*Agent, error) {
	// Open DB
	dbPath := filepath.Join(cfg.RepositoryPath, "metadata.db")
	db, err := persistence.Open(dbPath)
	if err != nil {
		return nil, err
	}
	// derive master key
	key := crypto.DeriveKey(passphrase, nil)
	store, err := storage.New(db, key)
	if err != nil {
		return nil, err
	}
	// Load ACL
	acl := auth.NewACL(cfg.ACL.Admins)

	// Generate or load identity keypair for signing / peer identity
	pub, priv, err := crypto.GenerateEd25519Keypair()
	if err != nil {
		return nil, err
	}

	// Setup P2P with libp2p
	p2phost, err := p2p.Setup(cfg, /* convert Ed25519Priv to libp2p's PrivKey */ nil)
	if err != nil {
		return nil, err
	}

	agent := &Agent{
		Config:    cfg,
		DB:        db,
		Store:     store,
		P2P:       p2phost,
		ACL:       acl,
		SignerPub: pub,
		SignerPriv: priv,
	}
	return agent, nil
}

func (a *Agent) RunDaemon(ctx context.Context) error {
	// Subscribe to sync topic, respond to incoming updates
	sub, err := a.P2P.Topic.Subscribe()
	if err != nil {
		return err
	}
	go a.handlePubSub(sub)

	// Graceful shutdown
	c := make(chan os.Signal, 1)
	signal.Notify(c, syscall.SIGINT, syscall.SIGTERM)
	select {
	case <-c:
		fmt.Println("Shutting down")
		a.P2P.Cancel()
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (a *Agent) handlePubSub(sub *pubsub.Subscription) {
	for {
		msg, err := sub.Next(a.P2P.Ctx)
		if err != nil {
			return
		}
		// handle incoming snapshot announcements, peer sync requests, etc.
		fmt.Printf("Received pubsub message from %s\n", msg.GetFrom().String())
		// TODO: verify signature, check ACLs, fetch missing chunks, etc.
	}
}

func (a *Agent) CreateAndSaveSnapshot(path string) error {
	snap, err := snapshots.CreateSnapshot(path, a.Store, a.SignerPub, a.SignerPriv, "", a.Config.Snapshot.MinChunkSize, a.Config.Snapshot.MaxChunkSize, a.Config.Snapshot.AvgChunkSize)
	if err != nil {
		return err
	}
	if err := versioning.SaveSnapshot(a.DB, snap); err != nil {
		return err
	}
	// Broadcast metadata to peers
	// TODO: publish encoded snapshot (signed) over pubsub
	return nil
}
