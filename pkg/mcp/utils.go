package mcp

import (
	"context"
	"database/sql"

	"github.com/google/uuid"
	"github.com/unowned-ai/mnemonic/pkg/memories"
)

// getJournalByName searches for a journal by its name. If not found, it returns nil, nil.
func getJournalByName(ctx context.Context, db *sql.DB, name string) (*memories.Journal, error) {
	journals, err := memories.ListJournals(ctx, db, false)
	if err != nil {
		return nil, err
	}
	for _, j := range journals {
		if j.Name == name {
			return &j, nil
		}
	}
	return nil, nil
}

// getEntryByTitleAndJournalID fetches an entry by its title within the specified journal.
// If no entry is found it returns nil, nil.
func getEntryByTitleAndJournalID(ctx context.Context, db *sql.DB, title string, journalID uuid.UUID) (*memories.Entry, error) {
	entries, err := memories.ListEntries(ctx, db, journalID, false)
	if err != nil {
		return nil, err
	}
	for _, e := range entries {
		if e.Title == title {
			return &e, nil
		}
	}
	return nil, nil
}
