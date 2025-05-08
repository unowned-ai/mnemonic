package db

const (
	// SchemaV1 defines the SQL statements for version 1 of the database schema.
	// This schema pertains to the 'memoriesdb' component.
	SchemaV1 = `
CREATE TABLE IF NOT EXISTS mnemonic_versions (
    component TEXT PRIMARY KEY,
    version INTEGER NOT NULL,
    created_at REAL DEFAULT (unixepoch())
);

CREATE TABLE IF NOT EXISTS journals (
    id UUID PRIMARY KEY,
    name VARCHAR(256) NOT NULL,
    description TEXT,
    active BOOLEAN DEFAULT TRUE,
    created_at REAL DEFAULT (unixepoch()),
    updated_at REAL DEFAULT (unixepoch())
);

CREATE TABLE IF NOT EXISTS entries (
    id UUID PRIMARY KEY,
    journal_id UUID NOT NULL REFERENCES journals(id) ON DELETE CASCADE,
    title VARCHAR(256) NOT NULL,
    content TEXT NOT NULL,
    content_type VARCHAR(64) DEFAULT 'text/plain',
    created_at REAL DEFAULT (unixepoch()),
    updated_at REAL DEFAULT (unixepoch())
);

CREATE TABLE IF NOT EXISTS tags (
    tag VARCHAR(256) PRIMARY KEY,
    created_at REAL DEFAULT (unixepoch()),
    updated_at REAL DEFAULT (unixepoch())
);

CREATE TABLE IF NOT EXISTS entry_tags (
    entry_id UUID NOT NULL REFERENCES entries(id) ON DELETE CASCADE,
    tag VARCHAR(256) NOT NULL REFERENCES tags(tag) ON DELETE CASCADE,
    created_at REAL DEFAULT (unixepoch()),
    PRIMARY KEY (entry_id, tag)
);
`
)
