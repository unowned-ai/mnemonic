package memories

import (
	"context"
	"database/sql"
	"errors"

	"github.com/google/uuid"
)

var (
	ErrEntryNotFound = errors.New("entry not found")
)

const (
	createEntryStatement = `
	INSERT INTO entries (id, journal_id, title, content, content_type) 
	VALUES (?, ?, ?, ?, ?)
	`

	getEntryStatement = `
	SELECT id, journal_id, title, content, content_type, created_at, updated_at 
	FROM entries 
	WHERE id = ?
	`

	listEntriesStatement = `
	SELECT id, journal_id, title, content, content_type, created_at, updated_at 
	FROM entries
	WHERE journal_id = ?
	ORDER BY updated_at DESC
	`

	updateEntryStatement = `
	UPDATE entries 
	SET title = ?, content = ?, content_type = ?, updated_at = unixepoch()
	WHERE id = ?
	`

	deleteEntryStatement = `
	DELETE FROM entries 
	WHERE id = ?
	`

	deleteEntriesByJournalStatement = `
	DELETE FROM entries 
	WHERE journal_id = ?
	`
)

func CreateEntry(ctx context.Context, db *sql.DB, journalID uuid.UUID, title, content, contentType string) (Entry, error) {
	entryID := uuid.New()

	// Check if journal exists
	_, err := GetJournal(ctx, db, journalID)
	if err != nil {
		if errors.Is(err, ErrJournalNotFound) {
			return Entry{}, ErrJournalNotFound
		}
		return Entry{}, err
	}

	// Use default content type if not provided
	if contentType == "" {
		contentType = "text/plain"
	}

	_, err = db.ExecContext(
		ctx,
		createEntryStatement,
		entryID,
		journalID,
		title,
		content,
		contentType,
	)
	if err != nil {
		return Entry{}, err
	}

	// Fetch the entry to get the timestamps that SQLite created
	return GetEntry(ctx, db, entryID)
}

func GetEntry(ctx context.Context, db *sql.DB, id uuid.UUID) (Entry, error) {
	var entry Entry

	err := db.QueryRowContext(ctx, getEntryStatement, id).Scan(
		&entry.ID,
		&entry.JournalID,
		&entry.Title,
		&entry.Content,
		&entry.ContentType,
		&entry.CreatedAt,
		&entry.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Entry{}, ErrEntryNotFound
		}
		return Entry{}, err
	}

	return entry, nil
}

// TODO: Add pagination support
func ListEntries(ctx context.Context, db *sql.DB, journalID uuid.UUID) ([]Entry, error) {
	// First check if the journal exists
	_, err := GetJournal(ctx, db, journalID)
	if err != nil {
		if errors.Is(err, ErrJournalNotFound) {
			return nil, ErrJournalNotFound
		}
		return nil, err
	}

	rows, err := db.QueryContext(ctx, listEntriesStatement, journalID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var entries []Entry
	for rows.Next() {
		var entry Entry

		err := rows.Scan(
			&entry.ID,
			&entry.JournalID,
			&entry.Title,
			&entry.Content,
			&entry.ContentType,
			&entry.CreatedAt,
			&entry.UpdatedAt,
		)
		if err != nil {
			return nil, err
		}

		entries = append(entries, entry)
	}

	if err = rows.Err(); err != nil {
		return nil, err
	}

	return entries, nil
}

func UpdateEntry(ctx context.Context, db *sql.DB, id uuid.UUID, title, content, contentType string) (Entry, error) {
	// First check if the entry exists
	existingEntry, err := GetEntry(ctx, db, id)
	if err != nil {
		return Entry{}, err
	}

	// Use existing values if not provided
	if title == "" {
		title = existingEntry.Title
	}
	if content == "" {
		content = existingEntry.Content
	}
	if contentType == "" {
		contentType = existingEntry.ContentType
	}

	res, err := db.ExecContext(
		ctx,
		updateEntryStatement,
		title,
		content,
		contentType,
		id,
	)
	if err != nil {
		return Entry{}, err
	}

	rowsAffected, err := res.RowsAffected()
	if err != nil {
		return Entry{}, err
	}

	if rowsAffected == 0 {
		return Entry{}, ErrEntryNotFound
	}

	// Fetch the updated entry to get the new values including the timestamp
	return GetEntry(ctx, db, id)
}

func DeleteEntry(ctx context.Context, db *sql.DB, id uuid.UUID) error {
	res, err := db.ExecContext(ctx, deleteEntryStatement, id)
	if err != nil {
		return err
	}

	rowsAffected, err := res.RowsAffected()
	if err != nil {
		return err
	}

	if rowsAffected == 0 {
		return ErrEntryNotFound
	}

	return nil
}

func DeleteEntriesByJournal(ctx context.Context, db *sql.DB, journalID uuid.UUID) (int64, error) {
	// First check if the journal exists
	_, err := GetJournal(ctx, db, journalID)
	if err != nil {
		return 0, err
	}

	res, err := db.ExecContext(ctx, deleteEntriesByJournalStatement, journalID)
	if err != nil {
		return 0, err
	}

	return res.RowsAffected()
}