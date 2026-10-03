package com.poskedai.admin.ui.product

import androidx.compose.foundation.layout.*
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.items
import androidx.compose.material3.*
import androidx.compose.runtime.Composable
import androidx.compose.runtime.*
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.layout.ContentScale
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.unit.dp
import androidx.lifecycle.viewmodel.compose.viewModel
import coil.compose.AsyncImage
import com.google.gson.JsonObject
import android.widget.Toast
import androidx.compose.ui.platform.LocalContext

@Composable
fun ProductListScreen(
    viewModel: ProductListViewModel = viewModel(),
    onNavigateToImport: () -> Unit = {},
    onNavigateToEdit: (JsonObject) -> Unit = {}
) {
    val uiState by viewModel.uiState.collectAsState()
    val actionState by viewModel.actionState.collectAsState()
    val isMasterEnabled by viewModel.isMasterEnabled.collectAsState()
    val categories by viewModel.categories.collectAsState()
    val context = LocalContext.current

    LaunchedEffect(actionState) {
        actionState?.let {
            Toast.makeText(context, it, Toast.LENGTH_SHORT).show()
            viewModel.clearActionState()
        }
    }

    LaunchedEffect(Unit) {
        viewModel.loadProducts()
        viewModel.loadMasterProductStatus()
        viewModel.loadCategories()
    }

    var showAddDialog by remember { mutableStateOf(false) }

    if (showAddDialog) {
        AddProductDialog(
            availableCategories = categories,
            onCreateCategory = { name, onCreated ->
                viewModel.createNewCategory(name, onCreated)
            },
            onDismiss = { showAddDialog = false },
            onSave = { name, category, barcode, photoUrl, isGeneratedBarcode ->
                viewModel.addProduct(context, name, category, barcode, photoUrl, isGeneratedBarcode)
                showAddDialog = false
            }
        )
    }

    Column(modifier = Modifier.fillMaxSize()) {
        Row(
            modifier = Modifier.fillMaxWidth().padding(16.dp),
            horizontalArrangement = Arrangement.SpaceBetween,
            verticalAlignment = Alignment.CenterVertically
        ) {
            Text(
                text = "List Produk Approved",
                style = MaterialTheme.typography.headlineSmall,
                fontWeight = FontWeight.Bold,
                modifier = Modifier.weight(1f)
            )
            Row(horizontalArrangement = Arrangement.spacedBy(8.dp)) {
                OutlinedButton(onClick = onNavigateToImport) {
                    Text("Import OFF")
                }
                Button(onClick = { showAddDialog = true }) {
                    Text("Tambah")
                }
            }
        }

        Card(
            modifier = Modifier.fillMaxWidth().padding(horizontal = 16.dp, vertical = 8.dp),
            colors = CardDefaults.cardColors(containerColor = MaterialTheme.colorScheme.surfaceVariant)
        ) {
            Row(
                modifier = Modifier.fillMaxWidth().padding(16.dp),
                horizontalArrangement = Arrangement.SpaceBetween,
                verticalAlignment = Alignment.CenterVertically
            ) {
                Column(modifier = Modifier.weight(1f)) {
                    Text(
                        text = "Status Master Produk Global",
                        style = MaterialTheme.typography.titleMedium,
                        fontWeight = FontWeight.Bold
                    )
                    Text(
                        text = if (isMasterEnabled) "Aktif (Terlihat di sidebar semua toko)" else "Nonaktif (Sembunyi dari sidebar toko)",
                        style = MaterialTheme.typography.bodySmall,
                        color = if (isMasterEnabled) MaterialTheme.colorScheme.primary else MaterialTheme.colorScheme.error
                    )
                }
                Switch(
                    checked = isMasterEnabled,
                    onCheckedChange = { viewModel.toggleMasterProductStatus(it) }
                )
            }
        }

        when (uiState) {
            is ProductListState.Loading -> {
                Box(modifier = Modifier.fillMaxSize(), contentAlignment = Alignment.Center) {
                    CircularProgressIndicator()
                }
            }
            is ProductListState.Success -> {
                val products = (uiState as ProductListState.Success).products
                if (products.isEmpty()) {
                    Box(modifier = Modifier.fillMaxSize(), contentAlignment = Alignment.Center) {
                        Text("Belum ada produk approved.")
                    }
                } else {
                    LazyColumn(
                        contentPadding = PaddingValues(16.dp),
                        verticalArrangement = Arrangement.spacedBy(8.dp)
                    ) {
                        items(products.size, key = { products[it].get("id")?.asString ?: it }) { index ->
                            val product = products[index]
                            var showDeleteConfirmDialog by remember { mutableStateOf(false) }

                            ProductItem(
                                product = product,
                                onEdit = { onNavigateToEdit(product) },
                                onDelete = { showDeleteConfirmDialog = true }
                            )

                            if (showDeleteConfirmDialog) {
                                AlertDialog(
                                    onDismissRequest = { showDeleteConfirmDialog = false },
                                    title = { Text("Konfirmasi Hapus") },
                                    text = { Text("Apakah Anda yakin ingin menghapus produk ini?") },
                                    confirmButton = {
                                        Button(onClick = {
                                            val id = product.get("id")?.asString ?: return@Button
                                            viewModel.deleteProduct(id)
                                            showDeleteConfirmDialog = false
                                        }) {
                                            Text("Hapus")
                                        }
                                    },
                                    dismissButton = {
                                        OutlinedButton(onClick = { showDeleteConfirmDialog = false }) {
                                            Text("Batal")
                                        }
                                    }
                                )
                            }
                        }
                    }
                }
            }
            is ProductListState.Error -> {
                Box(modifier = Modifier.fillMaxSize(), contentAlignment = Alignment.Center) {
                    Column(horizontalAlignment = Alignment.CenterHorizontally) {
                        Text(
                            text = (uiState as ProductListState.Error).message,
                            color = MaterialTheme.colorScheme.error
                        )
                        Spacer(modifier = Modifier.height(16.dp))
                        Button(onClick = { viewModel.loadProducts() }) {
                            Text("Retry")
                        }
                    }
                }
            }
        }
    }
}

@Composable
fun ProductItem(product: JsonObject, onEdit: () -> Unit, onDelete: () -> Unit) {
    val name = product.get("name")?.asString ?: "Unknown"
    val barcode = product.get("barcode")?.asString ?: "-"
    val category = product.get("category_name")?.asString ?: "-"
    val photoUrl = if (product.has("photo_url") && !product.get("photo_url").isJsonNull) {
        product.get("photo_url").asString
    } else ""

    val baseUrl = "https://api-go-v1.free-account.my.id"
    val fullImageUrl = if (photoUrl.startsWith("/")) "$baseUrl$photoUrl" else photoUrl

    Card(
        modifier = Modifier.fillMaxWidth(),
        elevation = CardDefaults.cardElevation(defaultElevation = 2.dp)
    ) {
        Row(
            modifier = Modifier
                .fillMaxWidth()
                .padding(16.dp),
            verticalAlignment = Alignment.CenterVertically
        ) {
            AsyncImage(
                model = fullImageUrl.ifEmpty { "https://via.placeholder.com/150" },
                contentDescription = name,
                modifier = Modifier.size(64.dp),
                contentScale = ContentScale.Crop
            )
            Spacer(modifier = Modifier.width(16.dp))
            Column(modifier = Modifier.weight(1f)) {
                Text(
                    text = name,
                    style = MaterialTheme.typography.titleMedium,
                    fontWeight = FontWeight.Bold
                )
                Text(
                    text = "Barcode: $barcode",
                    style = MaterialTheme.typography.bodyMedium
                )
                Text(
                    text = "Kategori: $category",
                    style = MaterialTheme.typography.bodySmall,
                    color = MaterialTheme.colorScheme.onSurfaceVariant
                )
            }
            Column {
                OutlinedButton(onClick = onEdit) {
                    Text("Edit")
                }
                Spacer(modifier = Modifier.height(4.dp))
                OutlinedButton(
                    onClick = onDelete,
                    colors = ButtonDefaults.outlinedButtonColors(contentColor = MaterialTheme.colorScheme.error)
                ) {
                    Text("Hapus")
                }
            }
        }
    }
}
