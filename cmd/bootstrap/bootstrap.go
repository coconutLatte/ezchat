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

	dht "github.com/libp2p/go-libp2p-kad-dht"
	relayv2 "github.com/libp2p/go-libp2p/p2p/protocol/circuitv2/relay"
)

const ProtocolID = "/chat/1.0.0"

func main() {
	listenPort := flag.Int("port", 9000, "listening port")
	flag.Parse()

	ctx := context.Background()

	// 生成一个固定的 RSA 密钥对（建议生产环境存下来，不要每次随机）
	priv, _, err := crypto.GenerateKeyPair(crypto.RSA, 2048)
	if err != nil {
		log.Fatal(err)
	}

	// 创建一个 libp2p Host，监听公网端口
	h, err := libp2p.New(
		libp2p.ListenAddrStrings(
			fmt.Sprintf("/ip4/0.0.0.0/tcp/%d", *listenPort),
			fmt.Sprintf("/ip6/::/tcp/%d", *listenPort),
		),
		libp2p.Identity(priv),
	)
	if err != nil {
		log.Fatal(err)
	}

	// 支持聊天协议，打印收到的消息
	h.SetStreamHandler(ProtocolID, func(s network.Stream) {
		rw := bufio.NewReadWriter(bufio.NewReader(s), bufio.NewWriter(s))
		msg, _ := rw.ReadString('\n')
		log.Printf("📩 [bootstrap] Received: %s", msg)
	})

	// 启动 Relay v2 服务
	_, err = relayv2.New(h)
	if err != nil {
		log.Fatalf("Failed to start relay: %v", err)
	}
	log.Println("✅ Relay v2 service started")

	// 启动 DHT 服务（全局模式）
	_, err = dht.New(ctx, h, dht.Mode(dht.ModeServer))
	if err != nil {
		log.Fatalf("Failed to start DHT: %v", err)
	}
	log.Println("✅ DHT service started")

	// 打印 multiaddr（其他节点用它作为 bootstrap）
	for _, addr := range h.Addrs() {
		fmt.Printf("Bootstrap node is listening on: %s/p2p/%s\n", addr, h.ID())
	}

	select {} // 阻塞，保持进程运行
}
