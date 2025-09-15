package main

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"log"

	libp2p "github.com/libp2p/go-libp2p"
	"github.com/libp2p/go-libp2p/core/crypto"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	ma "github.com/multiformats/go-multiaddr"
)

const ProtocolID = "/chat/1.0.0"

func main() {
	bootstrapAddr := flag.String("bootstrap", "", "bootstrap multiaddr")
	listenPort := flag.Int("port", 10000, "listening port")
	flag.Parse()

	ctx := context.Background()

	// 生成随机身份（生产环境可持久化）
	priv, _, err := crypto.GenerateKeyPair(crypto.RSA, 2048)
	if err != nil {
		log.Fatal(err)
	}

	// 创建 libp2p host
	h, err := libp2p.New(
		libp2p.ListenAddrStrings(fmt.Sprintf("/ip4/0.0.0.0/tcp/%d", *listenPort)),
		libp2p.Identity(priv),
	)
	if err != nil {
		log.Fatal(err)
	}

	log.Printf("✅ Peer started. ID: %s\n", h.ID())
	for _, addr := range h.Addrs() {
		fmt.Printf("Listening on: %s/p2p/%s\n", addr, h.ID())
	}

	// 设置消息处理器
	h.SetStreamHandler(ProtocolID, func(s network.Stream) {
		rw := bufio.NewReadWriter(bufio.NewReader(s), bufio.NewWriter(s))
		msg, _ := rw.ReadString('\n')
		log.Printf("📩 Received: %s", msg)
	})

	// 如果提供了 bootstrap 地址，就连过去
	if *bootstrapAddr != "" {
		maddr, err := ma.NewMultiaddr(*bootstrapAddr)
		if err != nil {
			log.Fatal(err)
		}
		peerinfo, err := peer.AddrInfoFromP2pAddr(maddr)
		if err != nil {
			log.Fatal(err)
		}

		if err := h.Connect(ctx, *peerinfo); err != nil {
			log.Fatalf("❌ Failed to connect to bootstrap: %v", err)
		}
		log.Println("✅ Connected to bootstrap node")

		// 尝试打开一个 stream 并发送消息
		s, err := h.NewStream(ctx, peerinfo.ID, ProtocolID)
		if err != nil {
			log.Fatalf("❌ Failed to create stream: %v", err)
		}
		rw := bufio.NewReadWriter(bufio.NewReader(s), bufio.NewWriter(s))
		_, _ = rw.WriteString("Hello from peer!\n")
		_ = rw.Flush()
		log.Println("📤 Sent message to bootstrap")
	}

	// 阻塞
	select {}
}
