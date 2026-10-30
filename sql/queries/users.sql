-- name: CreateUser :one
INSERT INTO users (id, created_at, updated_at, email, hashed_password, is_chirpy_red)
VALUES (
    $1,
    $2,
    $3,
    $4,
    $5,
    $6
)
RETURNING *;

-- name: UpdateUser :one
UPDATE users SET
    email = $1,
    hashed_password = $2,
    updated_at = $3
WHERE id = $4
RETURNING *;

-- name: GetUser :one
SELECT * FROM users WHERE email = $1;

-- name: UpgradeUser :exec
UPDATE users SET is_chirpy_red = true
WHERE id = $1
RETURNING *;

-- name: ResetUsers :exec
DELETE FROM users;