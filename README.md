# linear-orchestrator

Multi-account Linear MCP gateway. Stores several Linear API keys and exposes them through a single MCP server. Each tool call routes by required `account` argument.

## Build

```bash
make build         # produces ./bin/linear-orch (codesigned on macOS)
```

Requires Go 1.22+. On macOS, the linker flag `-linkmode=external` plus ad-hoc `codesign` is needed for the binary to launch on recent OS versions.

## Usage

```bash
# add an account (token from Linear: Settings → API → Personal API keys)
linear-orch add work --token=lin_api_xxx --note="Day job"
linear-orch add personal --token=lin_api_yyy

linear-orch list
linear-orch remove personal
linear-orch path     # print config file path

linear-orch serve    # run MCP server on stdio
```

Config file: `~/Library/Application Support/linear-orchestrator/config.json` on macOS, `~/.config/linear-orchestrator/config.json` on Linux. Mode 0600. Plain JSON.

## MCP wiring

Add to `~/.claude.json` or your client's MCP config:

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

## Tools exposed

Every tool requires `account` — the configured account name to route through.

| Tool | Purpose |
|------|---------|
| `list_accounts` | Enumerate configured account names. |
| `list_teams` | Workspace teams. |
| `list_projects` | Projects, optional `team` filter. |
| `list_issues` | Issues filtered by team / project / assignee / state. |
| `get_issue` | Fetch one issue by `ENG-123` or UUID. |
| `create_issue` | Create issue (`team`, `title` required). |
| `update_issue` | Update title / description / state / assignee. |
| `list_comments` | Comments on an issue. |
| `add_comment` | Append a comment. |

## Layout

```
cmd/linear-orch       CLI entrypoint
internal/config       JSON config file (account name → token)
internal/linear       GraphQL client (POST to api.linear.app/graphql)
internal/mcp          JSON-RPC 2.0 stdio MCP server
internal/tools        Tool registrations + handlers
```
