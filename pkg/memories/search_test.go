package memories

import (
	"context"
	"sort"
	"testing"

	"github.com/google/uuid"
)

// Helper function to sort MatchedEntry slices by MatchCount (desc) then by ID (asc for stable sort)
func sortMatchedEntries(entries []MatchedEntry) {
	sort.SliceStable(entries, func(i, j int) bool {
		if entries[i].MatchCount != entries[j].MatchCount {
			return entries[i].MatchCount > entries[j].MatchCount // Primary: MatchCount descending
		}
		return entries[i].Entry.ID.String() < entries[j].Entry.ID.String() // Secondary: ID ascending for tie-breaking
	})
}

func TestSearchEntriesByTagMatchSQL(t *testing.T) {
	testDB, journalID := setupTestDBWithJournal(t) // Assumes setupTestDBWithJournal is available from entries_test.go or similar
	defer testDB.Close()
	ctx := context.Background()

	// Setup: Create some entries and tags
	entry1 := createTestEntry(t, ctx, testDB, journalID, "Entry Alpha", "Content A", "text/plain")
	entry2 := createTestEntry(t, ctx, testDB, journalID, "Entry Beta", "Content B", "text/plain")
	entry3 := createTestEntry(t, ctx, testDB, journalID, "Entry Gamma", "Content C", "text/plain")

	// Tags for entry1: common, uniqueA, shared
	_ = TagEntry(ctx, testDB, entry1.ID, "common")
	_ = TagEntry(ctx, testDB, entry1.ID, "uniqueA")
	_ = TagEntry(ctx, testDB, entry1.ID, "shared")

	// Tags for entry2: common, uniqueB, shared
	_ = TagEntry(ctx, testDB, entry2.ID, "common")
	_ = TagEntry(ctx, testDB, entry2.ID, "uniqueB")
	_ = TagEntry(ctx, testDB, entry2.ID, "shared")

	// Tags for entry3: common, uniqueC
	_ = TagEntry(ctx, testDB, entry3.ID, "common")
	_ = TagEntry(ctx, testDB, entry3.ID, "uniqueC")

	t.Run("SearchWithMultipleMatchesAndRanking", func(t *testing.T) {
		queryTags := []string{"common", "shared"}
		results, err := SearchEntriesByTagMatchSQL(ctx, testDB, journalID, queryTags)
		if err != nil {
			t.Fatalf("SearchEntriesByTagMatchSQL failed: %v", err)
		}

		if len(results) != 3 { // entry1, entry2 (2 matches), entry3 (1 match)
			t.Fatalf("Expected 3 results, got %d. Results: %+v", len(results), results)
		}

		sortMatchedEntries(results) // Ensure stable order for assertions

		// Entry1 and Entry2 both match 2 tags.
		if results[0].MatchCount != 2 {
			t.Errorf("Expected first result to have 2 matches, got %d for ID %s", results[0].MatchCount, results[0].Entry.ID)
		}
		if results[1].MatchCount != 2 {
			t.Errorf("Expected second result to have 2 matches, got %d for ID %s", results[1].MatchCount, results[1].Entry.ID)
		}
		// Check that the first two are indeed entry1 and entry2 (order might vary based on secondary sort in SQL)
		if !((results[0].Entry.ID == entry1.ID && results[1].Entry.ID == entry2.ID) ||
			(results[0].Entry.ID == entry2.ID && results[1].Entry.ID == entry1.ID)) {
			t.Errorf("Expected first two results to be entry1 and entry2, got IDs %s and %s", results[0].Entry.ID, results[1].Entry.ID)
		}

		// Entry3 matches 1 tag ("common")
		if results[2].Entry.ID != entry3.ID || results[2].MatchCount != 1 {
			t.Errorf("Expected third result to be entry3 with 1 match, got ID %s with %d matches", results[2].Entry.ID, results[2].MatchCount)
		}
	})

	t.Run("SearchWithSingleTagMatch", func(t *testing.T) {
		queryTags := []string{"uniqueA"}
		results, err := SearchEntriesByTagMatchSQL(ctx, testDB, journalID, queryTags)
		if err != nil {
			t.Fatalf("SearchEntriesByTagMatchSQL failed: %v", err)
		}
		if len(results) != 1 {
			t.Fatalf("Expected 1 result, got %d", len(results))
		}
		if results[0].Entry.ID != entry1.ID || results[0].MatchCount != 1 {
			t.Errorf("Expected entry1 with 1 match, got ID %s with %d matches", results[0].Entry.ID, results[0].MatchCount)
		}
	})

	t.Run("SearchWithOneTagMatchingMultipleEntriesDifferently", func(t *testing.T) {
		queryTags := []string{"common"} // common is on entry1, entry2, entry3
		results, err := SearchEntriesByTagMatchSQL(ctx, testDB, journalID, queryTags)
		if err != nil {
			t.Fatalf("SearchEntriesByTagMatchSQL failed: %v", err)
		}
		if len(results) != 3 {
			t.Fatalf("Expected 3 results, got %d. Results: %+v", len(results), results)
		}
		// All should have MatchCount = 1. Order by updated_at DESC (implicitly by ID as created sequentially)
		foundE1, foundE2, foundE3 := false, false, false
		for _, r := range results {
			if r.MatchCount != 1 {
				t.Errorf("Expected MatchCount 1 for tag 'common', got %d for entry %s", r.MatchCount, r.Entry.ID)
			}
			if r.Entry.ID == entry1.ID {
				foundE1 = true
			}
			if r.Entry.ID == entry2.ID {
				foundE2 = true
			}
			if r.Entry.ID == entry3.ID {
				foundE3 = true
			}
		}
		if !foundE1 || !foundE2 || !foundE3 {
			t.Errorf("Did not find all entries (e1, e2, e3) that have the 'common' tag. E1:%t, E2:%t, E3:%t", foundE1, foundE2, foundE3)
		}
	})

	t.Run("SearchWithNonExistentTag", func(t *testing.T) {
		queryTags := []string{"nonexistenttag"}
		results, err := SearchEntriesByTagMatchSQL(ctx, testDB, journalID, queryTags)
		if err != nil {
			t.Fatalf("SearchEntriesByTagMatchSQL failed: %v", err)
		}
		if len(results) != 0 {
			t.Errorf("Expected 0 results for a non-existent tag, got %d", len(results))
		}
	})

	t.Run("SearchWithEmptyQueryTags", func(t *testing.T) {
		queryTags := []string{}
		results, err := SearchEntriesByTagMatchSQL(ctx, testDB, journalID, queryTags)
		if err != nil {
			t.Fatalf("SearchEntriesByTagMatchSQL failed: %v", err)
		}
		if len(results) != 0 {
			t.Errorf("Expected 0 results for empty query tags, got %d", len(results))
		}
	})

	t.Run("SearchInEmptyJournal", func(t *testing.T) {
		emptyJournalID := uuid.New() // Create a new journal ID that won't have entries
		// We need to ensure this journal actually exists for the query not to fail on journal existence itself.
		// CreateJournal is fine here as it's test setup.
		_, err := CreateJournal(ctx, testDB, "Empty Journal For Search Test", "")
		if err != nil {
			t.Fatalf("Failed to create empty journal for test: %v", err)
		}

		queryTags := []string{"common"}
		results, err := SearchEntriesByTagMatchSQL(ctx, testDB, emptyJournalID, queryTags)
		if err != nil {
			t.Fatalf("SearchEntriesByTagMatchSQL in empty journal failed: %v", err)
		}
		if len(results) != 0 {
			t.Errorf("Expected 0 results when searching in an empty (but existing) journal, got %d", len(results))
		}
	})
}
