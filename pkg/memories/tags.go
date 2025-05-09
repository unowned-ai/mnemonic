package memories

import (
	"context"
	"database/sql"
	"errors"

	"github.com/google/uuid"
)

var (
	ErrTagNotFound = errors.New("tag not found")
)

const (
	createTagStatement = `
	INSERT OR IGNORE INTO tags (tag, created_at, updated_at) 
	VALUES (?, unixepoch(), unixepoch())
	`

	attachTagToEntryStatement = `
	INSERT OR IGNORE INTO entry_tags (entry_id, tag, created_at) 
	VALUES (?, ?, unixepoch())
	`

	detachTagFromEntryStatement = `
	DELETE FROM entry_tags 
	WHERE entry_id = ? AND tag = ?
	`

	listTagsStatement = `
	SELECT t.tag, t.created_at, t.updated_at
	FROM tags t
	JOIN entry_tags et ON t.tag = et.tag
	JOIN entries e ON et.entry_id = e.id
	WHERE e.journal_id = ?
	GROUP BY t.tag
	ORDER BY t.tag
	`

	listTagsForEntryStatement = `
	SELECT t.tag, t.created_at, t.updated_at
	FROM tags t
	JOIN entry_tags et ON t.tag = et.tag
	WHERE et.entry_id = ?
	ORDER BY t.tag
	`

	deleteTagStatement = `
	DELETE FROM tags 
	WHERE tag = ?
	`
)

func TagEntry(ctx context.Context, db *sql.DB, entryID uuid.UUID, tagName string) error {
	_, err := GetEntry(ctx, db, entryID)
	if err != nil {
		return err
	}

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	_, err = tx.ExecContext(ctx, createTagStatement, tagName)
	if err != nil {
		return err
	}

	_, err = tx.ExecContext(ctx, attachTagToEntryStatement, entryID, tagName)
	if err != nil {
		return err
	}

	return tx.Commit()
}

func DetachTag(ctx context.Context, db *sql.DB, entryID uuid.UUID, tagName string) error {
	_, err := GetEntry(ctx, db, entryID)
	if err != nil {
		return err
	}

	res, err := db.ExecContext(ctx, detachTagFromEntryStatement, entryID, tagName)
	if err != nil {
		return err
	}

	rowsAffected, err := res.RowsAffected()
	if err != nil {
		return err
	}

	if rowsAffected == 0 {
		return ErrTagNotFound
	}

	return nil
}

func ListTags(ctx context.Context, db *sql.DB, journalID uuid.UUID) ([]Tag, error) {
	_, err := GetJournal(ctx, db, journalID)
	if err != nil {
		return nil, err
	}

	rows, err := db.QueryContext(ctx, listTagsStatement, journalID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tags []Tag
	for rows.Next() {
		var tag Tag

		err := rows.Scan(
			&tag.Tag,
			&tag.CreatedAt,
			&tag.UpdatedAt,
		)
		if err != nil {
			return nil, err
		}

		tags = append(tags, tag)
	}

	if err = rows.Err(); err != nil {
		return nil, err
	}

	return tags, nil
}

func ListTagsForEntry(ctx context.Context, db *sql.DB, entryID uuid.UUID) ([]Tag, error) {
	_, err := GetEntry(ctx, db, entryID)
	if err != nil {
		return nil, err
	}

	rows, err := db.QueryContext(ctx, listTagsForEntryStatement, entryID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tags []Tag
	for rows.Next() {
		var tag Tag

		err := rows.Scan(
			&tag.Tag,
			&tag.CreatedAt,
			&tag.UpdatedAt,
		)
		if err != nil {
			return nil, err
		}

		tags = append(tags, tag)
	}

	if err = rows.Err(); err != nil {
		return nil, err
	}

	return tags, nil
}

func DeleteTag(ctx context.Context, db *sql.DB, tagName string) error {
	res, err := db.ExecContext(ctx, deleteTagStatement, tagName)
	if err != nil {
		return err
	}

	rowsAffected, err := res.RowsAffected()
	if err != nil {
		return err
	}

	if rowsAffected == 0 {
		return ErrTagNotFound
	}

	return nil
}
