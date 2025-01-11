package types

import (
	"database/sql/driver"
	"encoding/json"
	"errors"
	"net/netip"
)

type Route struct {
	Prefix netip.Prefix `json:"prefix"`
	// Type
}

func (route Route) String() string {
	return route.Prefix.String()
}

type Routes []Route

func (routes Routes) Value() (driver.Value, error) {
	if routes != nil {
		b, err := json.Marshal(routes)
		if err != nil {
			return nil, err
		}
		return string(b), nil
	}
	return nil, nil
}

func (routes *Routes) Scan(value interface{}) error {
	if routes == nil {
		return nil
	}

	b, ok := value.(string)
	if !ok {
		return errors.New("failed to cast routes scan to string")
	}
	err := json.Unmarshal([]byte(b), routes)
	if err != nil {
		return err
	}
	return nil
}
