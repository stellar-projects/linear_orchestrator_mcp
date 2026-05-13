package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/serdarcoskun/linear-orchestrator/internal/config"
	"github.com/serdarcoskun/linear-orchestrator/internal/linear"
)

type baseArgs struct {
	Account string `json:"account"`
}

func makeListIssues(cfg *config.Config) func(context.Context, json.RawMessage) (string, error) {
	return func(ctx context.Context, raw json.RawMessage) (string, error) {
		var a struct {
			baseArgs
			Team     string `json:"team"`
			Project  string `json:"project"`
			Assignee string `json:"assignee"`
			State    string `json:"state"`
			Limit    int    `json:"limit"`
		}
		if err := json.Unmarshal(raw, &a); err != nil {
			return "", err
		}
		if a.Limit == 0 {
			a.Limit = 25
		}
		c, err := clientFor(cfg, a.Account)
		if err != nil {
			return "", err
		}
		filter := map[string]any{}
		if a.Team != "" {
			if isUUID(a.Team) {
				filter["team"] = map[string]any{"id": map[string]any{"eq": a.Team}}
			} else {
				filter["team"] = map[string]any{"key": map[string]any{"eq": a.Team}}
			}
		}
		if a.Project != "" {
			filter["project"] = map[string]any{"id": map[string]any{"eq": a.Project}}
		}
		if a.Assignee != "" {
			if strings.Contains(a.Assignee, "@") {
				filter["assignee"] = map[string]any{"email": map[string]any{"eq": a.Assignee}}
			} else {
				filter["assignee"] = map[string]any{"id": map[string]any{"eq": a.Assignee}}
			}
		}
		if a.State != "" {
			filter["state"] = map[string]any{"name": map[string]any{"eq": a.State}}
		}

		query := `
			query Issues($filter: IssueFilter, $first: Int) {
				issues(filter: $filter, first: $first) {
					nodes {
						id identifier title url
						state { name type }
						assignee { name email }
						team { key name }
						project { name id }
						priority
						createdAt updatedAt
					}
				}
			}`
		var out struct {
			Issues struct {
				Nodes []map[string]any `json:"nodes"`
			} `json:"issues"`
		}
		err = c.Query(ctx, query, map[string]any{"filter": filter, "first": a.Limit}, &out)
		if err != nil {
			return "", err
		}
		return jsonStr(out.Issues.Nodes), nil
	}
}

func makeGetIssue(cfg *config.Config) func(context.Context, json.RawMessage) (string, error) {
	return func(ctx context.Context, raw json.RawMessage) (string, error) {
		var a struct {
			baseArgs
			ID string `json:"id"`
		}
		if err := json.Unmarshal(raw, &a); err != nil {
			return "", err
		}
		c, err := clientFor(cfg, a.Account)
		if err != nil {
			return "", err
		}
		query := `
			query Issue($id: String!) {
				issue(id: $id) {
					id identifier title description url
					state { name type }
					assignee { name email }
					team { key name }
					project { name id }
					priority createdAt updatedAt
				}
			}`
		var out struct {
			Issue map[string]any `json:"issue"`
		}
		err = c.Query(ctx, query, map[string]any{"id": a.ID}, &out)
		if err != nil {
			return "", err
		}
		if out.Issue == nil {
			return "", fmt.Errorf("issue not found: %s", a.ID)
		}
		return jsonStr(out.Issue), nil
	}
}

func makeCreateIssue(cfg *config.Config) func(context.Context, json.RawMessage) (string, error) {
	return func(ctx context.Context, raw json.RawMessage) (string, error) {
		var a struct {
			baseArgs
			Team        string `json:"team"`
			Title       string `json:"title"`
			Description string `json:"description"`
			Assignee    string `json:"assignee"`
			Project     string `json:"project"`
		}
		if err := json.Unmarshal(raw, &a); err != nil {
			return "", err
		}
		c, err := clientFor(cfg, a.Account)
		if err != nil {
			return "", err
		}
		teamID, err := resolveTeamID(ctx, c, a.Team)
		if err != nil {
			return "", err
		}
		input := map[string]any{
			"teamId": teamID,
			"title":  a.Title,
		}
		if a.Description != "" {
			input["description"] = a.Description
		}
		if a.Project != "" {
			input["projectId"] = a.Project
		}
		if a.Assignee != "" {
			uid, err := resolveUserID(ctx, c, a.Assignee)
			if err != nil {
				return "", err
			}
			input["assigneeId"] = uid
		}
		query := `
			mutation Create($input: IssueCreateInput!) {
				issueCreate(input: $input) {
					success
					issue { id identifier title url }
				}
			}`
		var out struct {
			IssueCreate struct {
				Success bool           `json:"success"`
				Issue   map[string]any `json:"issue"`
			} `json:"issueCreate"`
		}
		err = c.Query(ctx, query, map[string]any{"input": input}, &out)
		if err != nil {
			return "", err
		}
		if !out.IssueCreate.Success {
			return "", fmt.Errorf("issueCreate returned success=false")
		}
		return jsonStr(out.IssueCreate.Issue), nil
	}
}

func makeUpdateIssue(cfg *config.Config) func(context.Context, json.RawMessage) (string, error) {
	return func(ctx context.Context, raw json.RawMessage) (string, error) {
		var a struct {
			baseArgs
			ID          string `json:"id"`
			Title       string `json:"title"`
			Description string `json:"description"`
			State       string `json:"state"`
			Assignee    string `json:"assignee"`
		}
		if err := json.Unmarshal(raw, &a); err != nil {
			return "", err
		}
		c, err := clientFor(cfg, a.Account)
		if err != nil {
			return "", err
		}
		issueID, teamID, err := resolveIssue(ctx, c, a.ID)
		if err != nil {
			return "", err
		}
		input := map[string]any{}
		if a.Title != "" {
			input["title"] = a.Title
		}
		if a.Description != "" {
			input["description"] = a.Description
		}
		if a.Assignee != "" {
			uid, err := resolveUserID(ctx, c, a.Assignee)
			if err != nil {
				return "", err
			}
			input["assigneeId"] = uid
		}
		if a.State != "" {
			sid, err := resolveStateID(ctx, c, teamID, a.State)
			if err != nil {
				return "", err
			}
			input["stateId"] = sid
		}
		if len(input) == 0 {
			return "", fmt.Errorf("nothing to update")
		}
		query := `
			mutation Update($id: String!, $input: IssueUpdateInput!) {
				issueUpdate(id: $id, input: $input) {
					success
					issue { id identifier title url state { name } assignee { name email } }
				}
			}`
		var out struct {
			IssueUpdate struct {
				Success bool           `json:"success"`
				Issue   map[string]any `json:"issue"`
			} `json:"issueUpdate"`
		}
		err = c.Query(ctx, query, map[string]any{"id": issueID, "input": input}, &out)
		if err != nil {
			return "", err
		}
		if !out.IssueUpdate.Success {
			return "", fmt.Errorf("issueUpdate returned success=false")
		}
		return jsonStr(out.IssueUpdate.Issue), nil
	}
}

func makeAddComment(cfg *config.Config) func(context.Context, json.RawMessage) (string, error) {
	return func(ctx context.Context, raw json.RawMessage) (string, error) {
		var a struct {
			baseArgs
			ID   string `json:"id"`
			Body string `json:"body"`
		}
		if err := json.Unmarshal(raw, &a); err != nil {
			return "", err
		}
		c, err := clientFor(cfg, a.Account)
		if err != nil {
			return "", err
		}
		issueID, _, err := resolveIssue(ctx, c, a.ID)
		if err != nil {
			return "", err
		}
		query := `
			mutation Comment($input: CommentCreateInput!) {
				commentCreate(input: $input) {
					success
					comment { id body url createdAt user { name email } }
				}
			}`
		var out struct {
			CommentCreate struct {
				Success bool           `json:"success"`
				Comment map[string]any `json:"comment"`
			} `json:"commentCreate"`
		}
		input := map[string]any{"issueId": issueID, "body": a.Body}
		err = c.Query(ctx, query, map[string]any{"input": input}, &out)
		if err != nil {
			return "", err
		}
		if !out.CommentCreate.Success {
			return "", fmt.Errorf("commentCreate returned success=false")
		}
		return jsonStr(out.CommentCreate.Comment), nil
	}
}

func makeListComments(cfg *config.Config) func(context.Context, json.RawMessage) (string, error) {
	return func(ctx context.Context, raw json.RawMessage) (string, error) {
		var a struct {
			baseArgs
			ID    string `json:"id"`
			Limit int    `json:"limit"`
		}
		if err := json.Unmarshal(raw, &a); err != nil {
			return "", err
		}
		if a.Limit == 0 {
			a.Limit = 50
		}
		c, err := clientFor(cfg, a.Account)
		if err != nil {
			return "", err
		}
		query := `
			query IssueComments($id: String!, $first: Int) {
				issue(id: $id) {
					comments(first: $first) {
						nodes {
							id body url createdAt updatedAt
							user { name email }
						}
					}
				}
			}`
		var out struct {
			Issue struct {
				Comments struct {
					Nodes []map[string]any `json:"nodes"`
				} `json:"comments"`
			} `json:"issue"`
		}
		err = c.Query(ctx, query, map[string]any{"id": a.ID, "first": a.Limit}, &out)
		if err != nil {
			return "", err
		}
		return jsonStr(out.Issue.Comments.Nodes), nil
	}
}

func makeAddProjectUpdate(cfg *config.Config) func(context.Context, json.RawMessage) (string, error) {
	return func(ctx context.Context, raw json.RawMessage) (string, error) {
		var a struct {
			baseArgs
			Project string `json:"project"`
			Body    string `json:"body"`
			Health  string `json:"health"`
		}
		if err := json.Unmarshal(raw, &a); err != nil {
			return "", err
		}
		c, err := clientFor(cfg, a.Account)
		if err != nil {
			return "", err
		}
		input := map[string]any{
			"projectId": a.Project,
			"body":      a.Body,
		}
		if a.Health != "" {
			switch a.Health {
			case "onTrack", "atRisk", "offTrack":
				input["health"] = a.Health
			default:
				return "", fmt.Errorf("invalid health: %s (want onTrack|atRisk|offTrack)", a.Health)
			}
		}
		query := `
			mutation ProjectUpdateCreate($input: ProjectUpdateCreateInput!) {
				projectUpdateCreate(input: $input) {
					success
					projectUpdate {
						id body url health createdAt
						user { name email }
						project { id name }
					}
				}
			}`
		var out struct {
			ProjectUpdateCreate struct {
				Success       bool           `json:"success"`
				ProjectUpdate map[string]any `json:"projectUpdate"`
			} `json:"projectUpdateCreate"`
		}
		if err := c.Query(ctx, query, map[string]any{"input": input}, &out); err != nil {
			return "", err
		}
		if !out.ProjectUpdateCreate.Success {
			return "", fmt.Errorf("projectUpdateCreate returned success=false")
		}
		return jsonStr(out.ProjectUpdateCreate.ProjectUpdate), nil
	}
}

func makeListProjectUpdates(cfg *config.Config) func(context.Context, json.RawMessage) (string, error) {
	return func(ctx context.Context, raw json.RawMessage) (string, error) {
		var a struct {
			baseArgs
			Project string `json:"project"`
			Limit   int    `json:"limit"`
		}
		if err := json.Unmarshal(raw, &a); err != nil {
			return "", err
		}
		if a.Limit == 0 {
			a.Limit = 50
		}
		c, err := clientFor(cfg, a.Account)
		if err != nil {
			return "", err
		}
		query := `
			query ProjectUpdates($id: String!, $first: Int) {
				project(id: $id) {
					projectUpdates(first: $first) {
						nodes {
							id body url health createdAt updatedAt
							user { name email }
						}
					}
				}
			}`
		var out struct {
			Project struct {
				ProjectUpdates struct {
					Nodes []map[string]any `json:"nodes"`
				} `json:"projectUpdates"`
			} `json:"project"`
		}
		if err := c.Query(ctx, query, map[string]any{"id": a.Project, "first": a.Limit}, &out); err != nil {
			return "", err
		}
		return jsonStr(out.Project.ProjectUpdates.Nodes), nil
	}
}

func makeListProjects(cfg *config.Config) func(context.Context, json.RawMessage) (string, error) {
	return func(ctx context.Context, raw json.RawMessage) (string, error) {
		var a struct {
			baseArgs
			Team  string `json:"team"`
			Limit int    `json:"limit"`
		}
		if err := json.Unmarshal(raw, &a); err != nil {
			return "", err
		}
		if a.Limit == 0 {
			a.Limit = 50
		}
		c, err := clientFor(cfg, a.Account)
		if err != nil {
			return "", err
		}
		filter := map[string]any{}
		if a.Team != "" {
			if isUUID(a.Team) {
				filter["accessibleTeams"] = map[string]any{"some": map[string]any{"id": map[string]any{"eq": a.Team}}}
			} else {
				filter["accessibleTeams"] = map[string]any{"some": map[string]any{"key": map[string]any{"eq": a.Team}}}
			}
		}
		query := `
			query Projects($filter: ProjectFilter, $first: Int) {
				projects(filter: $filter, first: $first) {
					nodes {
						id name description url state
						lead { name email }
						teams { nodes { key name } }
					}
				}
			}`
		var out struct {
			Projects struct {
				Nodes []map[string]any `json:"nodes"`
			} `json:"projects"`
		}
		err = c.Query(ctx, query, map[string]any{"filter": filter, "first": a.Limit}, &out)
		if err != nil {
			return "", err
		}
		return jsonStr(out.Projects.Nodes), nil
	}
}

func makeListTeams(cfg *config.Config) func(context.Context, json.RawMessage) (string, error) {
	return func(ctx context.Context, raw json.RawMessage) (string, error) {
		var a baseArgs
		if err := json.Unmarshal(raw, &a); err != nil {
			return "", err
		}
		c, err := clientFor(cfg, a.Account)
		if err != nil {
			return "", err
		}
		query := `
			query Teams {
				teams(first: 100) {
					nodes { id key name description }
				}
			}`
		var out struct {
			Teams struct {
				Nodes []map[string]any `json:"nodes"`
			} `json:"teams"`
		}
		err = c.Query(ctx, query, nil, &out)
		if err != nil {
			return "", err
		}
		return jsonStr(out.Teams.Nodes), nil
	}
}

func resolveTeamID(ctx context.Context, c *linear.Client, teamRef string) (string, error) {
	if isUUID(teamRef) {
		return teamRef, nil
	}
	query := `
		query TeamByKey($key: String!) {
			teams(filter: { key: { eq: $key } }, first: 1) {
				nodes { id }
			}
		}`
	var out struct {
		Teams struct {
			Nodes []struct {
				ID string `json:"id"`
			} `json:"nodes"`
		} `json:"teams"`
	}
	if err := c.Query(ctx, query, map[string]any{"key": teamRef}, &out); err != nil {
		return "", err
	}
	if len(out.Teams.Nodes) == 0 {
		return "", fmt.Errorf("team not found: %s", teamRef)
	}
	return out.Teams.Nodes[0].ID, nil
}

func resolveUserID(ctx context.Context, c *linear.Client, ref string) (string, error) {
	if isUUID(ref) {
		return ref, nil
	}
	query := `
		query UserByEmail($email: String!) {
			users(filter: { email: { eq: $email } }, first: 1) {
				nodes { id }
			}
		}`
	var out struct {
		Users struct {
			Nodes []struct {
				ID string `json:"id"`
			} `json:"nodes"`
		} `json:"users"`
	}
	if err := c.Query(ctx, query, map[string]any{"email": ref}, &out); err != nil {
		return "", err
	}
	if len(out.Users.Nodes) == 0 {
		return "", fmt.Errorf("user not found: %s", ref)
	}
	return out.Users.Nodes[0].ID, nil
}

func resolveIssue(ctx context.Context, c *linear.Client, ref string) (id, teamID string, err error) {
	query := `
		query IssueRef($id: String!) {
			issue(id: $id) { id team { id } }
		}`
	var out struct {
		Issue struct {
			ID   string `json:"id"`
			Team struct {
				ID string `json:"id"`
			} `json:"team"`
		} `json:"issue"`
	}
	if err := c.Query(ctx, query, map[string]any{"id": ref}, &out); err != nil {
		return "", "", err
	}
	if out.Issue.ID == "" {
		return "", "", fmt.Errorf("issue not found: %s", ref)
	}
	return out.Issue.ID, out.Issue.Team.ID, nil
}

func resolveStateID(ctx context.Context, c *linear.Client, teamID, stateName string) (string, error) {
	query := `
		query State($teamId: ID!, $name: String!) {
			workflowStates(filter: { team: { id: { eq: $teamId } }, name: { eq: $name } }, first: 1) {
				nodes { id }
			}
		}`
	var out struct {
		WorkflowStates struct {
			Nodes []struct {
				ID string `json:"id"`
			} `json:"nodes"`
		} `json:"workflowStates"`
	}
	if err := c.Query(ctx, query, map[string]any{"teamId": teamID, "name": stateName}, &out); err != nil {
		return "", err
	}
	if len(out.WorkflowStates.Nodes) == 0 {
		return "", fmt.Errorf("state not found in team: %s", stateName)
	}
	return out.WorkflowStates.Nodes[0].ID, nil
}

func isUUID(s string) bool {
	if len(s) != 36 {
		return false
	}
	for i, ch := range s {
		if i == 8 || i == 13 || i == 18 || i == 23 {
			if ch != '-' {
				return false
			}
			continue
		}
		if !((ch >= '0' && ch <= '9') || (ch >= 'a' && ch <= 'f') || (ch >= 'A' && ch <= 'F')) {
			return false
		}
	}
	return true
}
