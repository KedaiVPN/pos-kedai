package com.poskedai.admin.ui.product

import androidx.lifecycle.ViewModel
import androidx.lifecycle.viewModelScope
import com.poskedai.core.network.ApproveDraftRequest
import com.poskedai.core.network.OFFDraftDto
import com.poskedai.core.network.RetrofitClient
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.asStateFlow
import kotlinx.coroutines.launch

sealed class ImportProductState {
    object Loading : ImportProductState()
    data class Success(val drafts: List<OFFDraftDto>) : ImportProductState()
    data class Error(val message: String) : ImportProductState()
}

class ImportProductViewModel : ViewModel() {
    private val _uiState = MutableStateFlow<ImportProductState>(ImportProductState.Loading)
    val uiState: StateFlow<ImportProductState> = _uiState.asStateFlow()

    private val _isFetching = MutableStateFlow(false)
    val isFetching: StateFlow<Boolean> = _isFetching.asStateFlow()

    private val _actionMessage = MutableStateFlow<String?>(null)
    val actionMessage: StateFlow<String?> = _actionMessage.asStateFlow()

    private val _errorDialogMessage = MutableStateFlow<String?>(null)
    val errorDialogMessage: StateFlow<String?> = _errorDialogMessage.asStateFlow()

    private val _currentPage = MutableStateFlow(1)
    val currentPage: StateFlow<Int> = _currentPage.asStateFlow()

    init {
        loadDrafts()
    }

    fun loadDrafts() {
        viewModelScope.launch {
            _uiState.value = ImportProductState.Loading
            try {
                val response = RetrofitClient.adminApi.getOFFDrafts()
                if (response.isSuccessful) {
                    val drafts = response.body()?.drafts ?: emptyList()
                    _uiState.value = ImportProductState.Success(drafts)
                } else {
                    _uiState.value = ImportProductState.Error("Gagal memuat draft: ${response.code()}")
                }
            } catch (e: Exception) {
                _uiState.value = ImportProductState.Error(e.message ?: "Gagal terhubung ke server")
            }
        }
    }

    fun fetchFromOFF() {
        viewModelScope.launch {
            _isFetching.value = true
            try {
                val page = _currentPage.value
                val response = RetrofitClient.adminApi.fetchFromOFF(page = page, pageSize = 5)
                if (response.isSuccessful) {
                    val body = response.body()
                    val inserted = body?.inserted ?: 0
                    val skipped = body?.skipped ?: 0
                    _actionMessage.value = "Berhasil menarik $inserted produk baru (skip $skipped yang sudah ada)"
                    _currentPage.value = page + 1
                    loadDrafts()
                } else {
                    // Error spesifik berdasarkan HTTP code
                    val errorMsg = when (response.code()) {
                        503 -> "Layanan katalog sedang sibuk atau mengalami lonjakan trafik. Silakan coba lagi beberapa saat lagi."
                        429 -> "Permintaan sedang terlalu banyak. Tunggu beberapa menit sebelum mencoba lagi."
                        else -> "Produk belum dapat ditarik saat ini. Silakan coba lagi beberapa saat lagi."
                    }
                    _errorDialogMessage.value = errorMsg
                }
            } catch (e: java.net.UnknownHostException) {
                _errorDialogMessage.value = "Tidak dapat terhubung ke Open Food Facts. Periksa koneksi internet Anda."
            } catch (e: java.net.SocketTimeoutException) {
                _errorDialogMessage.value = "Koneksi ke Open Food Facts timeout. Server mungkin sedang lambat, coba lagi dalam beberapa saat."
            } catch (e: Exception) {
                _errorDialogMessage.value = "Produk belum dapat ditarik saat ini. Silakan coba lagi beberapa saat lagi."
            } finally {
                _isFetching.value = false
            }
        }
    }

    fun fetchFromS3() {
        viewModelScope.launch {
            _isFetching.value = true
            try {
                val page = _currentPage.value
                val response = RetrofitClient.adminApi.fetchFromS3(page = page, pageSize = 25)
                if (response.isSuccessful) {
                    val body = response.body()
                    val inserted = body?.inserted ?: 0
                    _actionMessage.value = "Berhasil menarik $inserted produk dari S3 dump"
                    _currentPage.value = page + 1
                    loadDrafts()
                } else {
                    // Error spesifik berdasarkan HTTP code
                    val errorMsg = when (response.code()) {
                        503 -> "Layanan dump S3 sedang tidak tersedia. Silakan coba lagi beberapa saat lagi."
                        else -> "Produk belum dapat ditarik dari S3 saat ini. Silakan coba lagi beberapa saat lagi."
                    }
                    _errorDialogMessage.value = errorMsg
                }
            } catch (e: java.net.UnknownHostException) {
                _errorDialogMessage.value = "Tidak dapat terhubung ke S3. Periksa koneksi internet Anda."
            } catch (e: java.net.SocketTimeoutException) {
                _errorDialogMessage.value = "Koneksi ke S3 timeout. Coba lagi dalam beberapa saat."
            } catch (e: Exception) {
                _errorDialogMessage.value = "Produk belum dapat ditarik dari S3 saat ini. Silakan coba lagi beberapa saat lagi."
            } finally {
                _isFetching.value = false
            }
        }
    }

    fun approveDraft(barcode: String, name: String, categoryId: String, unit: String) {
        viewModelScope.launch {
            try {
                val request = ApproveDraftRequest(
                    barcode = barcode,
                    name = name,
                    category_id = categoryId,
                    unit = unit
                )
                val response = RetrofitClient.adminApi.approveDraft(request)
                if (response.isSuccessful) {
                    _actionMessage.value = "Produk '$name' diapprove ke Master Produk"
                    loadDrafts()
                } else {
                    _actionMessage.value = "Gagal approve: ${response.code()}"
                }
            } catch (e: Exception) {
                _actionMessage.value = "Gagal approve: ${e.message}"
            }
        }
    }

    fun rejectDraft(barcode: String) {
        viewModelScope.launch {
            try {
                val response = RetrofitClient.adminApi.rejectDraft(barcode)
                if (response.isSuccessful) {
                    _actionMessage.value = "Draft $barcode dihapus"
                    loadDrafts()
                } else {
                    _actionMessage.value = "Gagal hapus draft: ${response.code()}"
                }
            } catch (e: Exception) {
                _actionMessage.value = "Gagal hapus draft: ${e.message}"
            }
        }
    }

    fun clearActionMessage() {
        _actionMessage.value = null
    }

    fun clearErrorDialog() {
        _errorDialogMessage.value = null
    }
}
