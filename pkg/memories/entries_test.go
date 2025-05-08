package memories

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/google/uuid"
)

func setupTestDBWithJournal(t *testing.T) (*sql.DB, uuid.UUID) {
	t.Helper()
	
	// Create the test database
	testDB := setupTestDB(t)

	// Create a test journal
	ctx := context.Background()
	journal, err := CreateJournal(ctx, testDB, "Test Journal", "Test journal for entries")
	if err != nil {
		t.Fatalf("Failed to create test journal: %v", err)
	}

	return testDB, journal.ID
}

func TestCreateEntry(t *testing.T) {
	testDB, journalID := setupTestDBWithJournal(t)
	defer testDB.Close()

	ctx := context.Background()
	title := "Test Entry"
	content := "This is the content of the test entry."
	contentType := "text/plain"

	entry, err := CreateEntry(ctx, testDB, journalID, title, content, contentType)
	if err != nil {
		t.Fatalf("CreateEntry failed: %v", err)
	}

	if entry.Title != title {
		t.Errorf("Expected entry title %s, got %s", title, entry.Title)
	}

	if entry.Content != content {
		t.Errorf("Expected entry content %s, got %s", content, entry.Content)
	}

	if entry.ContentType != contentType {
		t.Errorf("Expected entry content type %s, got %s", contentType, entry.ContentType)
	}

	if entry.JournalID != journalID {
		t.Errorf("Expected journal ID %s, got %s", journalID, entry.JournalID)
	}

	if entry.ID == uuid.Nil {
		t.Errorf("Expected entry ID to be set, got nil UUID")
	}

	// Verify the entry was actually stored in the database
	var storedID, storedJournalID uuid.UUID
	var storedTitle, storedContent, storedContentType string
	var createdAt, updatedAt float64

	err = testDB.QueryRow("SELECT id, journal_id, title, content, content_type, created_at, updated_at FROM entries WHERE id = ?", entry.ID).
		Scan(&storedID, &storedJournalID, &storedTitle, &storedContent, &storedContentType, &createdAt, &updatedAt)
	if err != nil {
		t.Fatalf("Failed to query database for stored entry: %v", err)
	}

	if storedID != entry.ID || storedJournalID != journalID || storedTitle != title || storedContent != content || storedContentType != contentType {
		t.Errorf("Stored entry data doesn't match created entry")
	}

	// Verify that SQLite set the timestamps
	if createdAt <= 0 {
		t.Errorf("Expected created_at to be set by SQLite, got %f", createdAt)
	}
	if updatedAt <= 0 {
		t.Errorf("Expected updated_at to be set by SQLite, got %f", updatedAt)
	}
	
	// Verify that timestamps in retrieved object match database
	if entry.CreatedAt != createdAt {
		t.Errorf("Entry CreatedAt (%f) doesn't match database value (%f)", entry.CreatedAt, createdAt)
	}
	if entry.UpdatedAt != updatedAt {
		t.Errorf("Entry UpdatedAt (%f) doesn't match database value (%f)", entry.UpdatedAt, updatedAt)
	}

	// Test creating an entry with a non-existent journal
	nonExistentJournalID := uuid.New()
	_, err = CreateEntry(ctx, testDB, nonExistentJournalID, "Title", "Content", "text/plain")
	if err != ErrJournalNotFound {
		t.Errorf("Expected ErrJournalNotFound for non-existent journal, got: %v", err)
	}

	// Test creating an entry with default content type
	entryDefaultContentType, err := CreateEntry(ctx, testDB, journalID, "Default Content Type", "Content", "")
	if err != nil {
		t.Fatalf("CreateEntry with default content type failed: %v", err)
	}
	if entryDefaultContentType.ContentType != "text/plain" {
		t.Errorf("Expected default content type 'text/plain', got %s", entryDefaultContentType.ContentType)
	}
}

func TestGetEntry(t *testing.T) {
	testDB, journalID := setupTestDBWithJournal(t)
	defer testDB.Close()

	ctx := context.Background()
	title := "Test Entry"
	content := "This is the content of the test entry."
	contentType := "text/plain"

	// Create an entry to retrieve
	entry, err := CreateEntry(ctx, testDB, journalID, title, content, contentType)
	if err != nil {
		t.Fatalf("Failed to create test entry: %v", err)
	}

	// Retrieve the entry
	retrievedEntry, err := GetEntry(ctx, testDB, entry.ID)
	if err != nil {
		t.Fatalf("GetEntry failed: %v", err)
	}

	// Compare the created and retrieved entries
	if entry.ID != retrievedEntry.ID {
		t.Errorf("Entry ID mismatch: created %v, retrieved %v", entry.ID, retrievedEntry.ID)
	}

	if entry.JournalID != retrievedEntry.JournalID {
		t.Errorf("Journal ID mismatch: created %v, retrieved %v", entry.JournalID, retrievedEntry.JournalID)
	}

	if entry.Title != retrievedEntry.Title {
		t.Errorf("Entry title mismatch: created %s, retrieved %s", entry.Title, retrievedEntry.Title)
	}

	if entry.Content != retrievedEntry.Content {
		t.Errorf("Entry content mismatch: created %s, retrieved %s", entry.Content, retrievedEntry.Content)
	}

	if entry.ContentType != retrievedEntry.ContentType {
		t.Errorf("Entry content type mismatch: created %s, retrieved %s", entry.ContentType, retrievedEntry.ContentType)
	}

	// Test retrieving non-existent entry
	nonExistentID := uuid.New()
	_, err = GetEntry(ctx, testDB, nonExistentID)
	if err != ErrEntryNotFound {
		t.Errorf("Expected ErrEntryNotFound for non-existent entry, got: %v", err)
	}
}

func TestListEntries(t *testing.T) {
	testDB, journalID := setupTestDBWithJournal(t)
	defer testDB.Close()

	ctx := context.Background()

	// Create a second journal
	secondJournal, err := CreateJournal(ctx, testDB, "Second Journal", "Another test journal")
	if err != nil {
		t.Fatalf("Failed to create second test journal: %v", err)
	}

	// Create several entries in different journals
	entries := []struct {
		journalID  uuid.UUID
		title      string
		content    string
		contentType string
	}{
		{journalID, "Entry 1", "Content 1", "text/plain"},
		{journalID, "Entry 2", "Content 2", "text/plain"},
		{secondJournal.ID, "Entry 3", "Content 3", "text/plain"},
		{secondJournal.ID, "Entry 4", "Content 4", "text/plain"},
	}

	for _, e := range entries {
		_, err := CreateEntry(ctx, testDB, e.journalID, e.title, e.content, e.contentType)
		if err != nil {
			t.Fatalf("Failed to create test entry: %v", err)
		}
	}

	// Test listing entries for the first journal
	entriesForJournal1, err := ListEntries(ctx, testDB, journalID)
	if err != nil {
		t.Fatalf("ListEntries failed: %v", err)
	}

	if len(entriesForJournal1) != 2 {
		t.Errorf("Expected 2 entries for journal 1, got %d", len(entriesForJournal1))
	}

	// Verify all entries belong to the first journal
	for _, e := range entriesForJournal1 {
		if e.JournalID != journalID {
			t.Errorf("Expected entry to belong to journal %s, got %s", journalID, e.JournalID)
		}
	}

	// Test listing entries for the second journal
	entriesForJournal2, err := ListEntries(ctx, testDB, secondJournal.ID)
	if err != nil {
		t.Fatalf("ListEntries failed: %v", err)
	}

	if len(entriesForJournal2) != 2 {
		t.Errorf("Expected 2 entries for journal 2, got %d", len(entriesForJournal2))
	}

	// Verify all entries belong to the second journal
	for _, e := range entriesForJournal2 {
		if e.JournalID != secondJournal.ID {
			t.Errorf("Expected entry to belong to journal %s, got %s", secondJournal.ID, e.JournalID)
		}
	}

	// Test listing entries for a non-existent journal
	nonExistentJournalID := uuid.New()
	_, err = ListEntries(ctx, testDB, nonExistentJournalID)
	if err != ErrJournalNotFound {
		t.Errorf("Expected ErrJournalNotFound for non-existent journal, got: %v", err)
	}
}

func TestUpdateEntry(t *testing.T) {
	testDB, journalID := setupTestDBWithJournal(t)
	defer testDB.Close()

	ctx := context.Background()
	title := "Original Title"
	content := "Original content"
	contentType := "text/plain"

	// Create an entry to update
	entry, err := CreateEntry(ctx, testDB, journalID, title, content, contentType)
	if err != nil {
		t.Fatalf("Failed to create test entry: %v", err)
	}

	// Wait a moment to ensure timestamps will be different
	time.Sleep(1 * time.Second)

	// Update the entry
	newTitle := "Updated Title"
	newContent := "Updated content"
	newContentType := "text/markdown"
	updatedEntry, err := UpdateEntry(ctx, testDB, entry.ID, newTitle, newContent, newContentType)
	if err != nil {
		t.Fatalf("UpdateEntry failed: %v", err)
	}

	if updatedEntry.Title != newTitle {
		t.Errorf("Expected updated title %s, got %s", newTitle, updatedEntry.Title)
	}

	if updatedEntry.Content != newContent {
		t.Errorf("Expected updated content %s, got %s", newContent, updatedEntry.Content)
	}

	if updatedEntry.ContentType != newContentType {
		t.Errorf("Expected updated content type %s, got %s", newContentType, updatedEntry.ContentType)
	}

	if updatedEntry.UpdatedAt <= entry.UpdatedAt {
		t.Errorf("Expected updated_at (%f) to be later than original (%f)", 
			updatedEntry.UpdatedAt, entry.UpdatedAt)
	}

	// Verify the changes in the database
	retrievedEntry, err := GetEntry(ctx, testDB, entry.ID)
	if err != nil {
		t.Fatalf("Failed to retrieve updated entry: %v", err)
	}

	if retrievedEntry.Title != newTitle || retrievedEntry.Content != newContent || retrievedEntry.ContentType != newContentType {
		t.Errorf("Retrieved entry doesn't match updated values")
	}

	// Verify updated_at is set correctly
	var updatedAt float64
	err = testDB.QueryRow("SELECT updated_at FROM entries WHERE id = ?", entry.ID).Scan(&updatedAt)
	if err != nil {
		t.Fatalf("Failed to query entry updated_at: %v", err)
	}
	
	if updatedAt <= 0 {
		t.Errorf("Expected updated_at to be set by SQLite after update, got %f", updatedAt)
	}
	
	if retrievedEntry.UpdatedAt != updatedAt {
		t.Errorf("Entry UpdatedAt (%f) doesn't match database value (%f)", retrievedEntry.UpdatedAt, updatedAt)
	}

	// Test partial update (only title)
	partialUpdateEntry, err := UpdateEntry(ctx, testDB, entry.ID, "Partial Update", "", "")
	if err != nil {
		t.Fatalf("Partial UpdateEntry failed: %v", err)
	}

	if partialUpdateEntry.Title != "Partial Update" {
		t.Errorf("Expected updated title 'Partial Update', got %s", partialUpdateEntry.Title)
	}

	if partialUpdateEntry.Content != newContent {
		t.Errorf("Expected content to remain %s, got %s", newContent, partialUpdateEntry.Content)
	}

	if partialUpdateEntry.ContentType != newContentType {
		t.Errorf("Expected content type to remain %s, got %s", newContentType, partialUpdateEntry.ContentType)
	}

	// Test updating non-existent entry
	nonExistentID := uuid.New()
	_, err = UpdateEntry(ctx, testDB, nonExistentID, "New Title", "New Content", "text/plain")
	if err != ErrEntryNotFound {
		t.Errorf("Expected ErrEntryNotFound for non-existent entry, got: %v", err)
	}
}

func TestDeleteEntry(t *testing.T) {
	testDB, journalID := setupTestDBWithJournal(t)
	defer testDB.Close()

	ctx := context.Background()
	
	// Create an entry to delete
	entry, err := CreateEntry(ctx, testDB, journalID, "Entry to Delete", "Will be deleted", "text/plain")
	if err != nil {
		t.Fatalf("Failed to create test entry: %v", err)
	}

	// Delete the entry
	err = DeleteEntry(ctx, testDB, entry.ID)
	if err != nil {
		t.Fatalf("DeleteEntry failed: %v", err)
	}

	// Verify the entry is deleted
	_, err = GetEntry(ctx, testDB, entry.ID)
	if err != ErrEntryNotFound {
		t.Errorf("Expected ErrEntryNotFound for deleted entry, got: %v", err)
	}

	// Test deleting non-existent entry
	nonExistentID := uuid.New()
	err = DeleteEntry(ctx, testDB, nonExistentID)
	if err != ErrEntryNotFound {
		t.Errorf("Expected ErrEntryNotFound for non-existent entry, got: %v", err)
	}
}

func TestDeleteEntriesByJournal(t *testing.T) {
	testDB, journalID := setupTestDBWithJournal(t)
	defer testDB.Close()

	ctx := context.Background()

	// Create a second journal
	secondJournal, err := CreateJournal(ctx, testDB, "Second Journal", "Another test journal")
	if err != nil {
		t.Fatalf("Failed to create second test journal: %v", err)
	}

	// Create several entries in different journals
	for i := 0; i < 3; i++ {
		_, err := CreateEntry(ctx, testDB, journalID, 
			 "Journal 1 Entry", "Content", "text/plain")
		if err != nil {
			t.Fatalf("Failed to create test entry for journal 1: %v", err)
		}
	}

	for i := 0; i < 2; i++ {
		_, err := CreateEntry(ctx, testDB, secondJournal.ID, 
			"Journal 2 Entry", "Content", "text/plain")
		if err != nil {
			t.Fatalf("Failed to create test entry for journal 2: %v", err)
		}
	}

	// Delete entries for the first journal
	count, err := DeleteEntriesByJournal(ctx, testDB, journalID)
	if err != nil {
		t.Fatalf("DeleteEntriesByJournal failed: %v", err)
	}

	if count != 3 {
		t.Errorf("Expected to delete 3 entries from journal 1, got %d", count)
	}

	// Verify entries for the first journal are deleted
	entriesForJournal1, err := ListEntries(ctx, testDB, journalID)
	if err != nil {
		t.Fatalf("ListEntries failed after deletion: %v", err)
	}

	if len(entriesForJournal1) != 0 {
		t.Errorf("Expected 0 entries for journal 1 after deletion, got %d", len(entriesForJournal1))
	}

	// Verify entries for the second journal still exist
	entriesForJournal2, err := ListEntries(ctx, testDB, secondJournal.ID)
	if err != nil {
		t.Fatalf("ListEntries failed for journal 2: %v", err)
	}

	if len(entriesForJournal2) != 2 {
		t.Errorf("Expected 2 entries for journal 2 to remain, got %d", len(entriesForJournal2))
	}
}