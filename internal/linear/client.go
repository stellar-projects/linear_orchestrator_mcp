package linear

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

const endpoint = "https://api.linear.app/graphql"

type Client struct {
	token string
	http  *http.Client
}

func New(token string) *Client {
	return &Client{
		token: token,
		http:  &http.Client{Timeout: 30 * time.Second},
	}
}

type gqlReq struct {
	Query     string         `json:"query"`
	Variables map[string]any `json:"variables,omitempty"`
}

type gqlErr struct {
	Message string `json:"message"`
}

type gqlResp struct {
	Data   json.RawMessage `json:"data"`
	Errors []gqlErr        `json:"errors,omitempty"`
}

func (c *Client) Query(ctx context.Context, query string, vars map[string]any, out any) error {
	body, err := json.Marshal(gqlReq{Query: query, Variables: vars})
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", c.token)

	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	if resp.StatusCode >= 400 {
		return fmt.Errorf("linear http %d: %s", resp.StatusCode, string(raw))
	}
	var r gqlResp
	if err := json.Unmarshal(raw, &r); err != nil {
		return fmt.Errorf("parse response: %w; body=%s", err, string(raw))
	}
	if len(r.Errors) > 0 {
		msgs := ""
		for i, e := range r.Errors {
			if i > 0 {
				msgs += "; "
			}
			msgs += e.Message
		}
		return fmt.Errorf("linear graphql: %s", msgs)
	}
	if out != nil {
		return json.Unmarshal(r.Data, out)
	}
	return nil
}
