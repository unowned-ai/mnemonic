package memories

import (
	"context"
	"database/sql"
	"errors"
	"sort"
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

// Helper to create an entry within a transaction for test setup
func createTestEntry(t *testing.T, ctx context.Context, db *sql.DB, journalID uuid.UUID, title, content, contentType string) Entry {
	t.Helper()
	entry, err := CreateEntry(ctx, db, journalID, title, content, contentType)
	if err != nil {
		t.Fatalf("CreateEntry failed in createTestEntry: %v", err)
	}
	return entry
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
	// GetEntry still uses *sql.DB and can be used to verify committed data.
	storedEntry, err := GetEntry(ctx, testDB, entry.ID)
	if err != nil {
		t.Fatalf("Failed to query database for stored entry using GetEntry: %v", err)
	}

	if storedEntry.ID != entry.ID || storedEntry.JournalID != journalID || storedEntry.Title != title || storedEntry.Content != content || storedEntry.ContentType != contentType {
		t.Errorf("Stored entry data doesn't match created entry data")
	}
	if storedEntry.CreatedAt <= 0 || entry.CreatedAt <= 0 {
		t.Errorf("Expected CreatedAt to be set, got stored: %f, entry: %f", storedEntry.CreatedAt, entry.CreatedAt)
	}
	if storedEntry.UpdatedAt <= 0 || entry.UpdatedAt <= 0 {
		t.Errorf("Expected UpdatedAt to be set, got stored: %f, entry: %f", storedEntry.UpdatedAt, entry.UpdatedAt)
	}
	if entry.CreatedAt != storedEntry.CreatedAt {
		t.Errorf("Entry CreatedAt (%f) doesn't match database value (%f)", entry.CreatedAt, storedEntry.CreatedAt)
	}
	if entry.UpdatedAt != storedEntry.UpdatedAt {
		t.Errorf("Entry UpdatedAt (%f) doesn't match database value (%f)", entry.UpdatedAt, storedEntry.UpdatedAt)
	}

	// Test creating an entry with a non-existent journal
	nonExistentJournalID := uuid.New()
	_, err = CreateEntry(ctx, testDB, nonExistentJournalID, "Title", "Content", "text/plain")
	if !errors.Is(err, ErrJournalNotFound) {
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

func assertTagsForEntry(t *testing.T, ctx context.Context, db *sql.DB, entryID uuid.UUID, expectedTagNames []string) {
	t.Helper()
	actualTags, err := ListTagsForEntry(ctx, db, entryID)
	if err != nil {
		t.Fatalf("ListTagsForEntry failed: %v", err)
	}

	actualTagNamesSlice := make([]string, len(actualTags))
	for i, tag := range actualTags {
		actualTagNamesSlice[i] = tag.Tag
	}

	if len(actualTagNamesSlice) != len(expectedTagNames) {
		t.Errorf("Expected %d tags, got %d. Expected: %v, Got: %v", len(expectedTagNames), len(actualTagNamesSlice), expectedTagNames, actualTagNamesSlice)
		return
	}

	sort.Strings(actualTagNamesSlice)
	sort.Strings(expectedTagNames)

	for i := range actualTagNamesSlice {
		if actualTagNamesSlice[i] != expectedTagNames[i] {
			t.Errorf("Tag mismatch. Expected tags %v, got %v", expectedTagNames, actualTagNamesSlice)
			return
		}
	}
}

func TestEntryTaggingWorkflow(t *testing.T) {
	testDB, journalID := setupTestDBWithJournal(t)
	defer testDB.Close()
	ctx := context.Background()

	// Step 1: Create an entry
	entry, err := CreateEntry(ctx, testDB, journalID, "Entry for Tagging", "Content", "text/plain")
	if err != nil {
		t.Fatalf("Failed to create entry for tagging test: %v", err)
	}

	// Case 1: Tag with new tags
	t.Run("TagWithNewTags", func(t *testing.T) {
		newTags := []string{"alpha", "beta"}
		for _, tagName := range newTags {
			if err := TagEntry(ctx, testDB, entry.ID, tagName); err != nil {
				t.Fatalf("TagEntry failed for new tag %s: %v", tagName, err)
			}
		}
		assertTagsForEntry(t, ctx, testDB, entry.ID, newTags)
		// Clean up tags for next sub-test if necessary, or use different entries
		// For simplicity, we'll assume tests can build on each other or use fresh entries.
		// Here, let's detach them to keep subtests somewhat isolated regarding this entry's tags.
		for _, tagName := range newTags {
			if err := DetachTag(ctx, testDB, entry.ID, tagName); err != nil {
				t.Logf("Warning: failed to detach tag %s during cleanup: %v", tagName, err)
			}
		}
	})

	// Case 2: Tag with existing tags
	t.Run("TagWithExistingTags", func(t *testing.T) {
		// Pre-populate some tags (they might exist from other tests or be created now)
		existingTagSetup := []string{"report", "data"}
		dummyEntry := createTestEntry(t, ctx, testDB, journalID, "Dummy For Existing Tags", "dummy", "text/plain")
		for _, tagName := range existingTagSetup {
			if err := TagEntry(ctx, testDB, dummyEntry.ID, tagName); err != nil {
				t.Fatalf("Failed to pre-populate existing tag %s: %v", tagName, err)
			}
		}

		// Now tag the main test entry with these existing tags
		for _, tagName := range existingTagSetup {
			if err := TagEntry(ctx, testDB, entry.ID, tagName); err != nil {
				t.Fatalf("TagEntry failed for existing tag %s: %v", tagName, err)
			}
		}
		assertTagsForEntry(t, ctx, testDB, entry.ID, existingTagSetup)
		for _, tagName := range existingTagSetup { // Cleanup
			if err := DetachTag(ctx, testDB, entry.ID, tagName); err != nil {
				t.Logf("Warning: failed to detach tag %s: %v", tagName, err)
			}
		}
	})

	// Case 3: Tag with mixed new and pre-existing tags
	t.Run("TagWithMixedTags", func(t *testing.T) {
		preExistingTag := "technical-report" // Ensure this is unique or created
		dummyEntry := createTestEntry(t, ctx, testDB, journalID, "Dummy For Mixed", "dummy", "text/plain")
		if err := TagEntry(ctx, testDB, dummyEntry.ID, preExistingTag); err != nil {
			t.Fatalf("Failed to pre-populate mixed tag %s: %v", preExistingTag, err)
		}

		mixedTags := []string{preExistingTag, "new-analysis-tag"}
		for _, tagName := range mixedTags {
			if err := TagEntry(ctx, testDB, entry.ID, tagName); err != nil {
				t.Fatalf("TagEntry failed for mixed tag %s: %v", tagName, err)
			}
		}
		assertTagsForEntry(t, ctx, testDB, entry.ID, mixedTags)
		for _, tagName := range mixedTags { // Cleanup
			if err := DetachTag(ctx, testDB, entry.ID, tagName); err != nil {
				t.Logf("Warning: failed to detach tag %s: %v", tagName, err)
			}
		}
	})

	// Case 4: Tag with no tags (effectively, do nothing related to tagging)
	t.Run("TagWithNoTags", func(t *testing.T) {
		// Call createTestEntry which doesn't add tags, then assert no tags exist.
		noTagEntry := createTestEntry(t, ctx, testDB, journalID, "No Tag Entry", "content", "text/plain")
		assertTagsForEntry(t, ctx, testDB, noTagEntry.ID, []string{})
	})
}

func TestGetEntry(t *testing.T) {
	testDB, journalID := setupTestDBWithJournal(t)
	defer testDB.Close()

	ctx := context.Background()
	title := "Test Entry for Get"
	content := "This is the content of the test entry."
	contentType := "text/plain"

	// Create an entry to retrieve, using the helper for transactional creation
	createdEntry := createTestEntry(t, ctx, testDB, journalID, title, content, contentType)

	// Retrieve the entry using the non-transactional GetEntry
	retrievedEntry, err := GetEntry(ctx, testDB, createdEntry.ID)
	if err != nil {
		t.Fatalf("GetEntry failed: %v", err)
	}

	if createdEntry.ID != retrievedEntry.ID {
		t.Errorf("Entry ID mismatch: created %v, retrieved %v", createdEntry.ID, retrievedEntry.ID)
	}
	// ... other comparisons ...
	if createdEntry.Title != retrievedEntry.Title {
		t.Errorf("Title mismatch")
	}

	// Test retrieving non-existent entry
	nonExistentID := uuid.New()
	_, err = GetEntry(ctx, testDB, nonExistentID)
	if !errors.Is(err, ErrEntryNotFound) {
		t.Errorf("Expected ErrEntryNotFound for non-existent entry, got: %v", err)
	}
}

func TestListEntries(t *testing.T) {
	testDB, journalID1 := setupTestDBWithJournal(t) // Journal 1
	defer testDB.Close()

	ctx := context.Background()

	// Create a second journal
	journal2, err := CreateJournal(ctx, testDB, "Second Journal", "Another test journal")
	if err != nil {
		t.Fatalf("Failed to create second test journal: %v", err)
	}
	journalID2 := journal2.ID

	// Create several entries in different journals
	_ = createTestEntry(t, ctx, testDB, journalID1, "J1 Entry 1", "Content", "text/plain")
	_ = createTestEntry(t, ctx, testDB, journalID1, "J1 Entry 2", "Content", "text/plain")
	_ = createTestEntry(t, ctx, testDB, journalID2, "J2 Entry 1", "Content", "text/plain")

	// Test listing entries for the first journal
	entriesForJournal1, err := ListEntries(ctx, testDB, journalID1, false)
	if err != nil {
		t.Fatalf("ListEntries for journal 1 failed: %v", err)
	}
	if len(entriesForJournal1) != 2 {
		t.Errorf("Expected 2 entries for journal 1, got %d", len(entriesForJournal1))
	}
	for _, e := range entriesForJournal1 {
		if e.JournalID != journalID1 {
			t.Errorf("Listed entry %s has incorrect journal ID %s, expected %s", e.ID, e.JournalID, journalID1)
		}
	}

	// Test listing entries for a non-existent journal
	nonExistentJournalID := uuid.New()
	_, err = ListEntries(ctx, testDB, nonExistentJournalID, false)
	if !errors.Is(err, ErrJournalNotFound) {
		t.Errorf("Expected ErrJournalNotFound for non-existent journal, got: %v", err)
	}
}

func TestUpdateEntry(t *testing.T) {
	testDB, journalID := setupTestDBWithJournal(t)
	defer testDB.Close()
	ctx := context.Background()

	createdEntry := createTestEntry(t, ctx, testDB, journalID, "Original", "OrigContent", "text/plain")
	time.Sleep(50 * time.Millisecond) // Ensure timestamp difference

	newTitle := "Updated Title"
	newContent := "Updated content"
	newContentType := "text/markdown"

	updatedEntry, err := UpdateEntry(ctx, testDB, createdEntry.ID, newTitle, newContent, newContentType)
	if err != nil {
		t.Fatalf("UpdateEntry failed: %v", err)
	}
	if updatedEntry.Title != newTitle {
		t.Errorf("Expected updated title %s, got %s", newTitle, updatedEntry.Title)
	}
	if updatedEntry.UpdatedAt < createdEntry.UpdatedAt {
		t.Errorf("Expected updated_at (%f) to be greater than or equal to original (%f)", updatedEntry.UpdatedAt, createdEntry.UpdatedAt)
	}

	// Verify with GetEntry
	fetchedAfterUpdate, _ := GetEntry(ctx, testDB, createdEntry.ID)
	if fetchedAfterUpdate.Title != newTitle {
		t.Errorf("Fetched title after update is incorrect")
	}

	// Test updating non-existent entry
	_, err = UpdateEntry(ctx, testDB, uuid.New(), "Title", "Content", "text/plain")
	if !errors.Is(err, ErrEntryNotFound) {
		t.Errorf("Expected ErrEntryNotFound for non-existent entry update, got: %v", err)
	}
}

func TestSoftDeleteAndCleanEntries(t *testing.T) {
	testDB, journalID := setupTestDBWithJournal(t)
	defer testDB.Close()
	ctx := context.Background()

	entry1 := createTestEntry(t, ctx, testDB, journalID, "Entry 1 to delete", "Content", "text/plain")
	entry2 := createTestEntry(t, ctx, testDB, journalID, "Entry 2 to keep", "Content", "text/plain")

	// Soft delete the first entry
	if err := DeleteEntry(ctx, testDB, entry1.ID); err != nil {
		t.Fatalf("DeleteEntry failed: %v", err)
	}

	deletedEntry, _ := GetEntry(ctx, testDB, entry1.ID)
	if !deletedEntry.Deleted {
		t.Errorf("Expected entry1 to be marked as deleted")
	}

	activeEntries, _ := ListEntries(ctx, testDB, journalID, false)
	if len(activeEntries) != 1 || activeEntries[0].ID != entry2.ID {
		t.Errorf("Expected 1 active entry (entry2), got %d", len(activeEntries))
	}

	allEntries, _ := ListEntries(ctx, testDB, journalID, true)
	if len(allEntries) != 2 {
		t.Errorf("Expected 2 total entries, got %d", len(allEntries))
	}

	// Clean deleted entries
	cleanedCount, err := CleanDeletedEntries(ctx, testDB, journalID)
	if err != nil {
		t.Fatalf("CleanDeletedEntries failed: %v", err)
	}
	if cleanedCount != 1 {
		t.Errorf("Expected to clean 1 entry, got %d", cleanedCount)
	}

	_, err = GetEntry(ctx, testDB, entry1.ID)
	if !errors.Is(err, ErrEntryNotFound) { // After clean, it should be gone
		t.Errorf("Expected ErrEntryNotFound for cleaned entry, got %v", err)
	}

	entriesAfterClean, _ := ListEntries(ctx, testDB, journalID, true)
	if len(entriesAfterClean) != 1 || entriesAfterClean[0].ID != entry2.ID {
		t.Errorf("Expected 1 entry (entry2) after cleaning, got %d", len(entriesAfterClean))
	}
}

func TestDeleteEntry(t *testing.T) { // This tests the soft delete functionality specifically
	testDB, journalID := setupTestDBWithJournal(t)
	defer testDB.Close()
	ctx := context.Background()

	entryToDelete := createTestEntry(t, ctx, testDB, journalID, "Entry for Deletion Test", "Content", "text/plain")

	if err := DeleteEntry(ctx, testDB, entryToDelete.ID); err != nil {
		t.Fatalf("DeleteEntry failed: %v", err)
	}

	deletedEntry, _ := GetEntry(ctx, testDB, entryToDelete.ID)
	if !deletedEntry.Deleted {
		t.Errorf("Expected entry to be marked as deleted")
	}

	// Test deleting non-existent entry
	err := DeleteEntry(ctx, testDB, uuid.New())
	if !errors.Is(err, ErrEntryNotFound) {
		t.Errorf("Expected ErrEntryNotFound for non-existent entry deletion, got: %v", err)
	}
}

func TestDeleteEntriesByJournal(t *testing.T) {
	testDB, journalID1 := setupTestDBWithJournal(t) // Journal 1
	defer testDB.Close()
	ctx := context.Background()

	journal2, _ := CreateJournal(ctx, testDB, "Second Journal", "Test Journal 2")
	journalID2 := journal2.ID

	// Create entries
	_ = createTestEntry(t, ctx, testDB, journalID1, "J1E1", "c", "t")
	_ = createTestEntry(t, ctx, testDB, journalID1, "J1E2", "c", "t")
	_ = createTestEntry(t, ctx, testDB, journalID2, "J2E1", "c", "t")

	count, err := DeleteEntriesByJournal(ctx, testDB, journalID1)
	if err != nil {
		t.Fatalf("DeleteEntriesByJournal failed: %v", err)
	}
	if count != 2 {
		t.Errorf("Expected to delete 2 entries from journal 1, got %d", count)
	}

	entriesJ1, _ := ListEntries(ctx, testDB, journalID1, true) // include deleted to be sure
	if len(entriesJ1) != 0 {
		// DeleteEntriesByJournal is a hard delete from entries table.
		// If they were soft deleted, this check would differ.
		// Assuming it's a hard delete of entry records.
		t.Errorf("Expected 0 entries for journal 1 after DeleteEntriesByJournal, got %d", len(entriesJ1))
	}

	entriesJ2, _ := ListEntries(ctx, testDB, journalID2, false)
	if len(entriesJ2) != 1 {
		t.Errorf("Expected 1 entry to remain in journal 2, got %d", len(entriesJ2))
	}
}
