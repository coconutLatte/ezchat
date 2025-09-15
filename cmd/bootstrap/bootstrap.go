package main

import (
	"context"
	"flag"
	"fmt"
	"log"

	"github.com/coconutLatte/ezchat/bootstrap"
)

func main() {
	listenHost := flag.String("host", "127.0.0.1", "listening host (ip)")
	listenPort := flag.Int("port", 9000, "listening port")
	flag.Parse()

	ctx := context.Background()

	addr, err := bootstrap.Start(ctx, *listenHost, *listenPort)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("%s\n", addr)

	select {}
}
