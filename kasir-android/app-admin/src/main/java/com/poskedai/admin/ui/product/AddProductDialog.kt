package com.poskedai.admin.ui.product

import androidx.compose.foundation.layout.*
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.filled.QrCodeScanner
import androidx.compose.material3.*
import androidx.compose.runtime.*
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.unit.dp
import androidx.compose.ui.window.Dialog
import androidx.compose.ui.window.DialogProperties
import com.google.gson.JsonObject
import com.poskedai.admin.ui.scanner.BarcodeScannerScreen
import android.graphics.Bitmap
import android.widget.Toast
import androidx.compose.ui.graphics.asImageBitmap
import androidx.compose.ui.platform.LocalContext
import com.poskedai.admin.utils.BarcodeUtils
import androidx.compose.foundation.Image
import java.util.UUID
import androidx.activity.compose.rememberLauncherForActivityResult
import androidx.activity.result.contract.ActivityResultContracts
import coil.compose.AsyncImage

@Composable
fun AddProductDialog(
    availableCategories: List<String>,
    onCreateCategory: (String, (String) -> Unit) -> Unit,
    onDismiss: () -> Unit,
    onSave: (String, String, String, String, Boolean) -> Unit
) {
    val context = LocalContext.current
    var name by remember { mutableStateOf("") }
    var category by remember { mutableStateOf("") }
    var barcode by remember { mutableStateOf("") }
    var photoUrl by remember { mutableStateOf("") }
    var showScanner by remember { mutableStateOf(false) }
    var generatedBarcodeBitmap by remember { mutableStateOf<Bitmap?>(null) }
    var isGeneratedBarcode by remember { mutableStateOf(false) }
    var categoryExpanded by remember { mutableStateOf(false) }
    var showNewCategoryDialog by remember { mutableStateOf(false) }
    var newCategoryName by remember { mutableStateOf("") }

    val imagePickerLauncher = rememberLauncherForActivityResult(
        contract = ActivityResultContracts.GetContent()
    ) { uri ->
        if (uri != null) {
            photoUrl = uri.toString()
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
                        barcode = it
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

    // Dialog untuk membuat kategori baru
    if (showNewCategoryDialog) {
        AlertDialog(
            onDismissRequest = { showNewCategoryDialog = false },
            title = { Text("Buat Kategori Baru") },
            text = {
                OutlinedTextField(
                    value = newCategoryName,
                    onValueChange = { newCategoryName = it },
                    label = { Text("Nama Kategori") },
                    modifier = Modifier.fillMaxWidth()
                )
            },
            confirmButton = {
                Button(
                    onClick = {
                        if (newCategoryName.isNotBlank()) {
                            onCreateCategory(newCategoryName) { createdName ->
                                category = createdName
                                newCategoryName = ""
                                showNewCategoryDialog = false
                            }
                        }
                    }
                ) {
                    Text("Buat")
                }
            },
            dismissButton = {
                OutlinedButton(onClick = { showNewCategoryDialog = false }) {
                    Text("Batal")
                }
            }
        )
        return
    }

    AlertDialog(
        onDismissRequest = onDismiss,
        title = { Text("Tambah Produk") },
        text = {
            Column(verticalArrangement = Arrangement.spacedBy(8.dp)) {
                if (photoUrl.isNotEmpty()) {
                    AsyncImage(
                        model = photoUrl,
                        contentDescription = "Selected Image",
                        modifier = Modifier.fillMaxWidth().height(150.dp)
                    )
                }
                Button(onClick = { imagePickerLauncher.launch("image/*") }) {
                    Text(if (photoUrl.isEmpty()) "Pilih Foto Produk" else "Ganti Foto Produk")
                }
                OutlinedTextField(
                    value = name,
                    onValueChange = { name = it },
                    label = { Text("Nama Produk") },
                    modifier = Modifier.fillMaxWidth()
                )
                @OptIn(ExperimentalMaterial3Api::class)
                ExposedDropdownMenuBox(
                    expanded = categoryExpanded,
                    onExpandedChange = { categoryExpanded = !categoryExpanded },
                    modifier = Modifier.fillMaxWidth()
                ) {
                    OutlinedTextField(
                        value = category,
                        onValueChange = {},
                        readOnly = true,
                        label = { Text("Kategori") },
                        trailingIcon = { ExposedDropdownMenuDefaults.TrailingIcon(expanded = categoryExpanded) },
                        modifier = Modifier.menuAnchor().fillMaxWidth()
                    )
                    ExposedDropdownMenu(
                        expanded = categoryExpanded,
                        onDismissRequest = { categoryExpanded = false }
                    ) {
                        DropdownMenuItem(
                            text = { Text("+ Buat Kategori Baru...", color = MaterialTheme.colorScheme.primary) },
                            onClick = {
                                categoryExpanded = false
                                showNewCategoryDialog = true
                            }
                        )
                        availableCategories.forEach { catName ->
                            DropdownMenuItem(
                                text = { Text(catName) },
                                onClick = {
                                    category = catName
                                    categoryExpanded = false
                                }
                            )
                        }
                    }
                }
                Row(verticalAlignment = Alignment.CenterVertically) {
                    OutlinedTextField(
                        value = barcode,
                        onValueChange = { barcode = it },
                        label = { Text("Barcode") },
                        modifier = Modifier.weight(1f)
                    )
                    IconButton(onClick = { showScanner = true }) {
                        Icon(Icons.Filled.QrCodeScanner, contentDescription = "Scan")
                    }
                }
                Row(
                    modifier = Modifier.fillMaxWidth(),
                    horizontalArrangement = Arrangement.SpaceBetween,
                    verticalAlignment = Alignment.CenterVertically
                ) {
                    Button(onClick = {
                        val generatedCode = UUID.randomUUID().toString().replace("-", "").take(12).uppercase()
                        barcode = generatedCode
                        isGeneratedBarcode = true
                        generatedBarcodeBitmap = BarcodeUtils.generateBarcode(generatedCode, 400, 150)
                    }) {
                        Text("Generate Barcode")
                    }
                }

                generatedBarcodeBitmap?.let { bitmap ->
                    Spacer(modifier = Modifier.height(8.dp))
                    Image(
                        bitmap = bitmap.asImageBitmap(),
                        contentDescription = "Generated Barcode",
                        modifier = Modifier.fillMaxWidth().height(100.dp)
                    )
                    Spacer(modifier = Modifier.height(8.dp))
                    Button(
                        onClick = {
                            val uri = BarcodeUtils.saveBarcodeToGallery(context, bitmap, barcode)
                            if (uri != null) {
                                Toast.makeText(context, "Barcode tersimpan di Galeri", Toast.LENGTH_SHORT).show()
                            } else {
                                Toast.makeText(context, "Gagal menyimpan barcode", Toast.LENGTH_SHORT).show()
                            }
                        },
                        modifier = Modifier.fillMaxWidth()
                    ) {
                        Text("Download Barcode")
                    }
                }
            }
        },
        confirmButton = {
            Button(onClick = { onSave(name, category, barcode, photoUrl, isGeneratedBarcode) }) {
                Text("Simpan")
            }
        },
        dismissButton = {
            OutlinedButton(onClick = onDismiss) {
                Text("Batal")
            }
        }
    )
}
