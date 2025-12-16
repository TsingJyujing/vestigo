-- name: GetDatasource :one
SELECT *
FROM datasource
WHERE id = ?
LIMIT 1;

-- name: NewDatasource :one
INSERT INTO datasource (id, name)
VALUES (?, ?)
RETURNING *;

-- name: DeleteDatasource :exec
DELETE
FROM datasource
WHERE id = ?;

-- name: ListDatasources :many
SELECT *
FROM datasource;

-- name: DeleteDocument :exec
DELETE
FROM document
WHERE id = ?;

-- name: NewDocument :one
INSERT INTO document (id, datasource_id, title, description)
VALUES (?, ?, ?, ?)
RETURNING *;

-- name: GetDocument :one
SELECT *
FROM document
WHERE id = ?
LIMIT 1;

-- name: NewTextChunk :one
INSERT INTO text_chunk (id, document_id, content)
VALUES (?, ?, ?)
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

