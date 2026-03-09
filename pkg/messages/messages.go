package messages

import (
	"context"
	"fmt"
	"io"
	"log/slog"

	"github.com/arjungandhi/messages/pkg/config"
)

// IncomingMessage is what `listen` outputs as JSON lines to stdout.
type IncomingMessage struct {
	RoomID     string `json:"room_id"`
	RoomName   string `json:"room_name"`
	Sender     string `json:"sender"`
	SenderName string `json:"sender_name"`
	Text       string `json:"text"`
	Timestamp  string `json:"timestamp"`
	EventID    string `json:"event_id"`
}

// OutgoingMessage is what `send` reads from stdin as JSON lines.
// Either room_id or user_id must be set. If user_id is set, a DM room is found or created.
type OutgoingMessage struct {
	RoomID string `json:"room_id"`
	UserID string `json:"user_id"`
	Text   string `json:"text"`
}

// Room represents a joined room/channel.
type Room struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// Client is the main entry point for interacting with messages.
type Client struct {
	Config   *config.Config
	provider *MatrixProvider
}

// New creates a new Client for the given account. If cfg is nil, default config is used.
// If accountName is empty, the default account from config is used.
func New(cfg *config.Config, accountName string) (*Client, error) {
	if cfg == nil {
		cfg = config.New()
	}
	if err := cfg.Load(); err != nil {
		return nil, err
	}

	name, acct, err := cfg.GetAccount(accountName)
	if err != nil {
		return nil, fmt.Errorf("%w. Run 'messages account add' first", err)
	}
	slog.Debug("using account", "name", name, "provider", acct.Provider)

	acctDir := cfg.AccountDir(name)

	var provider *MatrixProvider
	switch acct.Provider {
	case "matrix":
		p, err := NewMatrixProvider(acctDir)
		if err != nil {
			return nil, err
		}
		provider = p
	default:
		return nil, fmt.Errorf("unknown provider %q", acct.Provider)
	}

	if err := provider.Initialize(); err != nil {
		return nil, fmt.Errorf("%w. Run 'messages account add %s' to set up credentials", err, name)
	}

	return &Client{Config: cfg, provider: provider}, nil
}

// Close releases resources held by the client.
func (c *Client) Close() error {
	if c.provider != nil {
		return c.provider.Close()
	}
	return nil
}

// Listen long-polls for incoming messages, writing JSON lines to w.
// Blocks until ctx is cancelled.
func (c *Client) Listen(ctx context.Context, w io.Writer) error {
	return c.provider.Listen(ctx, w)
}

// Send sends a text message to a room.
func (c *Client) Send(ctx context.Context, roomID string, text string) error {
	return c.provider.Send(ctx, roomID, text)
}

// FindOrCreateDM returns the room ID for a direct message with the given user,
// creating the DM room if one doesn't already exist.
func (c *Client) FindOrCreateDM(ctx context.Context, userID string) (string, error) {
	return c.provider.FindOrCreateDM(ctx, userID)
}

// ListRooms returns all joined rooms.
func (c *Client) ListRooms(ctx context.Context) ([]Room, error) {
	return c.provider.ListRooms(ctx)
}
