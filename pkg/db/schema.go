package db

const (
	// SchemaV1 defines the SQL statements for version 1 of the database schema.
	// This schema pertains to the 'memoriesdb' component.
	SchemaV1 = `
CREATE TABLE IF NOT EXISTS recall_versions (
    component VARCHAR(64) PRIMARY KEY,
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
    deleted BOOLEAN DEFAULT FALSE,
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

-- Full text search virtual table for entry content
CREATE VIRTUAL TABLE IF NOT EXISTS entries_fts USING fts5(
    title,
    content,
    entry_id UNINDEXED
);

-- Trigger to keep FTS index updated on inserts
CREATE TRIGGER IF NOT EXISTS entries_fts_ai AFTER INSERT ON entries BEGIN
    INSERT INTO entries_fts(rowid, title, content, entry_id)
    VALUES (new.rowid, new.title, new.content, new.id);
END;

-- Trigger to keep FTS index updated on deletes
CREATE TRIGGER IF NOT EXISTS entries_fts_ad AFTER DELETE ON entries BEGIN
    DELETE FROM entries_fts WHERE rowid = old.rowid;
END;

-- Trigger to keep FTS index updated on updates
CREATE TRIGGER IF NOT EXISTS entries_fts_au AFTER UPDATE OF title, content ON entries BEGIN
    UPDATE entries_fts SET title = new.title, content = new.content, entry_id = new.id
    WHERE rowid = old.rowid;
END;
`
)
