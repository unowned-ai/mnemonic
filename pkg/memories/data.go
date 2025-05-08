package memories

import (
	"github.com/google/uuid"
)

type Journal struct {
	ID          uuid.UUID `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description,omitempty"`
	Active      bool      `json:"active"`
	CreatedAt   float64   `json:"created_at"`
	UpdatedAt   float64   `json:"updated_at"`
}

type Entry struct {
	ID          uuid.UUID `json:"id"`
	JournalID   uuid.UUID `json:"journal_id"`
	Title       string    `json:"title"`
	Content     string    `json:"content"`
	ContentType string    `json:"content_type"`
	CreatedAt   float64   `json:"created_at"`
	UpdatedAt   float64   `json:"updated_at"`
}

type Tag struct {
	Tag       string  `json:"tag"`
	CreatedAt float64 `json:"created_at"`
	UpdatedAt float64 `json:"updated_at"`
}

type EntryTag struct {
	EntryID   uuid.UUID `json:"entry_id"`
	Tag       string    `json:"tag"`
	CreatedAt float64   `json:"created_at"`
}