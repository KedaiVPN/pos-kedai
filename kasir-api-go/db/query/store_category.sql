-- name: GetStoreCategoryByName :one
SELECT * FROM store_categories WHERE store_id = $1 AND name = $2 LIMIT 1;

-- name: CreateStoreCategory :one
INSERT INTO store_categories (store_id, name, slug) VALUES ($1, $2, $3) RETURNING *;

-- name: GetAllStoreCategories :many
SELECT * FROM store_categories WHERE store_id = $1 ORDER BY name ASC;
