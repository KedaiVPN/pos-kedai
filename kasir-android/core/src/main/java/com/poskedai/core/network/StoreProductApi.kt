package com.poskedai.core.network

import retrofit2.http.Body
import retrofit2.http.GET
import retrofit2.http.POST

interface StoreProductApi {
    @GET("store/products")
    suspend fun getStoreProducts(): List<Map<String, Any>>

    @POST("store/products")
    suspend fun addStoreProduct(@Body request: Map<String, Any>): Map<String, Any>

    @GET("store/categories")
    suspend fun getStoreCategories(): List<Map<String, Any>>

    @POST("store/categories")
    suspend fun createStoreCategory(@Body request: Map<String, String>): Map<String, Any>
}
