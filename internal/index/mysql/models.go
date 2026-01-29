package mysql

import (
	"time"

	"gorm.io/gorm"
)

// Document represents a document in the database.
type Document struct {
	ID        string         `gorm:"primaryKey;type:varchar(255)" json:"id"`
	Source    string         `gorm:"type:varchar(500);not null" json:"source"`
	Title     string         `gorm:"type:varchar(500)" json:"title"`
	Format    string         `gorm:"type:varchar(50);not null" json:"format"`
	Hash      string         `gorm:"type:varchar(64);not null;index" json:"hash"`
	Metadata  string         `gorm:"type:json" json:"-"` // JSON string, use MetadataMap for access
	CreatedAt time.Time      `gorm:"index" json:"created_at"`
	UpdatedAt time.Time      `gorm:"index" json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"deleted_at,omitempty"`
}

// TableName returns the table name.
func (Document) TableName() string {
	return "documents"
}

// Chunk represents a text chunk in the database.
type Chunk struct {
	ID          string    `gorm:"primaryKey;type:varchar(255)" json:"id"`
	DocumentID  string    `gorm:"type:varchar(255);not null;index:idx_chunks_document_id" json:"document_id"`
	Content     string    `gorm:"type:text;not null" json:"content"`
	SectionPath string    `gorm:"type:varchar(500)" json:"section_path"`
	Position    int       `gorm:"not null" json:"position"`
	Metadata    string    `gorm:"type:json" json:"-"` // JSON string
	Hash        string    `gorm:"type:varchar(64);not null" json:"hash"`
	Version     int       `gorm:"default:1;index:idx_chunks_document_version" json:"version"`
	CreatedAt   time.Time `gorm:"index" json:"created_at"`
}

// TableName returns the table name.
func (Chunk) TableName() string {
	return "chunks"
}

// DocumentVersion represents a document version snapshot.
type DocumentVersion struct {
	ID         uint      `gorm:"primaryKey;autoIncrement" json:"id"`
	DocumentID string    `gorm:"type:varchar(255);not null;index:idx_doc_versions_doc_id;uniqueIndex:idx_doc_versions_unique" json:"document_id"`
	Version    int       `gorm:"not null;uniqueIndex:idx_doc_versions_unique" json:"version"`
	Hash       string    `gorm:"type:varchar(64);not null" json:"hash"`
	ChunkCount int       `gorm:"not null" json:"chunk_count"`
	ChangeNote string    `gorm:"type:text" json:"change_note"`
	CreatedAt  time.Time `gorm:"index" json:"created_at"`
	CreatedBy  string    `gorm:"type:varchar(255)" json:"created_by"`
}

// TableName returns the table name.
func (DocumentVersion) TableName() string {
	return "document_versions"
}


