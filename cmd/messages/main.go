package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"text/tabwriter"

	"github.com/arjungandhi/messages/internal/messages"
	"github.com/charmbracelet/huh"
	"github.com/spf13/cobra"
)

var accountFlag string

var rootCmd = &cobra.Command{
	Use:   "messages",
	Short: "manage your messages",
}

// --- account commands ---

var accountCmd = &cobra.Command{
	Use:   "account",
	Short: "manage accounts",
}

var accountAddCmd = &cobra.Command{
	Use:   "add <name>",
	Short: "add a new account",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		name := args[0]
		cfg := messages.NewConfig()
		if err := cfg.EnsureDir(); err != nil {
			return err
		}
		if err := cfg.Load(); err != nil {
			return err
		}

		if _, ok := cfg.Accounts[name]; ok {
			return fmt.Errorf("account %q already exists", name)
		}

		// pick provider
		var provider string
		form := huh.NewForm(huh.NewGroup(
			huh.NewSelect[string]().
				Title("Provider").
				Options(
					huh.NewOption("Beeper", "beeper"),
					huh.NewOption("Matrix", "matrix"),
				).
				Value(&provider),
		))
		if err := form.Run(); err != nil {
			return err
		}

		// pick permissions
		var read, write bool
		form = huh.NewForm(huh.NewGroup(
			huh.NewConfirm().Title("Enable read (sync & query)?").Value(&read),
			huh.NewConfirm().Title("Enable write (send messages)?").Value(&write),
		))
		if err := form.Run(); err != nil {
			return err
		}

		acctDir := cfg.AccountDir(name)
		if err := os.MkdirAll(acctDir, 0755); err != nil {
			return err
		}

		// provider-specific credential setup
		switch provider {
		case "beeper":
			var accessToken string
			form = huh.NewForm(
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
			p, err := messages.NewBeeperProvider(acctDir)
			if err != nil {
				return err
			}
			if err := p.SaveCredentials(&messages.BeeperCredentials{
				AccessToken: strings.TrimSpace(accessToken),
			}); err != nil {
				return err
			}
		case "matrix":
			var homeserverURL, userID, accessToken string
			form = huh.NewForm(
				huh.NewGroup(
					huh.NewNote().
						Title("Matrix Setup").
						Description("Enter your Matrix homeserver details and access token."),
				),
				huh.NewGroup(
					huh.NewInput().Title("Homeserver URL").Value(&homeserverURL).
						Placeholder("https://matrix.example.com").
						Validate(func(s string) error {
							if strings.TrimSpace(s) == "" {
								return fmt.Errorf("required")
							}
							return nil
						}),
					huh.NewInput().Title("User ID").Value(&userID).
						Placeholder("@user:example.com").
						Validate(func(s string) error {
							if strings.TrimSpace(s) == "" {
								return fmt.Errorf("required")
							}
							return nil
						}),
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
			p, err := messages.NewMatrixProvider(acctDir)
			if err != nil {
				return err
			}
			if err := p.SaveCredentials(&messages.MatrixCredentials{
				HomeserverURL: strings.TrimSpace(homeserverURL),
				UserID:        strings.TrimSpace(userID),
				AccessToken:   strings.TrimSpace(accessToken),
			}); err != nil {
				return err
			}
		}

		cfg.Accounts[name] = messages.AccountConfig{
			Provider: provider,
			Read:     read,
			Write:    write,
		}
		if cfg.Default == "" {
			cfg.Default = name
		}
		if err := cfg.Save(); err != nil {
			return err
		}
		fmt.Fprintf(os.Stderr, "Account %q added (provider: %s, read: %v, write: %v)\n", name, provider, read, write)
		if cfg.Default == name {
			fmt.Fprintf(os.Stderr, "Set as default account.\n")
		}
		return nil
	},
}

var accountListCmd = &cobra.Command{
	Use:   "list",
	Short: "list all accounts",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg := messages.NewConfig()
		if err := cfg.Load(); err != nil {
			return err
		}
		if len(cfg.Accounts) == 0 {
			fmt.Fprintln(os.Stderr, "No accounts configured. Run 'messages account add' to add one.")
			return nil
		}
		w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
		fmt.Fprintln(w, "NAME\tPROVIDER\tREAD\tWRITE\tDEFAULT")
		for name, acct := range cfg.Accounts {
			def := ""
			if name == cfg.Default {
				def = "*"
			}
			fmt.Fprintf(w, "%s\t%s\t%v\t%v\t%s\n", name, acct.Provider, acct.Read, acct.Write, def)
		}
		w.Flush()
		return nil
	},
}

var accountRemoveCmd = &cobra.Command{
	Use:   "remove <name>",
	Short: "remove an account",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		name := args[0]
		cfg := messages.NewConfig()
		if err := cfg.Load(); err != nil {
			return err
		}
		if _, ok := cfg.Accounts[name]; !ok {
			return fmt.Errorf("account %q not found", name)
		}

		var confirm bool
		form := huh.NewForm(huh.NewGroup(
			huh.NewConfirm().
				Title(fmt.Sprintf("Remove account %q?", name)).
				Description("This will delete the account config and credentials.").
				Value(&confirm),
		))
		if err := form.Run(); err != nil {
			return err
		}
		if !confirm {
			return nil
		}

		delete(cfg.Accounts, name)
		if cfg.Default == name {
			cfg.Default = ""
			// pick a new default if there are remaining accounts
			for n := range cfg.Accounts {
				cfg.Default = n
				break
			}
		}
		if err := cfg.Save(); err != nil {
			return err
		}
		// remove credentials directory
		os.RemoveAll(cfg.AccountDir(name))
		fmt.Fprintf(os.Stderr, "Account %q removed.\n", name)
		return nil
	},
}

var accountDefaultCmd = &cobra.Command{
	Use:   "default <name>",
	Short: "set the default account",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		name := args[0]
		cfg := messages.NewConfig()
		if err := cfg.Load(); err != nil {
			return err
		}
		if _, ok := cfg.Accounts[name]; !ok {
			return fmt.Errorf("account %q not found", name)
		}
		cfg.Default = name
		if err := cfg.Save(); err != nil {
			return err
		}
		fmt.Fprintf(os.Stderr, "Default account set to %q.\n", name)
		return nil
	},
}

// --- messaging commands ---

var syncCmd = &cobra.Command{
	Use:   "sync",
	Short: "sync messages",
	RunE: func(cmd *cobra.Command, args []string) error {
		mm, err := getManager(accountFlag)
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

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "list all conversations",
	RunE: func(cmd *cobra.Command, args []string) error {
		format, _ := cmd.Flags().GetString("output")
		mm, err := getManager(accountFlag)
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
		default:
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

var getCmd = &cobra.Command{
	Use:   "get <conversation-id>",
	Short: "get messages for a conversation",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		format, _ := cmd.Flags().GetString("output")
		mm, err := getManager(accountFlag)
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
		default:
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

var searchCmd = &cobra.Command{
	Use:   "search <contact-uid>",
	Short: "get messages for a contact UID",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		mm, err := getManager(accountFlag)
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

var sendCmd = &cobra.Command{
	Use:   "send <conversation-id> <message>",
	Short: "send a message to a conversation",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		mm, err := getManager(accountFlag)
		if err != nil {
			return err
		}
		defer mm.Close()
		if err := mm.Send(context.Background(), args[0], args[1]); err != nil {
			return err
		}
		fmt.Fprintln(os.Stderr, "Message sent.")
		return nil
	},
}

// --- helpers ---

func getManager(accountName string) (*messages.MessageManager, error) {
	cfg := messages.NewConfig()
	if err := cfg.Load(); err != nil {
		return nil, err
	}
	name, acct, err := cfg.GetAccount(accountName)
	if err != nil {
		return nil, fmt.Errorf("%w. Run 'messages account add' first", err)
	}
	acctDir := cfg.AccountDir(name)

	var provider messages.MessageProvider
	switch acct.Provider {
	case "beeper":
		p, err := messages.NewBeeperProvider(acctDir)
		if err != nil {
			return nil, err
		}
		provider = p
	case "matrix":
		p, err := messages.NewMatrixProvider(acctDir)
		if err != nil {
			return nil, err
		}
		provider = p
	default:
		return nil, fmt.Errorf("unknown provider %q", acct.Provider)
	}

	if err := provider.Initialize(); err != nil {
		return nil, fmt.Errorf("%w. Run 'messages account add %s' to set up credentials", err, name)
	}
	return messages.NewMessageManager(provider, acct, cfg.Dir)
}

func init() {
	rootCmd.PersistentFlags().StringVarP(&accountFlag, "account", "a", "", "account to use (default: from config)")

	listCmd.Flags().StringP("output", "o", "table", "output format: table or json")
	getCmd.Flags().StringP("output", "o", "table", "output format: table or json")

	accountCmd.AddCommand(accountAddCmd, accountListCmd, accountRemoveCmd, accountDefaultCmd)
	rootCmd.AddCommand(accountCmd, syncCmd, listCmd, getCmd, searchCmd, sendCmd)
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}
