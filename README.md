# chat-cli

A CLI-based chat app built for AI agents and humans. Multiple participants connect to shared rooms over NATS and exchange messages in real time.

## Prerequisites

| Requirement | Version | Install |
|-------------|---------|---------|
| Go | 1.21+ | https://go.dev/dl/ |
| NATS Server | any recent | `brew install nats-server` |

## Quick Start

### 1. Start a NATS server

```bash
nats-server
```

The server listens on `nats://localhost:4222` by default. You should see:

```
[INF] Server is ready
```

### 2. Build the CLI

```bash
git clone https://github.com/chenqid/agentchat.git
cd agentchat
go build -o agentchat ./cmd/agentchat
```

### 3. Join a room and start chatting

Open a terminal and run:

```bash
./agentchat join --name Alice --room general
```

Type any message and press **Enter** to send it. Messages from other participants appear in the same window in real time.

> You can join from as many terminals (or machines) as you like — everyone in the same room sees the same messages.

---

## Commands

### `join` — Interactive chat

Enter a room and chat. Sends and receives at the same time.

```bash
./agentchat join --name <your-name> --room <room>
```

Example:
```bash
./agentchat join --name Alice --room general
```

Once connected, type a message and press Enter to broadcast it. Press `Ctrl+C` to leave.

---

### `send` — Send a single message

Fire a one-off message and exit immediately. Great for scripts and automation.

```bash
./agentchat send "<message>" --name <your-name> --room <room>
```

Example:
```bash
./agentchat send "Build finished ✅" --name CI-Bot --room deploys
```

---

### `listen` — Receive only

Subscribe to a room and print messages as they arrive. Does not send anything.

```bash
./agentchat listen --name <your-name> --room <room>
```

Add `--json` to get machine-readable output:

```bash
./agentchat listen --name logger --room general --json
```

Output:
```json
{"id":"...","room":"general","from":"Alice","content":"Hello!","timestamp":"2026-04-09T..."}
```

---

## Global Flags

| Flag | Default | Description |
|------|---------|-------------|
| `--name`, `-n` | *(required)* | Your display name |
| `--room`, `-r` | `general` | Room to connect to |
| `--server`, `-s` | `nats://localhost:4222` | NATS server URL |
| `--json` | `false` | Output messages as JSON |

---

## Example: Two Terminals

**Terminal 1** — join as Alice:
```bash
./agentchat join --name Alice --room general
```

**Terminal 2** — send a message as Bob:
```bash
./agentchat send "Hey Alice!" --name Bob --room general
```

Terminal 1 immediately shows:
```
[general] Bob: Hey Alice!
```

---

## Running Tests

```bash
# All tests
go test ./...

# End-to-end tests only (starts an embedded NATS automatically)
go test ./tests/e2e -v
```

---

## Project Layout

```
cmd/agentchat/   # CLI entry point and commands (join, send, listen)
pkg/chat/        # NATS-backed chat client
pkg/message/     # Message model and JSON encode/decode
tests/e2e/       # End-to-end tests
```
