package messages

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

type AccountConfig struct {
	Provider string `yaml:"provider"`
	Read     bool   `yaml:"read"`
	Write    bool   `yaml:"write"`
}

type Config struct {
	Dir      string                   `yaml:"-"`
	Default  string                   `yaml:"default"`
	Accounts map[string]AccountConfig `yaml:"accounts"`
}

func NewConfig() *Config {
	cfg := &Config{Dir: defaultDir()}
	if d := os.Getenv("MESSAGES_DIR"); d != "" {
		cfg.Dir = d
	}
	return cfg
}

func defaultDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ".messages"
	}
	return filepath.Join(home, ".config", "messages")
}

func (c *Config) EnsureDir() error {
	return os.MkdirAll(c.Dir, 0755)
}

func (c *Config) ConfigPath() string {
	return filepath.Join(c.Dir, "config.yaml")
}

func (c *Config) AccountDir(name string) string {
	return filepath.Join(c.Dir, "accounts", name)
}

func (c *Config) Load() error {
	data, err := os.ReadFile(c.ConfigPath())
	if err != nil {
		if os.IsNotExist(err) {
			c.Accounts = make(map[string]AccountConfig)
			return nil
		}
		return fmt.Errorf("failed to read config: %w", err)
	}
	if err := yaml.Unmarshal(data, c); err != nil {
		return fmt.Errorf("failed to parse config: %w", err)
	}
	if c.Accounts == nil {
		c.Accounts = make(map[string]AccountConfig)
	}
	return nil
}

func (c *Config) Save() error {
	data, err := yaml.Marshal(c)
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}
	return os.WriteFile(c.ConfigPath(), data, 0644)
}

func (c *Config) GetAccount(name string) (string, AccountConfig, error) {
	if name == "" {
		name = c.Default
	}
	if name == "" {
		return "", AccountConfig{}, fmt.Errorf("no account specified and no default set")
	}
	acct, ok := c.Accounts[name]
	if !ok {
		return "", AccountConfig{}, fmt.Errorf("account %q not found", name)
	}
	return name, acct, nil
}

func (c *Config) Validate() error {
	if c.Default != "" {
		if _, ok := c.Accounts[c.Default]; !ok {
			return fmt.Errorf("default account %q not found in accounts", c.Default)
		}
	}
	for name, acct := range c.Accounts {
		switch acct.Provider {
		case "beeper":
		default:
			return fmt.Errorf("account %q: unknown provider %q (must be beeper)", name, acct.Provider)
		}
	}
	return nil
}
