<!-- PROJECT LOGO -->
<br />
<div align="center">
<h3 align="center">Messages</h3>

  <p align="center">
    a CLI for managing and querying your messages across providers
  </p>
</div>

# About

Messages is a CLI tool for syncing, querying, and sending messages across multiple messaging providers. It stores messages locally in SQLite for fast offline access.

Currently supported providers:
- **Beeper** — sync via Beeper Desktop API
- **Matrix** — connect directly to any Matrix homeserver via access token

## Project Structure

```
messages/
├── cmd/messages/          # CLI entrypoint
├── internal/messages/     # Core library (providers, config, db)
├── go.mod
└── README.md
```

## Install

```bash
go install github.com/arjungandhi/messages/cmd/messages@latest
```

## Usage

```bash
# Add an account
messages account add myaccount

# Sync messages
messages sync -a myaccount

# List conversations
messages list

# Get messages for a conversation
messages get <conversation-id>

# Send a message
messages send <conversation-id> "hello"
```

## Build

```bash
go build ./...
go test ./...
```
