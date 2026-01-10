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
