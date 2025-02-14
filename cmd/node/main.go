package main

import (
	"context"
	"flag"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/caldog20/calnet/node"
)

const (
  ConfigPath = "/tmp/nodeconfig"
)

func main() {
	hs, _ := os.Hostname()

	var hostname string
	var server string
	flag.StringVar(&hostname, "hostname", hs, "hostname")
	flag.StringVar(&server, "server", "http://caldogstun.ddns.net:8080", "server address")
	flag.Parse()

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM, syscall.SIGQUIT)
	defer cancel()

    

	log.Fatal(node.Run(ctx, server, hostname))
}

