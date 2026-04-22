package config

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
)

type Mode string

const (
	ModeLocal  Mode = "local"
	ModeServer Mode = "server"
	ModeClient Mode = "client"
)

type Config struct {
	Mode              Mode   `json:"mode"`
	ListenAddress     string `json:"listenAddress"`
	RemoteBaseURL     string `json:"remoteBaseUrl"`
	AccessibilityMode bool   `json:"accessibilityMode"`
}

type Public struct {
	Mode              Mode   `json:"mode"`
	ListenAddress     string `json:"listenAddress"`
	RemoteBaseURL     string `json:"remoteBaseUrl"`
	AccessibilityMode bool   `json:"accessibilityMode"`
}

func Default() Config {
	return Config{
		Mode:              ModeLocal,
		ListenAddress:     "127.0.0.1:8787",
		RemoteBaseURL:     "http://127.0.0.1:8787",
		AccessibilityMode: true,
	}
}

func DefaultPath() string {
	root, err := os.UserConfigDir()
	if err != nil || root == "" {
		return filepath.Join(".", "simplehermes.json")
	}
	return filepath.Join(root, "simplehermes", "config.json")
}

func Load(path string) (Config, error) {
	cfg := Default()

	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return cfg, nil
		}
		return Config{}, err
	}

	if err := json.Unmarshal(data, &cfg); err != nil {
		return Config{}, err
	}

	cfg.Normalize()
	return cfg, nil
}

func Save(path string, cfg Config) error {
	cfg.Normalize()

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}

	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')

	return os.WriteFile(path, data, 0o644)
}

func (c *Config) Normalize() {
	switch c.Mode {
	case ModeLocal, ModeServer, ModeClient:
	default:
		c.Mode = Default().Mode
	}

	c.ListenAddress = strings.TrimSpace(c.ListenAddress)
	if c.ListenAddress == "" {
		c.ListenAddress = Default().ListenAddress
	}

	c.RemoteBaseURL = strings.TrimSpace(c.RemoteBaseURL)
	if c.RemoteBaseURL == "" {
		c.RemoteBaseURL = Default().RemoteBaseURL
	}
	c.RemoteBaseURL = strings.TrimRight(c.RemoteBaseURL, "/")
}

func (c Config) Public() Public {
	c.Normalize()
	return Public{
		Mode:              c.Mode,
		ListenAddress:     c.ListenAddress,
		RemoteBaseURL:     c.RemoteBaseURL,
		AccessibilityMode: c.AccessibilityMode,
	}
}
