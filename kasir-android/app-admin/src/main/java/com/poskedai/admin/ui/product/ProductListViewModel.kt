package com.poskedai.admin.ui.product

import androidx.lifecycle.ViewModel
import androidx.lifecycle.viewModelScope
import com.poskedai.core.network.RetrofitClient
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.asStateFlow
import kotlinx.coroutines.launch
import com.google.gson.JsonObject
import android.content.Context
import android.net.Uri
import com.poskedai.admin.utils.ImageCompressor
import okhttp3.MediaType.Companion.toMediaTypeOrNull
import okhttp3.MultipartBody
import okhttp3.RequestBody.Companion.asRequestBody

sealed class ProductListState {
    object Loading : ProductListState()
    data class Success(val products: List<JsonObject>) : ProductListState()
    data class Error(val message: String) : ProductListState()
}

class ProductListViewModel : ViewModel() {
    private val _uiState = MutableStateFlow<ProductListState>(ProductListState.Loading)
    val uiState: StateFlow<ProductListState> = _uiState.asStateFlow()

    init {
        loadProducts()
    }

    fun loadCategories() {
        viewModelScope.launch {
            try {
                val response = RetrofitClient.productApi.getCategories()
                if (response.isSuccessful) {
                    val categoryList = response.body()?.map { it.get("name").asString } ?: emptyList()
                    _categories.value = categoryList
                }
            } catch (e: Exception) {
                // Keep empty list on error
            }
        }
    }

    fun createNewCategory(categoryName: String, onSuccess: (String) -> Unit) {
        viewModelScope.launch {
            try {
                val response = RetrofitClient.productApi.createCategory(mapOf("name" to categoryName))
                if (response.isSuccessful) {
                    loadCategories()
                    onSuccess(categoryName)
                } else {
                    _actionState.value = "Gagal membuat kategori: ${response.code()}"
                }
            } catch (e: Exception) {
                _actionState.value = "Gagal membuat kategori: ${e.message}"
            }
        }
    }

    private val _actionState = MutableStateFlow<String?>(null)
    val actionState: StateFlow<String?> = _actionState.asStateFlow()

    private val _isMasterEnabled = MutableStateFlow<Boolean>(true)
    val isMasterEnabled: StateFlow<Boolean> = _isMasterEnabled.asStateFlow()

    private val _categories = MutableStateFlow<List<String>>(emptyList())
    val categories: StateFlow<List<String>> = _categories.asStateFlow()

    fun loadMasterProductStatus() {
        viewModelScope.launch {
            try {
                val response = RetrofitClient.adminApi.getMasterProductStatus()
                if (response.isSuccessful) {
                    _isMasterEnabled.value = response.body()?.is_enabled ?: true
                }
            } catch (e: Exception) {
                // Keep default
            }
        }
    }

    fun toggleMasterProductStatus(enabled: Boolean) {
        viewModelScope.launch {
            try {
                val response = RetrofitClient.adminApi.toggleMasterProductStatus(
                    com.poskedai.core.network.ToggleMasterProductRequest(enabled)
                )
                if (response.isSuccessful) {
                    _isMasterEnabled.value = enabled
                    _actionState.value = "Status Halaman Master Produk berhasil diubah"
                } else {
                    _actionState.value = "Gagal mengubah status Master Produk"
                }
            } catch (e: Exception) {
                _actionState.value = "Error: ${e.message}"
            }
        }
    }

    fun loadProducts() {
        viewModelScope.launch {
            _uiState.value = ProductListState.Loading
            try {
                val response = RetrofitClient.productApi.getMasterProducts()
                if (response.isSuccessful) {
                    val products = response.body() ?: emptyList()
                    _uiState.value = ProductListState.Success(products)
                } else {
                    _uiState.value = ProductListState.Error("Error: ${response.code()}")
                }
            } catch (e: Exception) {
                _uiState.value = ProductListState.Error(e.message ?: "Failed to load products")
            }
        }
    }

    fun updateProduct(id: String, name: String, category: String, barcode: String, buyPrice: String, sellPrice: String, stock: Int, description: String, imageUrl: String) {
        viewModelScope.launch {
            try {
                val request = com.poskedai.core.network.PendingProductRequest(
                    name = name,
                    category = category,
                    barcode = barcode,
                    buy_price = buyPrice.toLongOrNull() ?: 0L,
                    sell_price = sellPrice.toLongOrNull() ?: 0L,
                    stock = stock,
                    description = description,
                    image_url = imageUrl
                )
                val response = RetrofitClient.productApi.updateProduct(id, "approved", request)
                if (response.isSuccessful) {
                    _actionState.value = "Produk berhasil diupdate"
                    loadProducts()
                } else {
                    _actionState.value = "Gagal update produk: ${response.code()}"
                }
            } catch (e: Exception) {
                _actionState.value = "Gagal update produk: ${e.message}"
            }
        }
    }

    fun clearActionState() {
        _actionState.value = null
    }


    fun deleteProduct(id: String) {
        viewModelScope.launch {
            try {
                val response = RetrofitClient.productApi.deleteProduct(id, "approved")
                if (response.isSuccessful) {
                    _actionState.value = "Produk berhasil dihapus"
                    loadProducts()
                } else {
                    _actionState.value = "Gagal hapus produk: ${response.code()}"
                }
            } catch (e: Exception) {
                _actionState.value = "Gagal hapus produk: ${e.message}"
            }
        }
    }


    private suspend fun uploadImageIfLocal(context: Context, uriString: String): String {
        if (!uriString.startsWith("content://") && !uriString.startsWith("file://")) {
            return uriString // Already a remote URL or empty
        }

        val uri = Uri.parse(uriString)
        val compressedFile = ImageCompressor.compressImageFromUri(context, uri)
            ?: throw Exception("Gagal mengkompresi gambar")

        val requestFile = compressedFile.asRequestBody("image/jpeg".toMediaTypeOrNull())
        val body = MultipartBody.Part.createFormData("image", compressedFile.name, requestFile)

        val response = RetrofitClient.productApi.uploadImage(body)
        if (response.isSuccessful) {
            val data = response.body()
            return data?.get("image_url") as? String ?: throw Exception("URL gambar tidak ditemukan dalam respons")
        } else {
            throw Exception("Gagal mengupload gambar: ${response.code()}")
        }
    }

    fun addProduct(context: Context, name: String, category: String, barcode: String, photoUrl: String, isGeneratedBarcode: Boolean) {
        viewModelScope.launch {
            try {
                _uiState.value = ProductListState.Loading
                val finalImageUrl = uploadImageIfLocal(context, photoUrl)

                val request = com.poskedai.core.network.MasterProductRequest(
                    name = name,
                    category_id = category, // backend might expect uuid, we'll let handler deal with it or adjust later if needed
                    barcode = barcode,
                    photo_url = finalImageUrl,
                    is_generated_barcode = isGeneratedBarcode
                )
                val response = RetrofitClient.productApi.submitMasterProduct(request)
                if (response.isSuccessful) {
                    _actionState.value = "Produk berhasil ditambahkan"
                    loadProducts()
                } else {
                    _actionState.value = "Gagal tambah produk: ${response.code()}"
                    loadProducts()
                }
            } catch (e: Exception) {
                _actionState.value = "Gagal tambah produk: ${e.message}"
                loadProducts()
            }
        }
    }
}
