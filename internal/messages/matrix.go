package messages

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"maunium.net/go/mautrix"
	"maunium.net/go/mautrix/event"
	"maunium.net/go/mautrix/id"
)

type MatrixCredentials struct {
	HomeserverURL string `json:"homeserver_url"`
	UserID        string `json:"user_id"`
	AccessToken   string `json:"access_token"`
}

type MatrixProvider struct {
	client *mautrix.Client
	userID id.UserID
	dir    string
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
	p.client = client
	return nil
}

func (p *MatrixProvider) Sync() ([]Conversation, []Message, error) {
	ctx := context.Background()
	var conversations []Conversation
	var allMessages []Message

	fmt.Println("Fetching rooms from Matrix...")
	joinedResp, err := p.client.JoinedRooms(ctx)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get joined rooms: %w", err)
	}

	for i, roomID := range joinedResp.JoinedRooms {
		displayName := p.getRoomDisplayName(ctx, roomID)
		fmt.Printf("\r\033[K[%d/%d] Syncing: %s", i+1, len(joinedResp.JoinedRooms), truncateString(displayName, 50))

		// Get members
		membersResp, err := p.client.JoinedMembers(ctx, roomID)
		if err != nil {
			fmt.Printf("\n  Warning: failed to get members for %s: %v\n", roomID, err)
			continue
		}

		participantUIDs := make([]string, 0, len(membersResp.Joined))
		memberNames := make(map[id.UserID]string, len(membersResp.Joined))
		for uid, member := range membersResp.Joined {
			participantUIDs = append(participantUIDs, string(uid))
			if member.DisplayName != "" {
				memberNames[uid] = member.DisplayName
			} else {
				memberNames[uid] = string(uid)
			}
		}

		roomType := "group"
		if len(membersResp.Joined) <= 2 {
			roomType = "single"
		}

		conv := Conversation{
			ID:               string(roomID),
			Platform:         "matrix",
			Title:            displayName,
			Type:             roomType,
			ParticipantUIDs:  participantUIDs,
			ParticipantCount: len(membersResp.Joined),
		}
		conversations = append(conversations, conv)

		// Get messages
		var from string
		chatMessageCount := 0
		for {
			resp, err := p.client.Messages(ctx, roomID, from, "", mautrix.DirectionBackward, nil, 100)
			if err != nil {
				fmt.Printf("\n  Warning: failed to get messages for %s: %v\n", roomID, err)
				break
			}
			if len(resp.Chunk) == 0 {
				break
			}
			for _, evt := range resp.Chunk {
				if evt.Type != event.EventMessage {
					continue
				}
				content := evt.Content.AsMessage()
				if content == nil {
					continue
				}

				senderName := evt.Sender.String()
				if name, ok := memberNames[evt.Sender]; ok {
					senderName = name
				}

				m := Message{
					ID:              evt.ID.String(),
					ContactUID:      evt.Sender.String(),
					Timestamp:       time.UnixMilli(evt.Timestamp),
					SenderUID:       evt.Sender.String(),
					SenderName:      senderName,
					ConversationUID: string(roomID),
					ChatTitle:       displayName,
					Text:            content.Body,
					Platform:        "matrix",
					PlatformID:      evt.ID.String(),
					IsSent:          evt.Sender == p.userID,
					SortKey:         fmt.Sprintf("%d", evt.Timestamp),
				}

				// Handle media attachments
				if content.MsgType != event.MsgText && content.MsgType != event.MsgNotice && content.MsgType != event.MsgEmote {
					att := Attachment{
						Type:     string(content.MsgType),
						SrcURL:   string(content.URL),
						FileName: content.Body,
					}
					if content.Info != nil {
						att.MimeType = content.Info.MimeType
						att.FileSize = float64(content.Info.Size)
						att.Width = content.Info.Width
						att.Height = content.Info.Height
						att.Duration = float64(content.Info.Duration) / 1000.0
					}
					m.Attachments = []Attachment{att}
				}

				allMessages = append(allMessages, m)
				chatMessageCount++
			}

			if chatMessageCount%10 == 0 {
				fmt.Printf("\r\033[K[%d/%d] Syncing: %s - %d messages", i+1, len(joinedResp.JoinedRooms), truncateString(displayName, 50), chatMessageCount)
			}

			from = resp.End
			if from == "" {
				break
			}
		}
	}

	fmt.Printf("\n\nSynced %d rooms with %d total messages\n", len(conversations), len(allMessages))
	return conversations, allMessages, nil
}

func (p *MatrixProvider) Send(ctx context.Context, chatID string, text string) error {
	_, err := p.client.SendText(ctx, id.RoomID(chatID), text)
	return err
}

func (p *MatrixProvider) getRoomDisplayName(ctx context.Context, roomID id.RoomID) string {
	// Try room name state event
	var nameContent event.RoomNameEventContent
	err := p.client.StateEvent(ctx, roomID, event.StateRoomName, "", &nameContent)
	if err == nil && nameContent.Name != "" {
		return nameContent.Name
	}

	// Try canonical alias
	var aliasContent event.CanonicalAliasEventContent
	err = p.client.StateEvent(ctx, roomID, event.StateCanonicalAlias, "", &aliasContent)
	if err == nil && aliasContent.Alias != "" {
		return string(aliasContent.Alias)
	}

	// Fall back to room ID
	return string(roomID)
}
