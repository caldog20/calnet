package node

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/caldog20/calnet/control"
	"github.com/caldog20/calnet/types"
)

type Client struct {
	hc         *http.Client
	serverAddr string
	publicKey  types.PublicKey
}

func NewClient(serverAddr string, key types.PublicKey) *Client {
	return &Client{
		hc:         &http.Client{},
		serverAddr: serverAddr,
		publicKey:  key,
	}
}

func (c *Client) Login(hostname string) (*control.NodeConfig, error) {
	loginReq := control.LoginRequest{
		NodeKey:      c.publicKey,
		Hostname:     hostname,
		ProvisionKey: "please",
	}
	b, err := json.Marshal(loginReq)
	if err != nil {
		return nil, err
	}

	req, _ := http.NewRequest(http.MethodPost, c.serverAddr + "/login", bytes.NewReader(b))
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

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	fmt.Println(string(body))

	loginResp := control.LoginResponse{}
	err = json.Unmarshal(body, &loginResp)
	if err != nil {
		return nil, err
	}

	return &loginResp.NodeConfig, nil
}

func (c *Client) Poll() (*control.PollResponse, error) {
	pollReq := control.PollRequest{
		NodeKey: c.publicKey,
	}
	b, err := json.Marshal(pollReq)
	if err != nil {
		return nil, err
	}
	req, _ := http.NewRequest(http.MethodPost, c.serverAddr+"/poll", bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.hc.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusRequestTimeout {
		return nil, nil
	} else if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("poll failed: %s", resp.Status)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	pollResp := control.PollResponse{}
	err = json.Unmarshal(body, &pollResp)
	if err != nil {
		return nil, err
	}
	return &pollResp, nil
}
