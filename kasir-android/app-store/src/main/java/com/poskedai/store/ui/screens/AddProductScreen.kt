package com.poskedai.store.ui.screens

import androidx.compose.foundation.layout.*
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.text.KeyboardOptions
import androidx.compose.foundation.verticalScroll
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.filled.Add
import androidx.compose.material.icons.filled.ArrowBack
import androidx.compose.material.icons.filled.CameraAlt
import androidx.compose.material.icons.filled.PhotoLibrary
import androidx.compose.material.icons.filled.QrCodeScanner
import androidx.compose.material3.*
import androidx.compose.runtime.*
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.text.input.KeyboardType
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.text.font.FontWeight

import android.graphics.Bitmap
import android.net.Uri
import androidx.activity.compose.rememberLauncherForActivityResult
import androidx.activity.result.contract.ActivityResultContracts
import androidx.compose.ui.graphics.asImageBitmap
import androidx.compose.foundation.Image
import androidx.compose.foundation.clickable
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.ui.draw.clip
import androidx.compose.ui.layout.ContentScale
import com.poskedai.store.utils.BarcodeUtils
import java.util.UUID
import androidx.lifecycle.viewmodel.compose.viewModel
import com.poskedai.store.ui.viewmodels.AddProductViewModel
import com.poskedai.store.ui.viewmodels.AddProductViewModelFactory
import com.poskedai.store.data.local.AppDatabase
import com.poskedai.store.data.repository.ProductRepository
import android.widget.Toast
import androidx.compose.ui.platform.LocalContext
import coil.compose.AsyncImage

@OptIn(ExperimentalMaterial3Api::class)
@Composable
fun AddProductScreen(
    productId: String? = null,
    capturedImageUri: String? = null,
    scannedBarcode: String? = null,
    onNavigateBack: () -> Unit,
    onNavigateToCamera: () -> Unit,
    onNavigateToScanner: () -> Unit,
    isPro: Boolean = false,
    onProRequired: () -> Unit = {}
) {
    val context = LocalContext.current

    // Using remember so it's only instantiated once per composable lifecycle
    val repository = remember {
        val database = AppDatabase.getDatabase(context)
        ProductRepository(database.productDao(), database.transactionDao(), context.applicationContext)
    }

    val viewModel: AddProductViewModel = viewModel(factory = AddProductViewModelFactory(repository))

    val isLoading by viewModel.isLoading.collectAsState()
    val isSuccess by viewModel.isSuccess.collectAsState()
    val errorMessage by viewModel.errorMessage.collectAsState()
    val formState by viewModel.formState.collectAsState()
    val categories by viewModel.categories.collectAsState()

    var categoryDropdownExpanded by remember { mutableStateOf(false) }
    var showNewCategoryDialog by remember { mutableStateOf(false) }
    var newCategoryInput by remember { mutableStateOf("") }

    var generatedBarcodeBitmap by remember { mutableStateOf<Bitmap?>(null) }

    LaunchedEffect(capturedImageUri) {
        if (capturedImageUri != null) {
            viewModel.updateFormState { it.copy(imageUri = capturedImageUri) }
        }
    }

    LaunchedEffect(scannedBarcode) {
        if (scannedBarcode != null) {
            viewModel.updateFormState { it.copy(barcode = scannedBarcode) }
        }
    }

    LaunchedEffect(productId) {
        if (productId != null) {
            viewModel.loadProduct(productId)
        }
    }

    val imagePickerLauncher = rememberLauncherForActivityResult(
        contract = ActivityResultContracts.GetContent()
    ) { uri: Uri? ->
        if (uri != null) {
            viewModel.updateFormState { it.copy(imageUri = uri.toString()) }
        }
    }

    LaunchedEffect(isSuccess) {
        if (isSuccess) {
            Toast.makeText(context, "Sukses menambahkan produk", Toast.LENGTH_SHORT).show()
            viewModel.resetState()
            onNavigateBack()
        }
    }

    LaunchedEffect(errorMessage) {
        errorMessage?.let {
            Toast.makeText(context, it, Toast.LENGTH_SHORT).show()
            viewModel.resetState()
        }
    }

    // Barcode Confirmation Dialog
    if (formState.showBarcodeConfirmDialog) {
        AlertDialog(
            onDismissRequest = { viewModel.dismissBarcodeDialog() },
            title = { 
                Text(
                    "Produk Tanpa Barcode?",
                    style = MaterialTheme.typography.titleLarge,
                    fontWeight = FontWeight.SemiBold
                )
            },
            text = {
                Column(verticalArrangement = Arrangement.spacedBy(12.dp)) {
                    Text(
                        "Seriusan gk ada barcode nya?",
                        style = MaterialTheme.typography.bodyLarge
                    )
                    Text(
                        "Barcode akan di generate otomatis dan dapat di ubah di menu produk & stok",
                        style = MaterialTheme.typography.bodyMedium,
                        color = MaterialTheme.colorScheme.onSurfaceVariant
                    )
                }
            },
            confirmButton = {
                Button(
                    onClick = { viewModel.confirmAutoGenerateAndSubmit(productId) },
                    modifier = Modifier.height(48.dp)
                ) {
                    Text("Ya, Lanjutkan")
                }
            },
            dismissButton = {
                Button(
                    onClick = { viewModel.dismissBarcodeDialog() },
                    modifier = Modifier.height(48.dp),
                    colors = ButtonDefaults.buttonColors(
                        containerColor = MaterialTheme.colorScheme.error
                    )
                ) {
                    Text("Tidak")
                }
            }
        )
    }

    Scaffold(
        topBar = {
            TopAppBar(
                title = { Text("Tambah Produk") },
                navigationIcon = {
                    IconButton(onClick = onNavigateBack) {
                        Icon(Icons.Filled.ArrowBack, contentDescription = "Kembali")
                    }
                }
            )
        }
    ) { paddingValues ->
        Column(
            modifier = Modifier
                .fillMaxSize()
                .padding(paddingValues)
                .padding(16.dp)
                .verticalScroll(rememberScrollState()),
            verticalArrangement = Arrangement.spacedBy(16.dp)
        ) {
            // Image Picker Section
            Card(
                modifier = Modifier.fillMaxWidth(),
                colors = CardDefaults.cardColors(containerColor = MaterialTheme.colorScheme.surfaceVariant)
            ) {
                Column(modifier = Modifier.padding(16.dp)) {
                    Text(text = "Foto Produk", style = MaterialTheme.typography.titleMedium)
                    Spacer(modifier = Modifier.height(8.dp))

                    val currentImageUri = formState.imageUri
                    if (currentImageUri != null) {
                        AsyncImage(
                            model = if (currentImageUri.startsWith("content://") || currentImageUri.startsWith("file://") || currentImageUri.startsWith("http")) {
                                Uri.parse(currentImageUri)
                            } else {
                                "${com.poskedai.core.network.RetrofitClient.IMAGE_BASE_URL}${if(currentImageUri.startsWith("/")) currentImageUri else "/$currentImageUri"}"
                            },
                            contentDescription = "Foto Produk",
                            contentScale = ContentScale.Crop,
                            modifier = Modifier
                                .fillMaxWidth()
                                .height(200.dp)
                                .clip(RoundedCornerShape(8.dp))
                        )
                        Spacer(modifier = Modifier.height(8.dp))
                    }

                    Row(
                        modifier = Modifier.fillMaxWidth(),
                        horizontalArrangement = Arrangement.spacedBy(8.dp)
                    ) {
                        Button(
                            onClick = { imagePickerLauncher.launch("image/*") },
                            modifier = Modifier.weight(1f)
                        ) {
                            Icon(Icons.Filled.PhotoLibrary, contentDescription = "Galeri", modifier = Modifier.size(18.dp))
                            Spacer(Modifier.width(4.dp))
                            Text("Galeri")
                        }
                        Button(
                            onClick = onNavigateToCamera,
                            modifier = Modifier.weight(1f)
                        ) {
                            Icon(Icons.Filled.CameraAlt, contentDescription = "Kamera", modifier = Modifier.size(18.dp))
                            Spacer(Modifier.width(4.dp))
                            Text("Kamera")
                        }
                    }
                }
            }

            OutlinedTextField(
                value = formState.name,
                onValueChange = { v -> viewModel.updateFormState { it.copy(name = v) } },
                label = { Text("Nama Produk") },
                modifier = Modifier.fillMaxWidth()
            )

            Row(horizontalArrangement = Arrangement.spacedBy(16.dp)) {
                OutlinedTextField(
                    value = formState.buyPrice,
                    onValueChange = { v -> viewModel.updateFormState { it.copy(buyPrice = v) } },
                    label = { Text("Harga Beli") },
                    keyboardOptions = KeyboardOptions(keyboardType = KeyboardType.Number),
                    modifier = Modifier.weight(1f)
                )
                OutlinedTextField(
                    value = formState.sellPrice,
                    onValueChange = { v -> viewModel.updateFormState { it.copy(sellPrice = v) } },
                    label = { Text("Harga Jual") },
                    keyboardOptions = KeyboardOptions(keyboardType = KeyboardType.Number),
                    modifier = Modifier.weight(1f)
                )
            }

            Row(
                modifier = Modifier.fillMaxWidth(),
                horizontalArrangement = Arrangement.spacedBy(8.dp)
            ) {
                Box(
                    modifier = Modifier.weight(1f)
                ) {
                    ExposedDropdownMenuBox(
                        expanded = categoryDropdownExpanded,
                        onExpandedChange = { categoryDropdownExpanded = !categoryDropdownExpanded },
                        modifier = Modifier.fillMaxWidth()
                    ) {
                        OutlinedTextField(
                            value = formState.category,
                            onValueChange = {},
                            label = { Text("Kategori Produk") },
                            readOnly = true,
                            trailingIcon = {
                                ExposedDropdownMenuDefaults.TrailingIcon(
                                    expanded = categoryDropdownExpanded
                                )
                            },
                            modifier = Modifier
                                .fillMaxWidth()
                                .menuAnchor()
                        )
                        ExposedDropdownMenu(
                            expanded = categoryDropdownExpanded,
                            onDismissRequest = { categoryDropdownExpanded = false },
                            modifier = Modifier.fillMaxWidth()
                        ) {
                            if (categories.isEmpty()) {
                                DropdownMenuItem(
                                    text = { Text("Belum ada kategori") },
                                    onClick = { },
                                    enabled = false
                                )
                            } else {
                                categories.forEach { cat ->
                                    DropdownMenuItem(
                                        text = { Text(cat) },
                                        onClick = {
                                            viewModel.updateFormState { it.copy(category = cat) }
                                            categoryDropdownExpanded = false
                                        }
                                    )
                                }
                            }
                            DropdownMenuItem(
                                text = { Text("+ Buat Kategori Baru...") },
                                onClick = {
                                    showNewCategoryDialog = true
                                    categoryDropdownExpanded = false
                                }
                            )
                        }
                    }
                }
            }

            // Stock Toggle Section
            Card(
                modifier = Modifier.fillMaxWidth(),
                colors = CardDefaults.cardColors(containerColor = MaterialTheme.colorScheme.surfaceVariant)
            ) {
                Column(modifier = Modifier.padding(16.dp)) {
                    Row(
                        modifier = Modifier.fillMaxWidth(),
                        horizontalArrangement = Arrangement.SpaceBetween,
                        verticalAlignment = Alignment.CenterVertically
                    ) {
                        Text(text = "Kelola Stok", style = MaterialTheme.typography.titleMedium)
                        Switch(
                            checked = formState.hasStock,
                            onCheckedChange = { v -> viewModel.updateFormState { it.copy(hasStock = v) } }
                        )
                    }

                    if (formState.hasStock) {
                        Spacer(modifier = Modifier.height(16.dp))
                        Row(
                            verticalAlignment = Alignment.CenterVertically,
                            horizontalArrangement = Arrangement.spacedBy(16.dp)
                        ) {
                            Button(onClick = { if (formState.stockCount > 0) viewModel.updateFormState { it.copy(stockCount = it.stockCount - 1) } }) {
                                Text("-")
                            }
                            OutlinedTextField(
                                value = formState.stockCount.toString(),
                                onValueChange = { v -> viewModel.updateFormState { it.copy(stockCount = v.toIntOrNull() ?: 0) } },
                                keyboardOptions = KeyboardOptions(keyboardType = KeyboardType.Number),
                                modifier = Modifier.weight(1f)
                            )
                            Button(onClick = { viewModel.updateFormState { it.copy(stockCount = it.stockCount + 1) } }) {
                                Text("+")
                            }
                        }

                        Spacer(modifier = Modifier.height(16.dp))
                        Row(
                            modifier = Modifier.fillMaxWidth(),
                            horizontalArrangement = Arrangement.SpaceBetween,
                            verticalAlignment = Alignment.CenterVertically
                        ) {
                            Row(
                                verticalAlignment = Alignment.CenterVertically,
                                horizontalArrangement = Arrangement.spacedBy(8.dp)
                            ) {
                                Text(text = "Aktifkan notifikasi stok", style = MaterialTheme.typography.titleMedium)
                                if (!isPro) {
                                    Surface(
                                        color = Color(0xFFFFE082),
                                        shape = RoundedCornerShape(4.dp)
                                    ) {
                                        Text(
                                            "PRO",
                                            fontSize = 10.sp,
                                            fontWeight = FontWeight.Bold,
                                            color = Color(0xFFE65100),
                                            modifier = Modifier.padding(horizontal = 4.dp, vertical = 2.dp)
                                        )
                                    }
                                }
                            }
                            Switch(
                                checked = if (isPro) formState.isStockNotificationEnabled else false,
                                onCheckedChange = { v ->
                                    if (!isPro) {
                                        onProRequired()
                                    } else {
                                        viewModel.updateFormState { it.copy(isStockNotificationEnabled = v) }
                                    }
                                }
                            )
                        }

                        if (formState.isStockNotificationEnabled) {
                            Spacer(modifier = Modifier.height(16.dp))
                            OutlinedTextField(
                                value = formState.minStockCount.toString(),
                                onValueChange = { v -> viewModel.updateFormState { it.copy(minStockCount = v.toIntOrNull() ?: 0) } },
                                label = { Text("Stok Minimal") },
                                keyboardOptions = KeyboardOptions(keyboardType = KeyboardType.Number),
                                modifier = Modifier.fillMaxWidth()
                            )
                        }
                    } else {
                        Spacer(modifier = Modifier.height(8.dp))
                        Text(
                            text = "Stok tidak terbatas (unlimited)",
                            style = MaterialTheme.typography.bodySmall,
                            color = MaterialTheme.colorScheme.onSurfaceVariant
                        )
                    }
                }
            }

            OutlinedTextField(
                value = formState.description,
                onValueChange = { v -> viewModel.updateFormState { it.copy(description = v) } },
                label = { Text("Deskripsi (Opsional)") },
                modifier = Modifier.fillMaxWidth(),
                minLines = 3
            )

            // Barcode Section
            Card(
                modifier = Modifier.fillMaxWidth(),
                colors = CardDefaults.cardColors(containerColor = MaterialTheme.colorScheme.surfaceVariant)
            ) {
                Column(modifier = Modifier.padding(16.dp)) {
                    Text(text = "Barcode Produk", style = MaterialTheme.typography.titleMedium)
                    Spacer(modifier = Modifier.height(8.dp))

                    Row(
                        modifier = Modifier.fillMaxWidth(),
                        horizontalArrangement = Arrangement.spacedBy(8.dp),
                        verticalAlignment = Alignment.CenterVertically
                    ) {
                        OutlinedTextField(
                            value = formState.barcode,
                            onValueChange = { v -> viewModel.updateFormState { it.copy(barcode = v) } },
                            label = { Text("Kode Barcode") },
                            modifier = Modifier.weight(1f)
                        )
                        IconButton(onClick = onNavigateToScanner) {
                            Icon(Icons.Filled.QrCodeScanner, contentDescription = "Scan Barcode")
                        }
                        Button(
                            onClick = {
                                val generatedCode = UUID.randomUUID().toString().substring(0, 12).uppercase()
                                viewModel.updateFormState { it.copy(barcode = generatedCode) }
                                generatedBarcodeBitmap = BarcodeUtils.generateBarcode(generatedCode, 400, 150)
                            }
                        ) {
                            Text("Generate")
                        }
                    }

                    if (generatedBarcodeBitmap != null) {
                        Spacer(modifier = Modifier.height(16.dp))
                        Image(
                            bitmap = generatedBarcodeBitmap!!.asImageBitmap(),
                            contentDescription = "Generated Barcode",
                            modifier = Modifier
                                .fillMaxWidth()
                                .height(100.dp),
                            contentScale = ContentScale.Fit
                        )
                        TextButton(
                            onClick = {
                                val uri = BarcodeUtils.saveBarcodeToGallery(context, generatedBarcodeBitmap!!, formState.barcode)
                                if (uri != null) {
                                    Toast.makeText(context, "Barcode disimpan di galeri", Toast.LENGTH_SHORT).show()
                                } else {
                                    Toast.makeText(context, "Gagal menyimpan barcode", Toast.LENGTH_SHORT).show()
                                }
                            },
                            modifier = Modifier.align(Alignment.CenterHorizontally)
                        ) {
                            Text("Simpan Barcode ke HP")
                        }
                    }
                }
            }

            Button(
                onClick = {
                    viewModel.submitProduct(productId = productId)
                },
                modifier = Modifier
                    .fillMaxWidth()
                    .padding(vertical = 16.dp),
                enabled = !isLoading
            ) {
                if (isLoading) {
                    CircularProgressIndicator(modifier = Modifier.size(24.dp), color = MaterialTheme.colorScheme.onPrimary)
                } else {
                    Text("Simpan Produk")
                }
            }
        }
    }

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
                TextButton(
                    onClick = {
                        if (newCategoryInput.isNotBlank()) {
                            viewModel.createNewCategory(newCategoryInput.trim())
                            newCategoryInput = ""
                            showNewCategoryDialog = false
                        }
                    }
                ) {
                    Text("Simpan")
                }
            },
            dismissButton = {
                TextButton(onClick = {
                    newCategoryInput = ""
                    showNewCategoryDialog = false
                }) {
                    Text("Batal")
                }
            }
        )
    }
}
