package com.poskedai.store.data.repository

import android.content.Context
import android.net.Uri
import com.poskedai.core.network.PendingProductRequest
import com.poskedai.core.network.RetrofitClient
import com.poskedai.store.utils.ImageCompressor
import okhttp3.MediaType.Companion.toMediaTypeOrNull
import okhttp3.MultipartBody
import okhttp3.RequestBody.Companion.asRequestBody
import com.poskedai.store.data.local.TransactionDao
import com.poskedai.store.data.local.ProductDao
import com.poskedai.store.data.local.LocalTransactionEntity
import com.poskedai.store.data.local.LocalTransactionItemEntity
import com.poskedai.core.network.TokenManager
import com.poskedai.store.data.local.ProductEntity
import kotlinx.coroutines.flow.first
import kotlinx.coroutines.flow.Flow
import java.util.UUID
import com.google.gson.JsonObject
import androidx.work.OneTimeWorkRequestBuilder
import androidx.work.WorkManager
import androidx.work.Data
import androidx.work.NetworkType
import androidx.work.Constraints
import com.poskedai.store.worker.ImageDownloadWorker

class ProductRepository(
    private val productDao: ProductDao,
    private val transactionDao: TransactionDao,
    private val context: Context
) {

    val allProducts: Flow<List<ProductEntity>> = productDao.getAllProducts()

    suspend fun reduceStockLocally(productId: String, quantity: Int) {
        productDao.reduceStock(productId, quantity)
    }


    suspend fun syncStoreProducts() {
        val storeId = TokenManager(context).getStoreId() ?: return
        val response = RetrofitClient.productApi.getStoreProducts(storeId)
        if (response.isSuccessful) {
            val remoteProducts = response.body() ?: emptyList()

            val remoteIds = mutableListOf<String>()

            remoteProducts.forEach { product ->
                val id = product.get("id")?.asString ?: return@forEach
                remoteIds.add(id)
                val name = product.get("local_name")?.asString ?: ""

                // If a pending product with the same name exists locally (and just got approved), remove it
                if (name.isNotEmpty()) {
                    val pendingId = productDao.getPendingProductIdByName(name)
                    if (pendingId != null) {
                        // Update transactions referring to the temporary pending ID to use the new official ID
                        transactionDao.updateTransactionItemProductId(pendingId, id)
                    }
                    productDao.deletePendingProductByName(name)
                }
                val buyPrice = product.get("buy_price")?.asLong?.toString() ?: "0"
                val sellPrice = product.get("sell_price")?.asLong?.toString() ?: "0"
                val stock = product.get("stock")?.asInt ?: 0
                val minStock = product.get("min_stock")?.asInt ?: 0
                val isStockNotificationEnabled = product.get("is_stock_notification_enabled")?.asBoolean ?: false
                val localCategory = if (product.get("local_category") != null && !product.get("local_category").isJsonNull) product.get("local_category").asString else ""
                val categoryName = if (product.get("category_name") != null && !product.get("category_name").isJsonNull) product.get("category_name").asString else ""
                val category = if (localCategory.isNotEmpty()) localCategory else categoryName

                val barcode = if (product.get("barcode") != null && !product.get("barcode").isJsonNull) product.get("barcode").asString else ""
                var imageUrl = if (product.get("image_url") != null && !product.get("image_url").isJsonNull && product.get("image_url").asString.isNotEmpty()) {
                    product.get("image_url").asString
                } else if (product.get("photo_url") != null && !product.get("photo_url").isJsonNull && product.get("photo_url").asString.isNotEmpty()) {
                    product.get("photo_url").asString
                } else {
                    ""
                }

                // Jika imageUrl dari server kosong (misal karena master product dihapus),
                // gunakan imageUrl lokal jika ada, agar foto yang sudah terdownload tidak hilang
                if (imageUrl.isEmpty()) {
                    val localProduct = productDao.getProductById(id)
                    if (localProduct != null && !localProduct.imageUrl.isNullOrEmpty()) {
                        imageUrl = localProduct.imageUrl
                    }
                }

                val description = if (product.get("description") != null && !product.get("description").isJsonNull) product.get("description").asString else ""
                val createdAtStr = if (product.get("created_at") != null && !product.get("created_at").isJsonNull) product.get("created_at").asString else ""
                val createdAtMillis = try {
                    if (createdAtStr.isNotEmpty()) {
                        val format = java.text.SimpleDateFormat("yyyy-MM-dd'T'HH:mm:ssXXX", java.util.Locale.getDefault())
                        format.parse(createdAtStr)?.time ?: System.currentTimeMillis()
                    } else {
                        System.currentTimeMillis()
                    }
                } catch (e: Exception) {
                    System.currentTimeMillis()
                }

                val entity = ProductEntity(
                    id = id,
                    name = name,
                    buyPrice = buyPrice,
                    sellPrice = sellPrice,
                    stock = stock,
                    minStock = minStock,
                    isStockNotificationEnabled = isStockNotificationEnabled,
                    category = category,
                    description = description,
                    barcode = barcode,
                    imageUrl = imageUrl,
                    isSynced = true,
                    pendingSync = false,
                    createdAt = createdAtMillis
                )
                productDao.insertProduct(entity)
            }

            // Cleanup local products that were deleted on the server
            val localSyncedIds = productDao.getSyncedProductIds()
            val idsToDelete = localSyncedIds.filterNot { it in remoteIds }
            if (idsToDelete.isNotEmpty()) {
                productDao.deleteProductsByIds(idsToDelete)
            }

            // Queue image downloads using WorkManager
            // Instead of passing array which might hit 10KB limit, the worker will query Room database for images
            val constraints = Constraints.Builder()
                .setRequiredNetworkType(NetworkType.CONNECTED)
                .build()

            val workRequest = OneTimeWorkRequestBuilder<ImageDownloadWorker>()
                .setConstraints(constraints)
                .build()

            WorkManager.getInstance(context).enqueue(workRequest)
        } else {
            throw Exception("Gagal sync data: ${response.message()}")
        }
    }

    suspend fun getMasterProducts(): List<JsonObject> {
        val response = RetrofitClient.productApi.getMasterProducts()
        if (response.isSuccessful) {
            return response.body() ?: emptyList()
        } else {
            val errorMsg = com.poskedai.core.utils.ApiErrorParser.fromErrorBody(response.errorBody()?.string(), response.message())
            throw Exception("Gagal memuat master produk: $errorMsg")
        }
    }

    suspend fun addStoreProductFromMaster(
        storeId: String,
        masterProductId: String,
        buyPrice: String,
        sellPrice: String,
        stock: Int,
        minStock: Int,
        localName: String,
        localCategory: String,
        barcode: String,
        imageUrl: String,
        description: String,
        isStockNotificationEnabled: Boolean = false
    ) {
        val request = mapOf<String, Any>(
            "store_id" to storeId,
            "master_product_id" to masterProductId,
            "buy_price" to (buyPrice.toLongOrNull() ?: 0L),
            "sell_price" to (sellPrice.toLongOrNull() ?: 0L),
            "stock" to stock,
            "min_stock" to minStock,
            "local_name" to localName,
            "local_category" to localCategory,
            "is_stock_notification_enabled" to isStockNotificationEnabled
        )
        val response = RetrofitClient.productApi.addStoreProduct(request)
        if (response.isSuccessful) {
            val body = response.body()
            // Ensure ID is properly handled as string
            val newId = body?.get("id")?.toString() ?: UUID.randomUUID().toString()

            val entity = ProductEntity(
                id = newId,
                name = localName,
                buyPrice = buyPrice,
                sellPrice = sellPrice,
                stock = stock,
                category = localCategory,
                description = description,
                barcode = barcode,
                imageUrl = imageUrl,
                isSynced = true,
                pendingSync = false
            )
            productDao.insertProduct(entity)
        } else {
            val errorBody = response.errorBody()?.string()
            val errorMessage = if (!errorBody.isNullOrEmpty()) {
                try {
                    val jsonError = org.json.JSONObject(errorBody)
                    jsonError.optString("error", response.message())
                } catch (e: Exception) {
                    errorBody
                }
            } else {
                response.message()
            }
            throw Exception("Gagal menambahkan produk ke toko: $errorMessage")
        }
    }

    suspend fun addProductLocalAndSync(
        existingId: String?,
        name: String,
        buyPrice: Long,
        sellPrice: Long,
        stock: Int,
        minStock: Int = 0,
        category: String,
        description: String,
        barcode: String,
        imageUrl: String,
        isStockNotificationEnabled: Boolean = false
    ): Result<Unit> {
        val localId = existingId ?: UUID.randomUUID().toString()

        // Preserve existing sync state if editing
        val existingProduct = if (existingId != null) getProductById(existingId) else null
        val isCurrentlySynced = existingProduct?.isSynced ?: false
        val statusParam = if (isCurrentlySynced) "approved" else "pending"

        val entity = ProductEntity(
            id = localId,
            name = name,
            buyPrice = buyPrice.toString(),
            sellPrice = sellPrice.toString(),
            stock = stock,
            minStock = minStock,
            category = category,
            description = description,
            barcode = barcode,
            imageUrl = imageUrl,
            isStockNotificationEnabled = isStockNotificationEnabled,
            isSynced = isCurrentlySynced,
            pendingSync = true // Always needs sync after edit
        )

        // 1. Save to local Room Database first (Offline-First approach)
        productDao.insertProduct(entity)

        // 2. Attempt to sync to backend immediately
        return try {
            val finalImageUrl = uploadImageIfLocal(imageUrl, barcode)

            val storeId = TokenManager(context).getStoreId()

            val request = PendingProductRequest(
                name = name,
                buy_price = buyPrice,
                sell_price = sellPrice,
                stock = stock,
                min_stock = minStock,
                category = category,
                description = description,
                barcode = barcode,
                image_url = finalImageUrl,
                store_id = storeId,
                is_stock_notification_enabled = isStockNotificationEnabled
            )

            val response = if (existingId != null) {
                RetrofitClient.productApi.updateProduct(existingId, statusParam, request)
            } else {
                RetrofitClient.productApi.submitPendingProduct(request)
            }

            if (response.isSuccessful) {
                productDao.markAsSynced(localId)
                Result.success(Unit)
            } else {
                Result.failure(Exception("Gagal melakukan sinkronisasi dengan server."))
            }
        } catch (e: Exception) {
            // Return failure instead of success so UI can show error message
            Result.failure(e)
        }
    }

    private suspend fun uploadImageIfLocal(uriString: String, barcode: String? = null): String {
        if (!uriString.startsWith("content://") && !uriString.startsWith("file://")) {
            return uriString // Already a remote URL or empty
        }

        val uri = Uri.parse(uriString)
        val compressedFile = ImageCompressor.compressImageFromUri(context, uri)
            ?: throw Exception("Gagal mengkompresi gambar")

        val requestFile = compressedFile.asRequestBody("image/jpeg".toMediaTypeOrNull())
        val body = MultipartBody.Part.createFormData("image", compressedFile.name, requestFile)

        // Include barcode as form-data if provided
        val barcodeBody = barcode?.let {
            RequestBody.create("text/plain".toMediaTypeOrNull(), it)
        }

        val response = RetrofitClient.productApi.uploadImage(body, barcodeBody)
        if (response.isSuccessful) {
            val data = response.body()
            return data?.get("image_url") as? String ?: throw Exception("URL gambar tidak ditemukan dalam respons")
        } else {
            throw Exception("Gagal mengupload gambar: ${response.code()}")
        }
    }


    suspend fun getProductByBarcode(barcode: String): ProductEntity? {
        // Find product where barcode string matches exactly
        val products = productDao.getAllProducts().first()
        return products.find { it.barcode?.trim() == barcode.trim() }
    }



    suspend fun saveTransaction(
        transaction: LocalTransactionEntity,
        items: List<LocalTransactionItemEntity>
    ) {
        transactionDao.insertTransaction(transaction)
        transactionDao.insertTransactionItems(items)
    }

    suspend fun getTransaction(transactionId: String): LocalTransactionEntity? {
        return transactionDao.getTransactionById(transactionId)
    }

    suspend fun getTransactionItems(transactionId: String): List<LocalTransactionItemEntity> {
        return transactionDao.getTransactionItems(transactionId)
    }


    suspend fun getProductById(id: String): ProductEntity? {
        return productDao.getProductById(id)
    }

    suspend fun deleteProduct(id: String, isSynced: Boolean) {
        if (isSynced) {
            // Jika produk sudah sinkron dengan server, hapus dari server dulu
            try {
                val response = RetrofitClient.productApi.deleteStoreProductSpecific(id)
                if (response.isSuccessful || response.code() == 404) {
                    // Sukses dihapus dari server atau tidak ditemukan di server
                    productDao.deleteProduct(id)
                } else {
                    val errorMsg = com.poskedai.core.utils.ApiErrorParser.fromResponse(response.code(), response.errorBody()?.string())
                    throw Exception("Gagal menghapus produk: $errorMsg")
                }
            } catch (e: Exception) {
                throw Exception(com.poskedai.core.utils.ApiErrorParser.parse(e))
            }
        } else {
            // Jika produk pending (belum sinkron), langsung hapus lokal saja
            productDao.deleteProduct(id)
        }
    }

    // Placeholder functions to satisfy existing CatalogViewModel temporarily
    suspend fun getCatalogProducts(): Result<List<Map<String, Any>>> {
        return try {
            val response = RetrofitClient.catalogApi.getProducts()
            Result.success(response)
        } catch (e: Exception) {
            Result.failure(Exception(com.poskedai.core.utils.ApiErrorParser.parse(e)))
        }
    }

    suspend fun addStoreProduct(masterProductId: String, buyPrice: Double, sellPrice: Double, initialStock: Int, minStock: Int): Result<Map<String, Any>> {
        return try {
            val requestMap = mapOf(
                "master_product_id" to masterProductId,
                "buy_price" to buyPrice.toLong(),
                "sell_price" to sellPrice.toLong(),
                "stock" to initialStock,
                "min_stock" to minStock
            )
            val response = RetrofitClient.productApi.addStoreProduct(requestMap)
            if (response.isSuccessful) {
                Result.success(response.body() ?: emptyMap())
            } else {
                val errorMsg = response.errorBody()?.string() ?: "Gagal menambahkan produk ke toko"
                Result.failure(Exception(errorMsg))
            }
        } catch (e: Exception) {
            Result.failure(Exception(com.poskedai.core.utils.ApiErrorParser.parse(e)))
        }
    }
}
