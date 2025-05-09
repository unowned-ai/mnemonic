package memories

import (
	"context"
	"testing"

	"github.com/google/uuid"
)

func TestTagEntry(t *testing.T) {
	testDB, journalID := setupTestDBWithJournal(t)
	defer testDB.Close()

	ctx := context.Background()
	
	// Create an entry to tag
	entry, err := CreateEntry(ctx, testDB, journalID, "Entry to Tag", "Content", "text/plain")
	if err != nil {
		t.Fatalf("Failed to create test entry: %v", err)
	}

	// Tag the entry
	tagName := "test-tag"
	err = TagEntry(ctx, testDB, entry.ID, tagName)
	if err != nil {
		t.Fatalf("TagEntry failed: %v", err)
	}

	// Verify the tag was attached by listing tags for the entry
	tags, err := ListTagsForEntry(ctx, testDB, entry.ID)
	if err != nil {
		t.Fatalf("ListTagsForEntry failed: %v", err)
	}

	if len(tags) != 1 {
		t.Errorf("Expected 1 tag for entry, got %d", len(tags))
	}

	if len(tags) > 0 && tags[0].Tag != tagName {
		t.Errorf("Expected tag name %s, got %s", tagName, tags[0].Tag)
	}

	// Test tagging with a non-existent entry
	nonExistentID := uuid.New()
	err = TagEntry(ctx, testDB, nonExistentID, tagName)
	if err != ErrEntryNotFound {
		t.Errorf("Expected ErrEntryNotFound for non-existent entry, got: %v", err)
	}

	// Test adding the same tag again (should not error due to ON CONFLICT IGNORE)
	err = TagEntry(ctx, testDB, entry.ID, tagName)
	if err != nil {
		t.Errorf("Adding the same tag twice should not error, got: %v", err)
	}
}

func TestListTags(t *testing.T) {
	testDB, journalID := setupTestDBWithJournal(t)
	defer testDB.Close()

	ctx := context.Background()

	// Create a second journal
	secondJournal, err := CreateJournal(ctx, testDB, "Second Journal", "Another test journal")
	if err != nil {
		t.Fatalf("Failed to create second test journal: %v", err)
	}

	// Create entries in different journals
	entry1, err := CreateEntry(ctx, testDB, journalID, "Entry 1", "Content 1", "text/plain")
	if err != nil {
		t.Fatalf("Failed to create test entry 1: %v", err)
	}

	entry2, err := CreateEntry(ctx, testDB, journalID, "Entry 2", "Content 2", "text/plain")
	if err != nil {
		t.Fatalf("Failed to create test entry 2: %v", err)
	}

	entry3, err := CreateEntry(ctx, testDB, secondJournal.ID, "Entry 3", "Content 3", "text/plain")
	if err != nil {
		t.Fatalf("Failed to create test entry 3: %v", err)
	}

	// Add tags to entries
	tags := []struct {
		entryID uuid.UUID
		tag     string
	}{
		{entry1.ID, "tag1"},
		{entry1.ID, "tag2"},
		{entry2.ID, "tag2"},
		{entry2.ID, "tag3"},
		{entry3.ID, "tag4"},
	}

	for _, tag := range tags {
		err := TagEntry(ctx, testDB, tag.entryID, tag.tag)
		if err != nil {
			t.Fatalf("Failed to tag entry: %v", err)
		}
	}

	// Test listing tags for the first journal
	tagsForJournal1, err := ListTags(ctx, testDB, journalID)
	if err != nil {
		t.Fatalf("ListTags failed: %v", err)
	}

	if len(tagsForJournal1) != 3 {
		t.Errorf("Expected 3 unique tags for journal 1, got %d", len(tagsForJournal1))
	}

	// Test listing tags for the second journal
	tagsForJournal2, err := ListTags(ctx, testDB, secondJournal.ID)
	if err != nil {
		t.Fatalf("ListTags failed: %v", err)
	}

	if len(tagsForJournal2) != 1 {
		t.Errorf("Expected 1 tag for journal 2, got %d", len(tagsForJournal2))
	}

	// Test listing tags for a non-existent journal
	nonExistentJournalID := uuid.New()
	_, err = ListTags(ctx, testDB, nonExistentJournalID)
	if err != ErrJournalNotFound {
		t.Errorf("Expected ErrJournalNotFound for non-existent journal, got: %v", err)
	}
}

func TestDetachTag(t *testing.T) {
	testDB, journalID := setupTestDBWithJournal(t)
	defer testDB.Close()

	ctx := context.Background()
	
	// Create an entry and tag it
	entry, err := CreateEntry(ctx, testDB, journalID, "Entry to Tag", "Content", "text/plain")
	if err != nil {
		t.Fatalf("Failed to create test entry: %v", err)
	}

	tagName := "test-tag"
	err = TagEntry(ctx, testDB, entry.ID, tagName)
	if err != nil {
		t.Fatalf("TagEntry failed: %v", err)
	}

	// Detach the tag
	err = DetachTag(ctx, testDB, entry.ID, tagName)
	if err != nil {
		t.Fatalf("DetachTag failed: %v", err)
	}

	// Verify the tag was detached
	tags, err := ListTagsForEntry(ctx, testDB, entry.ID)
	if err != nil {
		t.Fatalf("ListTagsForEntry failed: %v", err)
	}

	if len(tags) != 0 {
		t.Errorf("Expected 0 tags for entry after detaching, got %d", len(tags))
	}

	// Test detaching a non-existent tag
	err = DetachTag(ctx, testDB, entry.ID, "non-existent-tag")
	if err != ErrTagNotFound {
		t.Errorf("Expected ErrTagNotFound for non-existent tag, got: %v", err)
	}
}

func TestDeleteTag(t *testing.T) {
	testDB, journalID := setupTestDBWithJournal(t)
	defer testDB.Close()

	ctx := context.Background()
	
	// Create entries and tag them
	entry1, err := CreateEntry(ctx, testDB, journalID, "Entry 1", "Content 1", "text/plain")
	if err != nil {
		t.Fatalf("Failed to create test entry 1: %v", err)
	}

	entry2, err := CreateEntry(ctx, testDB, journalID, "Entry 2", "Content 2", "text/plain")
	if err != nil {
		t.Fatalf("Failed to create test entry 2: %v", err)
	}

	tagName := "tag-to-delete"
	err = TagEntry(ctx, testDB, entry1.ID, tagName)
	if err != nil {
		t.Fatalf("TagEntry failed for entry 1: %v", err)
	}

	err = TagEntry(ctx, testDB, entry2.ID, tagName)
	if err != nil {
		t.Fatalf("TagEntry failed for entry 2: %v", err)
	}

	// Verify the tag is attached to both entries
	tags1, err := ListTagsForEntry(ctx, testDB, entry1.ID)
	if err != nil {
		t.Fatalf("ListTagsForEntry failed for entry 1: %v", err)
	}
	if len(tags1) != 1 {
		t.Errorf("Expected 1 tag for entry 1, got %d", len(tags1))
	}

	tags2, err := ListTagsForEntry(ctx, testDB, entry2.ID)
	if err != nil {
		t.Fatalf("ListTagsForEntry failed for entry 2: %v", err)
	}
	if len(tags2) != 1 {
		t.Errorf("Expected 1 tag for entry 2, got %d", len(tags2))
	}

	// Delete the tag
	err = DeleteTag(ctx, testDB, tagName)
	if err != nil {
		t.Fatalf("DeleteTag failed: %v", err)
	}

	// Verify the tag is detached from both entries
	tags1After, err := ListTagsForEntry(ctx, testDB, entry1.ID)
	if err != nil {
		t.Fatalf("ListTagsForEntry failed for entry 1 after deletion: %v", err)
	}
	if len(tags1After) != 0 {
		t.Errorf("Expected 0 tags for entry 1 after deletion, got %d", len(tags1After))
	}

	tags2After, err := ListTagsForEntry(ctx, testDB, entry2.ID)
	if err != nil {
		t.Fatalf("ListTagsForEntry failed for entry 2 after deletion: %v", err)
	}
	if len(tags2After) != 0 {
		t.Errorf("Expected 0 tags for entry 2 after deletion, got %d", len(tags2After))
	}

	// Test deleting a non-existent tag
	err = DeleteTag(ctx, testDB, "non-existent-tag")
	if err != ErrTagNotFound {
		t.Errorf("Expected ErrTagNotFound for non-existent tag, got: %v", err)
	}
}