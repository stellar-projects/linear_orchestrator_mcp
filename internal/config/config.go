package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

type Account struct {
	Token string `json:"token"`
	Note  string `json:"note,omitempty"`
}

type Config struct {
	Accounts map[string]Account `json:"accounts"`
}

func Path() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "linear-orchestrator", "config.json"), nil
}

func Load() (*Config, error) {
	p, err := Path()
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(p)
	if errors.Is(err, os.ErrNotExist) {
		return &Config{Accounts: map[string]Account{}}, nil
	}
	if err != nil {
		return nil, err
	}
	var c Config
	if err := json.Unmarshal(data, &c); err != nil {
		return nil, fmt.Errorf("parse config %s: %w", p, err)
	}
	if c.Accounts == nil {
		c.Accounts = map[string]Account{}
	}
	return &c, nil
}

func (c *Config) Save() error {
	p, err := Path()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(p), 0o700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(p, data, 0o600)
}

func (c *Config) Get(name string) (Account, error) {
	a, ok := c.Accounts[name]
	if !ok {
		return Account{}, fmt.Errorf("account %q not configured", name)
	}
	return a, nil
}
