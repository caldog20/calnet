package client

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/caldog20/calnet/pkg/controlapi"
	"github.com/caldog20/calnet/pkg/keys"
)

const (
	TestControlURL = "http://127.0.0.1:8080"
)

var (
	nodePrivate    = keys.NewPrivateKey()
	controlPrivate = keys.NewPrivateKey()
	c              *Client
)

func TestMain(m *testing.M) {
	nodePrivate = keys.NewPrivateKey()
	controlPrivate = keys.NewPrivateKey()
	c = New(controlPrivate, nodePrivate.PublicKey(), TestControlURL)
	os.Exit(m.Run())
}

func TestControlClientLogin(t *testing.T) {
	login, err := c.Login(context.TODO())
	if err != nil {
		t.Fatal(err)
	}
	if login.LoggedIn != true {
		t.Fatalf("got logged in false, expected true")
	}
}

func TestControlClientPoll(t *testing.T) {
	resp := &controlapi.PollResponse{}
	gotResp := make(chan struct{})
	ctx, cancel := context.WithTimeout(context.TODO(), time.Second*10)
	defer cancel()

	err := c.StartPoll(ctx, func(pr *controlapi.PollResponse) {
		resp = pr
		gotResp <- struct{}{}
	})
	if err != nil {
		t.Fatal(err)
	}

	select {
	case <-gotResp:
	case <-ctx.Done():
		t.Fatal("context expired before resp received")
	}

	if resp == nil {
		t.Fatal("nil poll response")
	}

	if resp.Config == nil {
		t.Fatalf("got nil node config, expected proper config")
	}

	if resp.KeyExpired {
		t.Fatalf("got key expired, expected registered new node key")
	}
}
