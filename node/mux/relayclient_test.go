package mux

import (
	"testing"
	"time"

	"github.com/caldog20/calnet/types"
)

func TestRelayClientConnect(t *testing.T) {
	nodeKey := types.NewPrivateKey()
	pubkey := nodeKey.Public()
	client := &RelayClient{
		NodeKey:  pubkey,
		DialAddr: "http://127.0.0.1:8080",
	}

	err := client.Dial()
	if err != nil {
		t.Fatal(err)
	}

	time.Sleep(time.Second * 3)
	if !client.IsConnected() {
		t.Fatal("client couldn't make initial connection")
	}
  client.closeAndReconnect()
  time.Sleep(time.Second * 2)
	if !client.IsConnected() {
		t.Fatal("client couldn't reconnect")
	}
}
