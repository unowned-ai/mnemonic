package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/unowned-ai/recall/pkg/mcp"
)

var mcpCmd = &cobra.Command{
	Use:   "mcp",
	Short: "Run the Recall MCP server (stdio)",
	Long: `Start a Model Context Protocol (MCP) server that exposes all recall
journals, entries, tags and search functionality as MCP tools via STDIO.

If the --memory-aware flag is provided, an additional tool named 'get_memory_overview'
will be registered. This tool is designed to be called by an LLM at the start of an
interaction to receive an overview of available journals.

The --db flag is optional. If not provided, a system-specific default location will be used:
- Windows: %USERPROFILE%\AppData\Roaming\recall\recall.db
- macOS: ~/Library/Application Support/recall/recall.db
- Linux: ~/.local/share/recall/recall.db

Example (Server Mode):
  recall mcp
  recall mcp --db recall.db

Example (Server with Memory Aware Tool active):
  recall mcp --memory-aware
  recall mcp --memory-aware --db /path/to/my.db`,
	RunE: func(cmd *cobra.Command, args []string) error {
		memoryAware, _ := cmd.Flags().GetBool("memory-aware")

		// Create server wrapper.
		srv, err := mcp.NewRecallMCPServer(dbPath, walMode, syncMode)
		if err != nil {
			return err
		}

		// Register all standard tools.
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

		// Conditionally register the memory overview tool
		if memoryAware {
			// This function will be created in pkg/mcp/
			mcp.RegisterMemoryOverviewTool(s, db)
			fmt.Fprintf(os.Stderr, "Memory Overview tool ('get_memory_overview') is active.\n")
		}

		// Log to stderr so we don't contaminate the JSON-RPC stream on stdout.
		fmt.Fprintf(os.Stderr, "Recall MCP server started. DB: %s (WAL: %t, Sync: %s)\n", srv.DbPath, walMode, syncMode)
		availableToolsMsg := "Available tools: ping, create_journal, list_journals, get_journal, update_journal, delete_journal, create_entry, list_entries, get_entry, update_entry, delete_entry, manage_entry_tags, list_tags, search_entries"
		if memoryAware {
			availableToolsMsg += ", get_memory_overview"
		}
		fmt.Fprintln(os.Stderr, availableToolsMsg)
		fmt.Fprintln(os.Stderr, "Listening for MCP JSON-RPC on STDIN/STDOUT ... (Ctrl+C to quit)")

		// Run the server (blocks until stdio closes).
		return srv.Start()
	},
}

func init() {
	mcpCmd.Flags().Bool("memory-aware", false, "If set, registers an additional 'get_memory_overview' tool for LLM initialization")
}
