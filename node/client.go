package node

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/netip"

	"github.com/caldog20/calnet/types"
	"github.com/coder/websocket"
)

type Client struct {
	ws         *websocket.Conn
	hc         *http.Client
	serverAddr string
	publicKey  types.PublicKey
	ip         netip.Addr
}

func NewClient(serverAddr string, key types.PublicKey) *Client {
	return &Client{
		ws:         nil,
		hc:         &http.Client{},
		serverAddr: serverAddr,
		publicKey:  key,
	}
}

func (c *Client) Login(hostname string) (*types.NodeConfig, error) {
	loginReq := types.LoginRequest{
		Hostname:     hostname,
		ProvisionKey: "please",
	}
	b, err := json.Marshal(loginReq)
	if err != nil {
		return nil, err
	}

	req, _ := http.NewRequest(http.MethodPost, "http://"+c.serverAddr+"/login", bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Node-Key", c.publicKey.String())
	resp, err := c.hc.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("login failed: %s", resp.Status)
	}

	body, err := io.ReadAll(req.Body)
	if err != nil {
		return nil, err
	}

	loginResp := types.LoginResponse{}
	err = json.Unmarshal(body, &loginResp)
	if err != nil {
		return nil, err
	}

	return &loginResp.NodeConfig, nil
}

func (c *Client) SendEndointUpdate(endpoints []types.Endpoint) error {
	if endpoints == nil {
		return nil
	}

	if c.ws == nil {
		return nil
	}

	req := &types.NodeUpdateRequest{
		Endpoints: endpoints,
	}

	b, err := json.Marshal(req)
	if err != nil {
		return err
	}

	err = c.ws.Write(context.Background(), websocket.MessageText, b)
	if err != nil {
		return err
	}

	return nil
}

func (c *Client) ReceiveUpdates(ctx context.Context) chan *types.NodeUpdateResponse {
	header := http.Header{}
	header.Set("X-Node-Key", c.publicKey.String())
	conn, resp, err := websocket.Dial(ctx, "ws://"+c.serverAddr+"/updates", &websocket.DialOptions{
		HTTPHeader: header,
	})
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println(resp.StatusCode)
	c.ws = conn

	ch := make(chan *types.NodeUpdateResponse)

	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			default:
			}
			update := &types.NodeUpdateResponse{}
			_, b, err := conn.Read(ctx)
			if err != nil {
				close(ch)
				log.Fatal(err)
			}
			err = json.Unmarshal(b, update)
			if err != nil {
				close(ch)
				log.Fatal(err)
			}
			ch <- update
		}
	}()

	return ch
}
