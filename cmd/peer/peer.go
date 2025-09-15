package main

import (
	"bufio"
	"context"
	"fmt"
	"log"
	"time"

	libp2p "github.com/libp2p/go-libp2p"
	dht "github.com/libp2p/go-libp2p-kad-dht"
	"github.com/libp2p/go-libp2p/core/crypto"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	routingdisc "github.com/libp2p/go-libp2p/p2p/discovery/routing"
	discutil "github.com/libp2p/go-libp2p/p2p/discovery/util"
	routedhost "github.com/libp2p/go-libp2p/p2p/host/routed"
	ma "github.com/multiformats/go-multiaddr"
	"github.com/spf13/cobra"
)

const ProtocolID = "/chat/1.0.0"

var (
	flagBootstrap string
	flagPort      int
	flagRoom      string
	flagMessage   string
)

func main() {
	rootCmd := &cobra.Command{
		Use:   "peer",
		Short: "Run a libp2p chat peer",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := context.Background()

			// 生成随机身份（生产环境可持久化）
			priv, _, err := crypto.GenerateKeyPair(crypto.RSA, 2048)
			if err != nil {
				return err
			}

			// 创建 libp2p host
			baseHost, err := libp2p.New(
				libp2p.ListenAddrStrings(fmt.Sprintf("/ip4/0.0.0.0/tcp/%d", flagPort)),
				libp2p.Identity(priv),
				libp2p.NATPortMap(),
			)
			if err != nil {
				return err
			}

			log.Printf("✅ Peer started. ID: %s\n", baseHost.ID())
			for _, addr := range baseHost.Addrs() {
				fmt.Printf("Listening on: %s/p2p/%s\n", addr, baseHost.ID())
			}

			// 如果提供了 bootstrap 地址，就连过去（用于路由/DHT接入）
			if flagBootstrap != "" {
				log.Printf("Connecting to bootstrap: %s", flagBootstrap)
				maddr, err := ma.NewMultiaddr(flagBootstrap)
				if err != nil {
					log.Printf("Failed to new multiaddr: %v", err)
					return err
				}
				peerinfo, err := peer.AddrInfoFromP2pAddr(maddr)
				if err != nil {
					log.Printf("Failed to addr info from p2p addr: %v", err)
					return err
				}

				if err := baseHost.Connect(ctx, *peerinfo); err != nil {
					log.Printf("Failed to connect to bootstrap: %v", err)
					return fmt.Errorf("failed to connect to bootstrap: %w", err)
				}
				log.Println("✅ Connected to bootstrap node")
			}

			// 初始化 DHT 并引导
			kdht, err := dht.New(ctx, baseHost, dht.Mode(dht.ModeAuto))
			if err != nil {
				return fmt.Errorf("failed to start DHT: %w", err)
			}
			if err := kdht.Bootstrap(ctx); err != nil {
				return fmt.Errorf("failed to bootstrap DHT: %w", err)
			}

			// 用 RoutedHost 包装，让 Connect 可以通过 DHT 解析对端地址
			h := routedhost.Wrap(baseHost, kdht)

			// 设置消息处理器
			h.SetStreamHandler(ProtocolID, func(s network.Stream) {
				rw := bufio.NewReadWriter(bufio.NewReader(s), bufio.NewWriter(s))
				msg, _ := rw.ReadString('\n')
				log.Printf("📩 Received: %s", msg)
			})

			// 路由预热片刻，提升首次解析成功率
			time.Sleep(2 * time.Second)

			// 基于 DHT 的服务发现
			rd := routingdisc.NewRoutingDiscovery(kdht)

			// Advertise 自己到房间，然后查找其他 peers（该函数不返回值，会后台循环续约）
			discutil.Advertise(ctx, rd, flagRoom)
			log.Printf("📣 Advertising in room '%s'", flagRoom)

			// 查找并尝试直连其它 peer
			peerCh, err := rd.FindPeers(ctx, flagRoom)
			if err != nil {
				return fmt.Errorf("failed to find peers: %w", err)
			}

			go func() {
				for p := range peerCh {
					if p.ID == "" || p.ID == h.ID() {
						continue
					}
					ctxDial, cancel := context.WithTimeout(ctx, 15*time.Second)
					if err := h.Connect(ctxDial, p); err != nil {
						log.Printf("⚠️  Failed to connect to peer %s: %v", p.ID, err)
						cancel()
						continue
					}
					cancel()
					log.Printf("🤝 Connected to peer %s", p.ID)

					// 打开聊天流并发送一条消息
					s, err := h.NewStream(ctx, p.ID, ProtocolID)
					if err != nil {
						log.Printf("❌ Failed to create stream to %s: %v", p.ID, err)
						continue
					}
					rw := bufio.NewReadWriter(bufio.NewReader(s), bufio.NewWriter(s))
					_, _ = rw.WriteString(flagMessage + "\n")
					_ = rw.Flush()
					log.Printf("📤 Sent message to %s", p.ID)
				}
			}()

			// 阻塞
			select {}
		},
	}

	rootCmd.Flags().StringVar(&flagBootstrap, "bootstrap", "", "bootstrap multiaddr")
	rootCmd.Flags().IntVar(&flagPort, "port", 10000, "listening port")
	rootCmd.Flags().StringVar(&flagRoom, "room", "ezchat", "rendezvous room name")
	rootCmd.Flags().StringVar(&flagMessage, "message", "Hello from peer!", "message to send on connect")

	if err := rootCmd.Execute(); err != nil {
		log.Fatal(err)
	}
}
