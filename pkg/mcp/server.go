package mcp

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/mark3labs/mcp-go/server"
	recallpkg "github.com/unowned-ai/recall/pkg"
	pkgdb "github.com/unowned-ai/recall/pkg/db"
)

// GetDefaultDBPath returns a system-appropriate default path for the database.
func GetDefaultDBPath() string {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		// Fallback to current directory if home dir can't be determined
		return "recall.db"
	}

	switch runtime.GOOS {
	case "windows":
		return filepath.Join(homeDir, "AppData", "Roaming", "recall", "recall.db")
	case "darwin":
		return filepath.Join(homeDir, "Library", "Application Support", "recall", "recall.db")
	default: // linux and others
		return filepath.Join(homeDir, ".local", "share", "recall", "recall.db")
	}
}

type RecallMCPServer struct {
	mcpServer *server.MCPServer
	db        *sql.DB
	DbPath    string
}

// NewRecallMCPServer spins up an MCP server backed by the SQLite database at dbPath.
func NewRecallMCPServer(dbPath string) (*RecallMCPServer, error) {
	if dbPath == "" {
		dbPath = GetDefaultDBPath()
	}

	// Expand ~ to home directory if present
	if strings.HasPrefix(dbPath, "~/") {
		homeDir, err := os.UserHomeDir()
		if err == nil {
			dbPath = filepath.Join(homeDir, dbPath[2:])
		}
	}

	// Ensure parent directory exists
	dbDir := filepath.Dir(dbPath)
	if _, err := os.Stat(dbDir); os.IsNotExist(err) {
		if err := os.MkdirAll(dbDir, 0755); err != nil {
			return nil, fmt.Errorf("failed to create directory for database: %w", err)
		}
	}

	// Create base MCP server.
	s := server.NewMCPServer(
		"Recall MCP Server",
		recallpkg.Version,
		server.WithResourceCapabilities(true, true),
		server.WithLogging(),
		server.WithRecovery(),
	)

	// Open database (WAL + FULL).
	dbConn, err := pkgdb.OpenDBConnection(dbPath, true, "FULL")
	if err != nil {
		return nil, fmt.Errorf("failed to open database connection: %w", err)
	}

	// Automatically initialize or migrate the database schema.
	if err := pkgdb.UpgradeDB(dbConn, dbPath, pkgdb.TargetSchemaVersion); err != nil {
		// Attempt to close the DB connection if upgrade fails.
		dbConn.Close()
		return nil, fmt.Errorf("failed to initialize/upgrade database schema for '%s': %w", dbPath, err)
	}

	return &RecallMCPServer{
		mcpServer: s,
		db:        dbConn,
		DbPath:    dbPath,
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
		// TRUNCATE mode waits for transactions and writes the WAL back to the main DB.
		_, err := s.db.Exec("PRAGMA wal_checkpoint(TRUNCATE);")
		if err != nil {
			fmt.Fprintf(os.Stderr, "Warning: WAL checkpoint failed during close: %v\n", err)
		}
		return s.db.Close()
	}
	return nil
}
