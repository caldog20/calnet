package main

import (
	"context"
	"flag"
	"os"
	"os/signal"
	"syscall"
)

func main() {
	hs, _ := os.Hostname()

	var hostname string
	flag.StringVar(&hostname, "hostname", hs, "hostname")
	flag.Parse()

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
  _ = ctx
}
