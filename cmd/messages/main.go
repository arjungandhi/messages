package main

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"text/tabwriter"

	"github.com/arjungandhi/messages/pkg/config"
	"github.com/arjungandhi/messages/pkg/messages"
	"github.com/charmbracelet/huh"
	"github.com/spf13/cobra"
)

var accountFlag string
var verboseFlag bool
var outputFlag string

var rootCmd = &cobra.Command{
	Use:   "messages",
	Short: "unix-style matrix client",
	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		level := slog.LevelWarn
		if verboseFlag {
			level = slog.LevelDebug
		}
		slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
			Level: level,
		})))
	},
}

// --- account commands ---

var accountCmd = &cobra.Command{
	Use:   "account",
	Short: "manage accounts",
}

var accountAddCmd = &cobra.Command{
	Use:   "add <name>",
	Short: "add a new matrix account",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		name := args[0]
		cfg := config.New()
		if err := cfg.EnsureDir(); err != nil {
			return err
		}
		if err := cfg.Load(); err != nil {
			return err
		}

		if _, ok := cfg.Accounts[name]; ok {
			return fmt.Errorf("account %q already exists", name)
		}

		var homeserverURL, userID, accessToken string
		form := huh.NewForm(
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

		acctDir := cfg.AccountDir(name)
		if err := os.MkdirAll(acctDir, 0755); err != nil {
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

		cfg.Accounts[name] = config.AccountConfig{
			Provider: "matrix",
		}
		if cfg.Default == "" {
			cfg.Default = name
		}
		if err := cfg.Save(); err != nil {
			return err
		}
		fmt.Fprintf(os.Stderr, "Account %q added.\n", name)
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
		cfg := config.New()
		if err := cfg.Load(); err != nil {
			return err
		}
		if len(cfg.Accounts) == 0 {
			fmt.Fprintln(os.Stderr, "No accounts configured. Run 'messages account add' to add one.")
			return nil
		}
		w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
		fmt.Fprintln(w, "NAME\tPROVIDER\tDEFAULT")
		for name, acct := range cfg.Accounts {
			def := ""
			if name == cfg.Default {
				def = "*"
			}
			fmt.Fprintf(w, "%s\t%s\t%s\n", name, acct.Provider, def)
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
		cfg := config.New()
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
			for n := range cfg.Accounts {
				cfg.Default = n
				break
			}
		}
		if err := cfg.Save(); err != nil {
			return err
		}
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
		cfg := config.New()
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

// --- list commands ---

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "list resources",
}

var listRoomsCmd = &cobra.Command{
	Use:   "rooms",
	Short: "list joined rooms",
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := messages.New(nil, accountFlag)
		if err != nil {
			return err
		}
		defer client.Close()
		rooms, err := client.ListRooms(context.Background())
		if err != nil {
			return err
		}

		switch outputFlag {
		case "json":
			enc := json.NewEncoder(os.Stdout)
			for _, r := range rooms {
				if err := enc.Encode(r); err != nil {
					return err
				}
			}
		default:
			w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
			fmt.Fprintln(w, "ID\tNAME")
			for _, r := range rooms {
				fmt.Fprintf(w, "%s\t%s\n", r.ID, r.Name)
			}
			w.Flush()
		}
		return nil
	},
}

// --- listen command ---

var listenCmd = &cobra.Command{
	Use:   "listen",
	Short: "listen for messages, output JSON lines to stdout",
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := messages.New(nil, accountFlag)
		if err != nil {
			return err
		}
		defer client.Close()

		ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
		defer cancel()

		ch, err := client.Listen(ctx)
		if err != nil {
			return err
		}

		fmt.Fprintln(os.Stderr, "Listening for messages...")
		enc := json.NewEncoder(os.Stdout)
		for msg := range ch {
			if err := enc.Encode(msg); err != nil {
				fmt.Fprintf(os.Stderr, "error writing message: %v\n", err)
			}
		}
		return nil
	},
}

// --- send command ---

var sendCmd = &cobra.Command{
	Use:   "send [target] [message]",
	Short: "send a message to a room (!room_id) or user (@user:server) via args or JSON lines on stdin",
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := messages.New(nil, accountFlag)
		if err != nil {
			return err
		}
		defer client.Close()
		ctx := context.Background()

		// Args mode: messages send <target> <message>
		// target can be a room ID (!...) or a user ID (@...)
		if len(args) >= 2 {
			target := args[0]
			text := strings.Join(args[1:], " ")
			roomID, err := resolveTarget(ctx, client, target)
			if err != nil {
				return err
			}
			slog.Debug("sending message via args", "room_id", roomID, "text", text)
			if err := client.Send(ctx, roomID, text); err != nil {
				return err
			}
			fmt.Fprintln(os.Stderr, "Message sent.")
			return nil
		}

		// Stdin mode: read JSON lines
		slog.Debug("reading messages from stdin")
		scanner := bufio.NewScanner(os.Stdin)
		for scanner.Scan() {
			line := scanner.Text()
			if strings.TrimSpace(line) == "" {
				continue
			}
			var msg messages.OutgoingMessage
			if err := json.Unmarshal([]byte(line), &msg); err != nil {
				fmt.Fprintf(os.Stderr, "invalid JSON line: %v\n", err)
				continue
			}
			if msg.Text == "" {
				fmt.Fprintln(os.Stderr, "skipping message: text is required")
				continue
			}
			// Resolve target: use room_id if set, otherwise resolve user_id
			target := msg.RoomID
			if target == "" {
				target = msg.UserID
			}
			if target == "" {
				fmt.Fprintln(os.Stderr, "skipping message: room_id or user_id is required")
				continue
			}
			roomID, err := resolveTarget(ctx, client, target)
			if err != nil {
				fmt.Fprintf(os.Stderr, "resolve error: %v\n", err)
				continue
			}
			slog.Debug("sending message via stdin", "room_id", roomID, "text", msg.Text)
			if err := client.Send(ctx, roomID, msg.Text); err != nil {
				fmt.Fprintf(os.Stderr, "send error: %v\n", err)
				continue
			}
		}
		return scanner.Err()
	},
}

// --- helpers ---

// resolveTarget converts a target (room ID or user ID) to a room ID.
// User IDs (starting with @) are resolved to DM rooms.
func resolveTarget(ctx context.Context, client *messages.Client, target string) (string, error) {
	if strings.HasPrefix(target, "@") {
		slog.Debug("resolving user ID to DM room", "user_id", target)
		roomID, err := client.FindOrCreateDM(ctx, target)
		if err != nil {
			return "", fmt.Errorf("failed to resolve user %s: %w", target, err)
		}
		return roomID, nil
	}
	return target, nil
}

func init() {
	rootCmd.PersistentFlags().StringVarP(&accountFlag, "account", "a", "", "account to use (default: from config)")
	rootCmd.PersistentFlags().BoolVarP(&verboseFlag, "verbose", "v", false, "enable debug logging")

	listRoomsCmd.Flags().StringVarP(&outputFlag, "output", "o", "table", "output format (table, json)")
	listCmd.AddCommand(listRoomsCmd)

	accountCmd.AddCommand(accountAddCmd, accountListCmd, accountRemoveCmd, accountDefaultCmd)
	rootCmd.AddCommand(accountCmd, listCmd, listenCmd, sendCmd)
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}
