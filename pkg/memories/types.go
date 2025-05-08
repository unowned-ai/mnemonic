package memories

import (
	"time"

	"github.com/google/uuid"
)

// Journal represents a collection of entries.
type Journal struct {
	ID          uuid.UUID `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description,omitempty"`
	Active      bool      `json:"active"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// Entry represents a single memory record.
type Entry struct {
	ID          uuid.UUID `json:"id"`
	JournalID   uuid.UUID `json:"journal_id"`
	Title       string    `json:"title"`
	Content     string    `json:"content"`
	ContentType string    `json:"content_type,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
	Tags        []string  `json:"tags,omitempty"` // Added for convenience, will be populated from entry_tags
}

// Tag represents a keyword or label that can be associated with entries.
type Tag struct {
	Tag       string    `json:"tag"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// EntryTag represents the association between an entry and a tag.
// This struct is mainly for database operations and might not be directly exposed via API.
type EntryTag struct {
	EntryID   uuid.UUID `json:"entry_id"`
	Tag       string    `json:"tag"`
	CreatedAt time.Time `json:"created_at"`
}
