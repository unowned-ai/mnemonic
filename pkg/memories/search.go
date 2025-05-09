package memories

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"github.com/google/uuid"
)

// MatchedEntry holds an Entry and the count of matching tags from a search query.
type MatchedEntry struct {
	Entry      // Embed the existing Entry type
	MatchCount int
}

// SearchEntriesByTagMatchSQL searches for entries in a specific journal that match the given query tags.
// Entries are ranked by the number of matching tags in descending order.
// Only non-deleted entries with at least one matching tag are returned.
func SearchEntriesByTagMatchSQL(ctx context.Context, db *sql.DB, journalID uuid.UUID, queryTags []string) ([]MatchedEntry, error) {
	if len(queryTags) == 0 {
		return []MatchedEntry{}, nil // No tags to search for, return empty result.
	}

	// Construct the IN clause placeholders for the SQL query
	placeholders := strings.Repeat("?,", len(queryTags)-1) + "?"

	// SQL query to find entries, count matching tags, and order by match count
	// We also include a secondary sort by updated_at to have stable ordering for ties.
	// Note: All columns from the entries table must be listed in GROUP BY if they are in SELECT.
	sqlQuery := fmt.Sprintf(`
		SELECT
			e.id, e.journal_id, e.title, e.content, e.content_type, e.deleted, e.created_at, e.updated_at,
			COUNT(et.tag) as match_count
		FROM
			entries e
		JOIN
			entry_tags et ON e.id = et.entry_id
		WHERE
			e.journal_id = ?
			AND e.deleted = FALSE
			AND et.tag IN (%s)
		GROUP BY
			e.id, e.journal_id, e.title, e.content, e.content_type, e.deleted, e.created_at, e.updated_at
		HAVING
			COUNT(et.tag) > 0
		ORDER BY
			match_count DESC,
			e.updated_at DESC;
	`, placeholders)

	// Prepare arguments for the SQL query
	args := make([]interface{}, 0, 1+len(queryTags))
	args = append(args, journalID) // First argument is the journalID
	for _, tag := range queryTags {
		args = append(args, tag) // Subsequent arguments are the query tags
	}

	rows, err := db.QueryContext(ctx, sqlQuery, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to execute search query: %w", err)
	}
	defer rows.Close()

	var results []MatchedEntry
	for rows.Next() {
		var me MatchedEntry
		err := rows.Scan(
			&me.Entry.ID,
			&me.Entry.JournalID,
			&me.Entry.Title,
			&me.Entry.Content,
			&me.Entry.ContentType,
			&me.Entry.Deleted,
			&me.Entry.CreatedAt,
			&me.Entry.UpdatedAt,
			&me.MatchCount,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan search result row: %w", err)
		}
		results = append(results, me)
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating over search results: %w", err)
	}

	return results, nil
}
