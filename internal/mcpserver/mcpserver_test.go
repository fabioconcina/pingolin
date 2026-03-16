package mcpserver

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/mark3labs/mcp-go/client"
	"github.com/mark3labs/mcp-go/mcp"

	"github.com/fabioconcina/pingolin/internal/store"
)

func TestMCPServer_InitializeAndListTools(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.db")
	s, err := store.New(dbPath)
	if err != nil {
		t.Fatalf("creating store: %v", err)
	}
	defer s.Close()
	defer os.Remove(dbPath)

	srv := NewServer(s, "0.0.1-test", []string{"8.8.8.8"})

	c, err := client.NewInProcessClient(srv)
	if err != nil {
		t.Fatalf("creating client: %v", err)
	}

	ctx := context.Background()
	if err := c.Start(ctx); err != nil {
		t.Fatalf("starting client: %v", err)
	}
	defer c.Close()

	// Initialize
	initResult, err := c.Initialize(ctx, mcp.InitializeRequest{
		Params: mcp.InitializeParams{
			ProtocolVersion: "2024-11-05",
			ClientInfo:      mcp.Implementation{Name: "test-client", Version: "0.0.1"},
		},
	})
	if err != nil {
		t.Fatalf("initialize: %v", err)
	}

	if initResult.ServerInfo.Name != "pingolin" {
		t.Errorf("server name = %q, want %q", initResult.ServerInfo.Name, "pingolin")
	}
	if initResult.ServerInfo.Version != "0.0.1-test" {
		t.Errorf("server version = %q, want %q", initResult.ServerInfo.Version, "0.0.1-test")
	}

	// List tools
	toolsResult, err := c.ListTools(ctx, mcp.ListToolsRequest{})
	if err != nil {
		t.Fatalf("list tools: %v", err)
	}

	if len(toolsResult.Tools) != 1 {
		t.Fatalf("got %d tools, want 1", len(toolsResult.Tools))
	}
	if toolsResult.Tools[0].Name != "check_connection" {
		t.Errorf("tool name = %q, want %q", toolsResult.Tools[0].Name, "check_connection")
	}
}

func TestMCPServer_CallTool(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.db")
	s, err := store.New(dbPath)
	if err != nil {
		t.Fatalf("creating store: %v", err)
	}
	defer s.Close()
	defer os.Remove(dbPath)

	srv := NewServer(s, "0.0.1-test", []string{"8.8.8.8"})

	c, err := client.NewInProcessClient(srv)
	if err != nil {
		t.Fatalf("creating client: %v", err)
	}

	ctx := context.Background()
	if err := c.Start(ctx); err != nil {
		t.Fatalf("starting client: %v", err)
	}
	defer c.Close()

	// Initialize first (required before calling tools)
	_, err = c.Initialize(ctx, mcp.InitializeRequest{
		Params: mcp.InitializeParams{
			ProtocolVersion: "2024-11-05",
			ClientInfo:      mcp.Implementation{Name: "test-client", Version: "0.0.1"},
		},
	})
	if err != nil {
		t.Fatalf("initialize: %v", err)
	}

	// Call the check_connection tool (empty DB, should return healthy with no data)
	result, err := c.CallTool(ctx, mcp.CallToolRequest{
		Params: mcp.CallToolParams{
			Name: "check_connection",
		},
	})
	if err != nil {
		t.Fatalf("call tool: %v", err)
	}

	if result.IsError {
		t.Error("tool returned error")
	}
	if len(result.Content) == 0 {
		t.Fatal("tool returned no content")
	}
}
