# AGENTS.md
## Purpose
This file gives coding agents the practical rules for working in this repository.
Prefer the current checked-in code over future-looking design docs.

## Repository Snapshot
- Language: Go
- Module: `github.com/chenqid/agentchat`
- Primary binary: `./cmd/agentchat`
- Core packages: `./pkg/chat`, `./pkg/message`
- End-to-end tests: `./tests/e2e`
- Messaging backend used by the code: NATS
- CLI framework used by the code: Cobra

## Important Reality Check
The design documents describe a larger future platform.
The current codebase is a smaller MVP centered on a single CLI app.
When editing code, follow the current implementation unless the task explicitly asks for roadmap work.
Current implemented areas:
- `cmd/agentchat`: CLI entry point and commands
- `pkg/chat`: NATS-backed chat client wrapper
- `pkg/message`: message model and JSON encode/decode
- `tests/e2e`: process-level CLI tests with embedded NATS
Not present in the checked-in code:
- `cmd/server`
- `pkg/room`
- `pkg/identity`
- `pkg/transport`
- TUI-specific implementation
- dedicated persistence/history modules

## Instruction Sources
Checked during repo analysis:
- No existing root `AGENTS.md` was present before this file
- No `.cursorrules` file exists
- No `.cursor/rules/` directory exists
- No `.github/copilot-instructions.md` file exists
If any of those files are added later, treat them as additional instruction sources and keep this file aligned with them.

## Working Commands
Run all commands from the repository root.

### Build
- Build the CLI: `go build ./cmd/agentchat`
- Build to a named binary: `go build -o agentchat ./cmd/agentchat`
- Build all packages: `go build ./...`

### Format
- Format all Go files: `gofmt -w ./cmd ./pkg ./tests`
- Format a single file: `gofmt -w path/to/file.go`
`goimports` is not configured in the repo, so do not assume it is required.
Still, keep imports in standard Go order so `goimports` would make no semantic changes.

### Lint / Static Checks
- Static analysis: `go vet ./...`
There is no configured `golangci-lint`, `Makefile`, or CI wrapper in this repository.
If a task asks for linting, use `go vet ./...` unless new tooling is introduced.

### Tests
- Run all tests: `go test ./...`
- Run all tests without cache: `go test ./... -count=1`
- Run one package: `go test ./pkg/chat`
- Run another package: `go test ./pkg/message`
- Run CLI package tests: `go test ./cmd/agentchat`
- Run e2e tests only: `go test ./tests/e2e`

### Run a Single Test
Use `-run` with a fully anchored regex and point at the package that owns the test.
- Single unit test: `go test ./pkg/chat -run '^TestBroadcast$'`
- Another unit test: `go test ./pkg/message -run '^TestDecode_InvalidJSON$'`
- Single CLI test: `go test ./cmd/agentchat -run '^TestRequireName$'`
- Single e2e test: `go test ./tests/e2e -run '^TestE2E_SendAndListen$'`
If you only know part of the name, start broad and then narrow it:
- `go test ./tests/e2e -run 'TestE2E_'`

### Coverage
- Basic coverage run: `go test ./... -cover`

## Code Style
### Formatting
- Use `gofmt` formatting, always
- Keep lines and layout idiomatic instead of hand-aligned
- Do not introduce alternate formatting conventions

### Imports
- Group imports the standard Go way
- Standard library imports first
- Blank line
- External or module imports last
- Keep import aliases only when they add clarity or avoid collisions
Example patterns already in the repo:
- plain imports in `cmd/agentchat/main.go`
- aliased NATS server import in tests: `natsserver "github.com/nats-io/nats-server/v2/server"`

### Packages and File Layout
- Keep package names short, lowercase, and idiomatic
- Put the executable entry point under `cmd/agentchat`
- Put reusable logic under `pkg/...`
- Put process-level tests under `tests/e2e`
- Do not create new top-level directories unless the task clearly needs them

### Naming
- Exported identifiers use PascalCase: `Client`, `Message`, `NewClient`
- Unexported helpers use lower camel case: `requireName`, `subject`
- Test names use `TestXxx` and optionally `TestXxx_Scenario`
- Prefer clear domain names over abbreviations, except common short receiver names like `c`, `m`, `ns`, or `cmd`

### Types and APIs
- Prefer concrete structs for the core domain model
- Keep struct fields explicit and add JSON tags where wire format matters
- Use `omitempty` only when the absence of a field is meaningful
- Keep constructors simple and named `NewXxx` when they create initialized values
- Return `(value, error)` instead of hiding failures

### Error Handling
- Handle errors explicitly; do not ignore them unless the repo already treats that path as best-effort
- Wrap underlying errors with context using `fmt.Errorf(... %w ...)` when returning external-call failures
- Use plain validation errors for simple local checks like missing required flags
- Avoid `panic` in normal control flow
- Avoid `log.Fatal` inside library code
- In Cobra commands, prefer returning errors from `RunE` and let the command runner handle process exit

### Control Flow
- Prefer small functions with one clear responsibility
- Use early returns for validation and connection failures
- Keep CLI glue in `cmd/agentchat` and transport/message logic in packages
- Avoid broad refactors unless the task explicitly asks for them

### Comments and Documentation
- Keep comments short and useful
- Do not add comments for obvious code
- Match the local file style when editing existing files
- Current repo pattern: production code comments are mostly English; many test comments and assertion messages are Chinese
- Preserve existing language conventions within the file you are editing instead of rewriting everything for consistency

## Testing Conventions
- Prefer table-free direct tests when there are only a few straightforward assertions; that is the current repo style
- Use `t.Fatalf` for setup failures that should stop the test immediately
- Use `t.Errorf` when you want multiple assertions to report in one run
- Use `t.Error` for simple boolean expectation failures
- Only use `t.Parallel()` when the test is isolated from package globals and shared external resources

### NATS-Related Tests
- Unit tests in `pkg/chat` start an embedded NATS server instead of depending on an external server
- E2E tests build the real CLI binary in `TestMain` and spawn child processes
- If you change CLI flags, output, startup behavior, or room routing, check `tests/e2e` in addition to unit tests

## Editing Guidance For Agents
- Keep changes small and local
- Prefer current package boundaries over speculative architecture from design docs
- Do not add new dependencies unless the task clearly benefits from them
- If you add a command, update tests near that command and relevant e2e coverage when behavior is user-visible
- If you change message JSON shape, update both unit tests and e2e assertions
- If you introduce new tooling, document the exact command in this file

## Pre-PR Checklist
Before handing work back, run the smallest relevant set from this list:
- `gofmt -w` on changed Go files
- `go vet ./...`
- `go test ./...` or the narrowest package/test selection that proves the change
If you only touched docs, say so explicitly and skip Go commands unless the task asked for verification.
