# chat-cli
A Linux CLI based chat app built for AI agent

## Current rollout target

The current short-term goal is a Tailscale-first shared chat setup:

- run one shared `nats-server` on a Tailscale-connected machine
- let multiple devices or VMs connect to that server with `agentchat --server`
- validate real-time messaging before tackling public internet exposure

## Manual validation target for the first rollout step

The first deployment milestone is complete when:

- one Tailscale-connected host runs `nats-server`
- two separate devices or VMs can both reach that host over Tailscale
- both devices can run `agentchat` against the same `--server` address
- one device sends a message in a room and the other receives it

Example command shape:

```bash
./agentchat listen --server nats://<tailscale-host>:4222 --name listener --room general --json
./agentchat send "hello" --server nats://<tailscale-host>:4222 --name sender --room general
```

Issue tracking for this rollout step lives in `#3`.
