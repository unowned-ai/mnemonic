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
func searchEntriesByTagMatchSQL(ctx context.Context, db *sql.DB, journalID uuid.UUID, queryTags []string) ([]MatchedEntry, error) {
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

// searchEntriesFullText performs a full text search combined with optional tag filtering.
// If queryTags is empty, no tag filtering is applied. textQuery must be non-empty.
// Results are ordered by the FTS rank and then by match count of tags.
func searchEntriesFullText(ctx context.Context, db *sql.DB, journalID uuid.UUID, queryTags []string, textQuery string) ([]MatchedEntry, error) {
	if strings.TrimSpace(textQuery) == "" {
		return nil, fmt.Errorf("textQuery must be non-empty for full text search")
	}

	var sb strings.Builder
	sb.WriteString(`SELECT
                e.id, e.journal_id, e.title, e.content, e.content_type, e.deleted, e.created_at, e.updated_at,
                COUNT(et.tag) as match_count,
                bm25(f) as rank
        FROM entries e
        JOIN entries_fts f ON e.id = f.entry_id
        LEFT JOIN entry_tags et ON e.id = et.entry_id`)

	var args []interface{}
	if len(queryTags) > 0 {
		placeholders := strings.Repeat("?,", len(queryTags)-1) + "?"
		sb.WriteString(" AND et.tag IN (" + placeholders + ")")
		for _, t := range queryTags {
			args = append(args, t)
		}
	}

	sb.WriteString(`
        WHERE e.journal_id = ? AND e.deleted = FALSE AND f MATCH ?
        GROUP BY e.id, e.journal_id, e.title, e.content, e.content_type, e.deleted, e.created_at, e.updated_at
        ORDER BY rank, match_count DESC, e.updated_at DESC;`)

	args = append(args, journalID, textQuery)

	rows, err := db.QueryContext(ctx, sb.String(), args...)
	if err != nil {
		return nil, fmt.Errorf("failed to execute full text search query: %w", err)
	}
	defer rows.Close()

	var results []MatchedEntry
	for rows.Next() {
		var me MatchedEntry
		var rank float64
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
			&rank,
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

// SearchEntries returns entries matching tags and/or full text.
// If textQuery is empty, the search is performed by tags only.
// When textQuery is provided, full text search is used with optional tag filtering.
func SearchEntries(ctx context.Context, db *sql.DB, journalID uuid.UUID, queryTags []string, textQuery string) ([]MatchedEntry, error) {
	if strings.TrimSpace(textQuery) != "" {
		return searchEntriesFullText(ctx, db, journalID, queryTags, textQuery)
	}
	return searchEntriesByTagMatchSQL(ctx, db, journalID, queryTags)
}
