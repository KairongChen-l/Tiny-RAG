package index

import (
	"time"
)

// StoredDocument represents a document record in the database.
type StoredDocument struct {
	ID        string
	Source    string
	Title     string
	Format    string
	Hash      string
	Metadata  map[string]string
	CreatedAt time.Time
	UpdatedAt time.Time
	DeletedAt *time.Time // Soft delete timestamp (nil if not deleted)
}

// StoredChunk represents a chunk record in the database.
type StoredChunk struct {
	ID          string
	DocumentID  string
	Content     string
	SectionPath string
	Position    int
	Metadata    map[string]string
	Hash        string
	Version     int
}
