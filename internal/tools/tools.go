package tools

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/serdarcoskun/linear-orchestrator/internal/config"
	"github.com/serdarcoskun/linear-orchestrator/internal/linear"
	"github.com/serdarcoskun/linear-orchestrator/internal/mcp"
)

func Register(s *mcp.Server, cfg *config.Config) {
	s.Register(mcp.Tool{
		Name:        "list_accounts",
		Description: "List configured Linear accounts available for routing.",
		InputSchema: json.RawMessage(`{"type":"object","properties":{},"additionalProperties":false}`),
		Handler: func(ctx context.Context, args json.RawMessage) (string, error) {
			names := make([]string, 0, len(cfg.Accounts))
			for k, a := range cfg.Accounts {
				if a.Note != "" {
					names = append(names, fmt.Sprintf("%s (%s)", k, a.Note))
				} else {
					names = append(names, k)
				}
			}
			b, _ := json.MarshalIndent(names, "", "  ")
			return string(b), nil
		},
	})

	s.Register(mcp.Tool{
		Name:        "list_issues",
		Description: "List issues in a Linear workspace. Filter by team, project, assignee, or state.",
		InputSchema: json.RawMessage(`{
			"type":"object",
			"properties":{
				"account":{"type":"string","description":"Configured account name"},
				"team":{"type":"string","description":"Team key or ID (optional)"},
				"project":{"type":"string","description":"Project ID (optional)"},
				"assignee":{"type":"string","description":"Assignee email or ID (optional)"},
				"state":{"type":"string","description":"Workflow state name (optional)"},
				"limit":{"type":"integer","default":25,"maximum":100}
			},
			"required":["account"],
			"additionalProperties":false
		}`),
		Handler: makeListIssues(cfg),
	})

	s.Register(mcp.Tool{
		Name:        "get_issue",
		Description: "Fetch a single Linear issue by identifier (e.g. ENG-123).",
		InputSchema: json.RawMessage(`{
			"type":"object",
			"properties":{
				"account":{"type":"string"},
				"id":{"type":"string","description":"Issue identifier like ENG-123 or UUID"}
			},
			"required":["account","id"],
			"additionalProperties":false
		}`),
		Handler: makeGetIssue(cfg),
	})

	s.Register(mcp.Tool{
		Name:        "create_issue",
		Description: "Create a Linear issue.",
		InputSchema: json.RawMessage(`{
			"type":"object",
			"properties":{
				"account":{"type":"string"},
				"team":{"type":"string","description":"Team key (e.g. ENG) or team UUID"},
				"title":{"type":"string"},
				"description":{"type":"string"},
				"assignee":{"type":"string","description":"Assignee email or UUID (optional)"},
				"project":{"type":"string","description":"Project UUID (optional)"}
			},
			"required":["account","team","title"],
			"additionalProperties":false
		}`),
		Handler: makeCreateIssue(cfg),
	})

	s.Register(mcp.Tool{
		Name:        "update_issue",
		Description: "Update fields on an existing Linear issue.",
		InputSchema: json.RawMessage(`{
			"type":"object",
			"properties":{
				"account":{"type":"string"},
				"id":{"type":"string","description":"Issue identifier (ENG-123) or UUID"},
				"title":{"type":"string"},
				"description":{"type":"string"},
				"state":{"type":"string","description":"Workflow state name"},
				"assignee":{"type":"string","description":"Assignee email or UUID"}
			},
			"required":["account","id"],
			"additionalProperties":false
		}`),
		Handler: makeUpdateIssue(cfg),
	})

	s.Register(mcp.Tool{
		Name:        "add_comment",
		Description: "Add a comment to a Linear issue.",
		InputSchema: json.RawMessage(`{
			"type":"object",
			"properties":{
				"account":{"type":"string"},
				"id":{"type":"string","description":"Issue identifier or UUID"},
				"body":{"type":"string"}
			},
			"required":["account","id","body"],
			"additionalProperties":false
		}`),
		Handler: makeAddComment(cfg),
	})

	s.Register(mcp.Tool{
		Name:        "list_comments",
		Description: "List comments on a Linear issue.",
		InputSchema: json.RawMessage(`{
			"type":"object",
			"properties":{
				"account":{"type":"string"},
				"id":{"type":"string","description":"Issue identifier or UUID"},
				"limit":{"type":"integer","default":50,"maximum":100}
			},
			"required":["account","id"],
			"additionalProperties":false
		}`),
		Handler: makeListComments(cfg),
	})

	s.Register(mcp.Tool{
		Name:        "list_projects",
		Description: "List projects, optionally filtered by team.",
		InputSchema: json.RawMessage(`{
			"type":"object",
			"properties":{
				"account":{"type":"string"},
				"team":{"type":"string","description":"Team key or UUID (optional)"},
				"limit":{"type":"integer","default":50,"maximum":100}
			},
			"required":["account"],
			"additionalProperties":false
		}`),
		Handler: makeListProjects(cfg),
	})

	s.Register(mcp.Tool{
		Name:        "list_teams",
		Description: "List all teams in the workspace.",
		InputSchema: json.RawMessage(`{
			"type":"object",
			"properties":{
				"account":{"type":"string"}
			},
			"required":["account"],
			"additionalProperties":false
		}`),
		Handler: makeListTeams(cfg),
	})
}

func clientFor(cfg *config.Config, account string) (*linear.Client, error) {
	a, err := cfg.Get(account)
	if err != nil {
		return nil, err
	}
	return linear.New(a.Token), nil
}

func jsonStr(v any) string {
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return fmt.Sprintf("marshal err: %v", err)
	}
	return string(b)
}
