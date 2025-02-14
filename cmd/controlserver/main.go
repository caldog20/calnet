package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/caldog20/calnet/control/controlserver"
	"github.com/caldog20/calnet/control/controlserver/store"
)

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	store, err := store.NewBoltStore("./bolt.db")
	if err != nil {
		log.Fatal(err)
	}
	defer store.Close()

	server := controlserver.NewServer(store)
	server.ListenAddr = ":8080"
	err = server.ListenAndServe()
	if err != nil {
		log.Fatal(err)
	}
	defer server.Close()

	<-ctx.Done()
}
