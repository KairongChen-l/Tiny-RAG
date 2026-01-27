// Package sqlite provides SQLite-based vector storage implementation.
package sqlite

// Schema contains the database schema SQL statements.
const Schema = `
-- Documents table
CREATE TABLE IF NOT EXISTS documents (
    id TEXT PRIMARY KEY,
    source TEXT NOT NULL,
    title TEXT,
    format TEXT NOT NULL,
    hash TEXT NOT NULL,
    metadata JSON,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    deleted_at TIMESTAMP
);

-- Full-text search virtual table (FTS5)
CREATE VIRTUAL TABLE IF NOT EXISTS documents_fts USING fts5(
    id UNINDEXED,
    title,
    content,
    content_rowid='rowid'
);

-- Chunks table
CREATE TABLE IF NOT EXISTS chunks (
    id TEXT PRIMARY KEY,
    document_id TEXT NOT NULL,
    content TEXT NOT NULL,
    section_path TEXT,
    position INTEGER NOT NULL,
    metadata JSON,
    hash TEXT NOT NULL,
    version INTEGER DEFAULT 1,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (document_id) REFERENCES documents(id) ON DELETE CASCADE
);

-- Indexes for chunks
CREATE INDEX IF NOT EXISTS idx_chunks_document_id ON chunks(document_id);
CREATE INDEX IF NOT EXISTS idx_chunks_document_version ON chunks(document_id, version);

-- Document versions table for version control
CREATE TABLE IF NOT EXISTS document_versions (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    document_id TEXT NOT NULL,
    version INTEGER NOT NULL,
    hash TEXT NOT NULL,
    chunk_count INTEGER NOT NULL,
    change_note TEXT,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    created_by TEXT,
    FOREIGN KEY (document_id) REFERENCES documents(id) ON DELETE CASCADE,
    UNIQUE(document_id, version)
);

-- Index for document versions
CREATE INDEX IF NOT EXISTS idx_document_versions_doc_id ON document_versions(document_id);
CREATE INDEX IF NOT EXISTS idx_document_versions_version ON document_versions(document_id, version DESC);

-- Jobs table
CREATE TABLE IF NOT EXISTS jobs (
    id TEXT PRIMARY KEY,
    type TEXT NOT NULL,
    status TEXT NOT NULL,
    progress INTEGER DEFAULT 0,
    error TEXT,
    result TEXT,
    payload BLOB,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Index for jobs
CREATE INDEX IF NOT EXISTS idx_jobs_status ON jobs(status);
CREATE INDEX IF NOT EXISTS idx_jobs_type_status ON jobs(type, status);
`

// VectorTableSchema creates the virtual table for vector storage.
// This requires sqlite-vec extension to be loaded.
// The dimension parameter should match the embedding model's output dimension.
func VectorTableSchema(dimension int) string {
	return `
CREATE VIRTUAL TABLE IF NOT EXISTS chunk_vectors USING vec0(
    chunk_id TEXT PRIMARY KEY,
    embedding float[` + itoa(dimension) + `]
);
`
}

// itoa converts int to string without importing strconv.
func itoa(i int) string {
	if i == 0 {
		return "0"
	}

	var b [20]byte
	idx := len(b)
	negative := i < 0
	if negative {
		i = -i
	}

	for i > 0 {
		idx--
		b[idx] = byte('0' + i%10)
		i /= 10
	}

	if negative {
		idx--
		b[idx] = '-'
	}

	return string(b[idx:])
}
