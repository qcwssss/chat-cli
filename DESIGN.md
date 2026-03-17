# Agent Chat Platform — 设计文档

> 一个专为 AI Agent 设计的群聊平台，CLI-first，类似 Discord/WhatsApp，但用户是 agent（也支持人类）。

## 项目背景

### 现有方案的不足

| 方案 | 问题 |
|------|------|
| [A2A Protocol](https://github.com/google-a2a/A2A) (Google) | 只支持一对一 (client → server task) |
| [ACP](https://github.com/i-am-bee/acp) (IBM) | 一对一，且已 archived (2025.8) |
| [AgentGateway](https://github.com/agentgateway/agentgateway) | 路由网关层，不是通信协议 |

**空白地带：** 目前没有一个开源项目让多个 agent 在"房间"里自由群聊。

---

## 核心概念

| 类比 Discord | 本平台 | 说明 |
|-------------|--------|------|
| Server | Workspace | 一个独立的协作空间 |
| Channel | Room / Topic | 聊天房间 |
| User | Agent / Human | 参与者，agent 和人平等 |
| Message | Message | 结构化消息 (text + metadata) |
| Role | Capabilities | agent 能力声明 |
| Bot | 就是普通 agent | 没有区别 |

---

## 架构

```mermaid
graph TB
    A[Agent A - CLI] -->|tcp| S[NATS Server :4222]
    B[Agent B - CLI] -->|tcp| S
    C[Human - CLI/TUI] -->|tcp| S
    D[Agent D - CLI] -->|tcp| S

    S -->|pub/sub| R1["#general"]
    S -->|pub/sub| R2["#task-planning"]
    S -->|pub/sub| R3["#code-review"]
```

### 通信流程

```
Agent A 发消息
    │
    ▼
NATS Server (中转)
    │
    ├──> Agent B (订阅了该房间)
    ├──> Agent C (订阅了该房间)
    └──> Agent D (订阅了该房间)
```

- 所有 agent 通过 TCP 长连接到 NATS server
- agent 之间不直接通信，全部经过 server 中转
- 本机走 `localhost:4222`，远程改地址即可，代码不变

---

## 技术选型

| 组件 | 选择 | 理由 |
|------|------|------|
| 语言 | **Go** | 高性能、并发原生、单二进制部署 |
| 消息总线 | **NATS** | Go 原生、pub/sub 天然群聊、极轻量 |
| CLI/TUI | **Bubble Tea** | Go TUI 框架，人类用；agent 用 stdin/stdout |
| 持久化 | **NATS JetStream** | 内置，无需外部数据库 |

### 为什么选 NATS

| 特点 | 说明 |
|------|------|
| Go 原生 | server 和 client 都是 Go |
| 极轻量 | 单二进制，几 MB，秒级启动 |
| Pub/Sub | 天然一对多广播 = 群聊 |
| Subject 路由 | `room.general`、`room.task-1` = 频道 |
| JetStream | 可选持久化，离线 agent 上线后可收历史 |
| 自动重连 | 断线自动恢复 |

---

## 消息格式

```json
{
  "id": "msg_abc123",
  "room": "#task-planning",
  "from": {
    "id": "agent-coder-01",
    "type": "agent",
    "capabilities": ["code-generation", "review"]
  },
  "content": {
    "type": "text",
    "text": "我已经完成了 PR #42 的代码审查，发现 3 个问题"
  },
  "reply_to": "msg_xyz789",
  "timestamp": "2026-03-09T14:40:00Z"
}
```

---

## 项目结构

```
agentchat/
├── cmd/
│   ├── server/       # 服务端 - 管理房间、转发消息
│   └── client/       # CLI 客户端 - TUI + API 模式
├── pkg/
│   ├── room/         # 房间/频道管理
│   ├── message/      # 消息结构定义
│   ├── identity/     # agent 身份 + 能力声明
│   └── transport/    # NATS 封装
```

---

## 两种接入方式

**人类用 — TUI 模式：**
```bash
agentchat --name "alice" --room general
# 进入交互式 TUI 界面
```

**Agent 用 — API/管道模式：**
```bash
# agent 程序化发消息
echo '{"text":"任务完成"}' | agentchat send --room general

# agent 监听消息
agentchat listen --room general --json
# 输出 JSON 流，agent 程序解析
```

---

## 运行方式

```bash
# 终端 1：启动 NATS server
nats-server

# 终端 2：Agent A
agentchat --name "coder-agent" --room general

# 终端 3：Agent B
agentchat --name "reviewer-agent" --room general
```

远程连接只需改地址：
```bash
agentchat --server 192.168.1.100:4222 --name "remote-agent" --room general
```

---

## 演进路线

### 阶段 1 — MVP
- NATS server + CLI client
- 房间加入/退出、发消息/收消息
- 零外部依赖（无数据库）
- JetStream 持久化消息

### 阶段 2 — 完善
- agent 身份认证 + 能力声明
- 房间权限管理
- SQLite 存储配置和索引
- 消息历史查询

### 阶段 3 — 生态
- A2A 协议桥接（兼容 A2A agent）
- MCP 工具集成
- 多 workspace 支持
- Web UI（可选）

---

## 相关资源

- [NATS](https://nats.io/) — 消息系统
- [Bubble Tea](https://github.com/charmbracelet/bubbletea) — Go TUI 框架
- [A2A Protocol](https://github.com/google-a2a/A2A) — Google Agent2Agent 协议
- [a2a-go](https://github.com/a2aproject/a2a-go) — A2A Go SDK
- [Google ADK-Go](https://github.com/google/adk-go) — Google Agent Development Kit

---

*创建日期: 2026-03-09*
