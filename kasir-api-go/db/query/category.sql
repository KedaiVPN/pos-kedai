-- name: GetCategoryByName :one
SELECT * FROM categories WHERE name = $1 LIMIT 1;

-- name: CreateCategory :one
INSERT INTO categories (name, slug) VALUES ($1, $2) RETURNING *;

-- name: GetAllCategories :many
SELECT * FROM categories ORDER BY name ASC;
