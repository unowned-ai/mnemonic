package memories

import (
	"database/sql"
	"time"

	"github.com/google/uuid"
)

// CreateJournal adds a new journal to the database.
func CreateJournal(db *sql.DB, name string, description string) (*Journal, error) {
	newID, err := uuid.NewRandom()
	if err != nil {
		return nil, err
	}

	now := time.Now()

	journal := &Journal{
		ID:          newID,
		Name:        name,
		Description: description,
		Active:      true, // Default to active
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	stmt, err := db.Prepare("INSERT INTO journals(id, name, description, active, created_at, updated_at) VALUES(?, ?, ?, ?, ?, ?)")
	if err != nil {
		return nil, err
	}
	defer stmt.Close()

	// When writing, .Unix() provides int64, which SQLite REAL can store.
	_, err = stmt.Exec(journal.ID, journal.Name, journal.Description, journal.Active, float64(journal.CreatedAt.UnixNano())/1e9, float64(journal.UpdatedAt.UnixNano())/1e9)
	if err != nil {
		return nil, err
	}

	return journal, nil
}

// ListJournals retrieves all journals from the database.
func ListJournals(db *sql.DB) ([]*Journal, error) {
	rows, err := db.Query("SELECT id, name, description, active, created_at, updated_at FROM journals ORDER BY created_at DESC")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var journals []*Journal
	for rows.Next() {
		var j Journal
		var createdAtFloat, updatedAtFloat float64
		if err := rows.Scan(&j.ID, &j.Name, &j.Description, &j.Active, &createdAtFloat, &updatedAtFloat); err != nil {
			return nil, err
		}
		j.CreatedAt = time.Unix(int64(createdAtFloat), int64((createdAtFloat-float64(int64(createdAtFloat)))*1e9))
		j.UpdatedAt = time.Unix(int64(updatedAtFloat), int64((updatedAtFloat-float64(int64(updatedAtFloat)))*1e9))
		journals = append(journals, &j)
	}

	if err = rows.Err(); err != nil {
		return nil, err
	}

	return journals, nil
}

// GetJournalByID retrieves a single journal by its ID from the database.
func GetJournalByID(db *sql.DB, id uuid.UUID) (*Journal, error) {
	var j Journal
	var createdAtFloat, updatedAtFloat float64

	row := db.QueryRow("SELECT id, name, description, active, created_at, updated_at FROM journals WHERE id = ?", id)
	err := row.Scan(&j.ID, &j.Name, &j.Description, &j.Active, &createdAtFloat, &updatedAtFloat)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil // Or a custom "not found" error
		}
		return nil, err
	}

	j.CreatedAt = time.Unix(int64(createdAtFloat), int64((createdAtFloat-float64(int64(createdAtFloat)))*1e9))
	j.UpdatedAt = time.Unix(int64(updatedAtFloat), int64((updatedAtFloat-float64(int64(updatedAtFloat)))*1e9))

	return &j, nil
}

// GetJournalByName retrieves a single journal by its name from the database.
func GetJournalByName(db *sql.DB, name string) (*Journal, error) {
	var j Journal
	var createdAtFloat, updatedAtFloat float64

	row := db.QueryRow("SELECT id, name, description, active, created_at, updated_at FROM journals WHERE name = ?", name)
	err := row.Scan(&j.ID, &j.Name, &j.Description, &j.Active, &createdAtFloat, &updatedAtFloat)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil // Explicitly return nil, nil for not found
		}
		return nil, err // Other scan or query errors
	}

	j.CreatedAt = time.Unix(int64(createdAtFloat), int64((createdAtFloat-float64(int64(createdAtFloat)))*1e9))
	j.UpdatedAt = time.Unix(int64(updatedAtFloat), int64((updatedAtFloat-float64(int64(updatedAtFloat)))*1e9))

	return &j, nil
}

// UpdateJournal updates specific fields of an existing journal in the database.
// It only updates fields for which a non-nil value is provided.
func UpdateJournal(db *sql.DB, id uuid.UUID, name *string, description *string, active *bool) (*Journal, error) {
	now := time.Now()
	query := "UPDATE journals SET updated_at = ?"
	args := []interface{}{float64(now.UnixNano()) / 1e9}

	if name != nil {
		query += ", name = ?"
		args = append(args, *name)
	}
	if description != nil {
		query += ", description = ?"
		args = append(args, *description)
	}
	if active != nil {
		query += ", active = ?"
		args = append(args, *active)
	}

	query += " WHERE id = ?"
	args = append(args, id)

	if name == nil && description == nil && active == nil {
		// If only updated_at is to change, we still need to run an update.
		// However, if no specific fields were provided to change, perhaps GetJournalByID is sufficient
		// if the user expectation is that no DB write occurs. Let's update updated_at anyway.
		// The current logic requires at least one of name, desc, active to be non-nil to reach here for update.
		// Let's make this more explicit: if nothing else is changing, just update the timestamp.
		// Forcing the update: if no other fields are provided, it will just update updated_at.
		// This is slightly different from previous where it might just return GetJournalByID.
	}

	stmt, err := db.Prepare(query)
	if err != nil {
		return nil, err
	}
	defer stmt.Close()

	result, err := stmt.Exec(args...)
	if err != nil {
		return nil, err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return nil, err
	}
	if rowsAffected == 0 {
		return nil, nil // Or a custom "not found" error / no rows affected
	}

	return GetJournalByID(db, id) // Fetch the updated journal
}

// DeleteJournal removes a journal and its associated entries (due to CASCADE) from the database.
func DeleteJournal(db *sql.DB, id uuid.UUID) error {
	stmt, err := db.Prepare("DELETE FROM journals WHERE id = ?")
	if err != nil {
		return err
	}
	defer stmt.Close()

	result, err := stmt.Exec(id)
	if err != nil {
		return err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		// This error is about getting RowsAffected, not the deletion itself if it already happened.
		return err
	}

	if rowsAffected == 0 {
		return sql.ErrNoRows // Indicate that no journal was found with that ID
	}

	return nil
}
