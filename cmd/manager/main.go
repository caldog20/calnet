package main

import (
	"context"
	"log"
	"net/netip"
	"os/signal"
	"syscall"

	"github.com/caldog20/calnet/manager"
)

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	store, err := manager.NewSqlStore("store.db")
	if err != nil {
		log.Fatal(err)
	}
	defer store.Close()

	prefix := netip.MustParsePrefix("100.66.0.0/24")
	ipam, err := manager.NewIPAM(prefix, store)

	server := manager.NewServer(":8080", store, ipam, true)

	err = server.Run(ctx)
	if err != nil {
		log.Fatal(err)
	}
}
