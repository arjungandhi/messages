package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/arjungandhi/messages"
	"github.com/charmbracelet/huh"
	Z "github.com/rwxrob/bonzai/z"
	"github.com/rwxrob/help"
)

var Cmd = &Z.Cmd{
	Name:     "messages",
	Summary:  "manage your messages",
	Commands: []*Z.Cmd{help.Cmd, initCmd, syncCmd, listCmd, getCmd, searchCmd},
}

var initCmd = &Z.Cmd{
	Name:    "init",
	Summary: "initialize beeper provider",
	Call: func(x *Z.Cmd, args ...string) error {
		cfg := messages.NewConfig()
		if err := cfg.EnsureDir(); err != nil {
			return err
		}

		provider, err := messages.NewBeeperProvider(cfg.Dir)
		if err != nil {
			return err
		}

		existingCreds, _ := provider.LoadCredentials()
		if existingCreds != nil && existingCreds.AccessToken != "" {
			var replace bool
			form := huh.NewForm(huh.NewGroup(
				huh.NewConfirm().
					Title("Existing credentials found").
					Description("Delete and enter new credentials?").
					Affirmative("Yes").
					Negative("No, keep existing").
					Value(&replace),
			))
			if err := form.Run(); err != nil {
				return err
			}
			if !replace {
				fmt.Fprintln(os.Stderr, "Keeping existing credentials.")
				return nil
			}
		}

		var accessToken string
		form := huh.NewForm(
			huh.NewGroup(
				huh.NewNote().
					Title("Beeper Setup").
					Description("Enter your Beeper access token.\nYou can find this in Beeper Desktop settings."),
			),
			huh.NewGroup(
				huh.NewInput().Title("Access Token").Value(&accessToken).Password(true).
					Validate(func(s string) error {
						if strings.TrimSpace(s) == "" {
							return fmt.Errorf("required")
						}
						return nil
					}),
			),
		)
		if err := form.Run(); err != nil {
			return err
		}

		creds := &messages.BeeperCredentials{
			AccessToken: strings.TrimSpace(accessToken),
		}
		if err := provider.SaveCredentials(creds); err != nil {
			return err
		}
		fmt.Fprintln(os.Stderr, "Beeper credentials saved. Run 'messages sync' to sync.")
		return nil
	},
}

var syncCmd = &Z.Cmd{
	Name:    "sync",
	Summary: "sync messages from beeper",
	Call: func(x *Z.Cmd, args ...string) error {
		mm, err := getManager()
		if err != nil {
			return err
		}
		defer mm.Close()
		if err := mm.Sync(); err != nil {
			return err
		}
		convs, err := mm.ListAllConversations()
		if err != nil {
			return err
		}
		fmt.Fprintf(os.Stderr, "Sync complete. %d conversations.\n", len(convs))
		return nil
	},
}

var listCmd = &Z.Cmd{
	Name:    "list",
	Summary: "list all conversations (pipe-friendly)",
	Call: func(x *Z.Cmd, args ...string) error {
		mm, err := getManager()
		if err != nil {
			return err
		}
		defer mm.Close()
		convs, err := mm.ListAllConversations()
		if err != nil {
			return err
		}
		for _, c := range convs {
			fmt.Printf("%s|%s|%s|%d|%d|%s\n",
				c.ID, c.Title, c.Platform,
				c.ParticipantCount, c.UnreadCount,
				c.LastActivity.Format("2006-01-02T15:04:05Z"),
			)
		}
		return nil
	},
}

var getCmd = &Z.Cmd{
	Name:    "get",
	Summary: "get messages for a conversation (outputs JSON)",
	Usage:   "<conversation-id>",
	NumArgs: 1,
	Call: func(x *Z.Cmd, args ...string) error {
		mm, err := getManager()
		if err != nil {
			return err
		}
		defer mm.Close()
		conv, err := mm.GetConversation(args[0])
		if err != nil {
			return err
		}
		if conv == nil {
			return fmt.Errorf("conversation not found: %s", args[0])
		}
		msgs, err := mm.GetMessagesForConversation(args[0])
		if err != nil {
			return err
		}
		result := struct {
			Conversation messages.Conversation `json:"conversation"`
			Messages     []messages.Message    `json:"messages"`
		}{
			Conversation: *conv,
			Messages:     msgs,
		}
		data, err := json.MarshalIndent(result, "", "  ")
		if err != nil {
			return err
		}
		fmt.Println(string(data))
		return nil
	},
}

var searchCmd = &Z.Cmd{
	Name:    "search",
	Summary: "get messages for a contact UID",
	Usage:   "<contact-uid>",
	NumArgs: 1,
	Call: func(x *Z.Cmd, args ...string) error {
		mm, err := getManager()
		if err != nil {
			return err
		}
		defer mm.Close()
		msgs, err := mm.GetMessagesForContact(args[0])
		if err != nil {
			return err
		}
		data, err := json.MarshalIndent(msgs, "", "  ")
		if err != nil {
			return err
		}
		fmt.Println(string(data))
		return nil
	},
}

func getManager() (*messages.MessageManager, error) {
	cfg := messages.NewConfig()
	if err := cfg.EnsureDir(); err != nil {
		return nil, err
	}
	provider, err := messages.NewBeeperProvider(cfg.Dir)
	if err != nil {
		return nil, err
	}
	if err := provider.Initialize(); err != nil {
		return nil, fmt.Errorf("%w. Run 'messages init' first", err)
	}
	return messages.NewMessageManager(provider, cfg.Dir)
}

func main() {
	Cmd.Run()
}
