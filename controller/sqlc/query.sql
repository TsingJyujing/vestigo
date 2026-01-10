-- name: DeleteDocument :exec
DELETE
FROM document
WHERE id = ?;

-- name: NewDocument :exec
INSERT INTO document (id, title, description, data)
VALUES (?, ?, ?, ?);

-- name: GetDocument :one
SELECT *
FROM document
WHERE id = ?
LIMIT 1;

-- name: NewTextChunk :one
INSERT INTO text_chunk (id, document_id, content, seg_content)
VALUES (?, ?, ?, ?)
RETURNING *;

-- name: ListTextChunksByDocumentID :many
SELECT *
FROM text_chunk
WHERE document_id = ?;

-- name: GetTextChunk :one
SELECT *
FROM text_chunk
WHERE id = ?
LIMIT 1;

-- name: DeleteTextChunk :exec
DELETE
FROM text_chunk
WHERE id = ?;

-- name: DeleteTextEmbeddingsByDocumentID :exec
DELETE FROM text_embedding 
WHERE text_chunk_id IN (
    SELECT id FROM text_chunk tc
    WHERE tc.document_id = ?
);

-- name: DeleteTextChunkFTSByDocumentID :exec
DELETE FROM text_chunk_fts 
WHERE id IN (
    SELECT id FROM text_chunk tc 
    WHERE tc.document_id = ?
);

-- name: DeleteTextChunksByDocumentID :exec
DELETE FROM text_chunk 
WHERE document_id = ?;

-- name: DeleteTextEmbeddingsByTextChunkID :exec
DELETE FROM text_embedding 
WHERE text_chunk_id = ?;

-- name: DeleteTextChunkFTSByID :exec
DELETE FROM text_chunk_fts 
WHERE id = ?;

-- name: InsertTextChunkFTS :exec
INSERT INTO text_chunk_fts (id, seg_content) 
VALUES (?, ?);
