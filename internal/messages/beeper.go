package messages

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	beeperapi "github.com/beeper/desktop-api-go"
	"github.com/beeper/desktop-api-go/option"
	"github.com/beeper/desktop-api-go/packages/param"
)

type BeeperCredentials struct {
	AccessToken string `json:"access_token"`
}

type BeeperProvider struct {
	client      *beeperapi.Client
	accessToken string
	dir         string
}

func NewBeeperProvider(dir string) (*BeeperProvider, error) {
	return &BeeperProvider{dir: dir}, nil
}

func (p *BeeperProvider) SaveCredentials(creds *BeeperCredentials) error {
	if err := os.MkdirAll(p.dir, 0755); err != nil {
		return fmt.Errorf("failed to create credentials directory: %w", err)
	}
	credsPath := filepath.Join(p.dir, "beeper_credentials.json")
	data, err := json.MarshalIndent(creds, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal credentials: %w", err)
	}
	if err := os.WriteFile(credsPath, data, 0600); err != nil {
		return fmt.Errorf("failed to write credentials: %w", err)
	}
	return nil
}

func (p *BeeperProvider) LoadCredentials() (*BeeperCredentials, error) {
	credsPath := filepath.Join(p.dir, "beeper_credentials.json")
	data, err := os.ReadFile(credsPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to read credentials: %w", err)
	}
	var creds BeeperCredentials
	if err := json.Unmarshal(data, &creds); err != nil {
		return nil, fmt.Errorf("failed to unmarshal credentials: %w", err)
	}
	return &creds, nil
}

func (p *BeeperProvider) Initialize() error {
	creds, err := p.LoadCredentials()
	if err != nil {
		return fmt.Errorf("failed to load credentials: %w", err)
	}
	if creds == nil || creds.AccessToken == "" {
		return fmt.Errorf("no credentials found")
	}
	p.accessToken = creds.AccessToken
	client := beeperapi.NewClient(
		option.WithAccessToken(creds.AccessToken),
	)
	p.client = &client
	return nil
}

func (p *BeeperProvider) Sync() ([]Conversation, []Message, error) {
	ctx := context.Background()
	var conversations []Conversation
	var allMessages []Message

	fmt.Println("Fetching conversations from Beeper...")
	chatsIter := p.client.Chats.ListAutoPaging(ctx, beeperapi.ChatListParams{})
	conversationCount := 0

	for chatsIter.Next() {
		chat := chatsIter.Current()
		conversationCount++

		conv := Conversation{
			ID:               chat.ID,
			AccountID:        chat.AccountID,
			Platform:         chat.Network,
			Title:            chat.Title,
			Type:             string(chat.Type),
			ParticipantUIDs:  extractParticipantUIDs(chat.Participants.Items),
			ParticipantCount: int(chat.Participants.Total),
			UnreadCount:      chat.UnreadCount,
			LastActivity:     chat.LastActivity,
			IsArchived:       chat.IsArchived,
			IsMuted:          chat.IsMuted,
			IsPinned:         chat.IsPinned,
		}
		conversations = append(conversations, conv)

		fmt.Printf("\r\033[K[%d] Syncing: %s (%s)", conversationCount, truncateString(chat.Title, 50), chat.Network)

		messagesIter := p.client.Messages.ListAutoPaging(ctx, chat.ID, beeperapi.MessageListParams{})
		chatMessageCount := 0
		for messagesIter.Next() {
			msg := messagesIter.Current()
			chatMessageCount++

			m := Message{
				ID:              msg.ID,
				ContactUID:      msg.SenderID,
				Timestamp:       msg.Timestamp,
				SenderUID:       msg.SenderID,
				SenderName:      msg.SenderName,
				ConversationUID: msg.ChatID,
				ChatTitle:       chat.Title,
				Text:            msg.Text,
				Platform:        chat.Network,
				PlatformID:      msg.ID,
				IsSent:          msg.IsSender,
				Attachments:     convertAttachments(msg.Attachments),
				SortKey:         msg.SortKey,
			}
			allMessages = append(allMessages, m)

			if chatMessageCount%10 == 0 {
				fmt.Printf("\r\033[K[%d] Syncing: %s (%s) - %d messages", conversationCount, truncateString(chat.Title, 50), chat.Network, chatMessageCount)
			}
		}
		if messagesIter.Err() != nil {
			fmt.Println()
			return nil, nil, fmt.Errorf("failed to fetch messages for chat %s: %w", chat.ID, messagesIter.Err())
		}
	}
	if chatsIter.Err() != nil {
		fmt.Println()
		return nil, nil, fmt.Errorf("failed to fetch chats: %w", chatsIter.Err())
	}

	fmt.Printf("\n\nSynced %d conversations with %d total messages\n", len(conversations), len(allMessages))
	return conversations, allMessages, nil
}

func (p *BeeperProvider) Send(ctx context.Context, chatID string, text string) error {
	_, err := p.client.Messages.Send(ctx, chatID, beeperapi.MessageSendParams{
		Text: param.NewOpt(text),
	})
	return err
}

func extractParticipantUIDs(participants []beeperapi.User) []string {
	uids := make([]string, len(participants))
	for i, p := range participants {
		uids[i] = p.ID
	}
	return uids
}

func convertAttachments(beeperAttachments []beeperapi.Attachment) []Attachment {
	attachments := make([]Attachment, len(beeperAttachments))
	for i, a := range beeperAttachments {
		attachments[i] = Attachment{
			Type:        string(a.Type),
			SrcURL:      a.SrcURL,
			FileName:    a.FileName,
			FileSize:    a.FileSize,
			MimeType:    a.MimeType,
			Duration:    a.Duration,
			Width:       int(a.Size.Width),
			Height:      int(a.Size.Height),
			IsGif:       a.IsGif,
			IsSticker:   a.IsSticker,
			IsVoiceNote: a.IsVoiceNote,
		}
	}
	return attachments
}

func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	if maxLen <= 3 {
		return s[:maxLen]
	}
	return s[:maxLen-3] + "..."
}
