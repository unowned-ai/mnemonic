package mcp

import (
	"database/sql"
	"fmt"
	"os"

	"github.com/mark3labs/mcp-go/server"
	recallpkg "github.com/unowned-ai/recall/pkg"
	pkgdb "github.com/unowned-ai/recall/pkg/db"
	recallutils "github.com/unowned-ai/recall/pkg/utils"
)

type RecallMCPServer struct {
	mcpServer *server.MCPServer
	db        *sql.DB
	DbPath    string
}

// NewRecallMCPServer spins up an MCP server backed by the SQLite database at dbPath.
func NewRecallMCPServer(dbPath string, walEnabled bool, syncPragma string) (*RecallMCPServer, error) {
	finalDBPath, err := recallutils.ResolveAndEnsureDBPath(dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve database path '%s': %w", dbPath, err)
	}

	// Create base MCP server.
	s := server.NewMCPServer(
		"Recall MCP Server",
		recallpkg.Version,
		server.WithResourceCapabilities(true, true),
		server.WithLogging(),
		server.WithRecovery(),
	)

	dbConn, err := pkgdb.OpenDBConnection(finalDBPath, walEnabled, syncPragma)
	if err != nil {
		return nil, fmt.Errorf("failed to open database connection: %w", err)
	}

	// Automatically initialize or migrate the database schema.
	if err := pkgdb.UpgradeDB(dbConn, finalDBPath, pkgdb.TargetSchemaVersion); err != nil {
		// Attempt to close the DB connection if upgrade fails.
		dbConn.Close()
		return nil, fmt.Errorf("failed to initialize/upgrade database schema for '%s': %w", finalDBPath, err)
	}

	return &RecallMCPServer{
		mcpServer: s,
		db:        dbConn,
		DbPath:    finalDBPath,
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
		// Checkpointing: https://www.sqlite.org/c3ref/wal_checkpoint_v2.html
		_, err := s.db.Exec("PRAGMA wal_checkpoint(TRUNCATE);")
		if err != nil {
			fmt.Fprintf(os.Stderr, "Warning: WAL checkpoint failed during close: %v\n", err)
		}
		return s.db.Close()
	}
	return nil
}
