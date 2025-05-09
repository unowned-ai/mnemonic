package db

import (
	"database/sql"
	"fmt"
	"net/url"
	"strings"

	_ "github.com/mattn/go-sqlite3" // SQLite driver
)

// validSyncModes lists the allowed values for the synchronous pragma.
var validSyncModes = map[string]bool{
	"OFF":    true,
	"NORMAL": true,
	"FULL":   true,
	"EXTRA":  true, // SQLite also supports EXTRA
}

// OpenDBConnection establishes a connection to a SQLite database with specified options.
// baseDSN is the initial data source name (e.g., file path).
// enableWAL sets the journal_mode to WAL if true.
// syncPragma sets the synchronous pragma (e.g., "OFF", "NORMAL", "FULL", "EXTRA").
func OpenDBConnection(baseDSN string, enableWAL bool, syncPragma string) (*sql.DB, error) {
	params := url.Values{}

	if enableWAL {
		params.Add("_journal_mode", "WAL")
	}

	if syncPragma != "" {
		ucSyncPragma := strings.ToUpper(syncPragma)
		if !validSyncModes[ucSyncPragma] {
			return nil, fmt.Errorf("invalid sync pragma value: %s. Must be one of OFF, NORMAL, FULL, EXTRA", syncPragma)
		}
		params.Add("_synchronous", ucSyncPragma)
	}

	constructedDSN := baseDSN
	if len(params) > 0 {
		if strings.Contains(baseDSN, "?") {
			constructedDSN += "&" + params.Encode()
		} else {
			constructedDSN += "?" + params.Encode()
		}
	}

	db, err := sql.Open("sqlite3", constructedDSN)
	if err != nil {
		return nil, fmt.Errorf("failed to open database with DSN '%s': %w", constructedDSN, err)
	}

	// Ping the database to ensure the connection is alive and the DSN is valid.
	if err = db.Ping(); err != nil {
		// Close the DB if ping fails, as the connection might be in a weird state.
		db.Close()
		return nil, fmt.Errorf("failed to ping database with DSN '%s': %w", constructedDSN, err)
	}

	// Enable foreign key support for this connection.
	// This is crucial for ON DELETE CASCADE and other FK actions to work.
	_, err = db.Exec("PRAGMA foreign_keys = ON;")
	if err != nil {
		db.Close() // Close DB if we can't set the pragma
		return nil, fmt.Errorf("failed to enable foreign key support for DSN '%s': %w", constructedDSN, err)
	}

	return db, nil
}
