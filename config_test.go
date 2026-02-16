package messages

import (
	"os"
	"path/filepath"
	"testing"
)

func TestNewConfig_Default(t *testing.T) {
	os.Unsetenv("MESSAGES_DIR")
	cfg := NewConfig()
	home, _ := os.UserHomeDir()
	want := filepath.Join(home, ".config", "messages")
	if cfg.Dir != want {
		t.Errorf("got %s, want %s", cfg.Dir, want)
	}
}

func TestNewConfig_EnvOverride(t *testing.T) {
	t.Setenv("MESSAGES_DIR", "/tmp/test-messages")
	cfg := NewConfig()
	if cfg.Dir != "/tmp/test-messages" {
		t.Errorf("got %s, want /tmp/test-messages", cfg.Dir)
	}
}

func TestConfig_EnsureDir(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "messages")
	cfg := &Config{Dir: dir}
	if err := cfg.EnsureDir(); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(dir)
	if err != nil {
		t.Fatalf("directory not created: %v", err)
	}
	if !info.IsDir() {
		t.Fatal("not a directory")
	}
}
