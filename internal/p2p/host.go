package p2p

import (
	"context"
	"fmt"
	"time"

	"github.com/hoangsonww/backupagent/config"
	"github.com/hoangsonww/backupagent/internal/monitoring"
	"github.com/hoangsonww/backupagent/internal/storage"
	libp2p "github.com/libp2p/go-libp2p"
	autonat "github.com/libp2p/go-libp2p-autonat"
	dht "github.com/libp2p/go-libp2p-kad-dht"
	pubsub "github.com/libp2p/go-libp2p-pubsub"
	"github.com/libp2p/go-libp2p/core/crypto"
	"github.com/libp2p/go-libp2p/core/host"
	peer "github.com/libp2p/go-libp2p/core/peer"
	peerstore "github.com/libp2p/go-libp2p/core/peerstore"
	discovery "github.com/libp2p/go-libp2p/p2p/discovery/routing"
	ma "github.com/multiformats/go-multiaddr"
)

type P2PHost struct {
	Host         host.Host
	PubSub       *pubsub.PubSub
	Topic        *pubsub.Topic
	DHT          *dht.IpfsDHT
	Ctx          context.Context
	Cancel       context.CancelFunc
	ChunkFetcher *ChunkFetcher
}

func Setup(cfg *config.Config, privKey crypto.PrivKey, store *storage.Store, signerPub, signerPriv []byte) (*P2PHost, error) {
	ctx, cancel := context.WithCancel(context.Background())
	logger := monitoring.GetLogger()

	opts := []libp2p.Option{
		libp2p.ListenAddrStrings(
			"/ip4/0.0.0.0/tcp/" + fmt.Sprint(cfg.ListenPort),
		),
		libp2p.NATPortMap(),
	}

	if cfg.NATTraversal.EnableAutoRelay {
		opts = append(opts, libp2p.EnableAutoRelay())
	}

	h, err := libp2p.New(
		opts...,
	)
	if err != nil {
		cancel()
		return nil, err
	}

	logger.Infof("P2P host started with ID: %s", h.ID().String())

	// DHT for peer discovery
	kadDHT, err := dht.New(ctx, h)
	if err != nil {
		cancel()
		return nil, err
	}
	if err := kadDHT.Bootstrap(ctx); err != nil {
		logger.WithError(err).Warn("DHT bootstrap failed")
	}

	// AutoNAT service to help with NAT awareness
	_, _ = autonat.New(h)

	// Bootstrap to provided peers
	for _, addr := range cfg.PeerBootstrap {
		maddr, err := ma.NewMultiaddr(addr)
		if err != nil {
			logger.WithError(err).Warnf("Invalid bootstrap address: %s", addr)
			continue
		}
		info, err := peer.AddrInfoFromP2pAddr(maddr)
		if err != nil {
			logger.WithError(err).Warnf("Failed to parse bootstrap peer: %s", addr)
			continue
		}
		h.Peerstore().AddAddrs(info.ID, info.Addrs, peerstore.PermanentAddrTTL)
		if err := h.Connect(ctx, *info); err != nil {
			logger.WithError(err).Warnf("Failed to connect to bootstrap peer: %s", info.ID)
		} else {
			logger.Infof("Connected to bootstrap peer: %s", info.ID)
			monitoring.GetMetrics().RecordPeerConnected()
		}
	}

	// Setup PubSub
	ps, err := pubsub.NewFloodSub(ctx, h)
	if err != nil {
		cancel()
		return nil, err
	}
	topic, err := ps.Join("backup-sync")
	if err != nil {
		cancel()
		return nil, err
	}

	logger.Info("Joined pubsub topic: backup-sync")

	// Rendezvous
	routingDiscovery := discovery.NewRoutingDiscovery(kadDHT)
	go func() {
		discovery.Advertise(ctx, routingDiscovery, "backupagent")
	}()

	// discover peers in background
	go func() {
		ticker := time.NewTicker(cfg.P2P.DiscoveryInterval)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				peerChan, err := routingDiscovery.FindPeers(ctx, "backupagent")
				if err != nil {
					logger.WithError(err).Debug("Peer discovery failed")
					continue
				}

				for pi := range peerChan {
					if pi.ID == h.ID() {
						continue
					}
					if h.Network().Connectedness(pi.ID) == 0 {
						h.Peerstore().AddAddrs(pi.ID, pi.Addrs, peerstore.PermanentAddrTTL)
						if err := h.Connect(ctx, pi); err == nil {
							logger.Infof("Discovered and connected to peer: %s", pi.ID)
							monitoring.GetMetrics().RecordPeerConnected()
							monitoring.GetMetrics().RecordPeerDiscovered()
						}
					}
				}
			}
		}
	}()

	// Initialize chunk fetcher
	chunkFetcher := NewChunkFetcher(
		store,
		signerPub,
		signerPriv,
		cfg.P2P.MaxConcurrentFetch,
		cfg.P2P.ChunkFetchTimeout,
	)

	return &P2PHost{
		Host:         h,
		PubSub:       ps,
		Topic:        topic,
		DHT:          kadDHT,
		Ctx:          ctx,
		Cancel:       cancel,
		ChunkFetcher: chunkFetcher,
	}, nil
}
