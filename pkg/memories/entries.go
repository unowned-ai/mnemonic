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
	INSERT INTO entries (id, journal_id, title, content, content_type, deleted) 
	VALUES (?, ?, ?, ?, ?, ?)
	`

	getEntryStatement = `
	SELECT id, journal_id, title, content, content_type, deleted, created_at, updated_at 
	FROM entries 
	WHERE id = ?
	`

	listEntriesStatement = `
	SELECT id, journal_id, title, content, content_type, deleted, created_at, updated_at 
	FROM entries
	WHERE journal_id = ? AND deleted = ?
	ORDER BY updated_at DESC
	`

	updateEntryStatement = `
	UPDATE entries 
	SET title = ?, content = ?, content_type = ?, updated_at = unixepoch()
	WHERE id = ?
	`

	softDeleteEntryStatement = `
	UPDATE entries 
	SET deleted = TRUE, updated_at = unixepoch()
	WHERE id = ?
	`

	cleanDeletedEntriesStatement = `
	DELETE FROM entries 
	WHERE journal_id = ? AND deleted = TRUE
	`

	deleteEntriesByJournalStatement = `
	DELETE FROM entries 
	WHERE journal_id = ?
	`
)

func CreateEntry(ctx context.Context, db *sql.DB, journalID uuid.UUID, title, content, contentType string) (Entry, error) {
	entryID := uuid.New()

	_, err := GetJournal(ctx, db, journalID)
	if err != nil {
		if errors.Is(err, ErrJournalNotFound) {
			return Entry{}, ErrJournalNotFound
		}
		return Entry{}, err
	}

	if contentType == "" {
		contentType = "text/plain"
	}

	deleted := false

	_, err = db.ExecContext(
		ctx,
		createEntryStatement,
		entryID,
		journalID,
		title,
		content,
		contentType,
		deleted,
	)
	if err != nil {
		return Entry{}, err
	}

	return GetEntry(ctx, db, entryID)
}

// GetEntry retrieves an entry using a database connection.
func GetEntry(ctx context.Context, db *sql.DB, id uuid.UUID) (Entry, error) {
	var entry Entry

	err := db.QueryRowContext(ctx, getEntryStatement, id).Scan(
		&entry.ID,
		&entry.JournalID,
		&entry.Title,
		&entry.Content,
		&entry.ContentType,
		&entry.Deleted,
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
func ListEntries(ctx context.Context, db *sql.DB, journalID uuid.UUID, includeDeleted bool) ([]Entry, error) {
	_, err := GetJournal(ctx, db, journalID)
	if err != nil {
		if errors.Is(err, ErrJournalNotFound) {
			return nil, ErrJournalNotFound
		}
		return nil, err
	}
	rows, err := db.QueryContext(ctx, listEntriesStatement, journalID, includeDeleted)
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
			&entry.Deleted,
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
	existingEntry, err := GetEntry(ctx, db, id)
	if err != nil {
		return Entry{}, err
	}

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

	return GetEntry(ctx, db, id)
}

func DeleteEntry(ctx context.Context, db *sql.DB, id uuid.UUID) error {
	_, err := GetEntry(ctx, db, id)
	if err != nil {
		return err
	}

	res, err := db.ExecContext(ctx, softDeleteEntryStatement, id)
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

func CleanDeletedEntries(ctx context.Context, db *sql.DB, journalID uuid.UUID) (int64, error) {
	_, err := GetJournal(ctx, db, journalID)
	if err != nil {
		return 0, err
	}

	res, err := db.ExecContext(ctx, cleanDeletedEntriesStatement, journalID)
	if err != nil {
		return 0, err
	}

	return res.RowsAffected()
}
