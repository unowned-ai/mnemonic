package db

import (
	"database/sql"
	"fmt"
	"strings"

	// No longer importing mnemonicsync "github.com/unowned-ai/mnemonic/pkg"
	// Will remove later if not needed, good for initial dev
	_ "github.com/mattn/go-sqlite3" // SQLite driver
)

const (
	// TargetSchemaVersion is the highest schema version this version of the code supports for the memoriesdb component.
	TargetSchemaVersion int64 = 1
	// MemoriesDBComponent is the name for the main memories database component.
	MemoriesDBComponent = "memoriesdb"
)

// GetComponentSchemaVersion retrieves the schema version for a given component.
// Returns 0 if the component is not found, the versions table is uninitialized, or the table doesn't exist.
func GetComponentSchemaVersion(db *sql.DB, componentName string) (int64, error) {
	query := `SELECT version FROM mnemonic_versions WHERE component = ?;`
	row := db.QueryRow(query, componentName)

	var version int64
	err := row.Scan(&version)
	if err != nil {
		if err == sql.ErrNoRows {
			// Component not found in the table, or table is empty.
			return 0, nil
		}
		// Check if the error is due to the table not existing
		if strings.Contains(err.Error(), "no such table") && strings.Contains(err.Error(), "mnemonic_versions") {
			// mnemonic_versions table itself doesn't exist, so definitely version 0.
			return 0, nil
		}
		// Another error occurred during scan.
		return 0, fmt.Errorf("failed to scan version for component '%s': %w", componentName, err)
	}
	return version, nil
}

// InitializeSchemaV1 creates the database schema (all tables for memoriesdb)
// and inserts the current target schema version for the memoriesdb component.
func InitializeSchemaV1(db *sql.DB) error {
	// Execute the schema creation SQL
	_, err := db.Exec(SchemaV1)
	if err != nil {
		return fmt.Errorf("failed to execute schema v1 SQL: %w", err)
	}

	// Insert or update the version for the memoriesdb component
	insertVersionSQL := `
INSERT INTO mnemonic_versions (component, version) VALUES (?, ?)
ON CONFLICT(component) DO UPDATE SET version = excluded.version, created_at = unixepoch();`

	_, err = db.Exec(insertVersionSQL, MemoriesDBComponent, TargetSchemaVersion)
	if err != nil {
		return fmt.Errorf("failed to insert/update version for component %s to %d: %w", MemoriesDBComponent, TargetSchemaVersion, err)
	}

	fmt.Printf("Component %s initialized/updated to schema version %d\n", MemoriesDBComponent, TargetSchemaVersion)
	return nil
}

// UpgradeDB applies necessary migrations to bring the database, represented by the *sql.DB connection,
// for the MemoriesDBComponent to the current target schema version.
// dbIdentifierForLog is used for logging purposes only.
func UpgradeDB(db *sql.DB, dbIdentifierForLog string) error {
	// Ping is removed, assumed to be done by caller if needed.
	// sql.Open and db.Close() are removed, managed by caller.

	currentDBVersion, err := GetComponentSchemaVersion(db, MemoriesDBComponent)
	if err != nil {
		// GetComponentSchemaVersion already provides good error context
		return err
	}

	// TargetSchemaVersion is defined in this package

	if currentDBVersion == 0 { // 0 indicates component not versioned or new DB
		fmt.Printf("Component %s in database '%s' appears to be uninitialized or at version 0. Initializing/Upgrading to schema version %d...\n", MemoriesDBComponent, dbIdentifierForLog, TargetSchemaVersion)
		err = InitializeSchemaV1(db) // This will set up tables and record version for MemoriesDBComponent
		if err != nil {
			return fmt.Errorf("failed to initialize component %s in database '%s': %w", MemoriesDBComponent, dbIdentifierForLog, err)
		}
		// Success message is printed by InitializeSchemaV1
		return nil
	} else if currentDBVersion == TargetSchemaVersion {
		fmt.Printf("Component %s in database '%s' is already up to date (schema version %d).\n", MemoriesDBComponent, dbIdentifierForLog, currentDBVersion)
		return nil
	} else if currentDBVersion < TargetSchemaVersion {
		// This is where migration logic for future versions would go for the MemoriesDBComponent.
		// For now, we only support initializing to the current version from an uninitialized state (version 0).
		return fmt.Errorf("component %s in database '%s' has schema version %d, which is older than application's target schema version %d. Automatic migration from this older version is not yet supported", MemoriesDBComponent, dbIdentifierForLog, currentDBVersion, TargetSchemaVersion)
	} else { // currentDBVersion > TargetSchemaVersion
		return fmt.Errorf("component %s in database '%s' has schema version %d, which is newer than application's target schema version %d. Please upgrade the application", MemoriesDBComponent, dbIdentifierForLog, currentDBVersion, TargetSchemaVersion)
	}
}
