package main

import (
	"fmt"

	"github.com/spf13/cobra"
	pkgmcp "github.com/unowned-ai/mnemonic/pkg/mcp"
)

// serverCmd starts the Mnemonic MCP server as part of the main CLI.
var serverCmd = &cobra.Command{
	Use:   "serve",
	Short: "Start the Mnemonic MCP server (stdio transport)",
	Long:  `Launches the MCP stdio server so that external AI agents can call Mnemonic tools.`,
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		if dbPath == "" {
			return fmt.Errorf("database path must be set using the --dbpath flag")
		}

		fmt.Println("Starting Mnemonic MCP Server…")
		fmt.Printf("Using database: %s\n", dbPath)

		// Build the server instance
		mcpServer, err := pkgmcp.NewMnemonicMCPServer(dbPath)
		if err != nil {
			return fmt.Errorf("failed to create Mnemonic MCP server: %w", err)
		}
		defer mcpServer.Close()

		// Register all available MCP tools
		pkgmcp.RegisterPingTool(mcpServer.MCPRawServer())
		pkgmcp.RegisterCreateJournalTool(mcpServer.MCPRawServer(), mcpServer.DB())
		pkgmcp.RegisterListJournalsTool(mcpServer.MCPRawServer(), mcpServer.DB())
		pkgmcp.RegisterGetJournalTool(mcpServer.MCPRawServer(), mcpServer.DB())
		pkgmcp.RegisterUpdateJournalTool(mcpServer.MCPRawServer(), mcpServer.DB())
		pkgmcp.RegisterDeleteJournalTool(mcpServer.MCPRawServer(), mcpServer.DB())
		// Entry tools
		pkgmcp.RegisterCreateEntryTool(mcpServer.MCPRawServer(), mcpServer.DB())
		pkgmcp.RegisterListEntriesTool(mcpServer.MCPRawServer(), mcpServer.DB())
		pkgmcp.RegisterGetEntryTool(mcpServer.MCPRawServer(), mcpServer.DB())
		pkgmcp.RegisterUpdateEntryTool(mcpServer.MCPRawServer(), mcpServer.DB())
		pkgmcp.RegisterDeleteEntryTool(mcpServer.MCPRawServer(), mcpServer.DB())
		pkgmcp.RegisterManageEntryTagsTool(mcpServer.MCPRawServer(), mcpServer.DB())
		// Tag tools
		pkgmcp.RegisterListTagsTool(mcpServer.MCPRawServer(), mcpServer.DB())
		// Search tools
		pkgmcp.RegisterSearchEntriesTool(mcpServer.MCPRawServer(), mcpServer.DB())

		fmt.Println("Mnemonic MCP Server tools registered. Starting stdio listener…")
		if err := mcpServer.Start(); err != nil {
			return fmt.Errorf("Mnemonic MCP server error: %w", err)
		}

		fmt.Println("Mnemonic MCP Server stopped.")
		return nil
	},
}
