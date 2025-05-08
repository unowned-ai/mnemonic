package memories

import (
	"database/sql"
	"fmt"
	"time"
	// No uuid needed for listing tags, unless we give tags IDs in the future
)

// ListTags retrieves all unique tags from the database.
func ListTags(db *sql.DB) ([]*Tag, error) {
	query := "SELECT tag, created_at, updated_at FROM tags ORDER BY tag ASC"

	rows, err := db.Query(query)
	if err != nil {
		return nil, fmt.Errorf("failed to query tags: %w", err)
	}
	defer rows.Close()

	var tags []*Tag
	for rows.Next() {
		var t Tag
		var createdAtFloat, updatedAtFloat float64

		if err := rows.Scan(&t.Tag, &createdAtFloat, &updatedAtFloat); err != nil {
			return nil, fmt.Errorf("failed to scan tag row: %w", err)
		}
		t.CreatedAt = time.Unix(int64(createdAtFloat), int64((createdAtFloat-float64(int64(createdAtFloat)))*1e9))
		t.UpdatedAt = time.Unix(int64(updatedAtFloat), int64((updatedAtFloat-float64(int64(updatedAtFloat)))*1e9))
		tags = append(tags, &t)
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating tag rows: %w", err)
	}

	return tags, nil
}
