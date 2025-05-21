package tui

import (
	"context"
	"database/sql"

	"github.com/unowned-ai/recall/pkg/memories"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/google/uuid"
)

// List journals from the database and return tea data
func listJournals(db *sql.DB) tea.Cmd {
	return func() tea.Msg {
		journals, err := memories.ListJournals(context.Background(), db, false)
		if err != nil {
			return err
		}
		return journals
	}
}

// List entries for journal from the database and return tea data
func listEntries(db *sql.DB, journalID uuid.UUID, includeDeleted bool) tea.Cmd {
	return func() tea.Msg {
		entries, err := memories.ListEntries(context.Background(), db, journalID, includeDeleted)
		if err != nil {
			return err
		}
		return entries
	}
}

type entryDetailsMsg struct {
	entry memories.Entry
	tags  []memories.Tag
}

// Get a combined message with the entry and its tags
func getEntryDetails(db *sql.DB, entryID uuid.UUID) tea.Cmd {
	return func() tea.Msg {
		entry, err := memories.GetEntry(context.Background(), db, entryID)
		if err != nil {
			return err
		}
		tags, err := memories.ListTagsForEntry(context.Background(), db, entry.ID)
		if err != nil {
			return err
		}
		return entryDetailsMsg{entry: entry, tags: tags}
	}
}

// Get database name and file path
func getDbPragmaList(db *sql.DB) (string, string) {
	var name, file string
	err := db.QueryRow(`PRAGMA database_list`).Scan(new(int), &name, &file)
	if err != nil {
		return name, file
	}
	return name, file
}
