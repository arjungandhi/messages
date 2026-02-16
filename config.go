package messages

import (
	"os"
	"path/filepath"
)

type Config struct {
	Dir string
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
