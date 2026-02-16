package messages

import (
	"context"
	"fmt"
	"path/filepath"
	"time"
)

type Attachment struct {
	Type        string  `json:"type"`
	SrcURL      string  `json:"src_url"`
	FileName    string  `json:"file_name"`
	FileSize    float64 `json:"file_size"`
	MimeType    string  `json:"mime_type"`
	Duration    float64 `json:"duration"`
	Width       int     `json:"width"`
	Height      int     `json:"height"`
	IsGif       bool    `json:"is_gif"`
	IsSticker   bool    `json:"is_sticker"`
	IsVoiceNote bool    `json:"is_voice_note"`
}

type Conversation struct {
	ID        string `json:"id"`
	AccountID string `json:"account_id"`
	Platform  string `json:"platform"`

	Title string `json:"title"`
	Type  string `json:"type"`

	ParticipantUIDs  []string `json:"participant_uids"`
	ParticipantCount int      `json:"participant_count"`

	UnreadCount  int64     `json:"unread_count"`
	LastActivity time.Time `json:"last_activity"`

	IsArchived bool `json:"is_archived"`
	IsMuted    bool `json:"is_muted"`
	IsPinned   bool `json:"is_pinned"`
}

type Message struct {
	ID string `json:"id"`

	ContactUID      string    `json:"contact_uid"`
	Timestamp       time.Time `json:"timestamp"`
	SenderUID       string    `json:"sender_uid"`
	SenderName      string    `json:"sender_name"`
	ConversationUID string    `json:"conversation_uid"`
	ChatTitle       string    `json:"chat_title"`
	Text            string    `json:"content"`
	Platform        string    `json:"platform"`
	PlatformID      string    `json:"platform_id"`

	IsSent      bool         `json:"is_sent"`
	Attachments []Attachment `json:"attachments"`
	SortKey     string       `json:"sort_key"`
}

type MessageProvider interface {
	Initialize() error
	Sync() ([]Conversation, []Message, error)
	Send(ctx context.Context, chatID string, text string) error
}

type MessageManager struct {
	provider MessageProvider
	account  AccountConfig
	db       *DB
}

func NewMessageManager(provider MessageProvider, acct AccountConfig, dbDir string) (*MessageManager, error) {
	dbPath := filepath.Join(dbDir, "messages.db")
	db, err := OpenDB(dbPath)
	if err != nil {
		return nil, err
	}
	return &MessageManager{
		provider: provider,
		account:  acct,
		db:       db,
	}, nil
}

func (mm *MessageManager) Close() error {
	return mm.db.Close()
}

func (mm *MessageManager) Sync() error {
	if !mm.account.Read {
		return fmt.Errorf("account does not have read permission")
	}
	conversations, messages, err := mm.provider.Sync()
	if err != nil {
		return err
	}
	if err := mm.db.SaveConversations(conversations); err != nil {
		return err
	}
	if err := mm.db.SaveMessages(messages); err != nil {
		return err
	}
	return nil
}

func (mm *MessageManager) Send(ctx context.Context, chatID string, text string) error {
	if !mm.account.Write {
		return fmt.Errorf("account does not have write permission")
	}
	return mm.provider.Send(ctx, chatID, text)
}

func (mm *MessageManager) GetMessagesForContact(contactUID string) ([]Message, error) {
	return mm.db.GetMessagesForContact(contactUID)
}

func (mm *MessageManager) GetLastContactDate(contactUID string) (*time.Time, error) {
	return mm.db.GetLastContactDate(contactUID)
}

func (mm *MessageManager) GetConversation(conversationUID string) (*Conversation, error) {
	return mm.db.GetConversation(conversationUID)
}

func (mm *MessageManager) GetConversationsForContact(contactUID string) ([]Conversation, error) {
	return mm.db.GetConversationsForContact(contactUID)
}

func (mm *MessageManager) ListAllConversations() ([]Conversation, error) {
	return mm.db.ListAllConversations()
}

func (mm *MessageManager) GetMessagesForConversation(conversationUID string) ([]Message, error) {
	return mm.db.GetMessagesForConversation(conversationUID)
}
