package memories

import (
	"context"
	"database/sql"
	"errors"

	"github.com/google/uuid"
)

var (
	ErrJournalNotFound = errors.New("journal not found")
)

const (
	createJournalStatement = `
	INSERT INTO journals (id, name, description, active) 
	VALUES (?, ?, ?, ?)
	`

	getJournalStatement = `
	SELECT id, name, description, active, created_at, updated_at 
	FROM journals 
	WHERE id = ?
	`

	listJournalsStatement = `
	SELECT id, name, description, active, created_at, updated_at 
	FROM journals
	WHERE active = ? OR ? = false
	ORDER BY updated_at DESC
	`

	updateJournalStatement = `
	UPDATE journals 
	SET name = ?, description = ?, active = ?, updated_at = unixepoch()
	WHERE id = ?
	`

	deleteJournalStatement = `
	DELETE FROM journals 
	WHERE id = ?
	`

	deleteInactiveJournalsStatement = `
	DELETE FROM journals 
	WHERE active = false
	`
)

func CreateJournal(ctx context.Context, db *sql.DB, name, description string) (Journal, error) {
	journalID := uuid.New()

	_, err := db.ExecContext(
		ctx,
		createJournalStatement,
		journalID,
		name,
		description,
		true, // active
	)
	if err != nil {
		return Journal{}, err
	}

	return GetJournal(ctx, db, journalID)
}

func GetJournal(ctx context.Context, db *sql.DB, id uuid.UUID) (Journal, error) {
	var journal Journal

	err := db.QueryRowContext(ctx, getJournalStatement, id).Scan(
		&journal.ID,
		&journal.Name,
		&journal.Description,
		&journal.Active,
		&journal.CreatedAt,
		&journal.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Journal{}, ErrJournalNotFound
		}
		return Journal{}, err
	}

	return journal, nil
}

// TODO: Add pagination support
func ListJournals(ctx context.Context, db *sql.DB, activeOnly bool) ([]Journal, error) {
	rows, err := db.QueryContext(ctx, listJournalsStatement, activeOnly, activeOnly)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var journals []Journal
	for rows.Next() {
		var journal Journal

		err := rows.Scan(
			&journal.ID,
			&journal.Name,
			&journal.Description,
			&journal.Active,
			&journal.CreatedAt,
			&journal.UpdatedAt,
		)
		if err != nil {
			return nil, err
		}

		journals = append(journals, journal)
	}

	if err = rows.Err(); err != nil {
		return nil, err
	}

	return journals, nil
}

func UpdateJournal(ctx context.Context, db *sql.DB, id uuid.UUID, name, description string, active bool) (Journal, error) {
	res, err := db.ExecContext(
		ctx,
		updateJournalStatement,
		name,
		description,
		active,
		id,
	)
	if err != nil {
		return Journal{}, err
	}

	rowsAffected, err := res.RowsAffected()
	if err != nil {
		return Journal{}, err
	}

	if rowsAffected == 0 {
		return Journal{}, ErrJournalNotFound
	}

	return GetJournal(ctx, db, id)
}

func DeleteJournal(ctx context.Context, db *sql.DB, id uuid.UUID) error {
	res, err := db.ExecContext(ctx, deleteJournalStatement, id)
	if err != nil {
		return err
	}

	rowsAffected, err := res.RowsAffected()
	if err != nil {
		return err
	}

	if rowsAffected == 0 {
		return ErrJournalNotFound
	}

	return nil
}

func DeleteInactiveJournals(ctx context.Context, db *sql.DB) (int64, error) {
	res, err := db.ExecContext(ctx, deleteInactiveJournalsStatement)
	if err != nil {
		return 0, err
	}

	return res.RowsAffected()
}
