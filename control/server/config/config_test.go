package config

import (
	"errors"
	"os"
	"testing"
)

func TestOpenOrCreateConfigFile(t *testing.T) {
	var conf Config
	err := conf.ReadConfigFromFile()
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			conf.SetDefaults()
			err := conf.WriteConfigFile()
			if err != nil {
				t.Fatal(err)
			}
			// Retry reading config file from disk
			err = conf.ReadConfigFromFile()
			if err != nil {
				t.Fatal(err)
			}
		}
	}
}
