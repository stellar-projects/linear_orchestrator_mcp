package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"sort"
	"strings"
	"syscall"

	"github.com/serdarcoskun/linear-orchestrator/internal/config"
	"github.com/serdarcoskun/linear-orchestrator/internal/mcp"
	"github.com/serdarcoskun/linear-orchestrator/internal/tools"
)

const version = "0.2.0"

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(2)
	}
	switch os.Args[1] {
	case "add":
		cmdAdd(os.Args[2:])
	case "list":
		cmdList()
	case "remove", "rm":
		cmdRemove(os.Args[2:])
	case "serve":
		cmdServe()
	case "path":
		p, err := config.Path()
		check(err)
		fmt.Println(p)
	case "-h", "--help", "help":
		usage()
	default:
		fmt.Fprintf(os.Stderr, "unknown command: %s\n", os.Args[1])
		usage()
		os.Exit(2)
	}
}

func usage() {
	fmt.Fprintf(os.Stderr, `linear-orch %s — multi-account Linear MCP gateway

Usage:
  linear-orch add <account> --token=<key> [--note=<text>]
  linear-orch add <account>            (reads token from stdin)
  linear-orch list
  linear-orch remove <account>
  linear-orch serve                    (run MCP server over stdio)
  linear-orch path                     (print config file path)

Config: ~/Library/Application Support/linear-orchestrator/config.json (macOS)
        ~/.config/linear-orchestrator/config.json (Linux)
`, version)
}

func check(err error) {
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func cmdAdd(args []string) {
	if len(args) < 1 {
		fmt.Fprintln(os.Stderr, "usage: linear-orch add <account> [--token=<key>] [--note=<text>]")
		os.Exit(2)
	}
	name := args[0]
	var token, note string
	for _, a := range args[1:] {
		switch {
		case strings.HasPrefix(a, "--token="):
			token = strings.TrimPrefix(a, "--token=")
		case strings.HasPrefix(a, "--note="):
			note = strings.TrimPrefix(a, "--note=")
		default:
			fmt.Fprintf(os.Stderr, "unknown flag: %s\n", a)
			os.Exit(2)
		}
	}
	if token == "" {
		fmt.Fprint(os.Stderr, "token: ")
		var t string
		fmt.Scanln(&t)
		token = strings.TrimSpace(t)
	}
	if token == "" {
		fmt.Fprintln(os.Stderr, "token required")
		os.Exit(2)
	}
	cfg, err := config.Load()
	check(err)
	cfg.Accounts[name] = config.Account{Token: token, Note: note}
	check(cfg.Save())
	fmt.Printf("saved account %q\n", name)
}

func cmdList() {
	cfg, err := config.Load()
	check(err)
	if len(cfg.Accounts) == 0 {
		fmt.Println("(no accounts configured)")
		return
	}
	names := make([]string, 0, len(cfg.Accounts))
	for k := range cfg.Accounts {
		names = append(names, k)
	}
	sort.Strings(names)
	for _, n := range names {
		a := cfg.Accounts[n]
		mask := maskToken(a.Token)
		if a.Note != "" {
			fmt.Printf("%-20s  %s  (%s)\n", n, mask, a.Note)
		} else {
			fmt.Printf("%-20s  %s\n", n, mask)
		}
	}
}

func cmdRemove(args []string) {
	if len(args) < 1 {
		fmt.Fprintln(os.Stderr, "usage: linear-orch remove <account>")
		os.Exit(2)
	}
	name := args[0]
	cfg, err := config.Load()
	check(err)
	if _, ok := cfg.Accounts[name]; !ok {
		fmt.Fprintf(os.Stderr, "no such account: %s\n", name)
		os.Exit(1)
	}
	delete(cfg.Accounts, name)
	check(cfg.Save())
	fmt.Printf("removed %q\n", name)
}

func cmdServe() {
	cfg, err := config.Load()
	check(err)
	srv := mcp.NewServer("linear-orchestrator", version)
	tools.Register(srv, cfg)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigs
		cancel()
	}()
	if err := srv.Run(ctx); err != nil {
		fmt.Fprintln(os.Stderr, "serve error:", err)
		os.Exit(1)
	}
}

func maskToken(t string) string {
	if len(t) <= 8 {
		return strings.Repeat("*", len(t))
	}
	return t[:4] + strings.Repeat("*", 8) + t[len(t)-4:]
}
