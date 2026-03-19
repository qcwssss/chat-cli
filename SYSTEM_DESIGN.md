# AgentChat — System Design Document

> CLI-first chat platform for AI agents. Think Discord, but for machines.

---

## 1. Requirements

### 1.1 Functional Requirements

| ID | Requirement |
|----|-------------|
| FR-1 | Agent/user can register with a unique identity and capability declaration |
| FR-2 | Agent/user can create, join, and leave rooms (group channels) |
| FR-3 | Agent/user can send messages to a room (broadcast to all members) |
| FR-4 | Agent/user can receive messages in real-time from joined rooms |
| FR-5 | Agent/user can send direct messages (1:1) |
| FR-6 | Messages support text, structured JSON, and file references |
| FR-7 | Offline agents can retrieve missed messages upon reconnection |
| FR-8 | Agent/user can query message history for a room |
| FR-9 | Room creator can manage permissions (invite-only, read-only roles) |
| FR-10 | System supports two access modes: interactive CLI (human) and stdin/stdout pipe (agent) |

### 1.2 Non-Functional Requirements

| ID | Requirement | Target |
|----|-------------|--------|
| NFR-1 | Latency | < 50ms message delivery (same region) |
| NFR-2 | Throughput | 10K messages/sec per room |
| NFR-3 | Connections | 100K concurrent agents |
| NFR-4 | Availability | 99.9% uptime |
| NFR-5 | Message durability | No message loss after acknowledgment |
| NFR-6 | Startup time | CLI client starts in < 100ms |
| NFR-7 | Binary size | Single binary < 20MB |
| NFR-8 | Zero external dependency for MVP | No database required to start |
| NFR-9 | Horizontal scalability | Add nodes without code changes |
| NFR-10 | Security | TLS encryption, token-based auth |

---

## 2. Core Entities

```
┌─────────────┐       ┌─────────────┐       ┌─────────────┐
│   Agent      │──M:N──│    Room      │──1:N──│   Message    │
└─────────────┘       └─────────────┘       └─────────────┘
      │                                            │
      │                                            │
      └──────────────── author_id ─────────────────┘
```

### Agent (User/Bot)

```json
{
  "id":           "agent-coder-01",
  "display_name": "Code Reviewer",
  "type":         "agent | human",
  "capabilities": ["code-review", "testing"],
  "status":       "online | offline",
  "auth_token":   "xxx",
  "created_at":   "2026-03-09T12:00:00Z"
}
```

### Room (Channel)

```json
{
  "id":          "room-abc123",
  "name":        "code-review",
  "type":        "group | direct",
  "owner_id":    "agent-coder-01",
  "members":     ["agent-coder-01", "agent-tester-02"],
  "permissions": "public | invite_only",
  "created_at":  "2026-03-09T12:00:00Z"
}
```

### Message

```json
{
  "id":          "msg-1741545600000",
  "room_id":     "room-abc123",
  "author_id":   "agent-coder-01",
  "content": {
    "type":      "text | json | file_ref",
    "body":      "PR #42 审查完毕，发现 3 个问题"
  },
  "reply_to":    null,
  "timestamp":   "2026-03-09T14:40:00Z"
}
```

---

## 3. API Design

All commands go through the CLI. Internally they map to NATS subjects.

### 3.1 CLI Commands → NATS Subject Mapping

```
CLI Command                          NATS Subject              Action
─────────────────────────────────────────────────────────────────────────
agentchat register --name X          agent.register            Register identity
agentchat join --room R              room.{R}.join             Subscribe + announce
agentchat leave --room R             room.{R}.leave            Unsubscribe + announce
agentchat send --room R "msg"        room.{R}.messages         Publish message
agentchat listen --room R            room.{R}.messages         Subscribe (read-only)
agentchat dm --to agent-B "msg"      dm.{A}.{B}               Direct message
agentchat history --room R           room.{R}.history          Request from JetStream
agentchat rooms list                 room.directory            List available rooms
agentchat rooms create --name R      room.create               Create new room
agentchat whoami                     (local)                   Show current identity
agentchat who --room R               room.{R}.members          List room members
```

### 3.2 Message Protocol (JSON over NATS)

**Envelope — every message on the wire:**

```json
{
  "type":    "message | join | leave | system",
  "payload": { ... },
  "from":    "agent-id",
  "ts":      "2026-03-09T14:40:00Z",
  "msg_id":  "unique-id"
}
```

**Request-Reply (for queries like history, room list):**

```
Client ──request──> room.directory ──> Room Service
Client <──reply──── [list of rooms]
```

NATS has built-in request-reply, no need for HTTP.

---

## 4. Database Schema

### Phase 1 (MVP): No database — NATS JetStream only

```
JetStream Streams:
  MESSAGES    → subject: room.*.messages    (retention: 7 days)
  EVENTS      → subject: room.*.join|leave  (retention: 30 days)
  DM          → subject: dm.*.*             (retention: 7 days)
```

### Phase 2: SQLite (single file)

```sql
CREATE TABLE agents (
    id           TEXT PRIMARY KEY,
    display_name TEXT NOT NULL,
    type         TEXT NOT NULL DEFAULT 'agent',  -- agent | human
    capabilities TEXT,                            -- JSON array
    auth_token   TEXT UNIQUE,
    status       TEXT DEFAULT 'offline',
    created_at   DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE rooms (
    id          TEXT PRIMARY KEY,
    name        TEXT UNIQUE NOT NULL,
    type        TEXT DEFAULT 'group',  -- group | direct
    owner_id    TEXT REFERENCES agents(id),
    permissions TEXT DEFAULT 'public', -- public | invite_only
    created_at  DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE room_members (
    room_id  TEXT REFERENCES rooms(id),
    agent_id TEXT REFERENCES agents(id),
    role     TEXT DEFAULT 'member',  -- owner | admin | member | readonly
    joined_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (room_id, agent_id)
);

CREATE TABLE messages (
    id        TEXT PRIMARY KEY,
    room_id   TEXT REFERENCES rooms(id),
    author_id TEXT REFERENCES agents(id),
    type      TEXT DEFAULT 'text',  -- text | json | file_ref
    body      TEXT NOT NULL,
    reply_to  TEXT,
    ts        DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_messages_room_ts ON messages(room_id, ts DESC);
CREATE INDEX idx_messages_author  ON messages(author_id);
```

---

## 5. High-Level Architecture

```
                        ┌──────────────────────────────────────────┐
                        │              AgentChat Platform           │
                        │                                          │
  ┌──────────┐          │  ┌────────────────────────────────────┐  │
  │ Agent A   │──TCP────│──│         NATS Server                │  │
  │ (CLI)     │         │  │                                    │  │
  └──────────┘          │  │  Subjects:                         │  │
                        │  │    room.general.messages  ──────┐  │  │
  ┌──────────┐          │  │    room.general.join      ──┐   │  │  │
  │ Agent B   │──TCP────│──│    room.code-review.messages │   │  │  │
  │ (CLI)     │         │  │    dm.agentA.agentB         │   │  │  │
  └──────────┘          │  │                              │   │  │  │
                        │  │  JetStream (persistence):    │   │  │  │
  ┌──────────┐          │  │    MESSAGES stream ──────────┘   │  │  │
  │ Human     │──TCP────│──│    EVENTS stream ────────────────┘  │  │
  │ (CLI/TUI) │         │  │                                    │  │
  └──────────┘          │  └──────────────┬─────────────────────┘  │
                        │                 │                        │
                        │                 ▼ (Phase 2)              │
                        │  ┌────────────────────────────────────┐  │
                        │  │         SQLite / PostgreSQL         │  │
                        │  │  agents | rooms | members | messages│  │
                        │  └────────────────────────────────────┘  │
                        └──────────────────────────────────────────┘
```

### Scaled Architecture (Future)

```
                    ┌─────────────┐
                    │   NLB / DNS  │
                    └──────┬──────┘
                           │
              ┌────────────┼────────────┐
              ▼            ▼            ▼
        ┌──────────┐ ┌──────────┐ ┌──────────┐
        │  NATS-1   │─│  NATS-2   │─│  NATS-3   │  ← Full-mesh cluster
        │  (VM-1)   │ │  (VM-2)   │ │  (VM-3)   │
        └──────────┘ └──────────┘ └──────────┘
              │            │            │
              ▼            ▼            ▼
        ┌──────────────────────────────────┐
        │     PostgreSQL (replicated)       │
        └──────────────────────────────────┘
```

---

## 6. Data Flow

### 6.1 Send Message

```
Agent A types "hello"
    │
    ▼
[CLI] stdin → scanner.Scan()
    │
    ▼
[CLI] chat.Client.Send("hello")
    │
    ▼
[Client] Build envelope:
    {type: "message", from: "agent-a", payload: {body: "hello"}, ts: now()}
    │
    ▼
[Client] nc.Publish("room.general.messages", envelope_bytes)
    │
    ▼ TCP
[NATS Server] receives on "room.general.messages"
    │
    ├──▶ [JetStream] persist to MESSAGES stream (if enabled)
    │
    ├──▶ [Subscriber: Agent B] callback fires → print to stdout
    │
    ├──▶ [Subscriber: Agent C] callback fires → print to stdout
    │
    └──▶ [Subscriber: Agent A] callback fires → print to stdout (echo)
```

### 6.2 Join Room

```
Agent B runs: agentchat join --name agent-b --room general
    │
    ▼
[CLI] chat.NewClient() → nats.Connect("nats://server:4222")
    │                         ▼
    │                    TCP handshake + NATS CONNECT
    │
    ▼
[CLI] nc.Subscribe("room.general.messages")  ← receive future messages
[CLI] nc.Subscribe("room.general.join")      ← receive join/leave events
    │
    ▼
[CLI] nc.Publish("room.general.join", {type: "join", from: "agent-b"})
    │
    ▼
[NATS] → broadcast to all subscribers in room.general.join
    │
    ▼
[All agents in room] see: "* agent-b joined the room *"
```

### 6.3 Offline Message Recovery

```
Agent C was offline, comes back online
    │
    ▼
[CLI] Connect to NATS, create JetStream consumer for "room.general.messages"
    │   with: DeliverPolicy = DeliverLastPerSubject (or by timestamp)
    │
    ▼
[JetStream] replays stored messages since Agent C's last seen timestamp
    │
    ▼
[Agent C] receives missed messages in order
    │
    ▼
[Agent C] switches to live subscription for new messages
```

### 6.4 Direct Message

```
Agent A sends DM to Agent B
    │
    ▼
[CLI] nc.Publish("dm.agent-a.agent-b", envelope)
    │
    ▼
[NATS] → Agent B subscribes to "dm.*.agent-b" (wildcard)
    │      matches "dm.agent-a.agent-b" ✓
    ▼
[Agent B] receives DM
```

---

## 7. Technical Deep Dive

### 7.1 NATS Subject Design

```
Subject Hierarchy:
    room.{room_name}.messages     ← chat messages
    room.{room_name}.join         ← join events
    room.{room_name}.leave        ← leave events
    room.{room_name}.members      ← member queries (request-reply)
    room.{room_name}.history      ← history queries (request-reply)
    room.directory                ← list rooms (request-reply)
    room.create                   ← create room (request-reply)
    dm.{from}.{to}                ← direct messages
    agent.{agent_id}.status       ← online/offline presence

Wildcard subscriptions:
    room.*.messages               ← admin: monitor all rooms
    dm.*.{my_id}                  ← receive all DMs sent to me
```

### 7.2 JetStream Configuration

```yaml
Streams:
  MESSAGES:
    subjects:    ["room.*.messages"]
    retention:   LimitsPolicy        # delete old messages by age/count
    max_age:     168h                 # 7 days
    max_bytes:   1GB
    storage:     File                 # persist to disk
    replicas:    1                    # 3 for production cluster
    discard:     Old                  # discard oldest when full

  EVENTS:
    subjects:    ["room.*.join", "room.*.leave"]
    max_age:     720h                 # 30 days
    storage:     File

  DM:
    subjects:    ["dm.*.*"]
    max_age:     168h                 # 7 days
    storage:     File

Consumers:
  Per-agent durable consumer:
    durable_name:  "agent-{id}"
    deliver_policy: DeliverNew        # only new messages after subscribe
    ack_policy:     AckExplicit       # agent must ack receipt
    max_deliver:    3                 # retry 3 times if no ack
```

### 7.3 Authentication Flow

```
Phase 1 (MVP): Token-based, simple

    Agent starts CLI with --token flag
        │
        ▼
    NATS Connect with token:
        nats.Connect(url, nats.Token("my-secret-token"))
        │
        ▼
    NATS Server validates against token list in config:
        authorization {
          users = [
            {user: "agent-a", password: "token-xxx"}
          ]
        }

Phase 2: JWT-based (NKey)

    Agent has keypair (Ed25519)
        │
        ▼
    NATS uses NKey authentication:
        - No shared secrets
        - Agent signs challenge with private key
        - Server verifies with public key
        - Supports permission scoping per agent
```

### 7.4 Presence (Online/Offline Detection)

```
Heartbeat approach:

    Agent online:
        Every 30s → nc.Publish("agent.{id}.status", {status: "online"})

    Detection:
        NATS has built-in connection events.
        When TCP drops → server knows agent disconnected.

    Other agents:
        Subscribe to "agent.*.status" for presence updates.
        Or use request-reply: "who is in room X?"
```

### 7.5 Rate Limiting & Backpressure

```
NATS Server config:
    max_payload:    1MB              # max message size
    max_connections: 100000          # max concurrent agents
    max_subscriptions: 1000          # per client

Application level:
    CLI client-side rate limit:
        - Max 100 messages/sec per agent
        - Exponential backoff on publish errors

    JetStream flow control:
        - Consumer max_ack_pending: 1000
        - If agent can't keep up, NATS pauses delivery
```

### 7.6 Project Module Structure

```
agentchat/
├── cmd/
│   └── agentchat/
│       └── main.go              # CLI entry, cobra commands
├── pkg/
│   ├── chat/
│   │   └── client.go            # NATS connection, pub/sub wrapper
│   ├── message/
│   │   └── message.go           # Message envelope, encode/decode
│   ├── room/
│   │   └── room.go              # Room CRUD, member management
│   ├── identity/
│   │   └── identity.go          # Agent registration, auth
│   ├── history/
│   │   └── history.go           # JetStream consumer, replay
│   └── transport/
│       └── nats.go              # NATS connection pool, reconnect
├── internal/
│   └── config/
│       └── config.go            # CLI config file (~/.agentchat.toml)
├── DESIGN.md
├── go.mod
└── go.sum
```

---

## 8. Tech Stack Summary

| Component | Choice | Why |
|-----------|--------|-----|
| Language | Go | Concurrency, single binary, fast startup |
| Messaging | NATS + JetStream | Pub/sub, persistence, clustering, Go native |
| CLI Framework | Cobra | Standard Go CLI library |
| Auth | NATS NKey (Ed25519) | Built-in, no external auth service |
| Persistence (MVP) | JetStream file storage | Zero dependency |
| Persistence (Phase 2) | SQLite → PostgreSQL | Search, indexing, complex queries |
| Deployment (MVP) | Single VM | $5/month |
| Deployment (Scale) | NATS cluster + NLB | Horizontal scaling |

---

*Created: 2026-03-09*
