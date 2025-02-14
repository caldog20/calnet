package controlclient

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sync"
	"time"

	"github.com/caldog20/calnet/control"
	"github.com/caldog20/calnet/types"
)

const (
	NodeKeyHeader = "X-Node-Key"

	LoginCtxTimeout = time.Second * 5
)

// ControlClient provides a client implementation to connect to the control server.
// The client handles login/authentication, polling for network updates, and managing the state from those updates
type ControlClient struct {
	nodeKey  types.PublicKey
	c        *http.Client
	endpoint *url.URL
	// Callback when network updates happen
	updateFn   func([]control.RemotePeer) error
	pollCtx    context.Context
	pollCancel context.CancelFunc

	mu       sync.Mutex
	loggedIn bool
	expired  bool
}

// NewControlClient creates a new control client, but does not start polling.
// Poll must be called to initiate polling
func NewControlClient(
	nodeKey types.PublicKey,
	endpoint string,
	updateHandler func([]control.RemotePeer) error,
) *ControlClient {
	url, err := url.Parse(endpoint)
	if err != nil {
		panic("invalid control server url")
	}

	c := &http.Client{}

	return &ControlClient{
		nodeKey:  nodeKey,
		c:        c,
		updateFn: updateHandler,
		endpoint: url,
	}
}

func (c *ControlClient) Login(hostname string, provisionKey string) (string, error) {
	loginReq := control.LoginRequest{
		NodeKey:      c.nodeKey,
		Hostname:     hostname,
		ProvisionKey: provisionKey,
	}

	loginCtx, cancel := context.WithTimeout(context.Background(), LoginCtxTimeout)
	loginResp, expired, err := c.login(ctx, loginReq)
    if err != nil {
        return 
    }

}

func (c *ControlClient) login(
	ctx context.Context,
	r *control.LoginRequest,
) (*control.LoginResponse, bool, error) {
	data, err := json.Marshal(r)
	if err != nil {
		return nil, false, err
	}

	req, err := http.NewRequestWithContext(
		ctx,
		http.MethodPost,
		c.endpoint.JoinPath("/login").String(),
		bytes.NewReader(data),
	)
	if err != nil {
		return nil, false, errors.New("login: error building http request with context")
	}

	req.Header.Set(NodeKeyHeader, c.nodeKey.String())
	resp, err := c.c.Do(req)
	defer resp.Body.Close()

	switch resp.StatusCode {
	case http.StatusBadRequest:
		return nil, false, fmt.Errorf("login: bad request: %s", resp.Body)
	case http.StatusInternalServerError:
		return nil, false, fmt.Errorf("login: control internal error: %s", resp.Body)
	case http.StatusUnauthorized:
		return nil, true, nil
	default:
	}

	loginResp := control.LoginResponse{}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, false, errors.New("error reading response body")
	}

	if err := json.Unmarshal(body, &loginResp); err != nil {
		return nil, false, errors.New("error unmarshing login response")
	}

	return &loginResp, false, nil
}

func (c *ControlClient) IsLoggedIn() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.loggedIn
}

func (c *ControlClient) IsExpired() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.expired
}
