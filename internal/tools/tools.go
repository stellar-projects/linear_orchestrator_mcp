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
				"project":{"type":"string","description":"Project UUID (optional)"},
				"parent":{"type":"string","description":"Parent issue identifier (ENG-123) or UUID. Set to make this a subtask (optional)."},
				"priority":{"type":"integer","minimum":0,"maximum":4,"description":"Priority: 0=None, 1=Urgent, 2=High, 3=Normal, 4=Low (optional)"},
				"labels":{"type":"array","items":{"type":"string"},"description":"Label names or UUIDs to apply (optional)"}
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
				"assignee":{"type":"string","description":"Assignee email or UUID"},
				"priority":{"type":"integer","minimum":0,"maximum":4,"description":"Priority: 0=None, 1=Urgent, 2=High, 3=Normal, 4=Low"},
				"labels":{"type":"array","items":{"type":"string"},"description":"Label names or UUIDs. Replaces the issue's full label set; [] clears all labels."}
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
		Name:        "list_subtasks",
		Description: "List subtasks (child issues) of a Linear issue.",
		InputSchema: json.RawMessage(`{
			"type":"object",
			"properties":{
				"account":{"type":"string"},
				"id":{"type":"string","description":"Parent issue identifier (ENG-123) or UUID"},
				"limit":{"type":"integer","default":50,"maximum":100}
			},
			"required":["account","id"],
			"additionalProperties":false
		}`),
		Handler: makeListSubtasks(cfg),
	})

	s.Register(mcp.Tool{
		Name:        "set_parent",
		Description: "Set or clear the parent of a Linear issue, turning it into a subtask (or detaching it).",
		InputSchema: json.RawMessage(`{
			"type":"object",
			"properties":{
				"account":{"type":"string"},
				"id":{"type":"string","description":"Issue identifier (ENG-123) or UUID"},
				"parent":{"type":"string","description":"Parent issue identifier or UUID. Empty string detaches the issue from its parent."}
			},
			"required":["account","id","parent"],
			"additionalProperties":false
		}`),
		Handler: makeSetParent(cfg),
	})

	s.Register(mcp.Tool{
		Name:        "add_relation",
		Description: "Create a relation between two Linear issues. Use type 'blocks' to mark that the issue blocks another (an inverse 'blocked by' is created automatically).",
		InputSchema: json.RawMessage(`{
			"type":"object",
			"properties":{
				"account":{"type":"string"},
				"id":{"type":"string","description":"Source issue identifier (ENG-123) or UUID"},
				"related":{"type":"string","description":"Target issue identifier or UUID"},
				"type":{"type":"string","enum":["blocks","related","duplicate"],"default":"blocks","description":"Relation type. 'blocks' means id blocks related."}
			},
			"required":["account","id","related"],
			"additionalProperties":false
		}`),
		Handler: makeAddRelation(cfg),
	})

	s.Register(mcp.Tool{
		Name:        "remove_relation",
		Description: "Delete an issue relation by its relation ID (see the 'relations' field from get_issue).",
		InputSchema: json.RawMessage(`{
			"type":"object",
			"properties":{
				"account":{"type":"string"},
				"relation_id":{"type":"string","description":"IssueRelation UUID"}
			},
			"required":["account","relation_id"],
			"additionalProperties":false
		}`),
		Handler: makeRemoveRelation(cfg),
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
		Name:        "add_project_update",
		Description: "Post a project update (status activity) to a Linear project. Optionally set health to onTrack, atRisk, or offTrack.",
		InputSchema: json.RawMessage(`{
			"type":"object",
			"properties":{
				"account":{"type":"string"},
				"project":{"type":"string","description":"Project UUID or exact project name"},
				"body":{"type":"string","description":"Update body (markdown)"},
				"health":{"type":"string","enum":["onTrack","atRisk","offTrack"],"description":"Project health (optional)"}
			},
			"required":["account","project","body"],
			"additionalProperties":false
		}`),
		Handler: makeAddProjectUpdate(cfg),
	})

	s.Register(mcp.Tool{
		Name:        "list_project_updates",
		Description: "List project updates (status activity feed) for a Linear project.",
		InputSchema: json.RawMessage(`{
			"type":"object",
			"properties":{
				"account":{"type":"string"},
				"project":{"type":"string","description":"Project UUID or exact project name"},
				"limit":{"type":"integer","default":50,"maximum":100}
			},
			"required":["account","project"],
			"additionalProperties":false
		}`),
		Handler: makeListProjectUpdates(cfg),
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
