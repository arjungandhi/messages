package messages

import (
	"testing"
	"time"
)

type mockProvider struct {
	conversations []Conversation
	messages      []Message
}

func (m *mockProvider) Sync() ([]Conversation, []Message, error) {
	return m.conversations, m.messages, nil
}

func TestMessageManager_Sync(t *testing.T) {
	dir := t.TempDir()
	provider := &mockProvider{
		conversations: []Conversation{
			{
				ID: "conv-1", AccountID: "acc-1", Platform: "whatsapp",
				Title: "Test Chat", Type: "single",
				ParticipantUIDs: []string{"u1"}, ParticipantCount: 1,
				LastActivity: time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC),
			},
		},
		messages: []Message{
			{
				ID: "msg-1", ContactUID: "c1",
				Timestamp:       time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC),
				SenderUID: "u1", SenderName: "Alice",
				ConversationUID: "conv-1", ChatTitle: "Test Chat",
				Text: "Hello", Platform: "whatsapp", PlatformID: "msg-1",
				SortKey: "1",
			},
		},
	}

	mm, err := NewMessageManager(provider, dir)
	if err != nil {
		t.Fatal(err)
	}
	defer mm.Close()

	if err := mm.Sync(); err != nil {
		t.Fatal(err)
	}

	convs, err := mm.ListAllConversations()
	if err != nil {
		t.Fatal(err)
	}
	if len(convs) != 1 {
		t.Fatalf("expected 1 conversation, got %d", len(convs))
	}
	if convs[0].Title != "Test Chat" {
		t.Errorf("title: got %q, want %q", convs[0].Title, "Test Chat")
	}

	msgs, err := mm.GetMessagesForConversation("conv-1")
	if err != nil {
		t.Fatal(err)
	}
	if len(msgs) != 1 {
		t.Fatalf("expected 1 message, got %d", len(msgs))
	}
	if msgs[0].Text != "Hello" {
		t.Errorf("text: got %q, want %q", msgs[0].Text, "Hello")
	}
}

func TestMessageManager_Queries(t *testing.T) {
	dir := t.TempDir()
	provider := &mockProvider{
		conversations: []Conversation{
			{
				ID: "c1", AccountID: "a1", Platform: "wa",
				Title: "Chat 1", Type: "single",
				ParticipantUIDs: []string{"u1", "u2"}, ParticipantCount: 2,
				LastActivity: time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC),
			},
			{
				ID: "c2", AccountID: "a1", Platform: "tg",
				Title: "Chat 2", Type: "group",
				ParticipantUIDs: []string{"u1", "u3"}, ParticipantCount: 2,
				LastActivity: time.Date(2025, 1, 14, 10, 0, 0, 0, time.UTC),
			},
		},
		messages: []Message{
			{
				ID: "m1", ContactUID: "u1",
				Timestamp: time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC),
				SenderUID: "u1", SenderName: "Alice",
				ConversationUID: "c1", ChatTitle: "Chat 1",
				Text: "msg1", Platform: "wa", PlatformID: "m1", SortKey: "1",
			},
			{
				ID: "m2", ContactUID: "u2",
				Timestamp: time.Date(2025, 1, 15, 11, 0, 0, 0, time.UTC),
				SenderUID: "u2", SenderName: "Bob",
				ConversationUID: "c1", ChatTitle: "Chat 1",
				Text: "msg2", Platform: "wa", PlatformID: "m2", SortKey: "2",
			},
		},
	}

	mm, err := NewMessageManager(provider, dir)
	if err != nil {
		t.Fatal(err)
	}
	defer mm.Close()
	mm.Sync()

	// ListAll
	convs, err := mm.ListAllConversations()
	if err != nil {
		t.Fatal(err)
	}
	if len(convs) != 2 {
		t.Fatalf("expected 2 conversations, got %d", len(convs))
	}

	// GetConversation
	conv, err := mm.GetConversation("c1")
	if err != nil {
		t.Fatal(err)
	}
	if conv == nil || conv.Title != "Chat 1" {
		t.Error("GetConversation failed")
	}

	// GetMessagesForContact
	msgs, err := mm.GetMessagesForContact("u1")
	if err != nil {
		t.Fatal(err)
	}
	if len(msgs) != 1 {
		t.Fatalf("expected 1 message for u1, got %d", len(msgs))
	}

	// GetLastContactDate
	date, err := mm.GetLastContactDate("u1")
	if err != nil {
		t.Fatal(err)
	}
	if date == nil {
		t.Fatal("expected date")
	}

	// GetConversationsForContact
	convsByContact, err := mm.GetConversationsForContact("u1")
	if err != nil {
		t.Fatal(err)
	}
	if len(convsByContact) != 2 {
		t.Fatalf("expected 2 conversations for u1, got %d", len(convsByContact))
	}
}
