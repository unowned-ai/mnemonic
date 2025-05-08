package memories

import (
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

// CreateEntry adds a new entry to a specified journal, optionally with tags.
// It handles tag creation and association within a database transaction.
func CreateEntry(db *sql.DB, journalID uuid.UUID, title string, content string, contentType string, tags []string) (*Entry, error) {
	newID, err := uuid.NewRandom()
	if err != nil {
		return nil, fmt.Errorf("failed to generate entry UUID: %w", err)
	}

	if contentType == "" {
		contentType = "text/plain" // Default content type
	}
	now := time.Now()
	nowFloat := float64(now.UnixNano()) / 1e9

	entry := &Entry{
		ID:          newID,
		JournalID:   journalID,
		Title:       title,
		Content:     content,
		ContentType: contentType,
		CreatedAt:   now,
		UpdatedAt:   now,
		// Tags field will be populated after successful transaction
	}

	tx, err := db.Begin()
	if err != nil {
		return nil, fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback() // Rollback if not committed

	// Insert the entry
	entryStmt, err := tx.Prepare("INSERT INTO entries(id, journal_id, title, content, content_type, created_at, updated_at) VALUES(?, ?, ?, ?, ?, ?, ?)")
	if err != nil {
		return nil, fmt.Errorf("failed to prepare entry insert statement: %w", err)
	}
	defer entryStmt.Close()

	_, err = entryStmt.Exec(entry.ID, entry.JournalID, entry.Title, entry.Content, entry.ContentType, nowFloat, nowFloat)
	if err != nil {
		return nil, fmt.Errorf("failed to execute entry insert: %w", err)
	}

	// Handle tags
	processedTags := []string{}
	if len(tags) > 0 {
		tagNowFloat := float64(time.Now().UnixNano()) / 1e9 // Potentially slightly different if time passes
		tagInsertStmt, err := tx.Prepare("INSERT INTO tags(tag, created_at, updated_at) VALUES(?, ?, ?) ON CONFLICT(tag) DO UPDATE SET updated_at = excluded.updated_at")
		if err != nil {
			return nil, fmt.Errorf("failed to prepare tag upsert statement: %w", err)
		}
		defer tagInsertStmt.Close()

		entryTagStmt, err := tx.Prepare("INSERT INTO entry_tags(entry_id, tag, created_at) VALUES(?, ?, ?)")
		if err != nil {
			return nil, fmt.Errorf("failed to prepare entry_tag insert statement: %w", err)
		}
		defer entryTagStmt.Close()

		for _, tagName := range tags {
			tn := strings.TrimSpace(tagName)
			if tn == "" {
				continue
			}
			// Use tagNowFloat for consistency in this loop for tags table
			_, err = tagInsertStmt.Exec(tn, tagNowFloat, tagNowFloat)
			if err != nil {
				return nil, fmt.Errorf("failed to insert tag '%s': %w", tn, err)
			}

			// Use a fresh timestamp for entry_tags creation
			_, err = entryTagStmt.Exec(entry.ID, tn, float64(time.Now().UnixNano())/1e9)
			if err != nil {
				return nil, fmt.Errorf("failed to associate tag '%s' with entry: %w", tn, err)
			}
			processedTags = append(processedTags, tn)
		}
	}

	if err = tx.Commit(); err != nil {
		return nil, fmt.Errorf("failed to commit transaction: %w", err)
	}

	entry.Tags = processedTags
	return entry, nil
}

// ListEntries retrieves entries, optionally filtered by journal ID and/or tags.
// If tagNames are provided, an entry must have all specified tags.
// The Tags field of each Entry struct will be populated.
func ListEntries(db *sql.DB, journalID *uuid.UUID, tagNames []string) ([]*Entry, error) {
	var args []interface{}
	query := `
SELECT
    e.id, e.journal_id, e.title, e.content, e.content_type, e.created_at, e.updated_at,
    GROUP_CONCAT(et.tag) as concated_tags
FROM entries e
LEFT JOIN entry_tags et ON e.id = et.entry_id
`

	whereClauses := []string{}

	if journalID != nil {
		whereClauses = append(whereClauses, "e.journal_id = ?")
		args = append(args, *journalID)
	}

	if len(tagNames) > 0 {
		placeholders := make([]string, len(tagNames))
		for i := range tagNames {
			placeholders[i] = "?"
			args = append(args, tagNames[i])
		}
		// Subquery to find entries that have all the specified tags
		// Note: Using COUNT(DISTINCT et_sub.tag) might be important if entry_tags could have duplicates (though schema has PK)
		tagFilterSubquery := fmt.Sprintf(
			"e.id IN (SELECT et_sub.entry_id FROM entry_tags et_sub WHERE et_sub.tag IN (%s) GROUP BY et_sub.entry_id HAVING COUNT(et_sub.tag) = ?)",
			strings.Join(placeholders, ","),
		)
		whereClauses = append(whereClauses, tagFilterSubquery)
		args = append(args, len(tagNames))
	}

	if len(whereClauses) > 0 {
		query += "WHERE " + strings.Join(whereClauses, " AND ")
	}

	query += " GROUP BY e.id, e.journal_id, e.title, e.content, e.content_type, e.created_at, e.updated_at ORDER BY e.created_at DESC"

	rows, err := db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to query entries: %w, query: %s, args: %v", err, query, args)
	}
	defer rows.Close()

	var entries []*Entry
	for rows.Next() {
		var e Entry
		var createdAtFloat, updatedAtFloat float64
		var concatedTags sql.NullString // Handles NULL if an entry has no tags

		if err := rows.Scan(&e.ID, &e.JournalID, &e.Title, &e.Content, &e.ContentType, &createdAtFloat, &updatedAtFloat, &concatedTags); err != nil {
			return nil, fmt.Errorf("failed to scan entry row: %w", err)
		}
		e.CreatedAt = time.Unix(int64(createdAtFloat), int64((createdAtFloat-float64(int64(createdAtFloat)))*1e9))
		e.UpdatedAt = time.Unix(int64(updatedAtFloat), int64((updatedAtFloat-float64(int64(updatedAtFloat)))*1e9))

		if concatedTags.Valid && concatedTags.String != "" {
			e.Tags = strings.Split(concatedTags.String, ",")
		} else {
			e.Tags = []string{} // Ensure Tags is an empty slice, not nil, if no tags
		}
		entries = append(entries, &e)
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating entry rows: %w", err)
	}

	return entries, nil
}

// GetEntryByID retrieves a single entry by its ID, including its tags.
func GetEntryByID(db *sql.DB, id uuid.UUID) (*Entry, error) {
	query := `
SELECT
    e.id, e.journal_id, e.title, e.content, e.content_type, e.created_at, e.updated_at,
    GROUP_CONCAT(et.tag) as concated_tags
FROM entries e
LEFT JOIN entry_tags et ON e.id = et.entry_id
WHERE e.id = ?
GROUP BY e.id, e.journal_id, e.title, e.content, e.content_type, e.created_at, e.updated_at
`
	row := db.QueryRow(query, id)

	var e Entry
	var createdAtFloat, updatedAtFloat float64
	var concatedTags sql.NullString // Handles NULL if an entry has no tags

	err := row.Scan(&e.ID, &e.JournalID, &e.Title, &e.Content, &e.ContentType, &createdAtFloat, &updatedAtFloat, &concatedTags)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil // Or a custom "not found" error
		}
		return nil, fmt.Errorf("failed to scan entry row: %w", err)
	}

	e.CreatedAt = time.Unix(int64(createdAtFloat), int64((createdAtFloat-float64(int64(createdAtFloat)))*1e9))
	e.UpdatedAt = time.Unix(int64(updatedAtFloat), int64((updatedAtFloat-float64(int64(updatedAtFloat)))*1e9))

	if concatedTags.Valid && concatedTags.String != "" {
		e.Tags = strings.Split(concatedTags.String, ",")
	} else {
		e.Tags = []string{} // Ensure Tags is an empty slice, not nil, if no tags
	}

	return &e, nil
}

// GetEntryByTitleAndJournalID retrieves a single entry by its exact title within a specific journal.
// Returns nil, nil if not found.
func GetEntryByTitleAndJournalID(db *sql.DB, title string, journalID uuid.UUID) (*Entry, error) {
	query := `
SELECT
    e.id, e.journal_id, e.title, e.content, e.content_type, e.created_at, e.updated_at,
    GROUP_CONCAT(et.tag) as concated_tags
FROM entries e
LEFT JOIN entry_tags et ON e.id = et.entry_id
WHERE e.title = ? AND e.journal_id = ?
GROUP BY e.id, e.journal_id, e.title, e.content, e.content_type, e.created_at, e.updated_at
`
	row := db.QueryRow(query, title, journalID)

	var e Entry
	var createdAtFloat, updatedAtFloat float64
	var concatedTags sql.NullString

	err := row.Scan(&e.ID, &e.JournalID, &e.Title, &e.Content, &e.ContentType, &createdAtFloat, &updatedAtFloat, &concatedTags)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil // Not found
		}
		return nil, fmt.Errorf("failed to scan entry row: %w", err)
	}

	e.CreatedAt = time.Unix(int64(createdAtFloat), int64((createdAtFloat-float64(int64(createdAtFloat)))*1e9))
	e.UpdatedAt = time.Unix(int64(updatedAtFloat), int64((updatedAtFloat-float64(int64(updatedAtFloat)))*1e9))

	if concatedTags.Valid && concatedTags.String != "" {
		e.Tags = strings.Split(concatedTags.String, ",")
	} else {
		e.Tags = []string{}
	}

	return &e, nil
}

// UpdateEntry updates specific fields of an existing entry in the database.
// It only updates fields (title, content, contentType) for which a non-nil value is provided.
// Tags are not modified by this function; use ManageEntryTags for that.
func UpdateEntry(db *sql.DB, id uuid.UUID, title *string, content *string, contentType *string) (*Entry, error) {
	now := time.Now()
	nowFloat := float64(now.UnixNano()) / 1e9
	query := "UPDATE entries SET updated_at = ?"
	args := []interface{}{nowFloat}

	fieldsToUpdate := 0
	if title != nil {
		query += ", title = ?"
		args = append(args, *title)
		fieldsToUpdate++
	}
	if content != nil {
		query += ", content = ?"
		args = append(args, *content)
		fieldsToUpdate++
	}
	if contentType != nil {
		query += ", content_type = ?"
		args = append(args, *contentType)
		fieldsToUpdate++
	}

	// If no actual fields to update were provided, just fetch the current entry.
	// The updated_at timestamp won't be updated unless we make an actual DB write here.
	// For consistency, we can force an update to updated_at or decide that if no fields are changing, no update occurs.
	// Let's choose to perform an update only if at least one field is changing.
	if fieldsToUpdate == 0 {
		// To ensure updated_at is bumped even if no other fields change, one might issue a minimal update.
		// However, current setup only proceeds if fieldsToUpdate > 0. So, if nothing changes, we just fetch.
		// A more explicit way: if only updated_at is to change, do a specific update.
		// For now, if no fields given, we just fetch. If fields are given, updated_at also updates.
		// If the request is just to "touch" the entry, a specific command might be better.
		// Let's refine: if no specific fields are to be updated, return the current entry without modification.
		return GetEntryByID(db, id) // No actual change to title, content, or contentType
	}

	query += " WHERE id = ?"
	args = append(args, id)

	tx, err := db.Begin()
	if err != nil {
		return nil, fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	stmt, err := tx.Prepare(query)
	if err != nil {
		return nil, fmt.Errorf("failed to prepare entry update statement: %w", err)
	}
	defer stmt.Close()

	result, err := stmt.Exec(args...)
	if err != nil {
		return nil, fmt.Errorf("failed to execute entry update: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return nil, fmt.Errorf("failed to get rows affected: %w", err)
	}
	if rowsAffected == 0 {
		tx.Rollback()   // Ensure rollback if no rows affected (e.g. ID not found)
		return nil, nil // Or a custom "not found" error
	}

	if err = tx.Commit(); err != nil {
		return nil, fmt.Errorf("failed to commit transaction: %w", err)
	}

	return GetEntryByID(db, id) // Fetch the updated entry with its tags
}

// DeleteEntry removes an entry from the database.
// Associated tags in entry_tags are removed by CASCADE constraint.
func DeleteEntry(db *sql.DB, id uuid.UUID) error {
	tx, err := db.Begin()
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback() // Rollback if not committed

	stmt, err := tx.Prepare("DELETE FROM entries WHERE id = ?")
	if err != nil {
		return fmt.Errorf("failed to prepare entry delete statement: %w", err)
	}
	defer stmt.Close()

	result, err := stmt.Exec(id)
	if err != nil {
		return fmt.Errorf("failed to execute entry delete: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		tx.Rollback()        // Ensure rollback if no rows affected
		return sql.ErrNoRows // Indicate that no entry was found with that ID
	}

	if err = tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}

// ManageEntryTags adds or removes tags for a given entry.
// It ensures tags exist in the 'tags' table and updates associations in 'entry_tags'.
// The entry's 'updated_at' timestamp is modified if any changes occur.
func ManageEntryTags(db *sql.DB, entryID uuid.UUID, tagsToAdd []string, tagsToRemove []string) error {
	tx, err := db.Begin()
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback() // Rollback if not committed

	// 1. Check if entry exists
	var exists bool
	err = tx.QueryRow("SELECT EXISTS(SELECT 1 FROM entries WHERE id = ?)", entryID).Scan(&exists)
	if err != nil {
		return fmt.Errorf("failed to check if entry exists: %w", err)
	}
	if !exists {
		return sql.ErrNoRows // Entry not found
	}

	changed := false
	nowForTags := time.Now() // Use a single timestamp for all tag operations in this call
	nowForTagsFloat := float64(nowForTags.UnixNano()) / 1e9

	// 2. Process tagsToRemove
	if len(tagsToRemove) > 0 {
		removeStmt, err := tx.Prepare("DELETE FROM entry_tags WHERE entry_id = ? AND tag = ?")
		if err != nil {
			return fmt.Errorf("failed to prepare tag removal statement: %w", err)
		}
		defer removeStmt.Close()

		for _, tagName := range tagsToRemove {
			tn := strings.TrimSpace(tagName)
			if tn == "" {
				continue
			}
			result, err := removeStmt.Exec(entryID, tn)
			if err != nil {
				return fmt.Errorf("failed to remove tag '%s': %w", tn, err)
			}
			if rAff, _ := result.RowsAffected(); rAff > 0 {
				changed = true
			}
		}
	}

	// 3. Process tagsToAdd
	if len(tagsToAdd) > 0 {
		// Ensure tags exist in the 'tags' table (INSERT ... ON CONFLICT DO NOTHING)
		tagUpsertStmt, err := tx.Prepare("INSERT INTO tags(tag, created_at, updated_at) VALUES(?, ?, ?) ON CONFLICT(tag) DO UPDATE SET updated_at = excluded.updated_at")
		if err != nil {
			return fmt.Errorf("failed to prepare tag upsert statement: %w", err)
		}
		defer tagUpsertStmt.Close()

		// Associate entry with tag (INSERT ... ON CONFLICT DO NOTHING for entry_tags)
		entryTagAssociateStmt, err := tx.Prepare("INSERT INTO entry_tags(entry_id, tag, created_at) VALUES(?, ?, ?) ON CONFLICT(entry_id, tag) DO NOTHING")
		if err != nil {
			return fmt.Errorf("failed to prepare entry-tag association statement: %w", err)
		}
		defer entryTagAssociateStmt.Close()

		for _, tagName := range tagsToAdd {
			tn := strings.TrimSpace(tagName)
			if tn == "" {
				continue
			}
			// Upsert tag in 'tags' table
			_, err = tagUpsertStmt.Exec(tn, nowForTagsFloat, nowForTagsFloat)
			if err != nil {
				return fmt.Errorf("failed to upsert tag '%s': %w", tn, err)
			}

			// Associate with entry
			result, err := entryTagAssociateStmt.Exec(entryID, tn, nowForTagsFloat)
			if err != nil {
				return fmt.Errorf("failed to associate tag '%s' with entry: %w", tn, err)
			}
			if rAff, _ := result.RowsAffected(); rAff > 0 {
				changed = true // A new association was made
			}
		}
	}

	// 4. Update entry's updated_at timestamp if any tags were actually added or removed
	if changed {
		updateEntryStmt, err := tx.Prepare("UPDATE entries SET updated_at = ? WHERE id = ?")
		if err != nil {
			return fmt.Errorf("failed to prepare entry update timestamp statement: %w", err)
		}
		defer updateEntryStmt.Close()
		_, err = updateEntryStmt.Exec(nowForTagsFloat, entryID)
		if err != nil {
			return fmt.Errorf("failed to update entry timestamp: %w", err)
		}
	}

	// 5. Commit transaction
	if err = tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}
