# linear-orchestrator

> Route Claude (and any MCP client) across multiple Linear workspaces through a single gateway.

[![Go Version](https://img.shields.io/badge/go-1.22%2B-00ADD8?logo=go)](https://go.dev/)
[![MCP](https://img.shields.io/badge/MCP-2024--11--05-5B5BD6)](https://modelcontextprotocol.io/)
[![License: MIT](https://img.shields.io/badge/license-MIT-green.svg)](LICENSE)
[![Platform](https://img.shields.io/badge/platform-macOS%20%7C%20Linux-lightgrey)]()

`linear-orchestrator` is a small Go binary that holds API keys for any number of Linear workspaces and exposes them through one [Model Context Protocol](https://modelcontextprotocol.io/) server. Tools route to the right workspace via a required `account` argument, so an LLM can mix calls across `work`, `client-a`, `personal`, etc. in the same conversation without re-authenticating or restarting the server.

The official Linear MCP only handles a single workspace per server instance. This project exists to fill that gap.

---

## Table of contents

- [Why](#why)
- [Features](#features)
- [Install](#install)
- [Quickstart](#quickstart)
- [Connect to Claude / your MCP client](#connect-to-claude--your-mcp-client)
- [CLI reference](#cli-reference)
- [Tools exposed](#tools-exposed)
- [Configuration](#configuration)
- [Security](#security)
- [Development](#development)
- [Architecture](#architecture)
- [Roadmap](#roadmap)
- [Contributing](#contributing)
- [License](#license)

---

## Why

Most Linear MCP integrations bind one server instance to one API key. Once you have separate workspaces — a day job, a freelance client, a personal account — you have to:

- Run a new MCP server process per workspace, **or**
- Constantly swap the API key in a `.env` file and restart the client.

Neither is convenient when an agent needs to read a ticket from one workspace and file a follow-up in another. `linear-orchestrator` keeps every key in one config, and every tool call carries the workspace name as an argument.

## Features

- 🔀 **Multi-account routing** — every tool takes an `account` parameter; the server picks the matching token at call time.
- 🧰 **Eleven tools, MVP-shaped** — list/get/create/update issues, list/add comments, list/add project updates, list teams and projects, plus an introspection tool that lists configured accounts.
- 🪶 **Zero runtime dependencies** — single static Go binary. No Node, no Python, no Docker.
- 🧱 **Hand-rolled GraphQL client** — Linear's API is GraphQL, so there's a 70-line client and no SDK to keep in sync.
- 🔐 **Local-only secrets** — keys stay in a `0600` JSON file under your user config dir; never shipped anywhere else.
- 🪟 **stdio JSON-RPC 2.0** — speaks MCP `2024-11-05` over stdio. Drop into Claude Desktop, Claude Code, Cursor, or anything else that speaks MCP.
- 🐹 **Plain Go** — `go build`, `go test`, no codegen step.

## Install

### From source

```bash
git clone https://github.com/serdarcoskun/linear-orchestrator
cd linear-orchestrator
make install        # builds, codesigns on macOS, installs to ~/.local/bin/linear-orch
```

Make sure `~/.local/bin` is on your `PATH`.

### Manual build

```bash
make build          # produces ./bin/linear-orch
./bin/linear-orch help
```

Requires Go 1.22+. On recent macOS releases the binary is built with `-linkmode=external` and ad-hoc `codesign`'d, otherwise dyld refuses to launch it ("missing LC_UUID load command"). The `Makefile` handles this automatically.

## Quickstart

```bash
# 1. Generate a personal API key in Linear:
#    Settings → API → Personal API keys → Create key

# 2. Register one or more accounts
linear-orch add work     --token=lin_api_xxx --note="Day job"
linear-orch add client-a --token=lin_api_yyy --note="Freelance"
linear-orch add personal --token=lin_api_zzz

linear-orch list
# work       lin_a********xxx1  (Day job)
# client-a   lin_a********yyy2  (Freelance)
# personal   lin_a********zzz3

# 3. Start the MCP server (stdio, talks to your client)
linear-orch serve
```

Once an MCP client is wired up (next section), an LLM can call:

```
list_accounts()
list_teams(account="work")
list_issues(account="client-a", team="ENG", state="In Progress", limit=10)
add_comment(account="personal", id="HOME-12", body="Bought the part.")
```

## Connect to Claude / your MCP client

### Claude Desktop / Claude Code

Add to your MCP server config (`~/.claude.json`, or whatever your client uses):

```json
{
  "mcpServers": {
    "linear-orch": {
      "command": "/ABSOLUTE/PATH/TO/linear-orch",
      "args": ["serve"]
    }
  }
}
```

> ⚠️ MCP clients do not expand `~` or `$HOME`. The `command` field must be a fully absolute path. Use `which linear-orch` to find it after `make install`.

Restart the client. `linear-orch` should appear in the connected servers list, exposing the nine tools below.

### Cursor / other MCP clients

Same pattern — point a stdio MCP server at `linear-orch serve`.

## CLI reference

```
linear-orch add <account> --token=<key> [--note=<text>]
linear-orch add <account>                       # reads token from stdin
linear-orch list                                # masked token preview
linear-orch remove <account>
linear-orch serve                               # MCP server on stdio
linear-orch path                                # print config file path
linear-orch help
```

## Tools exposed

Every tool requires an `account` argument — the name you registered with `linear-orch add`.

| Tool             | Required args              | Purpose                                                            |
|------------------|----------------------------|--------------------------------------------------------------------|
| `list_accounts`  | —                          | Enumerate configured account names.                                |
| `list_teams`     | `account`                  | All teams in the workspace.                                        |
| `list_projects`  | `account`                  | Projects, optional `team` filter (key or UUID).                    |
| `list_issues`    | `account`                  | Filter by `team`, `project`, `assignee`, `state`, `limit`.         |
| `get_issue`      | `account`, `id`            | Fetch one issue by `ENG-123` or UUID.                              |
| `create_issue`   | `account`, `team`, `title` | Optional `description`, `assignee` (email/UUID), `project`.        |
| `update_issue`   | `account`, `id`            | Update `title`, `description`, `state` (name), `assignee`.         |
| `list_comments`  | `account`, `id`            | Comments on an issue.                                              |
| `add_comment`    | `account`, `id`, `body`    | Append a comment.                                                  |
| `list_project_updates` | `account`, `project`       | Updates posted to a project.                                       |
| `add_project_update`   | `account`, `project`, `body` | Post an update; optional `health` (`onTrack`/`atRisk`/`offTrack`). |

**Convenience resolvers** — you can pass human-friendly references and the server resolves them to UUIDs before calling the GraphQL API:

- `team`: team key (e.g. `ENG`) → team UUID
- `assignee`: email → user UUID
- `state`: workflow state name (e.g. `In Progress`) → state UUID
- `id` on issues: `ENG-123` is accepted directly by Linear's API

## Configuration

Config lives at:

| OS      | Path                                                      |
|---------|-----------------------------------------------------------|
| macOS   | `~/Library/Application Support/linear-orchestrator/config.json` |
| Linux   | `~/.config/linear-orchestrator/config.json`               |

`linear-orch path` prints the actual location. The file is mode `0600` and looks like:

```json
{
  "accounts": {
    "work": {
      "token": "lin_api_...",
      "note": "Day job"
    },
    "personal": {
      "token": "lin_api_..."
    }
  }
}
```

You can hand-edit it; `linear-orch` re-reads it each time `serve` starts.

## Security

- Tokens are stored **in plaintext** in the config file (mode `0600`, owner-only). This is appropriate for a personal CLI but **not** for shared or root-accessible machines.
- The config path is never logged. Token previews in `linear-orch list` are masked: first 4 + last 4 characters only.
- The MCP server speaks **stdio only** — there is no network listener and no remote attack surface beyond the Linear API itself.
- HTTPS to `api.linear.app` is enforced by Go's stdlib defaults; no certificate pinning.
- A future version may opt into the macOS Keychain (`security` cmd) for token storage; tracked under [Roadmap](#roadmap).

If you find a security issue, please open a GitHub issue marked `security` or contact the maintainer directly rather than posting full details publicly.

## Development

```bash
make build          # build to ./bin/linear-orch
make test           # go test ./...
make vet            # go vet ./...
make fmt            # go fmt ./...
make clean          # remove ./bin
```

### Smoke-testing the MCP server by hand

```bash
printf '%s\n' \
  '{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"smoke","version":"0"}}}' \
  '{"jsonrpc":"2.0","method":"notifications/initialized"}' \
  '{"jsonrpc":"2.0","id":2,"method":"tools/list"}' \
  | ./bin/linear-orch serve
```

You should see two JSON responses (`initialize` result, then the tool list).

### Running against a real account

```bash
linear-orch add scratch --token=lin_api_...
echo '{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"list_teams","arguments":{"account":"scratch"}}}' \
  | ./bin/linear-orch serve
```

## Architecture

```
cmd/linear-orch       CLI entrypoint (add / list / remove / serve / path)
internal/config       JSON config file (account name → token)
internal/linear       GraphQL client — POST to api.linear.app/graphql
internal/mcp          JSON-RPC 2.0 stdio MCP server (no SDK dep)
internal/tools        Tool registrations + handlers
```

Request flow:

```
MCP client ──stdio──▶ internal/mcp ──▶ internal/tools
                                          │
                                          ├─ resolve `account` → token via internal/config
                                          ├─ resolve team key / email / state name → UUID
                                          └─ internal/linear → POST graphql → JSON
```

## Roadmap

- [ ] Optional macOS Keychain backend for token storage
- [ ] `cycles` and `roadmap` tools
- [ ] Issue search (`search_issues`) using Linear's text search
- [ ] Attachment upload
- [ ] OAuth 2.0 flow (in addition to personal API keys)
- [ ] Pagination for `list_*` tools
- [ ] Default-account mode (set one account as implicit so the LLM can omit `account`)
- [ ] Homebrew tap for one-line install

## Contributing

PRs welcome. The codebase is small (under ~1k LOC) and intentionally has zero third-party dependencies — please keep it that way unless there's a strong reason.

A good first PR is a new tool: copy an existing entry in [`internal/tools/tools.go`](internal/tools/tools.go), add the handler in [`internal/tools/handlers.go`](internal/tools/handlers.go), and write the GraphQL query inline. Run `make vet test build` and you're done.

When adding tools, please:

1. Keep `account` as a **required** argument.
2. Accept human-friendly references (team key, email, state name) and resolve them server-side.
3. Return JSON-stringified results — the MCP `content` text is consumed by an LLM, not a UI.

## License

MIT — see [LICENSE](LICENSE).
