package com.poskedai.admin.ui.product

import android.content.Context
import android.net.Uri
import androidx.lifecycle.ViewModel
import androidx.lifecycle.viewModelScope
import com.poskedai.core.network.RetrofitClient
import com.poskedai.core.network.PendingProductRequest
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.asStateFlow
import kotlinx.coroutines.launch
import com.google.gson.JsonObject
import com.poskedai.admin.utils.ImageCompressor
import okhttp3.MultipartBody
import okhttp3.RequestBody
import okhttp3.RequestBody.Companion.asRequestBody
import okhttp3.RequestBody.Companion.toRequestBody
import okhttp3.MediaType.Companion.toMediaTypeOrNull
import java.io.File

data class EditAdminProductUiState(
    val productName: String = "",
    val productBarcode: String = "",
    val productCategory: String = "",
    val photoUrl: String = "",
    val categories: List<String> = emptyList(),
    val isLoading: Boolean = false,
    val errorMessage: String? = null,
    val successMessage: String? = null
)

class EditAdminProductViewModel : ViewModel() {
    private val _uiState = MutableStateFlow(EditAdminProductUiState())
    val uiState: StateFlow<EditAdminProductUiState> = _uiState.asStateFlow()

    fun loadProduct(productJson: JsonObject) {
        android.util.Log.d("EditAdminProductVM", "loadProduct called with JSON: $productJson")
        
        // Multi-key fallback sama seperti RequestProductScreen
        val name = productJson.getStringSafe("name")
            .ifEmpty { productJson.getStringSafe("product_name") }
        val barcode = productJson.getStringSafe("barcode")
        val category = productJson.getStringSafe("category")
            .ifEmpty { productJson.getStringSafe("category_id") }
        val photoUrl = productJson.getStringSafe("image_url")
            .ifEmpty { productJson.getStringSafe("photo_url") }
            .ifEmpty { productJson.getStringSafe("image") }
            .ifEmpty { productJson.getStringSafe("url") }
        
        android.util.Log.d("EditAdminProductVM", "Parsed - name: $name, barcode: $barcode, category: $category, photoUrl: $photoUrl")
        android.util.Log.d("EditAdminProductVM", "JSON keys available: ${productJson.keySet()}")
        
        _uiState.value = _uiState.value.copy(
            productName = name,
            productBarcode = barcode,
            productCategory = category,
            photoUrl = photoUrl
        )
        loadCategories()
    }

    private fun loadCategories() {
        viewModelScope.launch {
            try {
                val response = RetrofitClient.productApi.getCategories()
                if (response.isSuccessful) {
                    val categoryList = response.body()?.map { it.get("name").asString } ?: emptyList()
                    _uiState.value = _uiState.value.copy(categories = categoryList)
                } else {
                    _uiState.value = _uiState.value.copy(categories = emptyList())
                }
            } catch (e: Exception) {
                _uiState.value = _uiState.value.copy(
                    errorMessage = "Gagal memuat kategori: ${e.message}",
                    categories = emptyList()
                )
            }
        }
    }

    fun updateProductName(newName: String) {
        _uiState.value = _uiState.value.copy(productName = newName)
    }

    fun updateProductBarcode(newBarcode: String) {
        _uiState.value = _uiState.value.copy(productBarcode = newBarcode)
    }

    fun updateProductCategory(newCategory: String) {
        _uiState.value = _uiState.value.copy(productCategory = newCategory)
    }

    fun updatePhotoUrl(newPhotoUrl: String) {
        _uiState.value = _uiState.value.copy(photoUrl = newPhotoUrl)
    }

    fun uploadPhotoFromUri(context: Context, uri: Uri, barcode: String) {
        viewModelScope.launch {
            try {
                _uiState.value = _uiState.value.copy(isLoading = true)
                
                val compressedFile = ImageCompressor.compressImageFromUri(context, uri)
                    ?: throw Exception("Gagal mengkompresi gambar")

                val requestFile = compressedFile.asRequestBody("image/jpeg".toMediaTypeOrNull())
                val body = MultipartBody.Part.createFormData("image", compressedFile.name, requestFile)

                val barcodeBody = barcode.takeIf { it.isNotEmpty() }?.let {
                    it.toRequestBody("text/plain".toMediaTypeOrNull())
                }

                val response = RetrofitClient.productApi.uploadImage(body, barcodeBody)
                if (response.isSuccessful) {
                    val imageUrl = response.body()?.get("image_url") as? String
                    if (imageUrl != null) {
                        _uiState.value = _uiState.value.copy(
                            photoUrl = imageUrl,
                            successMessage = "Foto berhasil diupload",
                            isLoading = false
                        )
                    } else {
                        throw Exception("URL foto tidak ditemukan dalam respons")
                    }
                } else {
                    throw Exception("Gagal upload foto: ${response.code()}")
                }
            } catch (e: Exception) {
                _uiState.value = _uiState.value.copy(
                    errorMessage = "Error: ${e.message}",
                    isLoading = false
                )
            }
        }
    }

    fun createNewCategory(categoryName: String) {
        viewModelScope.launch {
            try {
                _uiState.value = _uiState.value.copy(isLoading = true)
                
                val response = RetrofitClient.productApi.createCategory(mapOf("name" to categoryName))
                if (response.isSuccessful) {
                    val updatedCategories = _uiState.value.categories.toMutableList()
                    if (!updatedCategories.contains(categoryName)) {
                        updatedCategories.add(categoryName)
                    }
                    _uiState.value = _uiState.value.copy(
                        categories = updatedCategories,
                        productCategory = categoryName,
                        successMessage = "Kategori baru berhasil dibuat",
                        isLoading = false
                    )
                } else {
                    throw Exception("Gagal membuat kategori: ${response.code()}")
                }
            } catch (e: Exception) {
                _uiState.value = _uiState.value.copy(
                    errorMessage = "Gagal membuat kategori: ${e.message}",
                    isLoading = false
                )
            }
        }
    }

    private fun cleanPhotoUrl(url: String): String {
        if (url.isBlank()) return ""
        val baseUrl = "https://api-go-v1.free-account.my.id"
        return if (url.startsWith(baseUrl)) {
            url.substring(baseUrl.length)
        } else {
            url
        }
    }

    fun saveProductChanges(productId: String, isPending: Boolean, onSuccess: () -> Unit) {
        viewModelScope.launch {
            try {
                _uiState.value = _uiState.value.copy(isLoading = true)
                
                val request = PendingProductRequest(
                    name = _uiState.value.productName,
                    category = _uiState.value.productCategory,
                    barcode = _uiState.value.productBarcode,
                    buy_price = 0L,
                    sell_price = 0L,
                    stock = 0,
                    description = "",
                    image_url = cleanPhotoUrl(_uiState.value.photoUrl)
                )

                val status = if (isPending) "pending" else "approved"
                val response = RetrofitClient.productApi.updateProduct(productId, status, request)
                
                if (response.isSuccessful) {
                    _uiState.value = _uiState.value.copy(
                        successMessage = "Produk berhasil diupdate",
                        isLoading = false
                    )
                    onSuccess()
                } else {
                    throw Exception("Gagal update produk: ${response.code()}")
                }
            } catch (e: Exception) {
                _uiState.value = _uiState.value.copy(
                    errorMessage = "Error: ${e.message}",
                    isLoading = false
                )
            }
        }
    }

    fun saveAndAppoveProduct(productId: String, onSuccess: () -> Unit) {
        viewModelScope.launch {
            try {
                _uiState.value = _uiState.value.copy(isLoading = true)
                
                // First update product
                val updateRequest = PendingProductRequest(
                    name = _uiState.value.productName,
                    category = _uiState.value.productCategory,
                    barcode = _uiState.value.productBarcode,
                    buy_price = 0L,
                    sell_price = 0L,
                    stock = 0,
                    description = "",
                    image_url = cleanPhotoUrl(_uiState.value.photoUrl)
                )

                val updateResponse = RetrofitClient.productApi.updateProduct(productId, "pending", updateRequest)
                
                if (!updateResponse.isSuccessful) {
                    throw Exception("Gagal update produk: ${updateResponse.code()}")
                }

                // Then approve
                val approveResponse = RetrofitClient.adminApi.approveProduct(productId)
                
                if (approveResponse.isSuccessful) {
                    _uiState.value = _uiState.value.copy(
                        successMessage = "Produk berhasil disimpan dan diapprove",
                        isLoading = false
                    )
                    onSuccess()
                } else {
                    throw Exception("Gagal approve produk: ${approveResponse.code()}")
                }
            } catch (e: Exception) {
                _uiState.value = _uiState.value.copy(
                    errorMessage = "Error: ${e.message}",
                    isLoading = false
                )
            }
        }
    }

    fun clearMessages() {
        _uiState.value = _uiState.value.copy(errorMessage = null, successMessage = null)
    }

    private fun JsonObject.getStringSafe(key: String, defaultValue: String = ""): String {
        return if (this.has(key) && !this.get(key).isJsonNull) this.get(key).asString else defaultValue
    }
}
