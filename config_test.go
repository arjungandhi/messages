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

func TestConfig_LoadSave(t *testing.T) {
	dir := t.TempDir()
	cfg := &Config{
		Dir:     dir,
		Default: "test",
		Accounts: map[string]AccountConfig{
			"test": {Provider: "beeper", Read: true, Write: false},
		},
	}
	if err := cfg.Save(); err != nil {
		t.Fatal(err)
	}

	loaded := &Config{Dir: dir}
	if err := loaded.Load(); err != nil {
		t.Fatal(err)
	}
	if loaded.Default != "test" {
		t.Errorf("default: got %q, want %q", loaded.Default, "test")
	}
	acct, ok := loaded.Accounts["test"]
	if !ok {
		t.Fatal("account 'test' not found")
	}
	if acct.Provider != "beeper" {
		t.Errorf("provider: got %q, want %q", acct.Provider, "beeper")
	}
	if !acct.Read {
		t.Error("read: got false, want true")
	}
	if acct.Write {
		t.Error("write: got true, want false")
	}
}

func TestConfig_LoadMissing(t *testing.T) {
	cfg := &Config{Dir: t.TempDir()}
	if err := cfg.Load(); err != nil {
		t.Fatal(err)
	}
	if len(cfg.Accounts) != 0 {
		t.Errorf("expected empty accounts, got %d", len(cfg.Accounts))
	}
}

func TestConfig_GetAccount(t *testing.T) {
	cfg := &Config{
		Default: "main",
		Accounts: map[string]AccountConfig{
			"main": {Provider: "beeper", Read: true, Write: true},
			"bot":  {Provider: "matrix", Read: false, Write: true},
		},
	}

	// explicit name
	name, acct, err := cfg.GetAccount("bot")
	if err != nil {
		t.Fatal(err)
	}
	if name != "bot" || acct.Provider != "matrix" {
		t.Errorf("got %s/%s, want bot/matrix", name, acct.Provider)
	}

	// empty uses default
	name, acct, err = cfg.GetAccount("")
	if err != nil {
		t.Fatal(err)
	}
	if name != "main" || acct.Provider != "beeper" {
		t.Errorf("got %s/%s, want main/beeper", name, acct.Provider)
	}

	// missing account
	_, _, err = cfg.GetAccount("nope")
	if err == nil {
		t.Error("expected error for missing account")
	}
}

func TestConfig_GetAccount_NoDefault(t *testing.T) {
	cfg := &Config{
		Accounts: map[string]AccountConfig{
			"main": {Provider: "beeper"},
		},
	}
	_, _, err := cfg.GetAccount("")
	if err == nil {
		t.Error("expected error when no default set")
	}
}

func TestConfig_Validate(t *testing.T) {
	// valid
	cfg := &Config{
		Default: "a",
		Accounts: map[string]AccountConfig{
			"a": {Provider: "beeper"},
			"b": {Provider: "beeper"},
		},
	}
	if err := cfg.Validate(); err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	// bad default
	cfg.Default = "missing"
	if err := cfg.Validate(); err == nil {
		t.Error("expected error for bad default")
	}

	// bad provider
	cfg.Default = "a"
	cfg.Accounts["a"] = AccountConfig{Provider: "slack"}
	if err := cfg.Validate(); err == nil {
		t.Error("expected error for bad provider")
	}
}

func TestConfig_AccountDir(t *testing.T) {
	cfg := &Config{Dir: "/home/user/.config/messages"}
	got := cfg.AccountDir("bot")
	want := "/home/user/.config/messages/accounts/bot"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}
