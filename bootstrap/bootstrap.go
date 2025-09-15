package bootstrap

import (
	"context"
	"fmt"

	libp2p "github.com/libp2p/go-libp2p"
	dht "github.com/libp2p/go-libp2p-kad-dht"
	"github.com/libp2p/go-libp2p/core/crypto"
	relayv2 "github.com/libp2p/go-libp2p/p2p/protocol/circuitv2/relay"
)

// Start creates a libp2p bootstrap host that listens on a single IPv4 TCP address
// and starts Relay v2 and DHT in server mode. It returns a single identified
// multiaddr string like "/ip4/127.0.0.1/tcp/<port>/p2p/<PeerID>".
func Start(ctx context.Context, listenHost string, listenPort int) (string, error) {
	// Generate identity (for production, persist instead of random)
	priv, _, err := crypto.GenerateKeyPair(crypto.RSA, 2048)
	if err != nil {
		return "", err
	}

	// Create host listening on a single address
	h, err := libp2p.New(
		libp2p.ListenAddrStrings(fmt.Sprintf("/ip4/%s/tcp/%d", listenHost, listenPort)),
		libp2p.Identity(priv),
	)
	if err != nil {
		return "", err
	}

	// Start Relay v2 service
	if _, err := relayv2.New(h); err != nil {
		_ = h.Close()
		return "", err
	}

	// Start DHT in server mode
	if _, err := dht.New(ctx, h, dht.Mode(dht.ModeServer)); err != nil {
		_ = h.Close()
		return "", err
	}

	// Prepare a single identified multiaddr to return
	addrs := h.Addrs()
	if len(addrs) == 0 {
		_ = h.Close()
		return "", fmt.Errorf("no listen addresses available")
	}
	identified := fmt.Sprintf("%s/p2p/%s", addrs[0], h.ID())

	return identified, nil
}
