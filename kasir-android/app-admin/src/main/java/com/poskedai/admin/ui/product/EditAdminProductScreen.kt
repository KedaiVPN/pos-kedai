package com.poskedai.admin.ui.product

import android.net.Uri
import android.widget.Toast
import androidx.activity.compose.rememberLauncherForActivityResult
import androidx.activity.result.contract.ActivityResultContracts
import androidx.compose.foundation.background
import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.*
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.foundation.verticalScroll
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.filled.ArrowBack
import androidx.compose.material.icons.filled.CameraAlt
import androidx.compose.material.icons.filled.CloudDownload
import androidx.compose.material.icons.filled.PhotoLibrary
import androidx.compose.material.icons.filled.QrCodeScanner
import androidx.compose.material.icons.filled.Add
import androidx.compose.material3.*
import androidx.compose.runtime.*
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.layout.ContentScale
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.unit.dp
import androidx.lifecycle.viewmodel.compose.viewModel
import coil.compose.AsyncImage
import com.google.gson.JsonObject
import com.poskedai.admin.ui.scanner.BarcodeScannerScreen
import androidx.compose.ui.window.Dialog
import androidx.compose.ui.window.DialogProperties
import androidx.core.content.FileProvider
import java.io.File

@OptIn(ExperimentalMaterial3Api::class)
@Composable
fun EditAdminProductScreen(
    productJson: JsonObject,
    isPending: Boolean = true,
    onBackClick: () -> Unit,
    onSuccess: () -> Unit,
    viewModel: EditAdminProductViewModel = viewModel()
) {
    val context = LocalContext.current
    val uiState by viewModel.uiState.collectAsState()
    
    val productId = if (productJson.has("id") && !productJson.get("id").isJsonNull) productJson.get("id").asString else ""

    LaunchedEffect(productJson) {
        viewModel.loadProduct(productJson)
    }

    LaunchedEffect(uiState.errorMessage, uiState.successMessage) {
        uiState.errorMessage?.let {
            Toast.makeText(context, it, Toast.LENGTH_LONG).show()
            viewModel.clearMessages()
        }
        uiState.successMessage?.let {
            Toast.makeText(context, it, Toast.LENGTH_SHORT).show()
            viewModel.clearMessages()
        }
    }

    var showScanner by remember { mutableStateOf(false) }
    var showPhotoBottomSheet by remember { mutableStateOf(false) }
    var showNewCategoryDialog by remember { mutableStateOf(false) }
    var showServerPhotoDialog by remember { mutableStateOf(false) }
    var newCategoryInput by remember { mutableStateOf("") }
    var serverPhotos by remember { mutableStateOf<List<Map<String, String>>>(emptyList()) }
    var loadingServerPhotos by remember { mutableStateOf(false) }
    
    // Camera Image Capture Launcher
    var tempCameraUri by remember { mutableStateOf<Uri?>(null) }
    val cameraLauncher = rememberLauncherForActivityResult(ActivityResultContracts.TakePicture()) { success ->
        if (success && tempCameraUri != null) {
            viewModel.uploadPhotoFromUri(context, tempCameraUri!!, uiState.productBarcode)
        }
    }

    // Gallery Picker Launcher
    val galleryLauncher = rememberLauncherForActivityResult(ActivityResultContracts.GetContent()) { uri: Uri? ->
        uri?.let {
            viewModel.uploadPhotoFromUri(context, it, uiState.productBarcode)
        }
    }

    if (showScanner) {
        Dialog(
            onDismissRequest = { showScanner = false },
            properties = DialogProperties(usePlatformDefaultWidth = false)
        ) {
            Box(modifier = Modifier.fillMaxSize()) {
                BarcodeScannerScreen(
                    onBarcodeScanned = {
                        viewModel.updateProductBarcode(it)
                        showScanner = false
                    }
                )
                IconButton(
                    onClick = { showScanner = false },
                    modifier = Modifier.align(Alignment.TopStart).padding(16.dp)
                ) {
                    Text("Tutup", color = MaterialTheme.colorScheme.onPrimary)
                }
            }
        }
        return
    }

    Scaffold(
        topBar = {
            TopAppBar(
                title = { Text(if (isPending) "Edit Request Produk" else "Edit Produk Master") },
                navigationIcon = {
                    IconButton(onClick = onBackClick) {
                        Icon(Icons.Default.ArrowBack, contentDescription = "Kembali")
                    }
                }
            )
        }
    ) { innerPadding ->
        Column(
            modifier = Modifier
                .fillMaxSize()
                .padding(innerPadding)
                .padding(16.dp)
                .verticalScroll(rememberScrollState()),
            verticalArrangement = Arrangement.spacedBy(16.dp)
        ) {
            // Photo Preview & Change Section
            Card(
                modifier = Modifier
                    .fillMaxWidth()
                    .height(200.dp),
                shape = RoundedCornerShape(12.dp),
                elevation = CardDefaults.cardElevation(defaultElevation = 2.dp)
            ) {
                Box(modifier = Modifier.fillMaxSize()) {
                    val baseUrl = "https://api-go-v1.free-account.my.id"
                    val fullImageUrl = if (uiState.photoUrl.startsWith("/")) "$baseUrl${uiState.photoUrl}" else uiState.photoUrl
                    
                    AsyncImage(
                        model = fullImageUrl.ifEmpty { "https://via.placeholder.com/300x200" },
                        contentDescription = "Foto Produk",
                        modifier = Modifier.fillMaxSize(),
                        contentScale = ContentScale.Crop
                    )

                    Button(
                        onClick = { showPhotoBottomSheet = true },
                        modifier = Modifier
                            .align(Alignment.BottomEnd)
                            .padding(12.dp)
                    ) {
                        Icon(Icons.Default.CameraAlt, contentDescription = null, modifier = Modifier.size(18.dp))
                        Spacer(modifier = Modifier.width(6.dp))
                        Text("Ganti Foto")
                    }
                }
            }

            // Product Name Field
            OutlinedTextField(
                value = uiState.productName,
                onValueChange = { viewModel.updateProductName(it) },
                label = { Text("Nama Produk") },
                modifier = Modifier.fillMaxWidth(),
                singleLine = true
            )

            // Barcode Field
            Row(
                modifier = Modifier.fillMaxWidth(),
                verticalAlignment = Alignment.CenterVertically
            ) {
                OutlinedTextField(
                    value = uiState.productBarcode,
                    onValueChange = { viewModel.updateProductBarcode(it) },
                    label = { Text("Barcode") },
                    modifier = Modifier.weight(1f),
                    singleLine = true
                )
                Spacer(modifier = Modifier.width(8.dp))
                IconButton(onClick = { showScanner = true }) {
                    Icon(Icons.Default.QrCodeScanner, contentDescription = "Scan Barcode")
                }
            }

            // Category Dropdown + Add New Category
            Text(
                text = "Kategori Produk",
                style = MaterialTheme.typography.titleMedium,
                fontWeight = FontWeight.Bold
            )

            var categoryExpanded by remember { mutableStateOf(false) }
            ExposedDropdownMenuBox(
                expanded = categoryExpanded,
                onExpandedChange = { categoryExpanded = !categoryExpanded }
            ) {
                OutlinedTextField(
                    value = uiState.productCategory,
                    onValueChange = { viewModel.updateProductCategory(it) },
                    label = { Text("Kategori") },
                    readOnly = false,
                    trailingIcon = { ExposedDropdownMenuDefaults.TrailingIcon(expanded = categoryExpanded) },
                    modifier = Modifier
                        .menuAnchor()
                        .fillMaxWidth()
                )

                ExposedDropdownMenu(
                    expanded = categoryExpanded,
                    onDismissRequest = { categoryExpanded = false }
                ) {
                    uiState.categories.forEach { category ->
                        DropdownMenuItem(
                            text = { Text(category) },
                            onClick = {
                                viewModel.updateProductCategory(category)
                                categoryExpanded = false
                            }
                        )
                    }

                    HorizontalDivider()
                    DropdownMenuItem(
                        text = {
                            Row(verticalAlignment = Alignment.CenterVertically) {
                                Icon(Icons.Default.Add, contentDescription = null, tint = MaterialTheme.colorScheme.primary)
                                Spacer(modifier = Modifier.width(8.dp))
                                Text("+ Buat Kategori Baru", color = MaterialTheme.colorScheme.primary, fontWeight = FontWeight.Bold)
                            }
                        },
                        onClick = {
                            categoryExpanded = false
                            showNewCategoryDialog = true
                        }
                    )
                }
            }

            Spacer(modifier = Modifier.weight(1f))

            // Action Buttons
            Column(
                modifier = Modifier.fillMaxWidth(),
                verticalArrangement = Arrangement.spacedBy(8.dp)
            ) {
                if (isPending) {
                    Button(
                        onClick = {
                            viewModel.saveAndAppoveProduct(productId, onSuccess)
                        },
                        modifier = Modifier.fillMaxWidth(),
                        colors = ButtonDefaults.buttonColors(containerColor = MaterialTheme.colorScheme.primary)
                    ) {
                        Text("Simpan & Langsung Approve")
                    }
                }

                Row(
                    modifier = Modifier.fillMaxWidth(),
                    horizontalArrangement = Arrangement.spacedBy(8.dp)
                ) {
                    OutlinedButton(
                        onClick = onBackClick,
                        modifier = Modifier.weight(1f)
                    ) {
                        Text("Batal")
                    }

                    Button(
                        onClick = {
                            viewModel.saveProductChanges(productId, isPending, onSuccess)
                        },
                        modifier = Modifier.weight(1f)
                    ) {
                        Text("Simpan")
                    }
                }
            }
        }
    }

    // New Category Dialog
    if (showNewCategoryDialog) {
        AlertDialog(
            onDismissRequest = { showNewCategoryDialog = false },
            title = { Text("Buat Kategori Baru") },
            text = {
                OutlinedTextField(
                    value = newCategoryInput,
                    onValueChange = { newCategoryInput = it },
                    label = { Text("Nama Kategori") },
                    singleLine = true,
                    modifier = Modifier.fillMaxWidth()
                )
            },
            confirmButton = {
                Button(
                    onClick = {
                        if (newCategoryInput.isNotBlank()) {
                            viewModel.createNewCategory(newCategoryInput.trim())
                            newCategoryInput = ""
                            showNewCategoryDialog = false
                        }
                    }
                ) {
                    Text("Tambah")
                }
            },
            dismissButton = {
                TextButton(onClick = { showNewCategoryDialog = false }) {
                    Text("Batal")
                }
            }
        )
    }

    // Photo Options Modal BottomSheet
    if (showPhotoBottomSheet) {
        ModalBottomSheet(
            onDismissRequest = { showPhotoBottomSheet = false }
        ) {
            Column(
                modifier = Modifier
                    .fillMaxWidth()
                    .padding(24.dp),
                verticalArrangement = Arrangement.spacedBy(16.dp)
            ) {
                Text(
                    text = "Pilih Sumber Foto",
                    style = MaterialTheme.typography.titleMedium,
                    fontWeight = FontWeight.Bold
                )

                ListItem(
                    headlineContent = { Text("Buka Kamera") },
                    leadingContent = { Icon(Icons.Default.CameraAlt, contentDescription = null) },
                    modifier = Modifier
                        .clip(RoundedCornerShape(8.dp))
                        .clickable {
                            showPhotoBottomSheet = false
                            val tempFile = File.createTempFile("camera_photo_", ".jpg", context.cacheDir)
                            val uri = FileProvider.getUriForFile(
                                context,
                                "${context.packageName}.fileprovider",
                                tempFile
                            )
                            tempCameraUri = uri
                            cameraLauncher.launch(uri)
                        }
                )

                ListItem(
                    headlineContent = { Text("Pilih dari Galeri") },
                    leadingContent = { Icon(Icons.Default.PhotoLibrary, contentDescription = null) },
                    modifier = Modifier
                        .clip(RoundedCornerShape(8.dp))
                        .clickable {
                            showPhotoBottomSheet = false
                            galleryLauncher.launch("image/*")
                        }
                )

                ListItem(
                    headlineContent = { Text("Pilih dari Server") },
                    leadingContent = { Icon(Icons.Default.CloudDownload, contentDescription = null) },
                    modifier = Modifier
                        .clip(RoundedCornerShape(8.dp))
                        .clickable {
                            showPhotoBottomSheet = false
                            loadingServerPhotos = true
                            showServerPhotoDialog = true
                            // Fetch server photos
                            kotlinx.coroutines.MainScope().launch {
                                try {
                                    val response = com.poskedai.core.network.RetrofitClient.adminApi.getRecentUploads()
                                    if (response.isSuccessful) {
                                        serverPhotos = response.body()?.get("files") ?: emptyList()
                                    }
                                } catch (e: Exception) {
                                    android.widget.Toast.makeText(context, "Gagal memuat foto server: ${e.message}", android.widget.Toast.LENGTH_SHORT).show()
                                } finally {
                                    loadingServerPhotos = false
                                }
                            }
                        }
                )

                Spacer(modifier = Modifier.height(16.dp))
            }
        }
    }

    // Dialog Pilih Foto dari Server
    if (showServerPhotoDialog) {
        AlertDialog(
            onDismissRequest = { showServerPhotoDialog = false },
            title = { Text("Pilih Foto dari Server") },
            text = {
                if (loadingServerPhotos) {
                    Box(
                        modifier = Modifier
                            .fillMaxWidth()
                            .height(200.dp),
                        contentAlignment = Alignment.Center
                    ) {
                        CircularProgressIndicator()
                    }
                } else if (serverPhotos.isEmpty()) {
                    Box(
                        modifier = Modifier
                            .fillMaxWidth()
                            .height(150.dp),
                        contentAlignment = Alignment.Center
                    ) {
                        Text("Tidak ada foto ditemukan di server", color = MaterialTheme.colorScheme.onSurfaceVariant)
                    }
                } else {
                    androidx.compose.foundation.lazy.grid.LazyVerticalGrid(
                        columns = androidx.compose.foundation.lazy.grid.GridCells.Fixed(3),
                        horizontalArrangement = Arrangement.spacedBy(8.dp),
                        verticalArrangement = Arrangement.spacedBy(8.dp),
                        modifier = Modifier
                            .fillMaxWidth()
                            .heightIn(max = 350.dp)
                    ) {
                        items(serverPhotos.size) { index ->
                            val photo = serverPhotos[index]
                            val url = photo["url"] ?: ""
                            val filename = photo["filename"] ?: ""
                            Card(
                                shape = RoundedCornerShape(8.dp),
                                modifier = Modifier
                                    .aspectRatio(1f)
                                    .clickable {
                                        viewModel.updatePhotoUrl(url)
                                        showServerPhotoDialog = false
                                    }
                            ) {
                                Box(modifier = Modifier.fillMaxSize()) {
                                    coil.compose.AsyncImage(
                                        model = url,
                                        contentDescription = filename,
                                        modifier = Modifier.fillMaxSize(),
                                        contentScale = ContentScale.Crop
                                    )
                                }
                            }
                        }
                    }
                }
            },
            confirmButton = {},
            dismissButton = {
                TextButton(onClick = { showServerPhotoDialog = false }) {
                    Text("Tutup")
                }
            }
        )
    }
}
