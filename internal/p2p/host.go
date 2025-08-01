package p2p

import (
	"context"
	"time"

	libp2p "github.com/libp2p/go-libp2p"
	autonat "github.com/libp2p/go-libp2p-autonat"
	relay "github.com/libp2p/go-libp2p/p2p/host/relay"
	discovery "github.com/libp2p/go-libp2p/p2p/discovery"
	dht "github.com/libp2p/go-libp2p-kad-dht"
	pubsub "github.com/libp2p/go-libp2p-pubsub"
	peer "github.com/libp2p/go-libp2p/core/peer"
	peerstore "github.com/libp2p/go-libp2p/core/peerstore"
	ma "github.com/multiformats/go-multiaddr"

	"github.com/hoangsonww/backupagent/config"
)

type P2PHost struct {
	Host    host.Host
	PubSub  *pubsub.PubSub
	Topic   *pubsub.Topic
	DHT     *dht.IpfsDHT
	Ctx     context.Context
	Cancel  context.CancelFunc
}

func Setup(cfg *config.Config, privKey crypto.PrivKey) (*P2PHost, error) {
	ctx, cancel := context.WithCancel(context.Background())

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
		libp2p.Identity(privKey),
		opts...,
	)
	if err != nil {
		cancel()
		return nil, err
	}

	// DHT for peer discovery
	kadDHT, err := dht.New(ctx, h)
	if err != nil {
		cancel()
		return nil, err
	}
	if err := kadDHT.PopulateRoutingTable(); err != nil {
		// log but continue
	}

	// AutoNAT service to help with NAT awareness
	_, _ = autonat.New(ctx, h)

	// Bootstrap to provided peers
	for _, addr := range cfg.PeerBootstrap {
		maddr, err := ma.NewMultiaddr(addr)
		if err != nil {
			continue
		}
		info, err := peer.AddrInfoFromP2pAddr(maddr)
		if err != nil {
			continue
		}
		h.Peerstore().AddAddrs(info.ID, info.Addrs, peerstore.PermanentAddrTTL)
		_ = h.Connect(ctx, *info)
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

	// Rendezvous
	routingDiscovery := discovery.NewRoutingDiscovery(kadDHT)
	discovery.Advertise(ctx, routingDiscovery, "backupagent")
	// discover peers in background
	go func() {
		peerChan, _ := routingDiscovery.FindPeers(ctx, "backupagent")
		for pi := range peerChan {
			if pi.ID == h.ID() {
				continue
			}
			h.Peerstore().AddAddrs(pi.ID, pi.Addrs, peerstore.PermanentAddrTTL)
			_ = h.Connect(ctx, pi)
		}
	}()

	return &P2PHost{
		Host:   h,
		PubSub: ps,
		Topic:  topic,
		DHT:    kadDHT,
		Ctx:    ctx,
		Cancel: cancel,
	}, nil
}
