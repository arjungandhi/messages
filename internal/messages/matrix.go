package messages

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
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
	slog.Debug("creating matrix client", "homeserver", creds.HomeserverURL, "user_id", creds.UserID)
	client, err := mautrix.NewClient(creds.HomeserverURL, p.userID, creds.AccessToken)
	if err != nil {
		return fmt.Errorf("failed to create Matrix client: %w", err)
	}
	if creds.DeviceID != "" {
		client.DeviceID = id.DeviceID(creds.DeviceID)
		slog.Debug("using device ID", "device_id", creds.DeviceID)
	}
	p.client = client

	// Set up E2EE using a SQLite database for key storage
	dbPath := filepath.Join(p.dir, "crypto.db")
	slog.Debug("initializing E2EE crypto helper", "db_path", dbPath)
	helper, err := cryptohelper.NewCryptoHelper(client, []byte("messages"), dbPath)
	if err != nil {
		return fmt.Errorf("failed to create crypto helper: %w", err)
	}
	if err := helper.Init(context.Background()); err != nil {
		return fmt.Errorf("failed to init crypto helper: %w", err)
	}
	p.cryptoHelper = helper
	client.Crypto = helper

	slog.Debug("matrix provider initialized successfully")
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
		slog.Debug("received event", "type", evt.Type.Type, "sender", evt.Sender, "room_id", evt.RoomID, "event_id", evt.ID)
		if evt.Sender == p.userID {
			slog.Debug("skipping own message", "event_id", evt.ID)
			return
		}

		content := evt.Content.AsMessage()
		if content == nil {
			slog.Debug("skipping non-message event", "event_id", evt.ID)
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
	slog.Debug("preparing to send message", "room_id", roomID, "text_length", len(text))
	// Do an initial sync so the crypto helper learns room encryption state
	// and other users' device keys — required for encrypting outgoing messages.
	slog.Debug("performing initial sync for E2EE key exchange")
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

	slog.Debug("sending message", "room_id", roomID)
	_, err = p.client.SendText(ctx, id.RoomID(roomID), text)
	if err != nil {
		return err
	}
	slog.Debug("message sent successfully", "room_id", roomID)
	return nil
}

func (p *MatrixProvider) FindOrCreateDM(ctx context.Context, userID string) (string, error) {
	targetID := id.UserID(userID)

	// Check m.direct account data for existing DM rooms with this user
	var directChats map[id.UserID][]id.RoomID
	err := p.client.GetAccountData(ctx, event.AccountDataDirectChats.Type, &directChats)
	if err == nil {
		if rooms, ok := directChats[targetID]; ok {
			for _, roomID := range rooms {
				// Verify we're still in this room
				members, err := p.client.JoinedMembers(ctx, roomID)
				if err != nil {
					continue
				}
				if _, ok := members.Joined[targetID]; ok {
					slog.Debug("found existing DM room", "room_id", roomID, "user_id", userID)
					return string(roomID), nil
				}
			}
		}
	}

	// No existing DM found, create one
	slog.Debug("creating new DM room", "user_id", userID)
	createResp, err := p.client.CreateRoom(ctx, &mautrix.ReqCreateRoom{
		Invite:   []id.UserID{targetID},
		IsDirect: true,
		Preset:   "trusted_private_chat",
	})
	if err != nil {
		return "", fmt.Errorf("failed to create DM room: %w", err)
	}
	slog.Debug("created DM room", "room_id", createResp.RoomID, "user_id", userID)
	return string(createResp.RoomID), nil
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
