package messages

import (
	"context"
	"io"
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
type OutgoingMessage struct {
	RoomID string `json:"room_id"`
	Text   string `json:"text"`
}

// Provider is the interface for messaging backends.
// Implementations handle protocol-specific details (Matrix, etc.)
type Provider interface {
	// Initialize sets up the client connection using stored credentials.
	Initialize() error
	// Listen long-polls for incoming messages, writing JSON lines to w.
	// Blocks until ctx is cancelled.
	Listen(ctx context.Context, w io.Writer) error
	// Send sends a text message to a room/channel.
	Send(ctx context.Context, roomID string, text string) error
}
