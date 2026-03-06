package messages

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	_ "go.mau.fi/util/dbutil/litestream"
	"maunium.net/go/mautrix"
	"maunium.net/go/mautrix/crypto/cryptohelper"
	"maunium.net/go/mautrix/event"
	"maunium.net/go/mautrix/id"
)

type MatrixCredentials struct {
	HomeserverURL string `json:"homeserver_url"`
	UserID        string `json:"user_id"`
	AccessToken   string `json:"access_token"`
	DeviceID      string `json:"device_id,omitempty"`
}

type MatrixProvider struct {
	client       *mautrix.Client
	cryptoHelper *cryptohelper.CryptoHelper
	userID       id.UserID
	dir          string
}

func NewMatrixProvider(dir string) (*MatrixProvider, error) {
	return &MatrixProvider{dir: dir}, nil
}

func (p *MatrixProvider) SaveCredentials(creds *MatrixCredentials) error {
	if err := os.MkdirAll(p.dir, 0755); err != nil {
		return fmt.Errorf("failed to create credentials directory: %w", err)
	}
	credsPath := filepath.Join(p.dir, "matrix_credentials.json")
	data, err := json.MarshalIndent(creds, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal credentials: %w", err)
	}
	if err := os.WriteFile(credsPath, data, 0600); err != nil {
		return fmt.Errorf("failed to write credentials: %w", err)
	}
	return nil
}

func (p *MatrixProvider) LoadCredentials() (*MatrixCredentials, error) {
	credsPath := filepath.Join(p.dir, "matrix_credentials.json")
	data, err := os.ReadFile(credsPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to read credentials: %w", err)
	}
	var creds MatrixCredentials
	if err := json.Unmarshal(data, &creds); err != nil {
		return nil, fmt.Errorf("failed to unmarshal credentials: %w", err)
	}
	return &creds, nil
}

func (p *MatrixProvider) Initialize() error {
	creds, err := p.LoadCredentials()
	if err != nil {
		return fmt.Errorf("failed to load credentials: %w", err)
	}
	if creds == nil || creds.AccessToken == "" {
		return fmt.Errorf("no credentials found")
	}
	p.userID = id.UserID(creds.UserID)
	client, err := mautrix.NewClient(creds.HomeserverURL, p.userID, creds.AccessToken)
	if err != nil {
		return fmt.Errorf("failed to create Matrix client: %w", err)
	}
	if creds.DeviceID != "" {
		client.DeviceID = id.DeviceID(creds.DeviceID)
	}
	p.client = client

	// Set up E2EE using a SQLite database for key storage
	dbPath := filepath.Join(p.dir, "crypto.db")
	helper, err := cryptohelper.NewCryptoHelper(client, []byte("messages"), dbPath)
	if err != nil {
		return fmt.Errorf("failed to create crypto helper: %w", err)
	}
	if err := helper.Init(context.Background()); err != nil {
		return fmt.Errorf("failed to init crypto helper: %w", err)
	}
	p.cryptoHelper = helper
	client.Crypto = helper

	return nil
}

// Listen uses the Matrix sync loop to long-poll for incoming messages.
// Each message event (excluding our own) is written as a JSON line to w.
// Handles both plaintext and encrypted messages.
// Blocks until ctx is cancelled.
func (p *MatrixProvider) Listen(ctx context.Context, w io.Writer) error {
	syncer := p.client.Syncer.(*mautrix.DefaultSyncer)
	enc := json.NewEncoder(w)

	handleMessage := func(ctx context.Context, evt *event.Event) {
		if evt.Sender == p.userID {
			return
		}

		content := evt.Content.AsMessage()
		if content == nil {
			return
		}

		msg := IncomingMessage{
			RoomID:     string(evt.RoomID),
			RoomName:   p.getRoomDisplayName(ctx, evt.RoomID),
			Sender:     string(evt.Sender),
			SenderName: string(evt.Sender),
			Text:       content.Body,
			Timestamp:  time.UnixMilli(evt.Timestamp).UTC().Format(time.RFC3339),
			EventID:    string(evt.ID),
		}

		if err := enc.Encode(msg); err != nil {
			fmt.Fprintf(os.Stderr, "error writing message: %v\n", err)
		}
	}

	// The crypto helper (client.Crypto) automatically decrypts encrypted
	// events and re-dispatches them as EventMessage, so we only need this handler.
	syncer.OnEventType(event.EventMessage, handleMessage)

	p.client.SyncPresence = event.PresenceOffline

	fmt.Fprintln(os.Stderr, "Listening for messages...")
	defer p.Close()
	return p.client.SyncWithContext(ctx)
}

func (p *MatrixProvider) Close() error {
	if p.cryptoHelper != nil {
		return p.cryptoHelper.Close()
	}
	return nil
}

func (p *MatrixProvider) Send(ctx context.Context, roomID string, text string) error {
	// Do an initial sync so the crypto helper learns room encryption state
	// and other users' device keys — required for encrypting outgoing messages.
	resp, err := p.client.SyncRequest(ctx, 0, "", "", true, event.PresenceOffline)
	if err != nil {
		return fmt.Errorf("initial sync failed: %w", err)
	}
	syncer := p.client.Syncer.(*mautrix.DefaultSyncer)
	if err := syncer.ProcessResponse(ctx, resp, ""); err != nil {
		return fmt.Errorf("failed to process sync response: %w", err)
	}
	// Best-effort save of sync token; may fail for read-only stores.
	_ = p.client.Store.SaveNextBatch(ctx, p.userID, resp.NextBatch)

	_, err = p.client.SendText(ctx, id.RoomID(roomID), text)
	return err
}

func (p *MatrixProvider) getRoomDisplayName(ctx context.Context, roomID id.RoomID) string {
	var nameContent event.RoomNameEventContent
	err := p.client.StateEvent(ctx, roomID, event.StateRoomName, "", &nameContent)
	if err == nil && nameContent.Name != "" {
		return nameContent.Name
	}

	var aliasContent event.CanonicalAliasEventContent
	err = p.client.StateEvent(ctx, roomID, event.StateCanonicalAlias, "", &aliasContent)
	if err == nil && aliasContent.Alias != "" {
		return string(aliasContent.Alias)
	}

	return string(roomID)
}
