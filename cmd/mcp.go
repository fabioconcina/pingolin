package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/fabioconcina/pingolin/internal/mcpserver"
	"github.com/fabioconcina/pingolin/internal/store"
)

var mcpCmd = &cobra.Command{
	Use:   "mcp",
	Short: "Run as MCP server (stdio)",
	RunE:  runMCP,
}

func runMCP(cmd *cobra.Command, args []string) error {
	cfg, err := loadConfig()
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}

	s, err := store.New(cfg.Storage.Path)
	if err != nil {
		return fmt.Errorf("opening database: %w", err)
	}
	defer s.Close()

	return mcpserver.Run(s, version, cfg.Targets.ICMP)
}
