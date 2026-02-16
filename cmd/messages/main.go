package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"text/tabwriter"

	"github.com/arjungandhi/messages"
	"github.com/charmbracelet/huh"
	"github.com/rwxrob/bonzai"
	"github.com/rwxrob/bonzai/cmds/help"
	"github.com/rwxrob/bonzai/comp"
)

var Cmd = &bonzai.Cmd{
	Name:  "messages",
	Short: "manage your messages",
	Cmds:  []*bonzai.Cmd{help.Cmd, initCmd, syncCmd, listCmd, getCmd, searchCmd},
	Comp:  comp.CmdsOpts,
}

var initCmd = &bonzai.Cmd{
	Name:  "init",
	Short: "initialize beeper provider",
	Do: func(x *bonzai.Cmd, args ...string) error {
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

var syncCmd = &bonzai.Cmd{
	Name:  "sync",
	Short: "sync messages from beeper",
	Do: func(x *bonzai.Cmd, args ...string) error {
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

var listCmd = &bonzai.Cmd{
	Name:  "list",
	Short: "list all conversations (-o table|json)",
	Usage: "[-o format]",
	Opts:  "table|json",
	Do: func(x *bonzai.Cmd, args ...string) error {
		format, _, err := parseOutputFlag(args)
		if err != nil {
			return err
		}
		mm, err := getManager()
		if err != nil {
			return err
		}
		defer mm.Close()
		convs, err := mm.ListAllConversations()
		if err != nil {
			return err
		}
		switch format {
		case "json":
			data, err := json.MarshalIndent(convs, "", "  ")
			if err != nil {
				return err
			}
			fmt.Println(string(data))
		default: // table
			w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
			fmt.Fprintln(w, "ID\tTITLE\tPLATFORM\tPARTICIPANTS\tUNREAD\tLAST ACTIVITY")
			for _, c := range convs {
				fmt.Fprintf(w, "%s\t%s\t%s\t%d\t%d\t%s\n",
					c.ID, c.Title, c.Platform,
					c.ParticipantCount, c.UnreadCount,
					c.LastActivity.Format("2006-01-02T15:04:05Z"),
				)
			}
			w.Flush()
		}
		return nil
	},
}

var getCmd = &bonzai.Cmd{
	Name:  "get",
	Short: "get messages for a conversation (-o table|json)",
	Usage: "[-o format] <conversation-id>",
	Opts:  "table|json",
	Do: func(x *bonzai.Cmd, args ...string) error {
		format, rest, err := parseOutputFlag(args)
		if err != nil {
			return err
		}
		if len(rest) != 1 {
			return fmt.Errorf("expected 1 argument, got %d", len(rest))
		}
		mm, err := getManager()
		if err != nil {
			return err
		}
		defer mm.Close()
		conv, err := mm.GetConversation(rest[0])
		if err != nil {
			return err
		}
		if conv == nil {
			return fmt.Errorf("conversation not found: %s", rest[0])
		}
		msgs, err := mm.GetMessagesForConversation(rest[0])
		if err != nil {
			return err
		}
		switch format {
		case "json":
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
		default: // table
			fmt.Fprintf(os.Stderr, "Conversation: %s (%s, %s)\n\n", conv.Title, conv.Platform, conv.ID)
			w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
			fmt.Fprintln(w, "TIMESTAMP\tSENDER\tMESSAGE")
			for _, m := range msgs {
				text := m.Text
				if len(text) > 80 {
					text = text[:77] + "..."
				}
				text = strings.ReplaceAll(text, "\n", " ")
				fmt.Fprintf(w, "%s\t%s\t%s\n",
					m.Timestamp.Format("2006-01-02 15:04"),
					m.SenderName,
					text,
				)
			}
			w.Flush()
		}
		return nil
	},
}

var searchCmd = &bonzai.Cmd{
	Name:  "search",
	Short: "get messages for a contact UID",
	Usage: "<contact-uid>",
	Do: func(x *bonzai.Cmd, args ...string) error {
		if len(args) != 1 {
			return fmt.Errorf("expected 1 argument, got %d", len(args))
		}
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

func parseOutputFlag(args []string) (string, []string, error) {
	format := "table"
	var rest []string
	for i := 0; i < len(args); i++ {
		if args[i] == "-o" {
			if i+1 >= len(args) {
				return "", nil, fmt.Errorf("-o requires a format: table or json")
			}
			format = strings.ToLower(args[i+1])
			if format != "table" && format != "json" {
				return "", nil, fmt.Errorf("unknown output format %q: use table or json", format)
			}
			i++
		} else {
			rest = append(rest, args[i])
		}
	}
	return format, rest, nil
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
	Cmd.Exec()
}
