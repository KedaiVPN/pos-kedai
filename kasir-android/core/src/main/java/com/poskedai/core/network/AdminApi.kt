package com.poskedai.core.network

import com.google.gson.JsonObject
import retrofit2.Response
import retrofit2.http.Body
import retrofit2.http.DELETE
import retrofit2.http.GET
import retrofit2.http.POST
import retrofit2.http.PUT
import retrofit2.http.Path

data class AdminDashboardStatsDto(
    val approved_count: Int = 0,
    val pending_count: Int = 0,
    val total_stores: Int = 0,
    val pro_stores: Int = 0,
    val non_pro_stores: Int = 0
)

data class AdminStoreListItemDto(
    val id: String = "",
    val store_name: String = "",
    val owner_name: String = "",
    val is_blocked: Boolean = false
)

data class AdminStoreDetailDto(
    val id: String = "",
    val store_name: String = "",
    val owner_name: String = "",
    val email: String = "",
    val phone: String = "",
    val cashier_count: Int = 0,
    val is_pro: Boolean = false,
    val pro_expires_at: String? = null,
    val is_blocked: Boolean = false
)

data class UpdateProRequestDto(
    val days: Int
)

data class ToggleBlockRequestDto(
    val is_blocked: Boolean
)

data class MasterProductStatusResponse(
    val is_enabled: Boolean = true
)

data class ToggleMasterProductRequest(
    val is_enabled: Boolean
)

// Open Food Facts DTOs
data class OFFDraftDto(
    val barcode: String = "",
    val raw_name: String = "",
    val brand: String = "",
    val raw_category: String = "",
    val image_url: String = "",
    val fetched_at: String = ""
)

data class OFFDraftsResponse(
    val drafts: List<OFFDraftDto> = emptyList(),
    val count: Int = 0
)

data class OFFFetchResponse(
    val inserted: Int = 0,
    val skipped: Int = 0
)

data class ApproveDraftRequest(
    val barcode: String,
    val name: String,
    val category_id: String,
    val unit: String
)

interface AdminApi {
    @GET("admin/dashboard")
    suspend fun getDashboardStats(): Response<AdminDashboardStatsDto>

    @POST("admin/products/{id}/approve")
    suspend fun approveProduct(@Path("id") id: String): Response<JsonObject>

    @POST("admin/products/{id}/reject")
    suspend fun rejectProduct(@Path("id") id: String): Response<JsonObject>

    @GET("admin/stores")
    suspend fun getStores(): Response<List<AdminStoreListItemDto>>

    @GET("admin/stores/{id}")
    suspend fun getStoreDetail(@Path("id") id: String): Response<AdminStoreDetailDto>

    @PUT("admin/stores/{id}/pro")
    suspend fun updateStoreProStatus(
        @Path("id") id: String,
        @Body request: UpdateProRequestDto
    ): Response<JsonObject>

    @PUT("admin/stores/{id}/block")
    suspend fun toggleStoreBlock(
        @Path("id") id: String,
        @Body request: ToggleBlockRequestDto
    ): Response<JsonObject>

    @PUT("admin/master-product-status")
    suspend fun toggleMasterProductStatus(@Body request: ToggleMasterProductRequest): Response<JsonObject>

    @GET("admin/master-product-status")
    suspend fun getMasterProductStatus(): Response<MasterProductStatusResponse>

    @DELETE("admin/stores/{id}")
    suspend fun deleteStore(@Path("id") id: String): Response<JsonObject>

    // Open Food Facts Endpoints
    @POST("admin/off/fetch")
    suspend fun fetchFromOFF(
        @retrofit2.http.Query("page") page: Int = 1,
        @retrofit2.http.Query("page_size") pageSize: Int = 25
    ): Response<OFFFetchResponse>

    @GET("admin/off/drafts")
    suspend fun getOFFDrafts(): Response<OFFDraftsResponse>

    @POST("admin/off/drafts/approve")
    suspend fun approveDraft(@Body request: ApproveDraftRequest): Response<JsonObject>

    @DELETE("admin/off/drafts/{barcode}")
    suspend fun rejectDraft(@Path("barcode") barcode: String): Response<JsonObject>
}

