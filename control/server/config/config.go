package config

import (
	"encoding/json"
	"errors"
	"log"
	"net/netip"
	"os"
	"path/filepath"
)

const (
	ConfigFileName = "config.json"
	StoreFileName  = "store.db"
)

type Config struct {
	NetworkPrefix  netip.Prefix `json:"network_prefix"`
	StorePath      string       `json:"store_path"`
	HTTPPort       int          `json:"http_port"`
	AutoCertDomain string       `json:"autocert_domain"`
	Debug          bool         `json:"debug_mode"`
	// Auth Stuff
}

func (c *Config) ReadConfigFromFile() error {
	path := filepath.Join(ConfigPath(), ConfigFileName)
	configFile, err := os.Open(path)
	if err != nil {
		return err
	}
	defer configFile.Close()

	err = json.NewDecoder(configFile).Decode(c)
	if err != nil {
		return err
	}

	return nil
}

func (c *Config) WriteConfigFile() error {
	path := filepath.Join(ConfigPath(), ConfigFileName)
	f, err := os.Open(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			log.Printf("config file doesn't exist, creating %s", ConfigPath())
			configFilePath := filepath.Join(ConfigPath(), ConfigFileName)
			os.MkdirAll(ConfigPath(), 0700)
			if f, err = os.OpenFile(configFilePath, os.O_RDWR|os.O_CREATE|os.O_EXCL, 0666); err != nil {
				return err
			}

		} else {
			return err
		}
	}
	defer f.Close()
	enc := json.NewEncoder(f)
	enc.SetIndent("", " ")
	return enc.Encode(c)
}

func (c *Config) SetDefaults() {
	*c = Config{
		NetworkPrefix:  netip.MustParsePrefix("100.70.0.0/24"),
		StorePath:      filepath.Join(ConfigPath(), StoreFileName),
		HTTPPort:       8080,
		AutoCertDomain: "",
		Debug:          false,
	}
}

func ConfigPath() string {
	subDir := "calnet"
	homeDir, err := os.UserConfigDir()
	if err != nil {
		homeDir = "./"
	}
	return filepath.Join("./", subDir)
	return filepath.Join(homeDir, subDir)
}
