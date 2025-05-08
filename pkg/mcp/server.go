package mcp

import (
	"database/sql"
	"fmt"

	"github.com/mark3labs/mcp-go/server"
	mnemonicpkg "github.com/unowned-ai/mnemonic/pkg" // Renamed to avoid conflict with package name
	pkgdb "github.com/unowned-ai/mnemonic/pkg/db"
)

// MnemonicMCPServer wraps the MCP server with Mnemonic-specific functionalities.
type MnemonicMCPServer struct {
	mcpServer *server.MCPServer
	db        *sql.DB // Database connection
	dbPath    string  // Path to the database file
}

// NewMnemonicMCPServer creates and initializes a new MnemonicMCPServer.
// It takes the database path as configuration.
func NewMnemonicMCPServer(dbPath string) (*MnemonicMCPServer, error) {
	if dbPath == "" {
		return nil, fmt.Errorf("database path cannot be empty for MnemonicMCPServer")
	}

	// Initialize MCP server
	// TODO: Consider making server name and version configurable or constants.
	s := server.NewMCPServer(
		"Mnemonic MCP Server",
		mnemonicpkg.Version,                         // Use the actual package version
		server.WithResourceCapabilities(true, true), // Example capabilities
		server.WithLogging(),                        // Enable default logging
		server.WithRecovery(),                       // Enable default panic recovery
	)

	// Open DB connection (assuming WAL and NORMAL sync mode as defaults for server ops)
	// These could be made configurable if needed.
	dbConn, err := pkgdb.OpenDBConnection(dbPath, true, "NORMAL")
	if err != nil {
		return nil, fmt.Errorf("failed to open database connection for MCP server: %w", err)
	}

	// It's important to also run migrations if the DB is new or needs upgrade.
	// The CLI's 'db upgrade' command does this.
	// For an MCP server, we might want to ensure the DB is usable.
	// This could be a separate initialization step or integrated here.
	// For now, we assume the DB is correctly initialized/migrated externally or by a prior step.
	// Consider adding:
	// if err := pkgdb.UpgradeDB(dbConn, dbPath, pkgdb.TargetSchemaVersion); err != nil {
	// 	dbConn.Close() // Close on error
	// 	return nil, fmt.Errorf("failed to upgrade database for MCP server: %w", err)
	// }

	srv := &MnemonicMCPServer{
		mcpServer: s,
		db:        dbConn,
		dbPath:    dbPath,
	}

	// Tool registration will be done after server creation, similar to the example.
	// e.g., RegisterPingTool(srv.mcpServer)
	// e.g., RegisterListJournalsTool(srv.mcpServer, srv.db)

	return srv, nil
}

// Start runs the MCP server in stdio mode.
func (s *MnemonicMCPServer) Start() error {
	// Note: The MCP server should have its tools registered before starting.
	return server.ServeStdio(s.mcpServer)
}

// DB returns the underlying SQL DB connection.
// Tool handlers might need this to interact with pkg/memories.
func (s *MnemonicMCPServer) DB() *sql.DB {
	return s.db
}

// MCPRawServer returns the underlying mcp-go server instance.
// This is needed for tool registration from other packages (e.g. handlers).
func (s *MnemonicMCPServer) MCPRawServer() *server.MCPServer {
	return s.mcpServer
}

// Close performs cleanup, like closing the database connection.
func (s *MnemonicMCPServer) Close() error {
	if s.db != nil {
		return s.db.Close()
	}
	return nil
}
