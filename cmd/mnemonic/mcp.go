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

The --db flag is now optional. If not provided, a system-specific default location will be used:
- Windows: %USERPROFILE%\AppData\Roaming\recall\recall.db
- macOS: ~/Library/Application Support/recall/recall.db
- Linux: ~/.local/share/recall/recall.db

Example:

  mnemonic mcp --db mnemonic.db | tee server.log
  
  # Or simply use the default location:
  mnemonic mcp`,
	RunE: func(cmd *cobra.Command, args []string) error {

		// Create server wrapper.
		srv, resolvedDbPath, err := mcp.NewMnemonicMCPServer(dbPath)
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

		effectiveDbPath := dbPath
		if effectiveDbPath == "" {
			effectiveDbPath = resolvedDbPath
		}

		// Log to stderr so we don't contaminate the JSON-RPC stream on stdout.
		fmt.Fprintf(os.Stderr, "Mnemonic MCP server started. DB: %s\n", effectiveDbPath)
		fmt.Fprintln(os.Stderr, "Available tools: ping, create_journal, list_journals, get_journal, update_journal, delete_journal, create_entry, list_entries, get_entry, update_entry, delete_entry, manage_entry_tags, list_tags, search_entries")
		fmt.Fprintln(os.Stderr, "Listening for MCP JSON-RPC on STDIN/STDOUT ... (Ctrl+C to quit)")

		// Run the server (blocks until stdio closes).
		return srv.Start()
	},
}
