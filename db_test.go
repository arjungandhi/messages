package messages

import (
	"path/filepath"
	"testing"
	"time"
)

func testDB(t *testing.T) *DB {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "test.db")
	db, err := OpenDB(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}

func TestDB_OpenAndClose(t *testing.T) {
	db := testDB(t)
	if db == nil {
		t.Fatal("db is nil")
	}
}

func TestDB_SaveAndGetConversations(t *testing.T) {
	db := testDB(t)
	convs := []Conversation{
		{
			ID: "conv-1", AccountID: "acc-1", Platform: "whatsapp",
			Title: "Chat 1", Type: "single",
			ParticipantUIDs: []string{"user-1", "user-2"}, ParticipantCount: 2,
			UnreadCount: 3, LastActivity: time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC),
		},
		{
			ID: "conv-2", AccountID: "acc-1", Platform: "telegram",
			Title: "Group Chat", Type: "group",
			ParticipantUIDs: []string{"user-1", "user-3", "user-4"}, ParticipantCount: 3,
			UnreadCount: 0, LastActivity: time.Date(2025, 1, 14, 10, 0, 0, 0, time.UTC),
		},
	}
	if err := db.SaveConversations(convs); err != nil {
		t.Fatal(err)
	}

	all, err := db.ListAllConversations()
	if err != nil {
		t.Fatal(err)
	}
	if len(all) != 2 {
		t.Fatalf("expected 2, got %d", len(all))
	}
	// Sorted by last_activity DESC
	if all[0].ID != "conv-1" {
		t.Errorf("first conv should be conv-1, got %s", all[0].ID)
	}
	if all[0].Platform != "whatsapp" {
		t.Errorf("platform: got %s, want whatsapp", all[0].Platform)
	}
	if len(all[0].ParticipantUIDs) != 2 {
		t.Errorf("participants: got %d, want 2", len(all[0].ParticipantUIDs))
	}

	got, err := db.GetConversation("conv-1")
	if err != nil {
		t.Fatal(err)
	}
	if got == nil {
		t.Fatal("expected conversation, got nil")
	}
	if got.Title != "Chat 1" {
		t.Errorf("title: got %q, want %q", got.Title, "Chat 1")
	}

	got, err = db.GetConversation("nonexistent")
	if err != nil {
		t.Fatal(err)
	}
	if got != nil {
		t.Error("expected nil for nonexistent")
	}
}

func TestDB_SaveAndGetMessages(t *testing.T) {
	db := testDB(t)

	convs := []Conversation{
		{
			ID: "conv-1", AccountID: "acc-1", Platform: "whatsapp",
			Title: "Chat 1", Type: "single",
			ParticipantUIDs: []string{"user-1"}, ParticipantCount: 1,
			LastActivity: time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC),
		},
	}
	if err := db.SaveConversations(convs); err != nil {
		t.Fatal(err)
	}

	msgs := []Message{
		{
			ID: "msg-1", ContactUID: "contact-1",
			Timestamp: time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC),
			SenderUID: "user-1", SenderName: "Alice",
			ConversationUID: "conv-1", ChatTitle: "Chat 1",
			Text: "Hello", Platform: "whatsapp", PlatformID: "msg-1",
			IsSent: false, SortKey: "1",
		},
		{
			ID: "msg-2", ContactUID: "contact-1",
			Timestamp: time.Date(2025, 1, 15, 10, 5, 0, 0, time.UTC),
			SenderUID: "me", SenderName: "Me",
			ConversationUID: "conv-1", ChatTitle: "Chat 1",
			Text: "Hi!", Platform: "whatsapp", PlatformID: "msg-2",
			IsSent: true, SortKey: "2",
		},
		{
			ID: "msg-3", ContactUID: "contact-2",
			Timestamp: time.Date(2025, 1, 15, 11, 0, 0, 0, time.UTC),
			SenderUID: "user-2", SenderName: "Bob",
			ConversationUID: "conv-1", ChatTitle: "Chat 1",
			Text: "Hey", Platform: "whatsapp", PlatformID: "msg-3",
			IsSent: false, SortKey: "3",
		},
	}
	if err := db.SaveMessages(msgs); err != nil {
		t.Fatal(err)
	}

	// By conversation
	byConv, err := db.GetMessagesForConversation("conv-1")
	if err != nil {
		t.Fatal(err)
	}
	if len(byConv) != 3 {
		t.Fatalf("expected 3 messages, got %d", len(byConv))
	}

	// By contact
	byContact, err := db.GetMessagesForContact("contact-1")
	if err != nil {
		t.Fatal(err)
	}
	if len(byContact) != 2 {
		t.Fatalf("expected 2 messages for contact-1, got %d", len(byContact))
	}

	// Ignore duplicates
	if err := db.SaveMessages(msgs[:1]); err != nil {
		t.Fatal(err)
	}
	byConv, _ = db.GetMessagesForConversation("conv-1")
	if len(byConv) != 3 {
		t.Fatalf("duplicate insert changed count: got %d", len(byConv))
	}
}

func TestDB_GetLastContactDate(t *testing.T) {
	db := testDB(t)

	convs := []Conversation{
		{
			ID: "conv-1", AccountID: "acc-1", Platform: "whatsapp",
			Title: "Chat", Type: "single",
			ParticipantUIDs: []string{"u1"}, ParticipantCount: 1,
			LastActivity: time.Now(),
		},
	}
	db.SaveConversations(convs)

	msgs := []Message{
		{
			ID: "m1", ContactUID: "c1",
			Timestamp:       time.Date(2025, 1, 10, 10, 0, 0, 0, time.UTC),
			SenderUID: "u1", SenderName: "A",
			ConversationUID: "conv-1", ChatTitle: "Chat",
			Text: "old", Platform: "wa", PlatformID: "m1", SortKey: "1",
		},
		{
			ID: "m2", ContactUID: "c1",
			Timestamp:       time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC),
			SenderUID: "u1", SenderName: "A",
			ConversationUID: "conv-1", ChatTitle: "Chat",
			Text: "new", Platform: "wa", PlatformID: "m2", SortKey: "2",
		},
	}
	db.SaveMessages(msgs)

	date, err := db.GetLastContactDate("c1")
	if err != nil {
		t.Fatal(err)
	}
	if date == nil {
		t.Fatal("expected date, got nil")
	}
	want := time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC)
	if !date.Equal(want) {
		t.Errorf("got %v, want %v", date, want)
	}

	// No messages for contact
	date, err = db.GetLastContactDate("nonexistent")
	if err != nil {
		t.Fatal(err)
	}
	if date != nil {
		t.Errorf("expected nil, got %v", date)
	}
}
