package mcp

import (
	"database/sql"
	"fmt"

	"github.com/mark3labs/mcp-go/server"
	mnemonicpkg "github.com/unowned-ai/mnemonic/pkg"
	pkgdb "github.com/unowned-ai/mnemonic/pkg/db"
)

// MnemonicMCPServer wraps the MCP server with a database handle.
type MnemonicMCPServer struct {
	mcpServer *server.MCPServer
	db        *sql.DB
	dbPath    string
}

// NewMnemonicMCPServer spins up an MCP server backed by the SQLite database at dbPath.
func NewMnemonicMCPServer(dbPath string) (*MnemonicMCPServer, error) {
	if dbPath == "" {
		return nil, fmt.Errorf("database path cannot be empty for MnemonicMCPServer")
	}
	// Create base MCP server.
	s := server.NewMCPServer(
		"Mnemonic MCP Server",
		mnemonicpkg.Version,
		server.WithResourceCapabilities(true, true),
		server.WithLogging(),
		server.WithRecovery(),
	)

	// Open database (WAL + NORMAL sync by default).
	dbConn, err := pkgdb.OpenDBConnection(dbPath, true, "NORMAL")
	if err != nil {
		return nil, fmt.Errorf("failed to open database connection: %w", err)
	}

	return &MnemonicMCPServer{
		mcpServer: s,
		db:        dbConn,
		dbPath:    dbPath,
	}, nil
}

// Start runs the stdio event loop. Make sure to register tools beforehand.
func (s *MnemonicMCPServer) Start() error {
	return server.ServeStdio(s.mcpServer)
}

// DB returns the underlying *sql.DB.
func (s *MnemonicMCPServer) DB() *sql.DB {
	return s.db
}

// MCPRawServer exposes the raw mcp-go server (useful for additional configuration).
func (s *MnemonicMCPServer) MCPRawServer() *server.MCPServer {
	return s.mcpServer
}

// Close cleans up allocated resources.
func (s *MnemonicMCPServer) Close() error {
	if s.db != nil {
		return s.db.Close()
	}
	return nil
}
