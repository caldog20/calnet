package types

import (
	"database/sql/driver"
	"encoding/json"
	"errors"
	"net/netip"
)

type EndpointType int

const (
	EndpointTypeHost EndpointType = iota + 1
	EndpointTypeSTUN
)

type Endpoint struct {
	Endpoint netip.AddrPort `json:"endpoint"`
	Type     EndpointType   `json:"type"`
}

func NewEndpoint(endpoint string, endpointType EndpointType) (*Endpoint, error) {
	ap, err := netip.ParseAddrPort(endpoint)
	if err != nil {
		return nil, err
	}
	if !ap.IsValid() {
		return nil, errors.New("invalid endpoint address")
	}
	return &Endpoint{Endpoint: ap, Type: endpointType}, nil
}

func (ep *Endpoint) String() string {
	return ep.Endpoint.String()
}

func (ep *Endpoint) GetType() EndpointType {
	return ep.Type
}

type Endpoints []Endpoint

func (eps Endpoints) Value() (driver.Value, error) {
	if eps != nil {
		b, err := json.Marshal(eps)
		if err != nil {
			return nil, err
		}
		return string(b), nil
	}
	return nil, nil
}

func (eps *Endpoints) Scan(value interface{}) error {
	if value == nil {
		return nil
	}

	s, ok := value.(string)
	if !ok {
		return errors.New("failed to ip address scan into string")
	}
	err := json.Unmarshal([]byte(s), eps)
	if err != nil {
		return err
	}
	return nil
}
