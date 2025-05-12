package mcp

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/mark3labs/mcp-go/server"
	mnemonicpkg "github.com/unowned-ai/mnemonic/pkg"
	pkgdb "github.com/unowned-ai/mnemonic/pkg/db"
)

// GetDefaultDBPath returns a system-appropriate default path for the database.
func GetDefaultDBPath() string {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		// Fallback to current directory if home dir can't be determined
		return "mnemonic.db"
	}

	switch runtime.GOOS {
	case "windows":
		return filepath.Join(homeDir, "AppData", "Roaming", "mnemonic", "mnemonic.db")
	case "darwin":
		return filepath.Join(homeDir, "Library", "Application Support", "mnemonic", "mnemonic.db")
	default: // linux and others
		return filepath.Join(homeDir, ".local", "share", "mnemonic", "mnemonic.db")
	}
}

// MnemonicMCPServer wraps the MCP server with a database handle.
type MnemonicMCPServer struct {
	mcpServer *server.MCPServer
	db        *sql.DB
	dbPath    string
}

// NewMnemonicMCPServer spins up an MCP server backed by the SQLite database at dbPath.
func NewMnemonicMCPServer(dbPath string) (*MnemonicMCPServer, error) {
	// Set default path if not provided
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

	// Automatically initialize or migrate the database schema.
	if err := pkgdb.UpgradeDB(dbConn, dbPath, pkgdb.TargetSchemaVersion); err != nil {
		// Attempt to close the DB connection if upgrade fails.
		dbConn.Close()
		return nil, fmt.Errorf("failed to initialize/upgrade database schema for '%s': %w", dbPath, err)
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

// DBPath returns the database path used by this server.
func (s *MnemonicMCPServer) DBPath() string {
	return s.dbPath
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
