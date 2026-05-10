package mcp

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"sync"
)

const protocolVersion = "2024-11-05"

type Tool struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	InputSchema json.RawMessage `json:"inputSchema"`
	Handler     ToolHandler     `json:"-"`
}

type ToolHandler func(ctx context.Context, args json.RawMessage) (string, error)

type Server struct {
	name    string
	version string
	tools   map[string]Tool
	in      *bufio.Reader
	out     io.Writer
	mu      sync.Mutex
}

func NewServer(name, version string) *Server {
	return &Server{
		name:    name,
		version: version,
		tools:   map[string]Tool{},
		in:      bufio.NewReader(os.Stdin),
		out:     os.Stdout,
	}
}

func (s *Server) Register(t Tool) {
	s.tools[t.Name] = t
}

type rpcMsg struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Method  string          `json:"method,omitempty"`
	Params  json.RawMessage `json:"params,omitempty"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *rpcErr         `json:"error,omitempty"`
}

type rpcErr struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

func (s *Server) Run(ctx context.Context) error {
	for {
		line, err := s.in.ReadBytes('\n')
		if err != nil {
			if err == io.EOF {
				return nil
			}
			return err
		}
		if len(line) == 0 {
			continue
		}
		var msg rpcMsg
		if err := json.Unmarshal(line, &msg); err != nil {
			s.writeErr(nil, -32700, "parse error: "+err.Error())
			continue
		}
		s.handle(ctx, &msg)
	}
}

func (s *Server) handle(ctx context.Context, m *rpcMsg) {
	switch m.Method {
	case "initialize":
		s.writeResult(m.ID, map[string]any{
			"protocolVersion": protocolVersion,
			"capabilities": map[string]any{
				"tools": map[string]any{},
			},
			"serverInfo": map[string]any{
				"name":    s.name,
				"version": s.version,
			},
		})
	case "notifications/initialized", "notifications/cancelled":
		// no response for notifications
	case "ping":
		s.writeResult(m.ID, map[string]any{})
	case "tools/list":
		list := make([]map[string]any, 0, len(s.tools))
		for _, t := range s.tools {
			list = append(list, map[string]any{
				"name":        t.Name,
				"description": t.Description,
				"inputSchema": t.InputSchema,
			})
		}
		s.writeResult(m.ID, map[string]any{"tools": list})
	case "tools/call":
		var p struct {
			Name      string          `json:"name"`
			Arguments json.RawMessage `json:"arguments"`
		}
		if err := json.Unmarshal(m.Params, &p); err != nil {
			s.writeErr(m.ID, -32602, "invalid params: "+err.Error())
			return
		}
		t, ok := s.tools[p.Name]
		if !ok {
			s.writeErr(m.ID, -32601, "unknown tool: "+p.Name)
			return
		}
		text, err := t.Handler(ctx, p.Arguments)
		if err != nil {
			s.writeResult(m.ID, map[string]any{
				"content": []map[string]any{{"type": "text", "text": "error: " + err.Error()}},
				"isError": true,
			})
			return
		}
		s.writeResult(m.ID, map[string]any{
			"content": []map[string]any{{"type": "text", "text": text}},
		})
	default:
		if m.ID != nil {
			s.writeErr(m.ID, -32601, "method not found: "+m.Method)
		}
	}
}

func (s *Server) writeResult(id json.RawMessage, result any) {
	raw, err := json.Marshal(result)
	if err != nil {
		s.writeErr(id, -32603, "marshal: "+err.Error())
		return
	}
	s.write(rpcMsg{JSONRPC: "2.0", ID: id, Result: raw})
}

func (s *Server) writeErr(id json.RawMessage, code int, msg string) {
	s.write(rpcMsg{JSONRPC: "2.0", ID: id, Error: &rpcErr{Code: code, Message: msg}})
}

func (s *Server) write(m rpcMsg) {
	s.mu.Lock()
	defer s.mu.Unlock()
	m.JSONRPC = "2.0"
	data, err := json.Marshal(m)
	if err != nil {
		fmt.Fprintf(os.Stderr, "marshal err: %v\n", err)
		return
	}
	data = append(data, '\n')
	s.out.Write(data)
}
