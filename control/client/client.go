package client

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"log"
	"net/http"
	"net/url"
	"sync"

	"github.com/caldog20/calnet/pkg/controlapi"
	"github.com/caldog20/calnet/pkg/keys"
)

type Client struct {
	c          *http.Client
	controlURL *url.URL

	// Control Server Public Key
	controlPublic keys.PublicKey
	// Node Control Private Key
	controlPrivate keys.PrivateKey
	// Node Data Public Key
	nodePublic keys.PublicKey

	mu       sync.Mutex
	loggedIn bool
}

func New(controlKey keys.PrivateKey, nodeKey keys.PublicKey, serverAddr string) *Client {
	u, err := url.Parse(serverAddr)
	if err != nil {
		panic("invalid server url")
	}
	return &Client{
		c:              &http.Client{},
		controlURL:     u,
		controlPrivate: controlKey,
		nodePublic:     nodeKey,
	}
}

func (c *Client) getServerKey() error {
	resp, err := c.c.Get(c.controlURL.JoinPath("key").String())
	if err != nil {
		return err
	}

	serverKeyResp := &controlapi.ControlKey{}
	defer resp.Body.Close()
	b, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	err = json.Unmarshal(b, serverKeyResp)
	if err != nil {
		return err
	}

	if serverKeyResp.PublicKey.IsZero() {
		return errors.New("control server key is zero")
	}

	// c.mu.Lock()
	// defer c.mu.Unlock()
	c.controlPublic = serverKeyResp.PublicKey

	return nil
}

func (c *Client) Login(ctx context.Context) (*controlapi.LoginResponse, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.controlPublic.IsZero() {
		err := c.getServerKey()
		if err != nil {
			return nil, err
		}
	}

	loginReq := controlapi.LoginRequest{
		NodeKey:      c.nodePublic,
		ProvisionKey: "please",
	}

	b, err := json.Marshal(loginReq)
	if err != nil {
		return nil, err
	}

	encrypted := c.controlPrivate.EncryptBox(b, c.controlPublic)

	req, _ := http.NewRequest(
		"POST",
		c.controlURL.JoinPath("/login").String(),
		bytes.NewReader(encrypted),
	)
	req.Header.Set("X-Control-Key", c.controlPrivate.PublicKey().EncodeToString())
	resp, err := c.c.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	loginResp := controlapi.LoginResponse{}
	b, err = io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	decrypted, ok := c.controlPrivate.DecryptBox(b, c.controlPublic)
	if !ok {
		return nil, errors.New("error decrypting control login response")
	}

	err = json.Unmarshal(decrypted, &loginResp)
	if err != nil {
		return nil, err
	}

	if loginResp.LoggedIn {
		c.loggedIn = true
	}

	return &loginResp, nil
}

func (c *Client) StartPoll(ctx context.Context, callback func(*controlapi.PollResponse)) error {
	pollCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	pollReq := &controlapi.PollRequest{}

	c.mu.Lock()
	if !c.loggedIn {
		return errors.New("client must be logged in before polling")
	}
	if c.controlPublic.IsZero() {
		return errors.New("control server key is zero: cannot poll")
	}
	c.mu.Unlock()

	pollFunc := func() {
		for {
			select {
			case <-ctx.Done():
				return
			default:
			}

			c.mu.Lock()
			cKey := c.controlPrivate
			sKey := c.controlPublic
			nKey := c.nodePublic
			c.mu.Unlock()

			pollReq.NodeKey = nKey
			b, err := json.Marshal(pollReq)
			if err != nil {
				log.Printf("error marshalling poll request: %s", err)
				return
			}

			encrypted := cKey.EncryptBox(b, sKey)

			req, _ := http.NewRequestWithContext(
				pollCtx,
				"POST",
				c.controlURL.JoinPath("/poll").String(),
				bytes.NewReader(encrypted),
			)
			req.Header.Set("X-Control-Key", cKey.PublicKey().EncodeToString())

			resp, err := c.c.Do(req)
			if err != nil {
				log.Printf("polling fatal error: %s", err)
				return
			}
			defer resp.Body.Close()

			if resp.StatusCode == http.StatusNoContent {
				continue
			}

			b, err = io.ReadAll(resp.Body)
			if err != nil {
				log.Printf("error reading response body")
				continue
			}

			decrypted, ok := c.controlPrivate.DecryptBox(b, c.controlPublic)
			if !ok {
				log.Println("error decrypting control login response")
				continue
			}

			pollResp := &controlapi.PollResponse{}
			err = json.Unmarshal(decrypted, pollResp)
			if err != nil {
				log.Println("error unmarshalling poll response")
				continue
			}

			if pollResp.KeyExpired {
				log.Println("node key is now expired, stopping poll")
				c.mu.Lock()
				c.loggedIn = false
				c.mu.Unlock()
				cancel()
			}

			if callback != nil {
				callback(pollResp)
			}
		}
	}

	go pollFunc()

	return nil
}
