package com.poskedai.admin.ui.product

import androidx.compose.foundation.background
import androidx.compose.foundation.layout.*
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.items
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.filled.Close
import androidx.compose.material.icons.filled.Done
import androidx.compose.material.icons.filled.Refresh
import androidx.compose.material3.*
import androidx.compose.runtime.*
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.layout.ContentScale
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.unit.dp
import androidx.lifecycle.viewmodel.compose.viewModel
import coil.compose.AsyncImage
import com.poskedai.core.network.OFFDraftDto
import com.poskedai.core.network.RetrofitClient
import android.widget.Toast
import androidx.compose.ui.platform.LocalContext

@OptIn(ExperimentalMaterial3Api::class)
@Composable
fun ImportProductScreen(viewModel: ImportProductViewModel = viewModel()) {
    val uiState by viewModel.uiState.collectAsState()
    val isFetching by viewModel.isFetching.collectAsState()
    val actionMessage by viewModel.actionMessage.collectAsState()
    val errorDialogMessage by viewModel.errorDialogMessage.collectAsState()
    val currentPage by viewModel.currentPage.collectAsState()
    val context = LocalContext.current

    LaunchedEffect(actionMessage) {
        actionMessage?.let {
            Toast.makeText(context, it, Toast.LENGTH_SHORT).show()
            viewModel.clearActionMessage()
        }
    }

    var showApproveDialog by remember { mutableStateOf<OFFDraftDto?>(null) }

    if (showApproveDialog != null) {
        ApproveDialog(
            draft = showApproveDialog!!,
            onDismiss = { showApproveDialog = null },
            onApprove = { name, categoryId, unit ->
                viewModel.approveDraft(
                    barcode = showApproveDialog!!.barcode,
                    name = name,
                    categoryId = categoryId,
                    unit = unit
                )
                showApproveDialog = null
            }
        )
    }

    // Error Dialog Modal
    if (errorDialogMessage != null) {
        AlertDialog(
            onDismissRequest = { viewModel.clearErrorDialog() },
            title = { Text("Gagal Menarik dari Open Food Facts") },
            text = { Text(errorDialogMessage!!) },
            confirmButton = {
                Button(onClick = { viewModel.clearErrorDialog() }) {
                    Text("Mengerti")
                }
            }
        )
    }

    Column(modifier = Modifier.fillMaxSize()) {
        // Header dengan tombol fetch
        Card(
            modifier = Modifier.fillMaxWidth().padding(16.dp),
            colors = CardDefaults.cardColors(containerColor = MaterialTheme.colorScheme.primaryContainer)
        ) {
            Column(modifier = Modifier.padding(16.dp)) {
                Text(
                    text = "Import Produk dari Open Food Facts",
                    style = MaterialTheme.typography.titleLarge,
                    fontWeight = FontWeight.Bold,
                    color = MaterialTheme.colorScheme.onPrimaryContainer
                )
                Spacer(modifier = Modifier.height(8.dp))
                Text(
                    text = "Tarik produk Indonesia dari database OFF, edit nama & kategori, lalu approve ke Master Produk",
                    style = MaterialTheme.typography.bodyMedium,
                    color = MaterialTheme.colorScheme.onPrimaryContainer
                )
                Spacer(modifier = Modifier.height(12.dp))
                Row(
                    horizontalArrangement = Arrangement.SpaceBetween,
                    verticalAlignment = Alignment.CenterVertically,
                    modifier = Modifier.fillMaxWidth()
                ) {
                    Button(
                        onClick = { viewModel.fetchFromS3() },
                        enabled = !isFetching,
                        modifier = Modifier.weight(1f)
                    ) {
                        if (isFetching) {
                            CircularProgressIndicator(
                                modifier = Modifier.size(20.dp),
                                color = MaterialTheme.colorScheme.onPrimary,
                                strokeWidth = 2.dp
                            )
                            Spacer(modifier = Modifier.width(8.dp))
                        }
                        Text(if (isFetching) "Menarik..." else "Tarik 25 Produk (S3 Dump)")
                    }
                    Spacer(modifier = Modifier.width(8.dp))
                    IconButton(onClick = { viewModel.loadDrafts() }) {
                        Icon(Icons.Filled.Refresh, contentDescription = "Refresh")
                    }
                }
            }
        }

        // Loading Progress Banner
        if (isFetching) {
            Card(
                modifier = Modifier.fillMaxWidth().padding(horizontal = 16.dp),
                colors = CardDefaults.cardColors(containerColor = MaterialTheme.colorScheme.surfaceVariant)
            ) {
                Column(modifier = Modifier.padding(12.dp)) {
                    Text(
                        text = "Sedang mengunduh 25 produk & foto dari S3 dump...",
                        style = MaterialTheme.typography.bodySmall,
                        fontWeight = FontWeight.SemiBold
                    )
                    Spacer(modifier = Modifier.height(6.dp))
                    LinearProgressIndicator(modifier = Modifier.fillMaxWidth())
                }
            }
        }

        // Draft count
        when (val state = uiState) {
            is ImportProductState.Success -> {
                Text(
                    text = "Draft antrean: ${state.drafts.size} produk",
                    style = MaterialTheme.typography.bodyLarge,
                    fontWeight = FontWeight.Bold,
                    modifier = Modifier.padding(horizontal = 16.dp, vertical = 8.dp)
                )
            }
            else -> {}
        }

        // List draft
        when (val state = uiState) {
            is ImportProductState.Loading -> {
                Box(modifier = Modifier.fillMaxSize(), contentAlignment = Alignment.Center) {
                    CircularProgressIndicator()
                }
            }
            is ImportProductState.Success -> {
                if (state.drafts.isEmpty()) {
                    Box(modifier = Modifier.fillMaxSize(), contentAlignment = Alignment.Center) {
                        Column(horizontalAlignment = Alignment.CenterHorizontally) {
                            Text(
                                text = "Belum ada draft",
                                style = MaterialTheme.typography.titleMedium,
                                color = MaterialTheme.colorScheme.onSurfaceVariant
                            )
                            Text(
                                text = "Klik tombol di atas untuk tarik produk dari OFF",
                                style = MaterialTheme.typography.bodyMedium,
                                color = MaterialTheme.colorScheme.onSurfaceVariant
                            )
                        }
                    }
                } else {
                    LazyColumn(
                        modifier = Modifier.fillMaxSize(),
                        contentPadding = PaddingValues(16.dp),
                        verticalArrangement = Arrangement.spacedBy(12.dp)
                    ) {
                        items(state.drafts, key = { it.barcode }) { draft ->
                            DraftCard(
                                draft = draft,
                                onApprove = { showApproveDialog = draft },
                                onReject = { viewModel.rejectDraft(draft.barcode) }
                            )
                        }
                    }
                }
            }
            is ImportProductState.Error -> {
                Box(modifier = Modifier.fillMaxSize(), contentAlignment = Alignment.Center) {
                    Column(horizontalAlignment = Alignment.CenterHorizontally) {
                        Text(
                            text = "Error: ${state.message}",
                            style = MaterialTheme.typography.bodyLarge,
                            color = MaterialTheme.colorScheme.error
                        )
                        Spacer(modifier = Modifier.height(8.dp))
                        Button(onClick = { viewModel.loadDrafts() }) {
                            Text("Coba Lagi")
                        }
                    }
                }
            }
        }
    }
}

@Composable
fun DraftCard(
    draft: OFFDraftDto,
    onApprove: () -> Unit,
    onReject: () -> Unit
) {
    Card(
        modifier = Modifier.fillMaxWidth(),
        elevation = CardDefaults.cardElevation(defaultElevation = 2.dp)
    ) {
        Column(modifier = Modifier.padding(12.dp)) {
            Row(modifier = Modifier.fillMaxWidth()) {
                // Image dengan loading & error state
                if (draft.image_url.isNotEmpty()) {
                    var isImageLoading by remember(draft.image_url) { mutableStateOf(true) }
                    var isImageError by remember(draft.image_url) { mutableStateOf(false) }

                    Box(
                        modifier = Modifier.size(80.dp),
                        contentAlignment = Alignment.Center
                    ) {
                        val imageUrl = if (draft.image_url.startsWith("/")) {
                            "${RetrofitClient.IMAGE_BASE_URL}${draft.image_url}"
                        } else {
                            draft.image_url
                        }
                        
                        coil.compose.AsyncImage(
                            model = imageUrl,
                            contentDescription = "Foto Produk",
                            modifier = Modifier.size(80.dp),
                            contentScale = ContentScale.Crop,
                            onLoading = { isImageLoading = true },
                            onSuccess = { isImageLoading = false },
                            onError = {
                                isImageLoading = false
                                isImageError = true
                            }
                        )

                        // Loading shimmer
                        if (isImageLoading) {
                            CircularProgressIndicator(
                                modifier = Modifier.size(24.dp),
                                strokeWidth = 2.dp,
                                color = MaterialTheme.colorScheme.primary
                            )
                        }

                        // Error overlay
                        if (isImageError) {
                            Box(
                                modifier = Modifier.fillMaxSize().background(MaterialTheme.colorScheme.errorContainer),
                                contentAlignment = Alignment.Center
                            ) {
                                Icon(
                                    imageVector = Icons.Default.Close,
                                    contentDescription = "Foto Gagal",
                                    tint = MaterialTheme.colorScheme.error,
                                    modifier = Modifier.size(24.dp)
                                )
                            }
                        }
                    }
                    Spacer(modifier = Modifier.width(12.dp))
                }

                // Info
                Column(modifier = Modifier.weight(1f)) {
                    Text(
                        text = draft.raw_name,
                        style = MaterialTheme.typography.titleMedium,
                        fontWeight = FontWeight.Bold
                    )
                    if (draft.brand.isNotEmpty()) {
                        Text(
                            text = "Brand: ${draft.brand}",
                            style = MaterialTheme.typography.bodySmall,
                            color = MaterialTheme.colorScheme.onSurfaceVariant
                        )
                    }
                    Text(
                        text = "Barcode: ${draft.barcode}",
                        style = MaterialTheme.typography.bodySmall,
                        color = MaterialTheme.colorScheme.onSurfaceVariant
                    )
                    if (draft.raw_category.isNotEmpty()) {
                        Text(
                            text = "Kategori OFF: ${draft.raw_category}",
                            style = MaterialTheme.typography.bodySmall,
                            color = MaterialTheme.colorScheme.secondary
                        )
                    }
                }
            }

            Spacer(modifier = Modifier.height(12.dp))

            // Action buttons
            Row(
                modifier = Modifier.fillMaxWidth(),
                horizontalArrangement = Arrangement.End,
                verticalAlignment = Alignment.CenterVertically
            ) {
                OutlinedButton(
                    onClick = onReject,
                    colors = ButtonDefaults.outlinedButtonColors(
                        contentColor = MaterialTheme.colorScheme.error
                    )
                ) {
                    Icon(Icons.Filled.Close, contentDescription = null, modifier = Modifier.size(18.dp))
                    Spacer(modifier = Modifier.width(4.dp))
                    Text("Skip")
                }
                Spacer(modifier = Modifier.width(8.dp))
                Button(onClick = onApprove) {
                    Icon(Icons.Filled.Done, contentDescription = null, modifier = Modifier.size(18.dp))
                    Spacer(modifier = Modifier.width(4.dp))
                    Text("Approve")
                }
            }
        }
    }
}

@OptIn(ExperimentalMaterial3Api::class)
@Composable
fun ApproveDialog(
    draft: OFFDraftDto,
    onDismiss: () -> Unit,
    onApprove: (name: String, categoryId: String, unit: String) -> Unit
) {
    var name by remember { mutableStateOf(draft.raw_name) }
    var categoryId by remember { mutableStateOf("") }
    var unit by remember { mutableStateOf("pcs") }
    var categoryExpanded by remember { mutableStateOf(false) }
    var unitExpanded by remember { mutableStateOf(false) }

    val categories = listOf(
        "Makanan & Minuman",
        "Sembako",
        "Minuman",
        "Snack",
        "Rokok",
        "Toiletries",
        "Perlengkapan Rumah",
        "Lainnya"
    )

    val units = listOf("pcs", "pack", "box", "karton", "lusin", "kg", "liter")

    AlertDialog(
        onDismissRequest = onDismiss,
        title = { Text("Kurasi Produk") },
        text = {
            Column(verticalArrangement = Arrangement.spacedBy(12.dp)) {
                // Preview image
                if (draft.image_url.isNotEmpty()) {
                    val previewUrl = if (draft.image_url.startsWith("/")) {
                        "${RetrofitClient.IMAGE_BASE_URL}${draft.image_url}"
                    } else {
                        draft.image_url
                    }
                    AsyncImage(
                        model = previewUrl,
                        contentDescription = "Preview",
                        modifier = Modifier.fillMaxWidth().height(120.dp),
                        contentScale = ContentScale.Fit
                    )
                }

                Text(
                    text = "Barcode: ${draft.barcode}",
                    style = MaterialTheme.typography.bodySmall,
                    color = MaterialTheme.colorScheme.onSurfaceVariant
                )

                OutlinedTextField(
                    value = name,
                    onValueChange = { name = it },
                    label = { Text("Nama Produk (Edit Bahasa Indo)") },
                    modifier = Modifier.fillMaxWidth(),
                    supportingText = { Text("Nama asli: ${draft.raw_name}") }
                )

                // Category dropdown
                ExposedDropdownMenuBox(
                    expanded = categoryExpanded,
                    onExpandedChange = { categoryExpanded = !categoryExpanded }
                ) {
                    OutlinedTextField(
                        value = categoryId,
                        onValueChange = {},
                        readOnly = true,
                        label = { Text("Kategori POS Kedai") },
                        trailingIcon = { ExposedDropdownMenuDefaults.TrailingIcon(expanded = categoryExpanded) },
                        modifier = Modifier.fillMaxWidth().menuAnchor()
                    )
                    ExposedDropdownMenu(
                        expanded = categoryExpanded,
                        onDismissRequest = { categoryExpanded = false }
                    ) {
                        categories.forEach { cat ->
                            DropdownMenuItem(
                                text = { Text(cat) },
                                onClick = {
                                    categoryId = cat
                                    categoryExpanded = false
                                }
                            )
                        }
                    }
                }

                // Unit dropdown
                ExposedDropdownMenuBox(
                    expanded = unitExpanded,
                    onExpandedChange = { unitExpanded = !unitExpanded }
                ) {
                    OutlinedTextField(
                        value = unit,
                        onValueChange = {},
                        readOnly = true,
                        label = { Text("Satuan") },
                        trailingIcon = { ExposedDropdownMenuDefaults.TrailingIcon(expanded = unitExpanded) },
                        modifier = Modifier.fillMaxWidth().menuAnchor()
                    )
                    ExposedDropdownMenu(
                        expanded = unitExpanded,
                        onDismissRequest = { unitExpanded = false }
                    ) {
                        units.forEach { u ->
                            DropdownMenuItem(
                                text = { Text(u) },
                                onClick = {
                                    unit = u
                                    unitExpanded = false
                                }
                            )
                        }
                    }
                }
            }
        },
        confirmButton = {
            Button(
                onClick = {
                    if (name.isNotBlank() && categoryId.isNotBlank()) {
                        onApprove(name, categoryId, unit)
                    }
                },
                enabled = name.isNotBlank() && categoryId.isNotBlank()
            ) {
                Text("Approve & Simpan")
            }
        },
        dismissButton = {
            TextButton(onClick = onDismiss) {
                Text("Batal")
            }
        }
    )
}
