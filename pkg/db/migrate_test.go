package db

import (
	"database/sql"
	"fmt"
	"testing"

	_ "github.com/mattn/go-sqlite3" // SQLite driver, needed for tests
)

// checkTableExists is a test helper to verify if a table exists in the database.
func checkTableExists(t *testing.T, db *sql.DB, tableName string) {
	t.Helper()
	query := fmt.Sprintf("SELECT name FROM sqlite_master WHERE type='table' AND name='%s';", tableName)
	var name string
	err := db.QueryRow(query).Scan(&name)
	if err != nil {
		if err == sql.ErrNoRows {
			t.Errorf("Table '%s' does not exist, but it should.", tableName)
			return
		}
		t.Fatalf("Error checking if table '%s' exists: %v", tableName, err)
	}
	if name != tableName {
		t.Errorf("Table check query returned '%s' but expected '%s'", name, tableName)
	}
}

func TestUpgradeDB_NewDatabase(t *testing.T) {
	// Use OpenDBConnection to get an in-memory DB for testing
	// Using WAL and NORMAL sync, as these are our new defaults.
	db, err := OpenDBConnection(":memory:", true, "NORMAL")
	if err != nil {
		t.Fatalf("OpenDBConnection failed for in-memory DB: %v", err)
	}
	defer db.Close()

	// Call UpgradeDB, which should initialize the schema
	err = UpgradeDB(db, ":memory:") // ":memory:" as the identifier for logs
	if err != nil {
		t.Fatalf("UpgradeDB failed on a new in-memory database: %v", err)
	}

	// Verify all tables are created
	expectedTables := []string{"mnemonic_versions", "journals", "entries", "tags", "entry_tags"}
	for _, tableName := range expectedTables {
		checkTableExists(t, db, tableName)
	}

	// Verify the component version is set correctly
	version, err := GetComponentSchemaVersion(db, MemoriesDBComponent)
	if err != nil {
		t.Fatalf("GetComponentSchemaVersion failed after UpgradeDB: %v", err)
	}

	if version != TargetSchemaVersion {
		t.Errorf("Expected component '%s' to be at version %d, but got %d", MemoriesDBComponent, TargetSchemaVersion, version)
	}
}
