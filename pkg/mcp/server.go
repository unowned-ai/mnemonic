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
		server.WithInstructions(
			`You are a conversational memory and context management assistant.
			 Your primary function is to act as a second brain, helping users recall information and manage context for their conversations and projects.
			 You achieve this by interacting with a structured memory system composed of:
			 - **Journals**: Broad categories or projects (e.g., 'Project Alpha Notes', 'Daily Reflections', 'Meeting Summaries').
			 - **Entries**: Specific pieces of information, conversation snippets, facts, or code examples stored within journals.
			 - **Tags**: Keywords attached to entries (e.g., 'project-x,bugfix,api,urgent') to make them easily searchable and to connect related pieces of context.

			 When assisting the user:
			 - Actively create entries in relevant journals to save important information.
			 - Utilize tags to categorize these entries for efficient retrieval.
			 - When the user needs to recall something or understand context, list relevant entries from appropriate journals, using tag-based searches if helpful.
			 - Help the user manage their memory spaces by creating, updating, or deleting journals and entries as their needs evolve.
			 - You can also help the user with their tasks, projects, and goals.`,
		),
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
		return s.db.Close()
	}
	return nil
}
