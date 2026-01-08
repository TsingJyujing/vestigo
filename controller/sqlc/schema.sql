PRAGMA journal_mode=WAL;

CREATE TABLE IF NOT EXISTS datasource
(
    id         TEXT PRIMARY KEY,
    name       TEXT    NOT NULL,
    created_at INTEGER NOT NULL DEFAULT (strftime('%s', 'now'))
);

CREATE TABLE IF NOT EXISTS document
(
    id            TEXT PRIMARY KEY,
    datasource_id TEXT    NOT NULL,
    title         TEXT    NOT NULL,
    description   TEXT,
    created_at    INTEGER NOT NULL DEFAULT (strftime('%s', 'now')),
    FOREIGN KEY (datasource_id) REFERENCES datasource (id) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS text_chunk
(
    id          TEXT PRIMARY KEY,
    document_id TEXT    NOT NULL,
    content     TEXT    NOT NULL,
    seg_content TEXT    NOT NULL,
    created_at  INTEGER NOT NULL DEFAULT (strftime('%s', 'now')),
    FOREIGN KEY (document_id) REFERENCES document (id) ON DELETE CASCADE
);


CREATE VIRTUAL TABLE IF NOT EXISTS text_chunk_fts
    USING fts5
(
    seg_content,
    content='text_chunk',
    content_rowid='id',
    tokenize = 'unicode61'
);

CREATE TABLE IF NOT EXISTS text_embedding
(
    id            TEXT PRIMARY KEY,
    model_id      TEXT    NOT NULL,
    text_chunk_id TEXT    NOT NULL,
    vector        BLOB    NOT NULL,
    created_at    INTEGER NOT NULL DEFAULT (strftime('%s', 'now')),
    FOREIGN KEY (text_chunk_id) REFERENCES text_chunk (id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_text_embedding_model_id
    ON text_embedding (model_id);

