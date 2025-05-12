package mcp

import (
	"database/sql"
	"fmt"

	"github.com/mark3labs/mcp-go/server"
	recallpkg "github.com/unowned-ai/recall/pkg"
	pkgdb "github.com/unowned-ai/recall/pkg/db"
)

// RecallMCPServer wraps the MCP server with a database handle.
type RecallMCPServer struct {
	mcpServer *server.MCPServer
	db        *sql.DB
	dbPath    string
}

// NewRecallMCPServer spins up an MCP server backed by the SQLite database at dbPath.
func NewRecallMCPServer(dbPath string) (*RecallMCPServer, error) {
	if dbPath == "" {
		return nil, fmt.Errorf("database path cannot be empty for RecallMCPServer")
	}
	// Create base MCP server.
	s := server.NewMCPServer(
		"Recall MCP Server",
		recallpkg.Version,
		server.WithResourceCapabilities(true, true),
		server.WithLogging(),
		server.WithRecovery(),
	)

	// Open database (WAL + NORMAL sync by default).
	dbConn, err := pkgdb.OpenDBConnection(dbPath, true, "NORMAL")
	if err != nil {
		return nil, fmt.Errorf("failed to open database connection: %w", err)
	}

	return &RecallMCPServer{
		mcpServer: s,
		db:        dbConn,
		dbPath:    dbPath,
	}, nil
}

// Start runs the stdio event loop. Make sure to register tools beforehand.
func (s *RecallMCPServer) Start() error {
	return server.ServeStdio(s.mcpServer)
}

// DB returns the underlying *sql.DB.
func (s *RecallMCPServer) DB() *sql.DB {
	return s.db
}

// MCPRawServer exposes the raw mcp-go server (useful for additional configuration).
func (s *RecallMCPServer) MCPRawServer() *server.MCPServer {
	return s.mcpServer
}

// Close cleans up allocated resources.
func (s *RecallMCPServer) Close() error {
	if s.db != nil {
		return s.db.Close()
	}
	return nil
}
