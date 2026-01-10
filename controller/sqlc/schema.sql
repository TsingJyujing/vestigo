CREATE TABLE IF NOT EXISTS document
(
    id          TEXT PRIMARY KEY,
    title       TEXT    NOT NULL,
    description TEXT    NOT NULL DEFAULT '',
    data        TEXT    NOT NULL DEFAULT '{}',
    created_at  INTEGER NOT NULL DEFAULT (strftime('%s', 'now'))
) WITHOUT ROWID;

CREATE TABLE IF NOT EXISTS text_chunk
(
    id          TEXT PRIMARY KEY,
    document_id TEXT    NOT NULL,
    content     TEXT    NOT NULL,
    seg_content TEXT    NOT NULL,
    created_at  INTEGER NOT NULL DEFAULT (strftime('%s', 'now')),
    FOREIGN KEY (document_id) REFERENCES document (id) ON DELETE CASCADE
) WITHOUT ROWID;

CREATE VIRTUAL TABLE IF NOT EXISTS text_chunk_fts
    USING fts5
(
    id UNINDEXED,
    seg_content,
    tokenize = 'unicode61'
);

CREATE TABLE IF NOT EXISTS text_embedding
( -- Use default row ID for simplicity
    model_id      TEXT    NOT NULL,
    text_chunk_id TEXT    NOT NULL,
    vector        BLOB    NOT NULL,
    created_at    INTEGER NOT NULL DEFAULT (strftime('%s', 'now')),
    FOREIGN KEY (text_chunk_id) REFERENCES text_chunk (id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_text_embedding_model_id
    ON text_embedding (model_id);
