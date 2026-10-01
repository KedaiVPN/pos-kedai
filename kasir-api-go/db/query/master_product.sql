-- name: CreateMasterProduct :one
INSERT INTO master_products (
  barcode, name, photo_url, photo_path, category_id, brand_id, unit, source, is_generated_barcode, created_by
) VALUES (
  $1, $2, $3, $4, $5, $6, $7, $8, $9, $10
)
RETURNING *;

-- name: GetMasterProduct :one
SELECT * FROM master_products
WHERE id = $1 LIMIT 1;

-- name: ListMasterProducts :many
SELECT mp.*, c.name as category_name
FROM master_products mp
LEFT JOIN categories c ON mp.category_id = c.id
ORDER BY mp.id;

-- name: CountMasterProducts :one
SELECT COUNT(*) FROM master_products;

-- name: CheckMasterProductBarcodeExists :one
SELECT EXISTS(
  SELECT 1 FROM master_products WHERE barcode = $1
) AS exists;

-- name: DeleteMasterProduct :exec
DELETE FROM master_products WHERE id = $1;

-- name: DeleteTransactionItemsByMasterProduct :exec
DELETE FROM transaction_items WHERE master_product_id = $1;
