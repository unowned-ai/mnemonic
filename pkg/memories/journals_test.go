package memories

import (
	"context"
	"database/sql"
	"testing"

	"github.com/google/uuid"
	"github.com/unowned-ai/mnemonic/pkg/db"
)

func setupTestDB(t *testing.T) *sql.DB {
	t.Helper()

	// Use OpenDBConnection to get an in-memory DB for testing
	testDB, err := db.OpenDBConnection(":memory:", true, "NORMAL")
	if err != nil {
		t.Fatalf("Failed to open in-memory database: %v", err)
	}

	// Initialize the database schema
	if err := db.InitializeSchema(testDB, db.TargetSchemaVersion); err != nil {
		t.Fatalf("Failed to initialize schema: %v", err)
	}

	return testDB
}

func TestCreateJournal(t *testing.T) {
	testDB := setupTestDB(t)
	defer testDB.Close()

	ctx := context.Background()
	name := "Test Journal"
	description := "Test journal description"

	journal, err := CreateJournal(ctx, testDB, name, description)
	if err != nil {
		t.Fatalf("CreateJournal failed: %v", err)
	}

	if journal.Name != name {
		t.Errorf("Expected journal name %s, got %s", name, journal.Name)
	}

	if journal.Description != description {
		t.Errorf("Expected journal description %s, got %s", description, journal.Description)
	}

	if !journal.Active {
		t.Errorf("Expected journal to be active")
	}

	if journal.ID == uuid.Nil {
		t.Errorf("Expected journal ID to be set, got nil UUID")
	}

	// Verify the journal was actually stored in the database
	var storedID uuid.UUID
	var storedName, storedDesc string
	var storedActive bool
	var createdAt, updatedAt float64

	err = testDB.QueryRow("SELECT id, name, description, active, created_at, updated_at FROM journals WHERE id = ?", journal.ID).
		Scan(&storedID, &storedName, &storedDesc, &storedActive, &createdAt, &updatedAt)
	if err != nil {
		t.Fatalf("Failed to query database for stored journal: %v", err)
	}

	if storedID != journal.ID || storedName != name || storedDesc != description || !storedActive {
		t.Errorf("Stored journal data doesn't match created journal")
	}

	// Verify that SQLite set the timestamps
	if createdAt <= 0 {
		t.Errorf("Expected created_at to be set by SQLite, got %f", createdAt)
	}
	if updatedAt <= 0 {
		t.Errorf("Expected updated_at to be set by SQLite, got %f", updatedAt)
	}

	// Verify that timestamps in retrieved object match database
	if journal.CreatedAt != createdAt {
		t.Errorf("Journal CreatedAt (%f) doesn't match database value (%f)", journal.CreatedAt, createdAt)
	}
	if journal.UpdatedAt != updatedAt {
		t.Errorf("Journal UpdatedAt (%f) doesn't match database value (%f)", journal.UpdatedAt, updatedAt)
	}
}

func TestGetJournal(t *testing.T) {
	testDB := setupTestDB(t)
	defer testDB.Close()

	ctx := context.Background()
	name := "Test Journal"
	description := "Test journal description"

	// Create a journal to retrieve
	journal, err := CreateJournal(ctx, testDB, name, description)
	if err != nil {
		t.Fatalf("Failed to create test journal: %v", err)
	}

	// Retrieve the journal
	retrievedJournal, err := GetJournal(ctx, testDB, journal.ID)
	if err != nil {
		t.Fatalf("GetJournal failed: %v", err)
	}

	// Compare the created and retrieved journals
	if journal.ID != retrievedJournal.ID {
		t.Errorf("Journal ID mismatch: created %v, retrieved %v", journal.ID, retrievedJournal.ID)
	}

	if journal.Name != retrievedJournal.Name {
		t.Errorf("Journal name mismatch: created %s, retrieved %s", journal.Name, retrievedJournal.Name)
	}

	if journal.Description != retrievedJournal.Description {
		t.Errorf("Journal description mismatch: created %s, retrieved %s", journal.Description, retrievedJournal.Description)
	}

	if journal.Active != retrievedJournal.Active {
		t.Errorf("Journal active status mismatch: created %v, retrieved %v", journal.Active, retrievedJournal.Active)
	}

	// Test retrieving non-existent journal
	nonExistentID := uuid.New()
	_, err = GetJournal(ctx, testDB, nonExistentID)
	if err != ErrJournalNotFound {
		t.Errorf("Expected ErrJournalNotFound for non-existent journal, got: %v", err)
	}
}

func TestListJournals(t *testing.T) {
	testDB := setupTestDB(t)
	defer testDB.Close()

	ctx := context.Background()

	// Create several journals with different active states
	journals := []struct {
		name        string
		description string
		active      bool
		id          uuid.UUID
	}{
		{"Active 1", "Description 1", true, uuid.UUID{}},
		{"Active 2", "Description 2", true, uuid.UUID{}},
		{"Inactive 1", "Description 3", false, uuid.UUID{}},
		{"Inactive 2", "Description 4", false, uuid.UUID{}},
	}

	for i, j := range journals {
		journal, err := CreateJournal(ctx, testDB, j.name, j.description)
		if err != nil {
			t.Fatalf("Failed to create test journal %d: %v", i, err)
		}
		journals[i].id = journal.ID

		// Set active status if needed
		if !j.active {
			_, err = testDB.Exec("UPDATE journals SET active = false WHERE id = ?", journal.ID)
			if err != nil {
				t.Fatalf("Failed to update journal active status: %v", err)
			}
		}
	}

	// Test listing active journals only
	activeJournals, err := ListJournals(ctx, testDB, true)
	if err != nil {
		t.Fatalf("ListJournals failed when listing active only: %v", err)
	}

	if len(activeJournals) != 2 {
		t.Errorf("Expected 2 active journals, got %d", len(activeJournals))
	}

	for _, j := range activeJournals {
		if !j.Active {
			t.Errorf("ListJournals with activeOnly=true returned inactive journal: %v", j)
		}
	}

	// Test listing all journals
	allJournals, err := ListJournals(ctx, testDB, false)
	if err != nil {
		t.Fatalf("ListJournals failed when listing all journals: %v", err)
	}

	if len(allJournals) != 4 {
		t.Errorf("Expected 4 total journals, got %d", len(allJournals))
	}
}

func TestUpdateJournal(t *testing.T) {
	testDB := setupTestDB(t)
	defer testDB.Close()

	ctx := context.Background()
	name := "Original Name"
	description := "Original description"

	// Create a journal to update
	journal, err := CreateJournal(ctx, testDB, name, description)
	if err != nil {
		t.Fatalf("Failed to create test journal: %v", err)
	}

	// Update the journal
	newName := "Updated Name"
	newDescription := "Updated description"
	updatedJournal, err := UpdateJournal(ctx, testDB, journal.ID, newName, newDescription, false)
	if err != nil {
		t.Fatalf("UpdateJournal failed: %v", err)
	}

	if updatedJournal.Name != newName {
		t.Errorf("Expected updated name %s, got %s", newName, updatedJournal.Name)
	}

	if updatedJournal.Description != newDescription {
		t.Errorf("Expected updated description %s, got %s", newDescription, updatedJournal.Description)
	}

	if updatedJournal.Active {
		t.Errorf("Expected journal to be inactive, got active")
	}

	// Allow UpdatedAt to be equal if operations are within the same second due to unixepoch() resolution
	if updatedJournal.UpdatedAt < journal.UpdatedAt {
		t.Errorf("Expected updated_at (%f) to be greater than or equal to original (%f)",
			updatedJournal.UpdatedAt, journal.UpdatedAt)
	}

	// Verify the changes in the database
	retrievedJournal, err := GetJournal(ctx, testDB, journal.ID)
	if err != nil {
		t.Fatalf("Failed to retrieve updated journal: %v", err)
	}

	if retrievedJournal.Name != newName || retrievedJournal.Description != newDescription || retrievedJournal.Active {
		t.Errorf("Retrieved journal doesn't match updated values")
	}

	// Verify updated_at is set correctly
	var updatedAt float64
	err = testDB.QueryRow("SELECT updated_at FROM journals WHERE id = ?", journal.ID).Scan(&updatedAt)
	if err != nil {
		t.Fatalf("Failed to query journal updated_at: %v", err)
	}

	if updatedAt <= 0 {
		t.Errorf("Expected updated_at to be set by SQLite after update, got %f", updatedAt)
	}

	if retrievedJournal.UpdatedAt != updatedAt {
		t.Errorf("Journal UpdatedAt (%f) doesn't match database value (%f)", retrievedJournal.UpdatedAt, updatedAt)
	}

	// Test updating non-existent journal
	nonExistentID := uuid.New()
	_, err = UpdateJournal(ctx, testDB, nonExistentID, "New Name", "New Description", true)
	if err != ErrJournalNotFound {
		t.Errorf("Expected ErrJournalNotFound for non-existent journal, got: %v", err)
	}
}

func TestDeleteJournal(t *testing.T) {
	testDB := setupTestDB(t)
	defer testDB.Close()

	ctx := context.Background()

	// Create a journal to delete
	journal, err := CreateJournal(ctx, testDB, "Journal to Delete", "Will be deleted")
	if err != nil {
		t.Fatalf("Failed to create test journal: %v", err)
	}

	// Delete the journal
	err = DeleteJournal(ctx, testDB, journal.ID)
	if err != nil {
		t.Fatalf("DeleteJournal failed: %v", err)
	}

	// Verify the journal is deleted
	_, err = GetJournal(ctx, testDB, journal.ID)
	if err != ErrJournalNotFound {
		t.Errorf("Expected ErrJournalNotFound for deleted journal, got: %v", err)
	}

	// Test deleting non-existent journal
	nonExistentID := uuid.New()
	err = DeleteJournal(ctx, testDB, nonExistentID)
	if err != ErrJournalNotFound {
		t.Errorf("Expected ErrJournalNotFound for non-existent journal, got: %v", err)
	}
}

func TestDeleteInactiveJournals(t *testing.T) {
	testDB := setupTestDB(t)
	defer testDB.Close()

	ctx := context.Background()

	// Create a mix of active and inactive journals
	journalData := []struct {
		name   string
		active bool
	}{
		{"Active 1", true},
		{"Active 2", true},
		{"Inactive 1", false},
		{"Inactive 2", false},
		{"Inactive 3", false},
	}

	// Create all journals
	for _, jd := range journalData {
		journal, err := CreateJournal(ctx, testDB, jd.name, "Test description")
		if err != nil {
			t.Fatalf("Failed to create test journal: %v", err)
		}

		// Update active status if needed
		if !jd.active {
			_, err := testDB.Exec("UPDATE journals SET active = false WHERE id = ?", journal.ID)
			if err != nil {
				t.Fatalf("Failed to update journal active status: %v", err)
			}
		}
	}

	// Delete inactive journals
	count, err := DeleteInactiveJournals(ctx, testDB)
	if err != nil {
		t.Fatalf("DeleteInactiveJournals failed: %v", err)
	}

	if count != 3 {
		t.Errorf("Expected to delete 3 inactive journals, got %d", count)
	}

	// Verify only active journals remain
	journals, err := ListJournals(ctx, testDB, false)
	if err != nil {
		t.Fatalf("Failed to list journals after deletion: %v", err)
	}

	if len(journals) != 2 {
		t.Errorf("Expected 2 journals to remain, got %d", len(journals))
	}

	for _, j := range journals {
		if !j.Active {
			t.Errorf("Found inactive journal after DeleteInactiveJournals: %v", j)
		}
	}
}
