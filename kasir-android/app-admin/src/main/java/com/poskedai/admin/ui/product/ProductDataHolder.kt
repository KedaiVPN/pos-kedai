package com.poskedai.admin.ui.product

import com.google.gson.JsonObject

/**
 * Temporary holder for passing product data between screens safely
 * without relying on SavedStateHandle or URL encoding.
 */
object ProductDataHolder {
    var currentProductJson: JsonObject? = null
    
    fun set(json: JsonObject) {
        currentProductJson = json
    }
    
    fun getAndClear(): JsonObject? {
        val result = currentProductJson
        currentProductJson = null
        return result
    }
    
    fun get(): JsonObject? = currentProductJson
}
