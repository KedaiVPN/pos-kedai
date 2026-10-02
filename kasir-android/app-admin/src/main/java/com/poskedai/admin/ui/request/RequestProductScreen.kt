package com.poskedai.admin.ui.request

import android.widget.Toast
import androidx.compose.foundation.layout.*
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.material3.*
import androidx.compose.runtime.*
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.layout.ContentScale
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.unit.dp
import androidx.lifecycle.viewmodel.compose.viewModel
import coil.compose.AsyncImage
import com.google.gson.JsonObject
import com.google.gson.JsonNull

fun JsonObject.getStringSafe(key: String, defaultValue: String = ""): String {
    return if (this.has(key) && !this.get(key).isJsonNull) this.get(key).asString else defaultValue
}

@Composable
fun RequestProductScreen(
    viewModel: RequestProductViewModel = viewModel(),
    onNavigateToEdit: (JsonObject) -> Unit = {}
) {
    val uiState by viewModel.uiState.collectAsState()
    val actionState by viewModel.actionState.collectAsState()
    val context = LocalContext.current

    LaunchedEffect(actionState) {
        actionState?.let {
            Toast.makeText(context, it, Toast.LENGTH_SHORT).show()
            viewModel.clearActionState()
        }
    }

    LaunchedEffect(Unit) {
        viewModel.loadPendingProducts()
    }

    Column(modifier = Modifier.fillMaxSize()) {
        Text(
            text = "Request Produk (Pending)",
            style = MaterialTheme.typography.headlineSmall,
            fontWeight = FontWeight.Bold,
            modifier = Modifier.padding(16.dp)
        )

        when (uiState) {
            is RequestProductState.Loading -> {
                Box(modifier = Modifier.fillMaxSize(), contentAlignment = Alignment.Center) {
                    CircularProgressIndicator()
                }
            }
            is RequestProductState.Success -> {
                val products = (uiState as RequestProductState.Success).products
                if (products.isEmpty()) {
                    Box(modifier = Modifier.fillMaxSize(), contentAlignment = Alignment.Center) {
                        Text("Tidak ada request produk saat ini.")
                    }
                } else {
                    LazyColumn(
                        contentPadding = PaddingValues(16.dp),
                        verticalArrangement = Arrangement.spacedBy(8.dp)
                    ) {
                        items(products.size, key = { products[it].getStringSafe("id").ifEmpty { "index-$it" } }) { index ->
                            val product = products[index]
                            PendingProductItem(
                                product = product,
                                onApprove = { id -> viewModel.approveProduct(id) },
                                onReject = { id -> viewModel.rejectProduct(id) },
                                onEdit = { onNavigateToEdit(product) }
                            )
                        }
                    }
                }
            }
            is RequestProductState.Error -> {
                Box(modifier = Modifier.fillMaxSize(), contentAlignment = Alignment.Center) {
                    Column(horizontalAlignment = Alignment.CenterHorizontally) {
                        Text(
                            text = (uiState as RequestProductState.Error).message,
                            color = MaterialTheme.colorScheme.error
                        )
                        Spacer(modifier = Modifier.height(16.dp))
                        Button(onClick = { viewModel.loadPendingProducts() }) {
                            Text("Retry")
                        }
                    }
                }
            }
        }
    }
}

@Composable
fun PendingProductItem(
    product: JsonObject,
    onApprove: (String) -> Unit,
    onReject: (String) -> Unit,
    onEdit: () -> Unit
) {
    val id = product.getStringSafe("id")
    val name = product.getStringSafe("name", "Unknown")
    val barcode = product.getStringSafe("barcode", "-")
    val photoUrl = product.getStringSafe("image_url")

    val baseUrl = "https://api-go-v1.free-account.my.id"
    val fullImageUrl = if (photoUrl.startsWith("/")) "$baseUrl$photoUrl" else photoUrl

    Card(
        modifier = Modifier.fillMaxWidth(),
        elevation = CardDefaults.cardElevation(defaultElevation = 2.dp)
    ) {
        Column(modifier = Modifier.padding(16.dp)) {
            Row(
                modifier = Modifier.fillMaxWidth(),
                verticalAlignment = Alignment.CenterVertically
            ) {
                AsyncImage(
                    model = fullImageUrl.ifEmpty { "https://via.placeholder.com/150" },
                    contentDescription = name,
                    modifier = Modifier.size(64.dp),
                    contentScale = ContentScale.Crop
                )
                Spacer(modifier = Modifier.width(16.dp))
                Column {
                    Text(
                        text = name,
                        style = MaterialTheme.typography.titleMedium,
                        fontWeight = FontWeight.Bold
                    )
                    Text(
                        text = "Barcode: $barcode",
                        style = MaterialTheme.typography.bodyMedium
                    )
                }
            }

            Spacer(modifier = Modifier.height(16.dp))

            Row(
                modifier = Modifier.fillMaxWidth(),
                horizontalArrangement = Arrangement.SpaceBetween,
                verticalAlignment = Alignment.CenterVertically
            ) {
                OutlinedButton(onClick = onEdit) {
                    Text("Edit")
                }
                Row(
                    horizontalArrangement = Arrangement.End
                ) {
                    OutlinedButton(
                        onClick = { onReject(id) },
                        colors = ButtonDefaults.outlinedButtonColors(contentColor = MaterialTheme.colorScheme.error)
                    ) {
                        Text("Reject")
                    }
                    Spacer(modifier = Modifier.width(8.dp))
                    Button(onClick = { onApprove(id) }) {
                        Text("Approve")
                    }
                }
            }
        }
    }
}
