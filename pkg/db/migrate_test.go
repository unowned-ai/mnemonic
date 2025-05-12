package db

import (
	"database/sql"
	"fmt"
	"strings"
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
	db, err := OpenDBConnection(":memory:", true, "NORMAL")
	if err != nil {
		t.Fatalf("OpenDBConnection failed for in-memory DB: %v", err)
	}
	defer db.Close()

	// Call UpgradeDB, which should initialize the schema to the current TargetSchemaVersion (const)
	err = UpgradeDB(db, ":memory:", TargetSchemaVersion)
	if err != nil {
		t.Fatalf("UpgradeDB failed on a new in-memory database: %v", err)
	}

	// Verify all tables are created
	expectedTables := []string{"recall_versions", "journals", "entries", "tags", "entry_tags"}
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

func TestUpgradeDB_AlreadyUpToDate(t *testing.T) {
	db, err := OpenDBConnection(":memory:", true, "NORMAL")
	if err != nil {
		t.Fatalf("OpenDBConnection failed for in-memory DB: %v", err)
	}
	defer db.Close()

	// Initialize the database to the TargetSchemaVersion first
	if err := InitializeSchema(db, TargetSchemaVersion); err != nil {
		t.Fatalf("InitializeSchema failed: %v", err)
	}

	// Now, call UpgradeDB again. It should detect it's up to date.
	err = UpgradeDB(db, ":memory:", TargetSchemaVersion)
	if err != nil {
		t.Fatalf("UpgradeDB failed on an up-to-date database: %v", err)
	}

	// Verify the component version is still TargetSchemaVersion
	version, err := GetComponentSchemaVersion(db, MemoriesDBComponent)
	if err != nil {
		t.Fatalf("GetComponentSchemaVersion failed: %v", err)
	}
	if version != TargetSchemaVersion {
		t.Errorf("Expected component '%s' to be at version %d, but got %d", MemoriesDBComponent, TargetSchemaVersion, version)
	}
}

func TestUpgradeDB_OlderVersionNeedsMigration(t *testing.T) {
	db, err := OpenDBConnection(":memory:", true, "NORMAL")
	if err != nil {
		t.Fatalf("OpenDBConnection failed for in-memory DB: %v", err)
	}
	defer db.Close()

	const dbInitialSchemaVersion int64 = 1
	const appTargetsSchemaVersion int64 = 2 // Simulate app wanting version 2

	// Initialize the database to an older version (e.g., 1)
	if err := InitializeSchema(db, dbInitialSchemaVersion); err != nil {
		t.Fatalf("InitializeSchema to version %d failed: %v", dbInitialSchemaVersion, err)
	}

	// Call UpgradeDB, expecting the app to target a newer version (2)
	err = UpgradeDB(db, ":memory:", appTargetsSchemaVersion)
	if err == nil {
		t.Fatalf("UpgradeDB should have failed for an older DB version requiring migration, but it did not")
	}

	expectedErrorMsg := fmt.Sprintf("component %s in database ':memory:' has schema version %d, which is older than application's target schema version %d", MemoriesDBComponent, dbInitialSchemaVersion, appTargetsSchemaVersion)
	if !strings.Contains(err.Error(), expectedErrorMsg) {
		t.Errorf("UpgradeDB error message mismatch.\nExpected to contain: %s\nGot: %s", expectedErrorMsg, err.Error())
	}

	// Ensure the DB version was not changed by the failed upgrade attempt
	currentVersion, getErr := GetComponentSchemaVersion(db, MemoriesDBComponent)
	if getErr != nil {
		t.Fatalf("GetComponentSchemaVersion failed after attempted upgrade: %v", getErr)
	}
	if currentVersion != dbInitialSchemaVersion {
		t.Errorf("Database schema version changed from %d to %d after a failed upgrade attempt that should have been a no-op.", dbInitialSchemaVersion, currentVersion)
	}
}

func TestUpgradeDB_NewerVersionUnsupported(t *testing.T) {
	db, err := OpenDBConnection(":memory:", true, "NORMAL")
	if err != nil {
		t.Fatalf("OpenDBConnection failed for in-memory DB: %v", err)
	}
	defer db.Close()

	const dbInitialSchemaVersion int64 = 2  // DB is at version 2
	const appTargetsSchemaVersion int64 = 1 // Simulate app wanting version 1

	// Initialize the database to a newer version (e.g., 2)
	if err := InitializeSchema(db, dbInitialSchemaVersion); err != nil {
		t.Fatalf("InitializeSchema to version %d failed: %v", dbInitialSchemaVersion, err)
	}

	// Call UpgradeDB, expecting the app to target an older version (1)
	err = UpgradeDB(db, ":memory:", appTargetsSchemaVersion)
	if err == nil {
		t.Fatalf("UpgradeDB should have failed for a newer DB version, but it did not")
	}

	expectedErrorMsg := fmt.Sprintf("component %s in database ':memory:' has schema version %d, which is newer than application's target schema version %d", MemoriesDBComponent, dbInitialSchemaVersion, appTargetsSchemaVersion)
	if !strings.Contains(err.Error(), expectedErrorMsg) {
		t.Errorf("UpgradeDB error message mismatch.\nExpected to contain: %s\nGot: %s", expectedErrorMsg, err.Error())
	}

	// Ensure the DB version was not changed
	currentVersion, getErr := GetComponentSchemaVersion(db, MemoriesDBComponent)
	if getErr != nil {
		t.Fatalf("GetComponentSchemaVersion failed after attempted upgrade: %v", getErr)
	}
	if currentVersion != dbInitialSchemaVersion {
		t.Errorf("Database schema version changed from %d to %d after a failed upgrade attempt that should have been a no-op.", dbInitialSchemaVersion, currentVersion)
	}
}
