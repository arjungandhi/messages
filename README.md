# Messages

A Unix-style Matrix client.

```bash
# Echo bot (using jq)
messages listen | jq --unbuffered -c '{room_id: .room_id, text: .text}' | messages send

# Log all messages
messages listen >> messages.log

# Send a one-off message
messages send '!room:server' 'hello'
```

## How It Works

Two commands connect Matrix to stdin/stdout:

- **`messages listen`** — long-running, outputs JSON lines to stdout for each incoming message
- **`messages send`** — reads JSON lines from stdin OR takes args, sends to Matrix

Handlers are just programs that transform JSON lines. No plugin system needed.

### Message Format

`listen` outputs one JSON object per line:
```json
{"room_id":"!abc:matrix.org","room_name":"General","sender":"@user:matrix.org","sender_name":"@user:matrix.org","text":"hello","timestamp":"2026-03-05T10:00:00Z","event_id":"$xyz"}
```

`send` accepts either:
- **Args:** `messages send <room-id> <message>`
- **Stdin (JSON lines):** `{"room_id":"!abc:matrix.org","text":"response"}`

## Install

```bash
go install github.com/arjungandhi/messages/cmd/messages@latest
```

Or with Nix:
```bash
nix build github:arjungandhi/messages
```

## Setup

```bash
# Add a Matrix account
messages account add mybot

# List accounts
messages account list

# Set default account
messages account default mybot
```

## Development

```bash
# Build
go build ./...

# Test
go test ./...

# After cloning, enable git hooks:
git config core.hooksPath .githooks
```

The pre-commit hook automatically updates `vendorHash` in `flake.nix` when `go.mod`/`go.sum` change.
