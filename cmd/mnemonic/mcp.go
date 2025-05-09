package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/unowned-ai/mnemonic/pkg/mcp"
)

var mcpCmd = &cobra.Command{
	Use:   "mcp",
	Short: "Run the Mnemonic MCP server (stdio)",
	Long: `Start a Model Context Protocol (MCP) server that exposes all mnemonic
journals, entries, tags and search functionality as MCP tools via STDIO.

Example:

  mnemonic mcp --db mnemonic.db | tee server.log`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if dbPath == "" {
			return fmt.Errorf("database path is required (use --db flag)")
		}

		// Create server wrapper.
		srv, err := mcp.NewMnemonicMCPServer(dbPath)
		if err != nil {
			return err
		}

		// Register all tools.
		db := srv.DB()
		s := srv.MCPRawServer()

		mcp.RegisterPingTool(s)
		mcp.RegisterCreateJournalTool(s, db)
		mcp.RegisterListJournalsTool(s, db)
		mcp.RegisterGetJournalTool(s, db)
		mcp.RegisterUpdateJournalTool(s, db)
		mcp.RegisterDeleteJournalTool(s, db)

		mcp.RegisterCreateEntryTool(s, db)
		mcp.RegisterListEntriesTool(s, db)
		mcp.RegisterGetEntryTool(s, db)
		mcp.RegisterUpdateEntryTool(s, db)
		mcp.RegisterDeleteEntryTool(s, db)
		mcp.RegisterManageEntryTagsTool(s, db)
		mcp.RegisterListTagsTool(s, db)
		mcp.RegisterSearchEntriesTool(s, db)

		// Log to stderr so we don't contaminate the JSON-RPC stream on stdout.
		fmt.Fprintf(os.Stderr, "Mnemonic MCP server started. DB: %s\n", dbPath)
		fmt.Fprintln(os.Stderr, "Available tools: ping, create_journal, list_journals, get_journal, update_journal, delete_journal, create_entry, list_entries, get_entry, update_entry, delete_entry, manage_entry_tags, list_tags, search_entries")
		fmt.Fprintln(os.Stderr, "Listening for MCP JSON-RPC on STDIN/STDOUT ... (Ctrl+C to quit)")

		// Run the server (blocks until stdio closes).
		return srv.Start()
	},
}
